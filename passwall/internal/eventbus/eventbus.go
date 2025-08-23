package eventbus

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Event 事件接口
type Event interface {
	GetType() string
	GetTimestamp() time.Time
	GetData() map[string]interface{}
}

// EventHandler 事件处理器接口
type EventHandler interface {
	HandleEvent(event Event) error
}

// EventBus 事件总线接口
type EventBus interface {
	Publish(event Event) error
	Subscribe(handler EventHandler) error
	Unsubscribe(handler EventHandler) error
	GetHandlers() []EventHandler
}

// eventBus 事件总线实现
type eventBus struct {
	handlers map[EventHandler]struct{}
	mu       sync.RWMutex
}

// NewEventBus 创建新的事件总线
func NewEventBus() EventBus {
	return &eventBus{
		handlers: make(map[EventHandler]struct{}),
	}
}

// Publish 发布事件
func (eb *eventBus) Publish(event Event) error {
	if event == nil {
		return fmt.Errorf("event cannot be nil")
	}

	eb.mu.RLock()
	defer eb.mu.RUnlock()

	// 异步处理事件，避免阻塞发布者
	go func() {
		for handler := range eb.handlers {
			go func(h EventHandler) {
				if err := h.HandleEvent(event); err != nil {
					// 记录错误，但不影响其他处理器
					fmt.Printf("Event handler error: %v\n", err)
				}
			}(handler)
		}
	}()

	return nil
}

// Subscribe 订阅事件
func (eb *eventBus) Subscribe(handler EventHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	eb.handlers[handler] = struct{}{}
	return nil
}

// Unsubscribe 取消订阅
func (eb *eventBus) Unsubscribe(handler EventHandler) error {
	if handler == nil {
		return fmt.Errorf("handler cannot be nil")
	}

	eb.mu.Lock()
	defer eb.mu.Unlock()

	delete(eb.handlers, handler)
	return nil
}

// GetHandlers 获取所有处理器
func (eb *eventBus) GetHandlers() []EventHandler {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	handlers := make([]EventHandler, 0, len(eb.handlers))
	for handler := range eb.handlers {
		handlers = append(handlers, handler)
	}

	return handlers
}

// BaseEvent 基础事件结构
type BaseEvent struct {
	Type      string                 `json:"type"`
	Timestamp time.Time              `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

func (be *BaseEvent) GetType() string {
	return be.Type
}

func (be *BaseEvent) GetTimestamp() time.Time {
	return be.Timestamp
}

func (be *BaseEvent) GetData() map[string]interface{} {
	return be.Data
}

// NewBaseEvent 创建基础事件
func NewBaseEvent(eventType string, data map[string]interface{}) *BaseEvent {
	return &BaseEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

// LoggingEventHandler 日志事件处理器
type LoggingEventHandler struct{}

func (leh *LoggingEventHandler) HandleEvent(event Event) error {
	fmt.Printf("[Event] %s - %s: %v\n",
		event.GetTimestamp().Format("2006-01-02 15:04:05"),
		event.GetType(),
		event.GetData())
	return nil
}

// CompositeEventHandler 复合事件处理器
type CompositeEventHandler struct {
	handlers []EventHandler
}

func (ceh *CompositeEventHandler) HandleEvent(event Event) error {
	var firstError error

	for _, handler := range ceh.handlers {
		if err := handler.HandleEvent(event); err != nil {
			if firstError == nil {
				firstError = err
			}
			// 继续处理其他处理器
		}
	}

	return firstError
}

// AddHandler 添加处理器
func (ceh *CompositeEventHandler) AddHandler(handler EventHandler) {
	ceh.handlers = append(ceh.handlers, handler)
}

// RemoveHandler 移除处理器
func (ceh *CompositeEventHandler) RemoveHandler(handler EventHandler) {
	for i, h := range ceh.handlers {
		if h == handler {
			ceh.handlers = append(ceh.handlers[:i], ceh.handlers[i+1:]...)
			break
		}
	}
}

// NewCompositeEventHandler 创建复合事件处理器
func NewCompositeEventHandler(handlers ...EventHandler) *CompositeEventHandler {
	return &CompositeEventHandler{
		handlers: handlers,
	}
}

// AsyncEventHandler 异步事件处理器
type AsyncEventHandler struct {
	handler     EventHandler
	bufferSize  int
	workerCount int
	eventChan   chan Event
	ctx         context.Context
	cancel      context.CancelFunc
	wg          sync.WaitGroup
}

func (aeh *AsyncEventHandler) HandleEvent(event Event) error {
	select {
	case aeh.eventChan <- event:
		return nil
	default:
		return fmt.Errorf("event channel is full")
	}
}

// Start 启动异步处理器
func (aeh *AsyncEventHandler) Start() {
	for i := 0; i < aeh.workerCount; i++ {
		aeh.wg.Add(1)
		go aeh.worker()
	}
}

// Stop 停止异步处理器
func (aeh *AsyncEventHandler) Stop() {
	aeh.cancel()
	aeh.wg.Wait()
}

func (aeh *AsyncEventHandler) worker() {
	defer aeh.wg.Done()

	for {
		select {
		case event := <-aeh.eventChan:
			if err := aeh.handler.HandleEvent(event); err != nil {
				fmt.Printf("Async event handler error: %v\n", err)
			}
		case <-aeh.ctx.Done():
			return
		}
	}
}

// NewAsyncEventHandler 创建异步事件处理器
func NewAsyncEventHandler(handler EventHandler, bufferSize, workerCount int) *AsyncEventHandler {
	ctx, cancel := context.WithCancel(context.Background())

	return &AsyncEventHandler{
		handler:     handler,
		bufferSize:  bufferSize,
		workerCount: workerCount,
		eventChan:   make(chan Event, bufferSize),
		ctx:         ctx,
		cancel:      cancel,
	}
}

// FilteredEventHandler 过滤事件处理器
type FilteredEventHandler struct {
	handler    EventHandler
	eventTypes map[string]bool
	predicate  func(event Event) bool
}

func (feh *FilteredEventHandler) HandleEvent(event Event) error {
	// 检查事件类型过滤
	if len(feh.eventTypes) > 0 {
		if !feh.eventTypes[event.GetType()] {
			return nil // 跳过不匹配的事件类型
		}
	}

	// 检查自定义谓词过滤
	if feh.predicate != nil {
		if !feh.predicate(event) {
			return nil // 跳过不匹配的事件
		}
	}

	return feh.handler.HandleEvent(event)
}

// NewFilteredEventHandler 创建过滤事件处理器
func NewFilteredEventHandler(handler EventHandler, eventTypes []string, predicate func(event Event) bool) *FilteredEventHandler {
	typeMap := make(map[string]bool)
	for _, eventType := range eventTypes {
		typeMap[eventType] = true
	}

	return &FilteredEventHandler{
		handler:    handler,
		eventTypes: typeMap,
		predicate:  predicate,
	}
}

// RetryEventHandler 重试事件处理器
type RetryEventHandler struct {
	handler    EventHandler
	maxRetries int
	retryDelay time.Duration
	retryable  func(error) bool
}

func (reh *RetryEventHandler) HandleEvent(event Event) error {
	var lastError error

	for i := 0; i < reh.maxRetries; i++ {
		err := reh.handler.HandleEvent(event)
		if err == nil {
			return nil
		}

		lastError = err

		// 检查是否可重试
		if reh.retryable != nil && !reh.retryable(err) {
			return err
		}

		// 如果不是最后一次重试，等待延迟时间
		if i < reh.maxRetries-1 {
			time.Sleep(reh.retryDelay)
		}
	}

	return lastError
}

// NewRetryEventHandler 创建重试事件处理器
func NewRetryEventHandler(handler EventHandler, maxRetries int, retryDelay time.Duration, retryable func(error) bool) *RetryEventHandler {
	return &RetryEventHandler{
		handler:    handler,
		maxRetries: maxRetries,
		retryDelay: retryDelay,
		retryable:  retryable,
	}
}

// 全局事件总线实例
var GlobalEventBus = NewEventBus()

// 全局日志事件处理器
var GlobalLoggingEventHandler = &LoggingEventHandler{}

// 初始化全局事件总线
func init() {
	GlobalEventBus.Subscribe(GlobalLoggingEventHandler)
}
