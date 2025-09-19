package traffic

import (
	"encoding/json"
	"fmt"
	"net/url"
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
	//Memory        int64        `json:"memory"`
}

type Connection struct {
	ID          string    `json:"id"`
	Upload      int64     `json:"upload"`
	Download    int64     `json:"download"`
	Start       time.Time `json:"start"`
	Chains      []string  `json:"chains"`
	Rule        string    `json:"rule"`
	RulePayload string    `json:"rulePayload"`
	//MetaData    map[string]interface{} `json:"metadata"`
}

type NodeTraffic struct {
	NodeName string `json:"nodeName"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

// StatisticsService handles WebSocket connection and traffic data processing
type StatisticsService struct {
	wsURL        string
	secret       string
	historyConns sync.Map // {id: Connection}
	closeConns   sync.Map // {id: Connection}

	conn              *websocket.Conn
	baseRetryInterval time.Duration
	maxRetryInterval  time.Duration
	maxRetries        int

	ticker *time.Ticker
	done   chan struct{}

	proxyService proxy.ProxyService
	trafficRepo  repository.TrafficRepository
}

// NewTrafficStatisticsService creates a new traffic statistics service
func NewTrafficStatisticsService(wsURL string, secret string, proxyService proxy.ProxyService, trafficRepo repository.TrafficRepository) StatisticsService {
	return StatisticsService{
		wsURL:             wsURL,
		secret:            secret,
		proxyService:      proxyService,
		trafficRepo:       trafficRepo,
		historyConns:      sync.Map{},
		closeConns:        sync.Map{},
		baseRetryInterval: 5 * time.Second,
		maxRetryInterval:  10 * 60 * time.Second,
		maxRetries:        10,
	}
}

// Start starts the WebSocket connection and data processing
func (s *StatisticsService) Start() error {
	s.done = make(chan struct{})
	s.ticker = time.NewTicker(1 * time.Minute)

	// 启动定时处理任务
	go s.startPeriodicProcessing()

	return s.connectWithRetry()
}

// connectWithRetry establishes WebSocket connection with exponential backoff
func (s *StatisticsService) connectWithRetry() error {
	retryCount := 0

	wsURL := s.wsURL + "/connections"
	if s.secret != "" {
		if u, err := url.Parse(wsURL); err == nil {
			query := u.Query()
			query.Set("secret", s.secret)
			u.RawQuery = query.Encode()
			wsURL = u.String()
		} else {
			// 如果解析失败，回退到简单拼接
			wsURL += "?secret=" + s.secret
		}
	}

	for {
		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err == nil {
			s.conn = conn
			go s.readMessages()
			return nil
		}

		retryCount++
		if retryCount >= s.maxRetries {
			return err
		}

		// 阶梯式重试间隔
		interval := s.baseRetryInterval * time.Duration(1<<uint(retryCount-1))
		if interval > s.maxRetryInterval {
			interval = s.maxRetryInterval
		}

		log.Errorln("WebSocket connection failed, retrying in %v (attempt %d/%d)",
			interval, retryCount, s.maxRetries)
		time.Sleep(interval)
	}
}

// Stop stops the service
func (s *StatisticsService) Stop() {
	if s.conn != nil {
		_ = s.conn.Close()
	}

	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.done != nil {
		<-s.done
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

// readMessages reads and processes WebSocket messages
func (s *StatisticsService) readMessages() {
	defer func(conn *websocket.Conn) {
		err := conn.Close()
		if err != nil {

		}
	}(s.conn)

	log.Infoln("Traffic statistics service started")
	for {
		_, message, err := s.conn.ReadMessage()
		if err != nil {
			log.Errorln("WebSocket read error: %v", err)

			// 连接断开时自动重连
			go func() {
				time.Sleep(s.baseRetryInterval)
				_ = s.connectWithRetry()
			}()
			return
		}

		var traffic Connections
		if err := json.Unmarshal(message, &traffic); err != nil {
			log.Errorln("Failed to unmarshal traffic data: %v", err)
			continue
		}
		log.Debugln("traffic: %v", traffic)

		s.processTrafficData(traffic)
	}
}

// processTrafficData 与历史数据进行对比，如果有id本次未返回，则认为链接已断开，可以统计到总计流量中去
func (s *StatisticsService) processTrafficData(traffic Connections) {
	if traffic.Connections == nil || len(traffic.Connections) == 0 {
		// 无数据返回，历史数据全部统计到closeConns中
		s.historyConns.Range(func(id, conn interface{}) bool {
			s.closeConns.Store(id, conn)
			s.historyConns.Delete(id)
			return true
		})
	} else {
		// 首先计算historyConns和本次的差异，然后将historyConns中多出的数据统计到closeConns中
		currentConnections := make(map[string]Connection, len(traffic.Connections))
		for _, conn := range traffic.Connections {
			currentConnections[conn.ID] = conn
		}

		s.historyConns.Range(func(id, conn interface{}) bool {
			idStr := id.(string)
			_, exists := currentConnections[idStr]
			if !exists {
				s.closeConns.Store(idStr, conn)
			}
			s.historyConns.Delete(idStr)
			return true
		})
		// 再将本次的conn更新或添加到historyConns中
		for _, conn := range traffic.Connections {
			s.historyConns.Store(conn.ID, conn)
		}
	}
}

// 处理closeConns中的数据，id维度变成节点维度，需要统计每个节点的流量
func (s *StatisticsService) processCloseConns() {
	var nodeTrafficMap = make(map[string]*NodeTraffic)
	s.closeConns.Range(func(id, conn any) bool {
		connection, ok := conn.(Connection) // 注意去掉指针类型
		if !ok {
			log.Errorln("类型断言失败，预期类型 Connection，实际值: %v", conn)
			return true
		}
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
		return true
	})
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

	// 处理完成后清空closeConns，避免重复统计
	s.closeConns = sync.Map{}
}

// cleanNodeName 清理节点名称，移除前面的序号前缀如"[1]-"
func (s *StatisticsService) cleanNodeName(nodeName string) string {
	// 匹配模式: [数字]- (严格匹配，不考虑空格)
	// 例如: "[1]-节点名" -> "节点名"
	// "[1] - 节点名" -> "[1] - 节点名" (不匹配)
	// "[节点]自带[]" -> "[节点]自带[]" (保持不变)

	re := regexp.MustCompile(`^\[\d+\]-(.+)$`)
	matches := re.FindStringSubmatch(nodeName)
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
