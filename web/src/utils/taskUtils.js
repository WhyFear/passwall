import { message } from 'antd';
import { taskApi } from '../api';

export const TASK_STATE_RUNNING = 0;
export const TASK_STATE_FINISHED = 1;
export const TASK_STATE_CANCELING = 2;

export const isTaskActive = (taskStatus) => taskStatus
  && (taskStatus.state === TASK_STATE_RUNNING || taskStatus.state === TASK_STATE_CANCELING);

const taskDisplayNames = {
  speed_test: '测速',
  quick_wake: '快速唤醒',
};

// 获取任务状态的通用方法
export const fetchTaskStatus = async (taskType, setTaskStatus) => {
  try {
    const data = await taskApi.getTaskStatus(taskType);
    if (data) {
      setTaskStatus(data);
    } else {
      setTaskStatus(null);
    }
  } catch (error) {
    console.error(`获取${taskType}任务状态失败:`, error);
    setTaskStatus(null);
  }
};

// 停止任务的通用方法
export const stopTask = async (taskType, setTaskStatus) => {
  try {
    const data = await taskApi.stopTask(taskType);
    if (data?.timed_out) {
      message.warning(data.status_msg || '已请求取消，任务仍在清理中');
      await fetchTaskStatus(taskType, setTaskStatus);
      return;
    }
    message.success(`已停止${taskDisplayNames[taskType] || ''}任务`);
    await fetchTaskStatus(taskType, setTaskStatus);
  } catch (error) {
    message.error(`停止任务失败: ${error.message}`);
    console.error(error);
  }
};
