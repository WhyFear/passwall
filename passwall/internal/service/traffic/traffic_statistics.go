package traffic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"passwall/config"
	"passwall/internal/model"
	"passwall/internal/repository"

	"github.com/gorilla/websocket"
	"github.com/metacubex/mihomo/log"
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

type trackedValue struct {
	LastUpload   int64
	LastDownload int64
}

type trafficDelta struct {
	Upload   int64
	Download int64
}

// A generation owns every mutable resource used by one Start call.
type trafficGeneration struct {
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	startTime time.Time
	clients   []config.ClashAPIClient

	connectionsMu sync.Mutex
	connections   map[int]*websocket.Conn

	mu             sync.Mutex
	lastValues     []map[string]trackedValue
	pending        map[uint]trafficDelta
	finalFlushDone bool
}

func newTrafficGeneration(parent context.Context, startTime time.Time, clientCount int) *trafficGeneration {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	generation := &trafficGeneration{
		ctx:         ctx,
		cancel:      cancel,
		startTime:   startTime,
		connections: make(map[int]*websocket.Conn),
		lastValues:  make([]map[string]trackedValue, clientCount),
		pending:     make(map[uint]trafficDelta),
	}
	for i := range generation.lastValues {
		generation.lastValues[i] = make(map[string]trackedValue)
	}
	return generation
}

type StatisticsService struct {
	trafficRepo repository.TrafficRepository

	lifecycleMu sync.Mutex
	run         *trafficGeneration

	baseRetryInterval time.Duration
	maxRetryInterval  time.Duration
	maxRetries        int
	startTimeout      time.Duration
}

func NewTrafficStatisticsService(trafficRepo repository.TrafficRepository) StatisticsService {
	return StatisticsService{
		trafficRepo:       trafficRepo,
		baseRetryInterval: 5 * time.Second,
		maxRetryInterval:  10 * time.Minute,
		maxRetries:        10,
		startTimeout:      10 * time.Second,
	}
}

func (s *StatisticsService) startLocked(cfg config.ClashAPIConfig) error {
	if !cfg.Enable {
		return nil
	}
	if s.run != nil {
		return fmt.Errorf("traffic statistics service is already running")
	}
	if len(cfg.Clients) > 0 {
		if s.trafficRepo == nil {
			return fmt.Errorf("traffic repository is not configured")
		}
		if err := s.trafficRepo.ValidateProxyIDUnique(); err != nil {
			return err
		}
	}

	generation := newTrafficGeneration(nil, time.Now(), len(cfg.Clients))
	generation.clients = append([]config.ClashAPIClient(nil), cfg.Clients...)
	startupCtx, cancelStartup := context.WithTimeout(generation.ctx, s.startTimeout)
	defer cancelStartup()
	initialConnections := make([]*websocket.Conn, len(cfg.Clients))
	for i, client := range cfg.Clients {
		conn, err := s.dialClient(startupCtx, i, client, 1)
		if err != nil {
			generation.cancel()
			for _, opened := range initialConnections {
				if opened != nil {
					_ = opened.Close()
				}
			}
			return err
		}
		initialConnections[i] = conn
	}

	s.run = generation
	generation.wg.Add(1)
	go s.runPeriodicFlush(generation)
	for i, conn := range initialConnections {
		generation.setConnection(i, conn)
		generation.wg.Add(1)
		go s.runClient(generation, i, generation.clients[i], conn)
	}
	return nil
}

// Restart replaces one complete generation while holding the lifecycle lock.
func (s *StatisticsService) Restart(cfg config.ClashAPIConfig) error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	if err := s.stopLocked(); err != nil {
		return err
	}
	return s.startLocked(cfg)
}

// Stop synchronously waits for readers and the periodic flusher before the final flush.
func (s *StatisticsService) Stop() error {
	s.lifecycleMu.Lock()
	defer s.lifecycleMu.Unlock()
	return s.stopLocked()
}

func (s *StatisticsService) stopLocked() error {
	generation := s.run
	if generation == nil {
		return nil
	}

	generation.cancel()
	generation.closeConnections()
	generation.wg.Wait()

	generation.mu.Lock()
	done := generation.finalFlushDone
	generation.mu.Unlock()
	if !done {
		log.Infoln("Statistics service stopping, performing final traffic flush...")
		if err := s.flushTrafficToDB(generation); err != nil {
			return fmt.Errorf("final traffic flush: %w", err)
		}
		generation.mu.Lock()
		generation.finalFlushDone = true
		generation.mu.Unlock()
	}

	s.run = nil
	return nil
}

func (s *StatisticsService) dialClient(ctx context.Context, clientIndex int, client config.ClashAPIClient, attempts int) (*websocket.Conn, error) {
	wsURL, err := connectionsURL(client)
	if err != nil {
		return nil, fmt.Errorf("build connections URL for client %d: %w", clientIndex, err)
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		conn, _, dialErr := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
		if dialErr == nil {
			if ctx.Err() != nil {
				_ = conn.Close()
				return nil, ctx.Err()
			}
			log.Infoln("WebSocket connection established for client %d", clientIndex)
			return conn, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if attempt == attempts {
			log.Errorln("WebSocket connection failed for client %d after %d attempts (error type %T)", clientIndex, attempts, dialErr)
			return nil, fmt.Errorf("connect client %d after %d attempts", clientIndex, attempts)
		}

		interval := s.baseRetryInterval * time.Duration(1<<uint(attempt-1))
		if interval > s.maxRetryInterval {
			interval = s.maxRetryInterval
		}
		log.Errorln("WebSocket connection failed for client %d, retrying in %v (attempt %d/%d, error type %T)",
			clientIndex, interval, attempt, attempts, dialErr)
		timer := time.NewTimer(interval)
		select {
		case <-timer.C:
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("connect client %d failed", clientIndex)
}

func connectionsURL(client config.ClashAPIClient) (string, error) {
	u, err := url.Parse(client.URL)
	if err != nil {
		return "", fmt.Errorf("invalid client URL")
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/connections"
	if client.Secret != "" {
		query := u.Query()
		query.Set("token", client.Secret)
		u.RawQuery = query.Encode()
	}
	return u.String(), nil
}

func (s *StatisticsService) runClient(generation *trafficGeneration, clientIndex int, client config.ClashAPIClient, conn *websocket.Conn) {
	defer generation.wg.Done()
	for {
		if generation.ctx.Err() != nil {
			_ = conn.Close()
			generation.deleteConnection(clientIndex, conn)
			return
		}

		err := s.readMessages(generation, clientIndex, conn)
		generation.deleteConnection(clientIndex, conn)
		_ = conn.Close()
		if generation.ctx.Err() != nil {
			return
		}
		log.Errorln("WebSocket read error for client %d: %v", clientIndex, err)

		conn, err = s.dialClient(generation.ctx, clientIndex, client, s.maxRetries)
		if err != nil {
			if generation.ctx.Err() == nil {
				log.Errorln("WebSocket reconnect failed for client %d: %v", clientIndex, err)
			}
			return
		}
		generation.setConnection(clientIndex, conn)
	}
}

func (s *StatisticsService) readMessages(generation *trafficGeneration, clientIndex int, conn *websocket.Conn) error {
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			return err
		}

		var data Connections
		if err := json.Unmarshal(message, &data); err != nil {
			log.Errorln("Invalid connections payload for client %d: %v", clientIndex, err)
			continue
		}
		s.processTrafficDelta(generation, clientIndex, data)
	}
}

func (s *StatisticsService) processTrafficDelta(generation *trafficGeneration, clientIndex int, data Connections) {
	generation.mu.Lock()
	defer generation.mu.Unlock()
	if clientIndex < 0 || clientIndex >= len(generation.lastValues) {
		return
	}

	clientLastValues := generation.lastValues[clientIndex]
	currentConnIDs := make(map[string]struct{}, len(data.Connections))
	for _, conn := range data.Connections {
		currentConnIDs[conn.ID] = struct{}{}
		last, exists := clientLastValues[conn.ID]
		if !exists && conn.Start.Before(generation.startTime) {
			last = trackedValue{LastUpload: conn.Upload, LastDownload: conn.Download}
		}

		deltaUp := conn.Upload - last.LastUpload
		deltaDown := conn.Download - last.LastDownload
		if (deltaUp > 0 || deltaDown > 0) && len(conn.Chains) > 0 {
			if proxyID, ok := model.ParseRuntimeProxyID(conn.Chains[0]); ok {
				delta := generation.pending[proxyID]
				if deltaUp > 0 {
					delta.Upload += deltaUp
				}
				if deltaDown > 0 {
					delta.Download += deltaDown
				}
				generation.pending[proxyID] = delta
			}
		}

		// Builtin exits and empty chains still advance their snapshots.
		clientLastValues[conn.ID] = trackedValue{LastUpload: conn.Upload, LastDownload: conn.Download}
	}

	for id := range clientLastValues {
		if _, ok := currentConnIDs[id]; !ok {
			delete(clientLastValues, id)
		}
	}
}

func (s *StatisticsService) flushTrafficToDB(generation *trafficGeneration) error {
	generation.mu.Lock()
	if len(generation.pending) == 0 {
		generation.mu.Unlock()
		return nil
	}
	work := generation.pending
	generation.pending = make(map[uint]trafficDelta)
	generation.mu.Unlock()

	proxyIDs := make([]uint, 0, len(work))
	for proxyID := range work {
		proxyIDs = append(proxyIDs, proxyID)
	}
	sort.Slice(proxyIDs, func(i, j int) bool { return proxyIDs[i] < proxyIDs[j] })
	batch := make([]model.TrafficStatistics, 0, len(proxyIDs))
	for _, proxyID := range proxyIDs {
		delta := work[proxyID]
		batch = append(batch, model.TrafficStatistics{
			ProxyID:       proxyID,
			UploadTotal:   delta.Upload,
			DownloadTotal: delta.Download,
		})
	}

	err := func() (err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				err = fmt.Errorf("panic: %v", recovered)
			}
		}()
		return s.trafficRepo.IncrementTraffic(batch)
	}()
	if err == nil {
		return nil
	}

	// The UPSERT is one statement, so the whole failed batch can be added back.
	// ponytail: a lost commit acknowledgement can still duplicate; add batch IDs if that is observed.
	generation.mu.Lock()
	for proxyID, failed := range work {
		pending := generation.pending[proxyID]
		pending.Upload += failed.Upload
		pending.Download += failed.Download
		generation.pending[proxyID] = pending
	}
	generation.mu.Unlock()
	return err
}

func (s *StatisticsService) runPeriodicFlush(generation *trafficGeneration) {
	defer generation.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			if err := s.flushTrafficToDB(generation); err != nil {
				log.Errorln("Flush traffic failed: %v", err)
			}
		case <-generation.ctx.Done():
			return
		}
	}
}

func (generation *trafficGeneration) setConnection(clientIndex int, conn *websocket.Conn) {
	generation.connectionsMu.Lock()
	generation.connections[clientIndex] = conn
	generation.connectionsMu.Unlock()
}

func (generation *trafficGeneration) deleteConnection(clientIndex int, conn *websocket.Conn) {
	generation.connectionsMu.Lock()
	if generation.connections[clientIndex] == conn {
		delete(generation.connections, clientIndex)
	}
	generation.connectionsMu.Unlock()
}

func (generation *trafficGeneration) closeConnections() {
	generation.connectionsMu.Lock()
	for clientIndex, conn := range generation.connections {
		_ = conn.Close()
		delete(generation.connections, clientIndex)
	}
	generation.connectionsMu.Unlock()
}

func (s *StatisticsService) GetTrafficStatistics(proxyID uint) (*model.TrafficStatistics, error) {
	return s.trafficRepo.FindByProxyID(proxyID)
}

func (s *StatisticsService) BatchGetTrafficStatistics(proxyIDList []uint) (map[uint]*model.TrafficStatistics, error) {
	if len(proxyIDList) == 0 {
		return nil, fmt.Errorf("proxyIdList is empty")
	}
	return s.trafficRepo.FindByProxyIDList(proxyIDList)
}
