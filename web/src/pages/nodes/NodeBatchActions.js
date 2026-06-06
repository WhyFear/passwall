import {SettingOutlined, StopOutlined} from '@ant-design/icons';
import {Button, Dropdown, Progress} from 'antd';
import {isTaskActive, TASK_STATE_CANCELING} from '../../utils/taskUtils';

const TaskProgress = ({taskStatus, runningText, cancelingText, stopText, onStop}) => {
  const total = taskStatus?.total || 0;
  const completed = taskStatus?.completed || 0;
  const percent = total > 0 ? Math.round((completed / total) * 100) : 0;

  return (<div style={{display: 'flex', alignItems: 'center'}}>
    <Progress
      type="circle"
      percent={percent}
      size="small"
      style={{marginRight: 8}}
    />
    <span style={{marginRight: 8}}>
      {taskStatus.state === TASK_STATE_CANCELING ? cancelingText : runningText}: {completed}/{total}
    </span>
    <Button
      type="primary"
      danger
      icon={<StopOutlined/>}
      onClick={onStop}
      style={{margin: 0}}
    >
      {stopText}
    </Button>
  </div>);
};

const NodeBatchActions = ({
  taskStatus,
  quickWakeTaskStatus,
  onStopTask,
  onStopQuickWake,
  onBanProxy,
  onTestProxy,
  onExportSubscriptionUrl,
  onQuickWake,
  columnSettingMenu,
}) => (
  <div className="tab-bar-extra" style={{display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap'}}>
    {isTaskActive(taskStatus) && (
      <TaskProgress
        taskStatus={taskStatus}
        runningText="测速进行中"
        cancelingText="测速取消中"
        stopText="停止任务"
        onStop={onStopTask}
      />
    )}
    {isTaskActive(quickWakeTaskStatus) && (
      <TaskProgress
        taskStatus={quickWakeTaskStatus}
        runningText="唤醒进行中"
        cancelingText="唤醒取消中"
        stopText="停止唤醒"
        onStop={onStopQuickWake}
      />
    )}
    <Button
      type="primary"
      danger
      onClick={() => onBanProxy(null)}
      style={{margin: 0}}
    >
      批量禁用节点
    </Button>
    <Button
      type="primary"
      onClick={() => onTestProxy(null)}
      style={{margin: 0}}
    >
      按当前参数进行测速
    </Button>
    <Button
      type="primary"
      onClick={onExportSubscriptionUrl}
      style={{margin: 0}}
    >
      分享管理
    </Button>
    <Button
      type="primary"
      onClick={onQuickWake}
      style={{margin: 0}}
    >
      快速唤醒
    </Button>
    <Dropdown menu={{items: columnSettingMenu}} trigger={['click']}>
      <Button
        type="primary"
        icon={<SettingOutlined/>}
        style={{margin: 0}}
      >
        列设置
      </Button>
    </Dropdown>
  </div>
);

export default NodeBatchActions;
