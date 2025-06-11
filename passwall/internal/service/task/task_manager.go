package task

import (
	"context"
	"sync"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSpeedTest  TaskType = "speed_test"  // 测速
	TaskTypeReloadSubs TaskType = "reload_subs" // 重新加载订阅
	TaskTypeBanProxy   TaskType = "ban_proxy"   // 重新加载订阅
)

// TaskState 任务状态
type TaskState int

const (
	TaskStateRunning  TaskState = iota // 运行中
	TaskStateFinished                  // 已完成
)

// TaskManager 任务管理器接口
type TaskManager interface {
	// StartTask 开始一个新任务，返回任务上下文和是否成功
	// 如果同类型任务已在运行，则返回(nil, false)
	StartTask(ctx context.Context, taskType TaskType, total int) (context.Context, bool)

	// UpdateProgress 更新任务进度
	UpdateProgress(taskType TaskType, completed int, errMsg string)

	UpdateTotal(taskType TaskType, total int)

	// FinishTask 完成任务
	FinishTask(taskType TaskType, errMsg string)

	// CancelTask 取消任务，如果wait为true则等待任务完成
	// 返回是否成功取消和是否因等待超时
	CancelTask(taskType TaskType, wait bool) (bool, bool)

	// IsRunning 检查指定类型的任务是否正在运行
	IsRunning(taskType TaskType) bool

	// IsAnyRunning 检查是否有任何任务正在运行
	IsAnyRunning() bool

	// GetStatus 获取任务状态
	GetStatus(taskType TaskType) *TaskStatus

	GetAllStatus() map[TaskType]*TaskStatus
}

// TaskStatus 任务状态
type TaskStatus struct {
	Type       TaskType   // 任务类型
	State      TaskState  // 任务状态
	StartTime  time.Time  // 开始时间
	FinishTime *time.Time // 完成时间
	Progress   int        // 进度(0-100)
	Total      int        // 总任务数
	Completed  int        // 已完成任务数
	Error      string     // 错误信息
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
	tasks map[TaskType]*taskInfo
}

// NewTaskManager 创建任务管理器
func NewTaskManager() TaskManager {
	return &defaultTaskManager{
		tasks: make(map[TaskType]*taskInfo),
	}
}

// StartTask 开始一个新任务
func (m *defaultTaskManager) StartTask(ctx context.Context, taskType TaskType, total int) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否有同类型任务正在运行
	if task, exists := m.tasks[taskType]; exists && task.status.State == TaskStateRunning {
		return nil, false
	}

	// 创建任务上下文
	ctx, cancelFunc := context.WithCancel(ctx)

	// 创建任务信息
	task := &taskInfo{
		status: TaskStatus{
			Type:      taskType,
			State:     TaskStateRunning,
			StartTime: time.Now(),
			Total:     total,
		},
		ctx:        ctx,
		cancelFunc: cancelFunc,
		doneChan:   make(chan struct{}),
	}

	m.tasks[taskType] = task
	return ctx, true
}

// UpdateProgress 更新任务进度
func (m *defaultTaskManager) UpdateProgress(taskType TaskType, completed int, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskType]
	if !exists || task.status.State != TaskStateRunning {
		return
	}

	task.status.Completed = completed
	if task.status.Total > 0 {
		task.status.Progress = completed * 100 / task.status.Total
	}
	task.status.Error = errMsg
}

func (m *defaultTaskManager) UpdateTotal(taskType TaskType, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[taskType]
	if !exists || task.status.State != TaskStateRunning {
		return
	}
	task.status.Total = total
}

// FinishTask 完成任务
func (m *defaultTaskManager) FinishTask(taskType TaskType, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskType]
	if !exists {
		return
	}

	if task.status.State == TaskStateRunning {
		now := time.Now()
		task.status.FinishTime = &now
		task.status.State = TaskStateFinished
		task.status.Progress = 100
		task.status.Error = errMsg

		// 通知任务已完成
		select {
		case <-task.doneChan: // 已关闭
		default:
			close(task.doneChan)
		}
	}
}

// CancelTask 取消任务
func (m *defaultTaskManager) CancelTask(taskType TaskType, wait bool) (bool, bool) {
	m.mu.Lock()

	task, exists := m.tasks[taskType]
	if !exists || task.status.State != TaskStateRunning {
		m.mu.Unlock()
		return false, false
	}

	// 取消任务上下文
	task.cancelFunc()

	// 获取done通道的引用
	doneChan := task.doneChan

	m.mu.Unlock()

	// 如果不需要等待，直接返回
	if !wait {
		return true, false
	}

	// 等待任务完成，最多10秒
	timeout := false
	select {
	case <-doneChan:
		// 任务已完成
	case <-time.After(10 * time.Second):
		// 等待超时
		timeout = true
		// 强制完成任务
		m.FinishTask(taskType, "任务取消等待超时")
	}

	return true, timeout
}

// IsRunning 检查指定类型的任务是否正在运行
func (m *defaultTaskManager) IsRunning(taskType TaskType) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskType]
	return exists && task.status.State == TaskStateRunning
}

// IsAnyRunning 检查是否有任何任务正在运行
func (m *defaultTaskManager) IsAnyRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, task := range m.tasks {
		if task.status.State == TaskStateRunning {
			return true
		}
	}
	return false
}

// GetStatus 获取任务状态
func (m *defaultTaskManager) GetStatus(taskType TaskType) *TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskType]
	if !exists {
		return nil
	}

	// 返回状态副本
	status := task.status
	return &status
}

func (m *defaultTaskManager) GetAllStatus() map[TaskType]*TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	statusMap := make(map[TaskType]*TaskStatus)
	for taskType, task := range m.tasks {
		statusMap[taskType] = &task.status
	}
	return statusMap
}
