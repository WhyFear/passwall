package traffic

import (
	"encoding/json"
	"fmt"
	"net/url"
	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"
	"passwall/internal/service/proxy"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/metacubex/mihomo/log"

	"github.com/gorilla/websocket"
)

type Connections struct {
	Connections   []Connection `json:"connections"`
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
}

type Connection struct {
	ID          string    `json:"id"`
	Upload      int64     `json:"upload"`
	Download    int64     `json:"download"`
	Start       time.Time `json:"start"`
	Chains      []string  `json:"chains"`
	Rule        string    `json:"rule"`
	RulePayload string    `json:"rulePayload"`
}

type NodeTraffic struct {
	NodeName string `json:"nodeName"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

type ClashConfigProvider interface {
	GetClashClients() ([]config.ClashAPIClient, bool)
}

// trackedValue 用于记录连接上次的累计值，以便计算差值
type trackedValue struct {
	LastUpload   int64
	LastDownload int64
}

type StatisticsService struct {
	configProvider ClashConfigProvider
	connections    sync.Map // {clientIndex: *websocket.Conn}

	// key: clientIndex, value: map[connID]trackedValue
	lastValues []map[string]trackedValue

	// 暂存尚未写入数据库的增量流量数据 key: nodeName
	pendingTraffic map[string]*NodeTraffic

	mu                sync.Mutex
	startTime         time.Time
	baseRetryInterval time.Duration
	maxRetryInterval  time.Duration
	maxRetries        int
	ticker            *time.Ticker
	done              chan struct{}
	stopChan          chan struct{}
	stopOnce          sync.Once
	proxyService      proxy.ProxyService
	trafficRepo       repository.TrafficRepository
	cleanNameRegex    *regexp.Regexp
}

func NewTrafficStatisticsService(configProvider ClashConfigProvider, proxyService proxy.ProxyService, trafficRepo repository.TrafficRepository) StatisticsService {
	return StatisticsService{
		configProvider:    configProvider,
		pendingTraffic:    make(map[string]*NodeTraffic),
		proxyService:      proxyService,
		trafficRepo:       trafficRepo,
		baseRetryInterval: 5 * time.Second,
		maxRetryInterval:  10 * 60 * time.Second,
		maxRetries:        10,
		cleanNameRegex:    regexp.MustCompile(`^\[\d+]-(.+)$`),
	}
}

func (s *StatisticsService) Start() error {
	clients, enabled := s.configProvider.GetClashClients()
	if !enabled {
		return nil
	}

	s.mu.Lock()
	s.startTime = time.Now()
	s.lastValues = make([]map[string]trackedValue, len(clients))
	for i := range clients {
		s.lastValues[i] = make(map[string]trackedValue)
	}
	s.pendingTraffic = make(map[string]*NodeTraffic)
	s.mu.Unlock()

	s.done = make(chan struct{})
	s.stopChan = make(chan struct{})
	s.ticker = time.NewTicker(1 * time.Minute)
	go s.startPeriodicProcessing()
	return s.connectAllClients(clients)
}

func (s *StatisticsService) connectAllClients(clients []config.ClashAPIClient) error {
	for i, client := range clients {
		if err := s.connectClient(i, client); err != nil {
			log.Errorln("Failed to connect to client %d: %v", i, err)
		}
	}
	return nil
}

func (s *StatisticsService) connectClient(clientIndex int, client config.ClashAPIClient) error {
	wsURL := client.URL + "/connections"
	if client.Secret != "" {
		if u, err := url.Parse(wsURL); err == nil {
			query := u.Query()
			query.Set("token", client.Secret)
			u.RawQuery = query.Encode()
			wsURL = u.String()
		} else {
			wsURL += "?token=" + client.Secret
		}
	}

	for attempt := 1; attempt <= s.maxRetries; attempt++ {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			log.Infoln("WebSocket connection established for client %d", clientIndex)
			s.connections.Store(clientIndex, conn)
			go s.readMessages(clientIndex, conn)
			return nil
		}

		if attempt >= s.maxRetries {
			log.Errorln("Failed to connect to client %d after %d attempts: %v", clientIndex, s.maxRetries, err)
			return err
		}

		interval := s.baseRetryInterval * time.Duration(1<<uint(attempt-1))
		if interval > s.maxRetryInterval {
			interval = s.maxRetryInterval
		}

		log.Errorln("WebSocket connection failed for client %d, retrying in %v (attempt %d/%d)",
			clientIndex, interval, attempt, s.maxRetries)
		time.Sleep(interval)
	}
	return nil
}

func (s *StatisticsService) Stop() {
	s.stopOnce.Do(func() {
		if s.stopChan != nil {
			close(s.stopChan)
		}
	})

	s.connections.Range(func(key, value interface{}) bool {
		if conn, ok := value.(*websocket.Conn); ok {
			conn.Close()
		}
		return true
	})

	s.connections = sync.Map{}

	if s.ticker != nil {
		s.ticker.Stop()
		s.ticker = nil
	}

	// 在停止前最后结算一次流量
	log.Infoln("Statistics service stopping, performing final traffic flush...")
	s.flushTrafficToDB()

	if s.done != nil {
		select {
		case <-s.done:
		case <-time.After(5 * time.Second):
			log.Errorln("Stop timeout: periodic processing did not finish in time")
		}
		s.done = nil
	}
	s.stopOnce = sync.Once{}
}

func (s *StatisticsService) readMessages(clientIndex int, conn *websocket.Conn) {
	defer func() {
		s.connections.Delete(clientIndex)
		conn.Close()
		log.Infoln("Traffic statistics service stopped for client %d", clientIndex)
	}()

	for {
		select {
		case <-s.stopChan:
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorln("WebSocket read error for client %d: %v", clientIndex, err)
			select {
			case <-s.stopChan:
				return
			default:
				go func() {
					time.Sleep(s.baseRetryInterval)
					select {
					case <-s.stopChan:
						return
					default:
						clients, _ := s.configProvider.GetClashClients()
						if clientIndex < len(clients) {
							_ = s.connectClient(clientIndex, clients[clientIndex])
						}
					}
				}()
			}
			return
		}

		var data Connections
		if err := json.Unmarshal(message, &data); err != nil {
			continue
		}

		s.processTrafficDelta(clientIndex, data)
	}
}

// processTrafficDelta 计算本次接收到的数据与上次的增量
func (s *StatisticsService) processTrafficDelta(clientIndex int, data Connections) {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientLastValues := s.lastValues[clientIndex]
	currentConnIDs := make(map[string]bool)

	for _, conn := range data.Connections {
		currentConnIDs[conn.ID] = true

		last, exists := clientLastValues[conn.ID]
		if !exists {
			// 重启保护：如果连接是在服务启动前建立的，初始值设为当前值，增量设为0
			if conn.Start.Before(s.startTime) {
				last = trackedValue{
					LastUpload:   conn.Upload,
					LastDownload: conn.Download,
				}
			} else {
				last = trackedValue{0, 0}
			}
		}

		deltaUp := conn.Upload - last.LastUpload
		deltaDown := conn.Download - last.LastDownload

		// 如果增量为正，记录到暂存区
		if deltaUp > 0 || deltaDown > 0 {
			for _, nodeName := range conn.Chains {
				node, ok := s.pendingTraffic[nodeName]
				if !ok {
					node = &NodeTraffic{NodeName: nodeName}
					s.pendingTraffic[nodeName] = node
				}
				node.Upload += deltaUp
				node.Download += deltaDown
			}
		}

		// 更新快照
		clientLastValues[conn.ID] = trackedValue{
			LastUpload:   conn.Upload,
			LastDownload: conn.Download,
		}
	}

	// 清理已经关闭的连接 ID
	for id := range clientLastValues {
		if !currentConnIDs[id] {
			delete(clientLastValues, id)
		}
	}
}

func (s *StatisticsService) flushTrafficToDB() {
	s.mu.Lock()
	if len(s.pendingTraffic) == 0 {
		s.mu.Unlock()
		return
	}
	// 拷贝一份数据并清空暂存区，然后解锁执行慢速的 DB 操作
	workData := s.pendingTraffic
	s.pendingTraffic = make(map[string]*NodeTraffic)
	s.mu.Unlock()

	defer func() {
		if r := recover(); r != nil {
			log.Errorln("Flush traffic panic: %v", r)
		}
	}()

	for _, node := range workData {
		nodeProxy, err := s.proxyService.GetProxyByName(node.NodeName)
		if err != nil {
			continue
		}
		if nodeProxy == nil && strings.HasPrefix(node.NodeName, "[") {
			cleanName := s.cleanNodeName(node.NodeName)
			nodeProxy, _ = s.proxyService.GetProxyByName(cleanName)
		}

		if nodeProxy != nil {
			traffic, err := s.trafficRepo.FindByProxyID(nodeProxy.ID)
			if err != nil {
				continue
			}
			if traffic == nil {
				traffic = &model.TrafficStatistics{
					ProxyID:       nodeProxy.ID,
					UploadTotal:   node.Upload,
					DownloadTotal: node.Download,
				}
				_ = s.trafficRepo.Create(traffic)
			} else {
				traffic.UploadTotal += node.Upload
				traffic.DownloadTotal += node.Download
				_ = s.trafficRepo.UpdateTrafficByProxyID(traffic)
			}
		}
	}
}

// cleanNodeName 清理节点名称，移除前面的序号前缀如"[1]-"
func (s *StatisticsService) cleanNodeName(nodeName string) string {
	matches := s.cleanNameRegex.FindStringSubmatch(nodeName)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return nodeName
}

func (s *StatisticsService) startPeriodicProcessing() {
	defer close(s.done)
	for {
		select {
		case <-s.ticker.C:
			s.flushTrafficToDB()
		case <-s.done:
			return
		}
	}
}

func (s *StatisticsService) GetTrafficStatistics(proxyId uint) (*model.TrafficStatistics, error) {
	return s.trafficRepo.FindByProxyID(proxyId)
}

func (s *StatisticsService) BatchGetTrafficStatistics(proxyIdList []uint) (map[uint]*model.TrafficStatistics, error) {
	if len(proxyIdList) == 0 {
		return nil, fmt.Errorf("proxyIdList is empty")
	}
	return s.trafficRepo.FindByProxyIDList(proxyIdList)
}
