package task

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSpeedTest  TaskType = "speed_test"  // 测速
	TaskTypeReloadSubs TaskType = "reload_subs" // 重新加载订阅
	TaskTypeBanProxy   TaskType = "ban_proxy"   // 批量禁用代理
	TaskTypeCheckIp    TaskType = "check_ip"    // 检查IP
)

// TaskState 任务状态
type TaskState int

const (
	TaskStateRunning  TaskState = iota // 运行中
	TaskStateFinished                  // 已完成
)

// TaskManager 任务管理器接口
type TaskManager interface {
	// StartTask 开始一个新任务（全局任务，ResourceID 为 0）
	StartTask(ctx context.Context, taskType TaskType, total int) (context.Context, bool)

	// StartResourceTask 开始一个针对特定资源的任务
	StartResourceTask(ctx context.Context, taskType TaskType, resourceID uint, total int) (context.Context, bool)

	// UpdateProgress 更新任务进度
	UpdateProgress(taskType TaskType, completed int, errMsg string)
	// UpdateResourceProgress 更新特定资源任务的进度
	UpdateResourceProgress(taskType TaskType, resourceID uint, completed int, errMsg string)

	// UpdateTotal 更新任务总数量
	UpdateTotal(taskType TaskType, total int)

	// FinishTask 完成任务（全局任务）
	FinishTask(taskType TaskType, errMsg string)
	// FinishResourceTask 完成特定资源任务
	FinishResourceTask(taskType TaskType, resourceID uint, errMsg string)

	// CancelTask 取消任务
	CancelTask(taskType TaskType, wait bool) (bool, bool)

	// IsRunning 检查指定类型的全局任务是否正在运行
	IsRunning(taskType TaskType) bool
	// IsResourceRunning 检查特定资源任务是否正在运行
	IsResourceRunning(taskType TaskType, resourceID uint) bool

	// IsAnyRunning 检查是否有任何任务正在运行
	IsAnyRunning() bool

	// GetStatus 获取任务状态
	GetStatus(taskType TaskType) *TaskStatus
	// GetAllStatus 获取所有活跃和最近完成的任务状态
	GetAllStatus() []*TaskStatus
}

// TaskStatus 任务状态
type TaskStatus struct {
	Type       TaskType   `json:"type"`        // 任务类型
	ResourceID uint       `json:"resource_id"` // 资源ID (可选)
	State      TaskState  `json:"state"`       // 任务状态
	StartTime  time.Time  `json:"start_time"`  // 开始时间
	FinishTime *time.Time `json:"finish_time"` // 完成时间
	Progress   int        `json:"progress"`    // 进度(0-100)
	Total      int        `json:"total"`       // 总任务数
	Completed  int        `json:"completed"`   // 已完成任务数
	Error      string     `json:"error"`       // 错误信息
}

// 内部任务结构
type taskInfo struct {
	status     TaskStatus
	ctx        context.Context
	cancelFunc context.CancelFunc
	doneChan   chan struct{}
}

// defaultTaskManager 默认任务管理器实现
type defaultTaskManager struct {
	mu    sync.RWMutex
	tasks map[string]*taskInfo // key: taskType or taskType:resourceID
}

// NewTaskManager 创建任务管理器
func NewTaskManager() TaskManager {
	return &defaultTaskManager{
		tasks: make(map[string]*taskInfo),
	}
}

func (m *defaultTaskManager) getTaskKey(taskType TaskType, resourceID uint) string {
	if resourceID == 0 {
		return string(taskType)
	}
	return fmt.Sprintf("%s:%d", taskType, resourceID)
}

// StartTask 开始一个新任务
func (m *defaultTaskManager) StartTask(ctx context.Context, taskType TaskType, total int) (context.Context, bool) {
	return m.StartResourceTask(ctx, taskType, 0, total)
}

// StartResourceTask 开始一个针对特定资源的任务
func (m *defaultTaskManager) StartResourceTask(ctx context.Context, taskType TaskType, resourceID uint, total int) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getTaskKey(taskType, resourceID)

	// 检查是否有同类型同资源任务正在运行
	if t, exists := m.tasks[key]; exists && t.status.State == TaskStateRunning {
		return nil, false
	}

	// 创建任务上下文
	ctx, cancelFunc := context.WithCancel(ctx)

	// 创建任务信息
	t := &taskInfo{
		status: TaskStatus{
			Type:       taskType,
			ResourceID: resourceID,
			State:      TaskStateRunning,
			StartTime:  time.Now(),
			Total:      total,
		},
		ctx:        ctx,
		cancelFunc: cancelFunc,
		doneChan:   make(chan struct{}),
	}

	m.tasks[key] = t
	return ctx, true
}

// UpdateProgress 更新任务进度
func (m *defaultTaskManager) UpdateProgress(taskType TaskType, completed int, errMsg string) {
	m.UpdateResourceProgress(taskType, 0, completed, errMsg)
}

func (m *defaultTaskManager) UpdateResourceProgress(taskType TaskType, resourceID uint, completed int, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getTaskKey(taskType, resourceID)
	t, exists := m.tasks[key]
	if !exists || t.status.State != TaskStateRunning {
		return
	}

	t.status.Completed = completed
	if t.status.Total > 0 {
		t.status.Progress = completed * 100 / t.status.Total
	}
	t.status.Error = errMsg
}

func (m *defaultTaskManager) UpdateTotal(taskType TaskType, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.getTaskKey(taskType, 0)
	t, exists := m.tasks[key]
	if !exists || t.status.State != TaskStateRunning {
		return
	}
	if t.status.Completed >= total {
		t.status.Total = t.status.Completed
		return
	}
	t.status.Total = total
}

// FinishTask 完成任务
func (m *defaultTaskManager) FinishTask(taskType TaskType, errMsg string) {
	m.FinishResourceTask(taskType, 0, errMsg)
}

func (m *defaultTaskManager) FinishResourceTask(taskType TaskType, resourceID uint, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getTaskKey(taskType, resourceID)
	t, exists := m.tasks[key]
	if !exists {
		return
	}

	if t.status.State == TaskStateRunning {
		now := time.Now()
		t.status.FinishTime = &now
		t.status.State = TaskStateFinished
		t.status.Progress = 100
		t.status.Error = errMsg

		// 通知任务已完成
		select {
		case <-t.doneChan: // 已关闭
		default:
			close(t.doneChan)
		}
	}
}

// CancelTask 取消任务
func (m *defaultTaskManager) CancelTask(taskType TaskType, wait bool) (bool, bool) {
	m.mu.Lock()

	key := m.getTaskKey(taskType, 0)
	t, exists := m.tasks[key]
	if !exists || t.status.State != TaskStateRunning {
		m.mu.Unlock()
		return false, false
	}

	// 取消任务上下文
	t.cancelFunc()

	// 获取done通道的引用
	doneChan := t.doneChan

	m.mu.Unlock()

	// 如果不需要等待，直接返回
	if !wait {
		return true, false
	}

	// 等待任务完成，最多20秒
	timeout := false
	select {
	case <-doneChan:
		// 任务已完成
	case <-time.After(20 * time.Second):
		// 等待超时
		timeout = true
		// 强制完成任务
		m.FinishTask(taskType, "任务取消等待超时")
	}

	return true, timeout
}

// IsRunning 检查指定类型的任务是否正在运行
func (m *defaultTaskManager) IsRunning(taskType TaskType) bool {
	return m.IsResourceRunning(taskType, 0)
}

func (m *defaultTaskManager) IsResourceRunning(taskType TaskType, resourceID uint) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getTaskKey(taskType, resourceID)
	t, exists := m.tasks[key]
	return exists && t.status.State == TaskStateRunning
}

// IsAnyRunning 检查是否有任何任务正在运行
func (m *defaultTaskManager) IsAnyRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tasks {
		if t.status.State == TaskStateRunning {
			return true
		}
	}
	return false
}

// GetStatus 获取任务状态
func (m *defaultTaskManager) GetStatus(taskType TaskType) *TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getTaskKey(taskType, 0)
	t, exists := m.tasks[key]
	if !exists {
		return nil
	}

	// 返回状态副本
	status := t.status
	return &status
}

func (m *defaultTaskManager) GetAllStatus() []*TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var statuses []*TaskStatus
	for _, t := range m.tasks {
		statusCopy := t.status
		statuses = append(statuses, &statusCopy)
	}
	return statuses
}
