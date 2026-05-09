import {SettingOutlined, StopOutlined} from '@ant-design/icons';
import {Button, Dropdown, Progress} from 'antd';
import {isTaskActive, TASK_STATE_CANCELING} from '../../utils/taskUtils';

const NodeBatchActions = ({
  taskStatus,
  onStopTask,
  onBanProxy,
  onTestProxy,
  onExportSubscriptionUrl,
  columnSettingMenu,
}) => (
  <div className="tab-bar-extra" style={{display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap'}}>
    {isTaskActive(taskStatus) && (<div style={{display: 'flex', alignItems: 'center'}}>
      <Progress
        type="circle"
        percent={Math.round((taskStatus.completed / taskStatus.total) * 100)}
        size="small"
        style={{marginRight: 8}}
      />
      <span style={{marginRight: 8}}>
        {taskStatus.state === TASK_STATE_CANCELING ? '测速取消中' : '测速进行中'}: {taskStatus.completed}/{taskStatus.total}
      </span>
      <Button
        type="primary"
        danger
        icon={<StopOutlined/>}
        onClick={onStopTask}
        style={{margin: 0}}
      >
        停止任务
      </Button>
    </div>)}
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
