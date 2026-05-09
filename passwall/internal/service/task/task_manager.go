package task

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Sentinel errors for task conflict detection.
var (
	ErrTaskConflict       = errors.New("task conflict: conflicting resource access")
	ErrTaskAlreadyRunning = errors.New("task already running")
)

// IsConflictError reports whether err is a task-conflict sentinel error.
func IsConflictError(err error) bool {
	return errors.Is(err, ErrTaskConflict) || errors.Is(err, ErrTaskAlreadyRunning)
}

// TaskType 任务类型
type TaskType string

const (
	TaskTypeSpeedTest  TaskType = "speed_test"  // 测速
	TaskTypeReloadSubs TaskType = "reload_subs" // 重新加载订阅
	TaskTypeBanProxy   TaskType = "ban_proxy"   // 批量禁用代理
	TaskTypeCheckIp    TaskType = "check_ip"    // 检查IP
)

type AccessMode string

const (
	AccessModeRead  AccessMode = "read"
	AccessModeWrite AccessMode = "write"
)

const (
	ResourceProxies       = "proxies"
	ResourceSubscriptions = "subscriptions"
	ResourceSpeedHistory  = "speed_history"
	ResourceIPDetection   = "ip_detection"
)

type TaskAccess struct {
	Resource   string     `json:"resource"`
	ResourceID uint       `json:"resource_id"`
	Mode       AccessMode `json:"mode"`
}

type TaskSpec struct {
	Type       TaskType
	ResourceID uint
	Total      int
	Accesses   []TaskAccess
}

// TaskState 任务状态
type TaskState int

const (
	TaskStateRunning   TaskState = iota // 运行中
	TaskStateFinished                   // 已完成
	TaskStateCanceling                  // 取消中
)

// TaskManager 任务管理器接口
type TaskManager interface {
	// StartTask 开始一个新任务（全局任务，ResourceID 为 0）
	StartTask(ctx context.Context, taskType TaskType, total int) (context.Context, bool)

	// StartResourceTask 开始一个针对特定资源的任务
	StartResourceTask(ctx context.Context, taskType TaskType, resourceID uint, total int) (context.Context, bool)
	// StartTaskWithSpec 按任务读写资源声明启动任务
	StartTaskWithSpec(ctx context.Context, spec TaskSpec) (context.Context, bool)

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
	accesses   []TaskAccess
}

// defaultTaskManager 默认任务管理器实现
type defaultTaskManager struct {
	mu                 sync.RWMutex
	tasks              map[string]*taskInfo // key: taskType or taskType:resourceID
	cancelWaitTimeout  time.Duration
}

const defaultCancelWaitTimeout = 20 * time.Second

// NewTaskManager 创建任务管理器
func NewTaskManager() TaskManager {
	return &defaultTaskManager{
		tasks:             make(map[string]*taskInfo),
		cancelWaitTimeout: defaultCancelWaitTimeout,
	}
}

// NewTaskManagerWithTimeout 创建可自定义取消超时的任务管理器（供测试使用）。
func NewTaskManagerWithTimeout(timeout time.Duration) TaskManager {
	return &defaultTaskManager{
		tasks:             make(map[string]*taskInfo),
		cancelWaitTimeout: timeout,
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
	return m.StartTaskWithSpec(ctx, TaskSpec{
		Type:       taskType,
		ResourceID: resourceID,
		Total:      total,
	})
}

func (m *defaultTaskManager) StartTaskWithSpec(ctx context.Context, spec TaskSpec) (context.Context, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getTaskKey(spec.Type, spec.ResourceID)

	// 检查是否有同类型同资源任务正在运行或取消中
	if t, exists := m.tasks[key]; exists && isActiveState(t.status.State) {
		return nil, false
	}
	if m.hasConflictingAccessLocked(spec.Accesses) {
		return nil, false
	}

	// 创建任务上下文
	ctx, cancelFunc := context.WithCancel(ctx)

	// 创建任务信息
	t := &taskInfo{
		status: TaskStatus{
			Type:       spec.Type,
			ResourceID: spec.ResourceID,
			State:      TaskStateRunning,
			StartTime:  time.Now(),
			Total:      spec.Total,
		},
		ctx:        ctx,
		cancelFunc: cancelFunc,
		doneChan:   make(chan struct{}),
		accesses:   append([]TaskAccess(nil), spec.Accesses...),
	}

	m.tasks[key] = t
	return ctx, true
}

func (m *defaultTaskManager) hasConflictingAccessLocked(accesses []TaskAccess) bool {
	if len(accesses) == 0 {
		return false
	}
	for _, existing := range m.tasks {
		if !isActiveState(existing.status.State) {
			continue
		}
		for _, currentAccess := range accesses {
			for _, existingAccess := range existing.accesses {
				if accessesConflict(currentAccess, existingAccess) {
					return true
				}
			}
		}
	}
	return false
}

func accessesConflict(first TaskAccess, second TaskAccess) bool {
	if first.Resource == "" || second.Resource == "" {
		return false
	}
	if first.Resource != second.Resource {
		return false
	}
	if first.Mode == AccessModeRead && second.Mode == AccessModeRead {
		return false
	}
	if first.ResourceID != 0 && second.ResourceID != 0 && first.ResourceID != second.ResourceID {
		return false
	}
	return true
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
	if !exists || !isActiveState(t.status.State) {
		return
	}

	t.status.Completed = completed
	if t.status.Total > 0 {
		t.status.Progress = completed * 100 / t.status.Total
	}
	if t.status.State != TaskStateCanceling || errMsg != "" {
		t.status.Error = errMsg
	}
}

func (m *defaultTaskManager) UpdateTotal(taskType TaskType, total int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := m.getTaskKey(taskType, 0)
	t, exists := m.tasks[key]
	if !exists || !isActiveState(t.status.State) {
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

	if isActiveState(t.status.State) {
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
	if !exists || !isActiveState(t.status.State) {
		resourceTasks := m.activeResourceTasksLocked(taskType)
		if len(resourceTasks) > 0 {
			doneChans := make([]chan struct{}, 0, len(resourceTasks))
			for _, resourceTask := range resourceTasks {
				resourceTask.cancelFunc()
				resourceTask.status.State = TaskStateCanceling
				if resourceTask.status.Error == "" {
					resourceTask.status.Error = TaskCanceledMessage
				}
				doneChans = append(doneChans, resourceTask.doneChan)
			}
			m.mu.Unlock()
			if !wait {
				return true, false
			}
			timeout := waitForDone(doneChans, m.cancelWaitTimeout)
			if timeout {
				m.markCancelWaitTimeout(taskType)
			}
			return true, timeout
		}
		m.mu.Unlock()
		return false, false
	}

	// 取消任务上下文
	t.cancelFunc()
	t.status.State = TaskStateCanceling
	if t.status.Error == "" {
		t.status.Error = TaskCanceledMessage
	}

	// 获取done通道的引用
	doneChan := t.doneChan

	m.mu.Unlock()

	// 如果不需要等待，直接返回
	if !wait {
		return true, false
	}

	timeout := waitForDone([]chan struct{}{doneChan}, m.cancelWaitTimeout)
	if timeout {
		m.markCancelWaitTimeout(taskType)
	}
	return true, timeout
}

func (m *defaultTaskManager) activeResourceTasksLocked(taskType TaskType) []*taskInfo {
	resourceTasks := make([]*taskInfo, 0)
	for _, t := range m.tasks {
		if t.status.Type == taskType && t.status.ResourceID != 0 && isActiveState(t.status.State) {
			resourceTasks = append(resourceTasks, t)
		}
	}
	return resourceTasks
}

func waitForDone(doneChans []chan struct{}, timeout time.Duration) bool {
	timeoutCh := time.After(timeout)
	for _, doneChan := range doneChans {
		select {
		case <-doneChan:
		case <-timeoutCh:
			return true
		}
	}
	return false
}

func (m *defaultTaskManager) markCancelWaitTimeout(taskType TaskType) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, current := range m.tasks {
		if current.status.Type == taskType && current.status.State == TaskStateCanceling {
			current.status.Error = "任务取消等待超时，仍在清理中"
		}
	}
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
	return exists && isActiveState(t.status.State)
}

// IsAnyRunning 检查是否有任何任务正在运行
func (m *defaultTaskManager) IsAnyRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, t := range m.tasks {
		if isActiveState(t.status.State) {
			return true
		}
	}
	return false
}

func isActiveState(state TaskState) bool {
	return state == TaskStateRunning || state == TaskStateCanceling
}

// GetStatus 获取任务状态
func (m *defaultTaskManager) GetStatus(taskType TaskType) *TaskStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getTaskKey(taskType, 0)
	t, exists := m.tasks[key]
	if exists {
		// 返回状态副本
		status := t.status
		return &status
	}

	return m.aggregateResourceStatusLocked(taskType)
}

func (m *defaultTaskManager) aggregateResourceStatusLocked(taskType TaskType) *TaskStatus {
	var aggregate *TaskStatus
	for _, t := range m.tasks {
		if t.status.Type != taskType || t.status.ResourceID == 0 || !isActiveState(t.status.State) {
			continue
		}
		if aggregate == nil {
			status := TaskStatus{
				Type:      taskType,
				State:     TaskStateRunning,
				StartTime: t.status.StartTime,
			}
			aggregate = &status
		}
		if t.status.StartTime.Before(aggregate.StartTime) {
			aggregate.StartTime = t.status.StartTime
		}
		if t.status.State == TaskStateCanceling {
			aggregate.State = TaskStateCanceling
		}
		aggregate.Total += t.status.Total
		aggregate.Completed += t.status.Completed
		if aggregate.Error == "" {
			aggregate.Error = t.status.Error
		}
	}
	if aggregate == nil {
		return nil
	}
	if aggregate.Total > 0 {
		aggregate.Progress = aggregate.Completed * 100 / aggregate.Total
	}
	return aggregate
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
