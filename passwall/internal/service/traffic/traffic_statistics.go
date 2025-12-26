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
	"runtime/debug"
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
type StatisticsService struct {
	clients           []config.ClashAPIClient
	connections       sync.Map // {clientIndex: *websocket.Conn}
	historyConnsData  []map[string]Connection
	closeConnsData    []map[string]Connection
	mu                sync.Mutex
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

func NewTrafficStatisticsService(clients []config.ClashAPIClient, proxyService proxy.ProxyService, trafficRepo repository.TrafficRepository) StatisticsService {
	historyConns := make([]map[string]Connection, len(clients))
	closeConns := make([]map[string]Connection, len(clients))
	for i := range clients {
		historyConns[i] = make(map[string]Connection)
		closeConns[i] = make(map[string]Connection)
	}
	return StatisticsService{
		clients:           clients,
		historyConnsData:  historyConns,
		closeConnsData:    closeConns,
		proxyService:      proxyService,
		trafficRepo:       trafficRepo,
		baseRetryInterval: 5 * time.Second,
		maxRetryInterval:  10 * 60 * time.Second,
		maxRetries:        10,
		cleanNameRegex:    regexp.MustCompile(`^\[\d+]-(.+)$`),
	}
}

func (s *StatisticsService) Start() error {
	s.done = make(chan struct{})
	s.stopChan = make(chan struct{})
	s.ticker = time.NewTicker(1 * time.Minute)
	go s.startPeriodicProcessing()
	return s.connectAllClients()
}

func (s *StatisticsService) connectAllClients() error {
	for i, client := range s.clients {
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
	if s.done != nil {
		select {
		case <-s.done:
		case <-time.After(5 * time.Second):
			log.Errorln("Stop timeout: periodic processing did not finish in time")
		}
		s.done = nil
	}
}

func (s *StatisticsService) GetTrafficStatistics(proxyId uint) (*model.TrafficStatistics, error) {
	traffic, err := s.trafficRepo.FindByProxyID(proxyId)
	if err != nil {
		return nil, err
	}
	return traffic, nil
}

func (s *StatisticsService) BatchGetTrafficStatistics(proxyIdList []uint) (map[uint]*model.TrafficStatistics, error) {
	if proxyIdList == nil || len(proxyIdList) == 0 {
		return nil, fmt.Errorf("proxyIdList is empty")
	}
	trafficMap, err := s.trafficRepo.FindByProxyIDList(proxyIdList)
	if err != nil {
		return nil, err
	}
	return trafficMap, nil
}

func (s *StatisticsService) readMessages(clientIndex int, conn *websocket.Conn) {
	defer func() {
		s.connections.Delete(clientIndex)
		conn.Close()
		log.Infoln("Traffic statistics service stopped for client %d", clientIndex)
	}()

	log.Infoln("Traffic statistics service started for client %d", clientIndex)
	for {
		select {
		case <-s.stopChan:
			log.Infoln("Stop signal received, exiting readMessages for client %d", clientIndex)
			return
		default:
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			log.Errorln("WebSocket read error for client %d: %v", clientIndex, err)
			select {
			case <-s.stopChan:
				log.Infoln("Stop signal received, skipping reconnect for client %d", clientIndex)
				return
			default:
				go func() {
					time.Sleep(s.baseRetryInterval)
					select {
					case <-s.stopChan:
						log.Infoln("Stop signal received during reconnect delay for client %d", clientIndex)
						return
					default:
						client := s.clients[clientIndex]
						_ = s.connectClient(clientIndex, client)
					}
				}()
			}
			return
		}

		var traffic Connections
		if err := json.Unmarshal(message, &traffic); err != nil {
			log.Errorln("Failed to unmarshal traffic data for client %d: %v", clientIndex, err)
			continue
		}
		log.Debugln("Client %d traffic: %v", clientIndex, traffic)

		s.processTrafficData(clientIndex, traffic)
	}
}

func (s *StatisticsService) processTrafficData(clientIndex int, traffic Connections) {
	s.mu.Lock()
	defer s.mu.Unlock()

	historyConns := s.historyConnsData[clientIndex]
	closeConns := s.closeConnsData[clientIndex]

	if traffic.Connections == nil || len(traffic.Connections) == 0 {
		for id, conn := range historyConns {
			closeConns[id] = conn
			delete(historyConns, id)
		}
	} else {
		currentConnections := make(map[string]Connection, len(traffic.Connections))
		for _, conn := range traffic.Connections {
			currentConnections[conn.ID] = conn
		}

		for id, conn := range historyConns {
			if _, exists := currentConnections[id]; !exists {
				closeConns[id] = conn
			}
			delete(historyConns, id)
		}

		for _, conn := range traffic.Connections {
			historyConns[conn.ID] = conn
		}
	}

	s.historyConnsData[clientIndex] = historyConns
	s.closeConnsData[clientIndex] = closeConns
}

func (s *StatisticsService) processCloseConns() {
	s.mu.Lock()
	defer func() {
		if r := recover(); r != nil {
			log.Errorln("处理关闭连接时发生panic: %v", r)
			log.Errorln("堆栈信息: %s", debug.Stack())
		}
		s.mu.Unlock()
	}()

	var nodeTrafficMap = make(map[string]*NodeTraffic)
	for clientIndex := range s.closeConnsData {
		closeConns := s.closeConnsData[clientIndex]
		for id, connection := range closeConns {
			for _, chain := range connection.Chains {
				nodeTraffic, exists := nodeTrafficMap[chain]
				if !exists {
					nodeTraffic = &NodeTraffic{
						NodeName: chain,
						Upload:   connection.Upload,
						Download: connection.Download,
					}
					nodeTrafficMap[chain] = nodeTraffic
				} else {
					nodeTraffic.Upload += connection.Upload
					nodeTraffic.Download += connection.Download
				}
			}
			delete(closeConns, id)
		}
	}

	if len(nodeTrafficMap) == 0 {
		log.Infoln("当前无节点流量数据")
		return
	}
	// 处理节点名称并查找对应的代理
	for _, node := range nodeTrafficMap {
		nodeProxy, err := s.proxyService.GetProxyByName(node.NodeName)
		if err != nil {
			log.Errorln("获取节点信息失败，节点名称: %s, 错误: %v", node.NodeName, err)
			continue
		}
		if nodeProxy == nil && strings.HasPrefix(node.NodeName, "[") {
			cleanName := s.cleanNodeName(node.NodeName)
			nodeProxy, err = s.proxyService.GetProxyByName(cleanName)
			if err != nil {
				log.Errorln("获取节点信息失败，节点名称: %s, 错误: %v", cleanName, err)
				continue
			}
		}
		if nodeProxy != nil {
			// 判断traffic里是否有这个节点数据，如果有，则更新，否则新增
			log.Infoln("找到节点: %s, 本次上传流量: %d, 下载流量: %d", node.NodeName, node.Upload, node.Download)
			trafficStatistics, err := s.trafficRepo.FindByProxyID(nodeProxy.ID)
			if err != nil {
				log.Errorln("获取节点流量失败，节点名称: %s, 错误: %v", node.NodeName, err)
				continue
			}
			if trafficStatistics == nil {
				trafficStatistics = &model.TrafficStatistics{
					ProxyID:       nodeProxy.ID,
					UploadTotal:   node.Upload,
					DownloadTotal: node.Download,
				}
				err := s.trafficRepo.Create(trafficStatistics)
				if err != nil {
					log.Errorln("创建节点流量失败，节点信息: %v, 错误: %v", node, err)
					continue
				}
				log.Infoln("创建节点流量成功，节点信息: %v", trafficStatistics)
			} else {
				trafficStatistics.UploadTotal += node.Upload
				trafficStatistics.DownloadTotal += node.Download
				err := s.trafficRepo.UpdateTrafficByProxyID(trafficStatistics)
				if err != nil {
					log.Errorln("更新节点流量失败，节点信息: %v, 错误: %v", trafficStatistics, err)
					continue
				}
				log.Infoln("更新节点流量成功，节点信息: %v", trafficStatistics)
			}
		} else {
			log.Infoln("未找到节点: %s", node.NodeName)
		}
	}
	log.Infoln("处理完成，处理了 %v 个节点流量数据", len(nodeTrafficMap))
}

// cleanNodeName 清理节点名称，移除前面的序号前缀如"[1]-"
func (s *StatisticsService) cleanNodeName(nodeName string) string {
	// 匹配模式: [数字]- (严格匹配，不考虑空格)
	// 例如: "[1]-节点名" -> "节点名"
	// "[1] - 节点名" -> "[1] - 节点名" (不匹配)
	// "[节点]自带[]" -> "[节点]自带[]" (保持不变)

	matches := s.cleanNameRegex.FindStringSubmatch(nodeName)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}

	// 不匹配则返回原名称
	return nodeName
}

// startPeriodicProcessing 启动定时处理任务
func (s *StatisticsService) startPeriodicProcessing() {
	defer close(s.done) // 添加关闭通知

	for {
		select {
		case <-s.ticker.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						log.Errorln("处理流量统计时发生panic: %v", r)
						log.Errorln("堆栈信息: %s", debug.Stack())
					}
				}()

				log.Debugln("执行定时流量统计处理...")
				s.processCloseConns()
			}()
		case <-s.done:
			s.ticker.Stop()
			return
		}
	}
}
