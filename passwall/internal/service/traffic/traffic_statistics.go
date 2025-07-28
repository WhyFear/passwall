package traffic

import (
	"encoding/json"
	"net/url"
	"sync"
	"time"

	"github.com/metacubex/mihomo/log"

	"github.com/gorilla/websocket"
)

type Connections struct {
	Connections   []Connection `json:"connections"`
	DownloadTotal int64        `json:"downloadTotal"`
	UploadTotal   int64        `json:"uploadTotal"`
	Memory        int64        `json:"memory"`
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
}

// NewTrafficStatisticsService creates a new traffic statistics service
func NewTrafficStatisticsService(wsURL string, secret string) StatisticsService {
	return StatisticsService{
		wsURL:             wsURL,
		secret:            secret,
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

		log.Infoln("WebSocket connection failed, retrying in %v (attempt %d/%d)",
			interval, retryCount, s.maxRetries)
		time.Sleep(interval)
	}
}

// Stop stops the service
func (s *StatisticsService) Stop() {
	if s.conn != nil {
		s.conn.Close()
	}

	if s.ticker != nil {
		s.ticker.Stop()
	}
	if s.done != nil {
		close(s.done)
	}
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
			log.Infoln("WebSocket read error: %v", err)

			// 连接断开时自动重连
			go func() {
				time.Sleep(s.baseRetryInterval)
				_ = s.connectWithRetry()
			}()
			return
		}

		var traffic Connections
		if err := json.Unmarshal(message, &traffic); err != nil {
			log.Infoln("Failed to unmarshal traffic data: %v", err)
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
		connection, _ := conn.(*Connection)
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
	// todo 查找库里对应的节点，更新流量
	if len(nodeTrafficMap) > 0 {
		log.Debugln("处理完成，节点流量统计: %v", nodeTrafficMap)
		// todo 落库
	}

	// 处理完成后清空closeConns，避免重复统计
	s.closeConns.Range(func(id, conn any) bool {
		s.closeConns.Delete(id)
		return true
	})
}

// startPeriodicProcessing 启动定时处理任务
func (s *StatisticsService) startPeriodicProcessing() {
	for {
		select {
		case <-s.ticker.C:
			log.Debugln("执行定时流量统计处理...")
			s.processCloseConns()
		case <-s.done:
			return
		}
	}
}
