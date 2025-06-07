import { message } from 'antd';
import { taskApi } from '../api';

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
    await taskApi.stopTask(taskType);
    message.success(`已停止${taskType === 'speed_test' ? '测速' : ''}任务`);
    setTaskStatus(null);
  } catch (error) {
    message.error(`停止任务失败: ${error.message}`);
    console.error(error);
  }
}; 