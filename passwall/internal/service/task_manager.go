package service

import (
	"sync"
	"time"
)

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSpeedTest  TaskType = "speed_test"  // 测速
	TaskTypeReloadSubs TaskType = "reload_subs" // 重新加载订阅，没用上
)

// TaskStatus 任务状态
type TaskStatus struct {
	Type       TaskType   // 任务类型
	Running    bool       // 是否正在运行
	StartTime  time.Time  // 开始时间
	FinishTime *time.Time // 完成时间，nil表示未完成
	Progress   int        // 进度(0-100)
	Total      int        // 总任务数
	Completed  int        // 已完成任务数
	Error      string     // 错误信息，空字符串表示没有错误
}

// TaskManager 任务管理器
type TaskManager interface {
	// StartTask 开始任务，如果有同类型任务正在运行，返回false
	StartTask(taskType TaskType, total int) bool

	// UpdateTaskProgress 更新任务进度
	UpdateTaskProgress(taskType TaskType, completed int, err string) bool

	// FinishTask 完成任务
	FinishTask(taskType TaskType, err string) bool

	// IsTaskRunning 检查指定类型的任务是否正在运行
	IsTaskRunning(taskType TaskType) bool

	// IsAnyTaskRunning 检查是否有任何任务正在运行
	IsAnyTaskRunning() bool

	// GetTaskStatus 获取任务状态
	GetTaskStatus(taskType TaskType) *TaskStatus

	// GetAllTaskStatus 获取所有任务状态
	GetAllTaskStatus() map[TaskType]*TaskStatus
}

// taskManagerImpl 任务管理器实现
type taskManagerImpl struct {
	mutex        sync.RWMutex
	taskStatuses map[TaskType]*TaskStatus
}

// NewTaskManager 创建任务管理器
func NewTaskManager() TaskManager {
	return &taskManagerImpl{
		taskStatuses: make(map[TaskType]*TaskStatus),
	}
}

// StartTask 开始任务
func (tm *taskManagerImpl) StartTask(taskType TaskType, total int) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	// 检查是否有同类型任务正在运行
	if status, exists := tm.taskStatuses[taskType]; exists && status.Running {
		return false
	}

	// 创建新任务状态
	tm.taskStatuses[taskType] = &TaskStatus{
		Type:      taskType,
		Running:   true,
		StartTime: time.Now(),
		Progress:  0,
		Total:     total,
		Completed: 0,
		Error:     "",
	}

	return true
}

// UpdateTaskProgress 更新任务进度
func (tm *taskManagerImpl) UpdateTaskProgress(taskType TaskType, completed int, err string) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	status, exists := tm.taskStatuses[taskType]
	if !exists || !status.Running {
		return false
	}

	status.Completed = completed
	if status.Total > 0 {
		status.Progress = completed * 100 / status.Total
	}
	status.Error = err

	return true
}

// FinishTask 完成任务
func (tm *taskManagerImpl) FinishTask(taskType TaskType, err string) bool {
	tm.mutex.Lock()
	defer tm.mutex.Unlock()

	status, exists := tm.taskStatuses[taskType]
	if !exists {
		return false
	}

	status.Running = false
	status.Error = err
	status.Progress = 100
	now := time.Now()
	status.FinishTime = &now

	return true
}

// IsTaskRunning 检查指定类型的任务是否正在运行
func (tm *taskManagerImpl) IsTaskRunning(taskType TaskType) bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	status, exists := tm.taskStatuses[taskType]
	return exists && status.Running
}

// IsAnyTaskRunning 检查是否有任何任务正在运行
func (tm *taskManagerImpl) IsAnyTaskRunning() bool {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	for _, status := range tm.taskStatuses {
		if status.Running {
			return true
		}
	}

	return false
}

// GetTaskStatus 获取任务状态
func (tm *taskManagerImpl) GetTaskStatus(taskType TaskType) *TaskStatus {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	status, exists := tm.taskStatuses[taskType]
	if !exists {
		return nil
	}

	return status
}

// GetAllTaskStatus 获取所有任务状态
func (tm *taskManagerImpl) GetAllTaskStatus() map[TaskType]*TaskStatus {
	tm.mutex.RLock()
	defer tm.mutex.RUnlock()

	result := make(map[TaskType]*TaskStatus)
	for k, v := range tm.taskStatuses {
		result[k] = v
	}

	return result
}
