jest.mock('../../utils/taskUtils', () => ({
  TASK_STATE_CANCELING: 2,
  isTaskActive: status => status && (status.state === 0 || status.state === 2),
}));

import NodeBatchActions from './NodeBatchActions';

const baseProps = {
  taskStatus: null,
  quickWakeTaskStatus: null,
  ipDetectTaskStatus: null,
  onStopTask: jest.fn(),
  onStopQuickWake: jest.fn(),
  onStopIPDetect: jest.fn(),
  onBanProxy: jest.fn(),
  onTestProxy: jest.fn(),
  onDetectMissingIP: jest.fn(),
  onExportSubscriptionUrl: jest.fn(),
  onQuickWake: jest.fn(),
  columnSettingMenu: [],
};

const childrenOf = (props = {}) => NodeBatchActions({...baseProps, ...props}).props.children.filter(Boolean);

describe('NodeBatchActions missing IP detection', () => {
  beforeEach(() => jest.clearAllMocks());

  test('only renders the action when enabled and shows selected types in the tooltip', () => {
    const hidden = childrenOf({showDetectMissingIP: false});
    expect(hidden.some(child => child?.props?.children?.props?.children === '补全检测信息')).toBe(false);

    const visible = childrenOf({
      showDetectMissingIP: true,
      filteredNodeTypes: ['ss', 'trojan'],
    });
    const tooltip = visible.find(child => child?.props?.children?.props?.children === '补全检测信息');
    expect(tooltip).toBeTruthy();
    expect(tooltip.props.title.props.children[0].props.children).toBe('检测节点状态：正常');
    expect(tooltip.props.title.props.children[1].props.children).toEqual(['当前筛选节点类型：', 'ss、trojan']);
    tooltip.props.children.props.onClick();
    expect(baseProps.onDetectMissingIP).toHaveBeenCalledTimes(1);
  });

  test('shows check_ip progress and disables duplicate starts while active', () => {
    const children = childrenOf({
      showDetectMissingIP: true,
      ipDetectTaskStatus: {state: 0, total: 4, completed: 2},
    });
    const progress = children.find(child => child?.props?.runningText === 'IP检测进行中');
    const tooltip = children.find(child => child?.props?.children?.props?.children === '补全检测信息');

    expect(progress).toBeTruthy();
    expect(progress.props.taskStatus.completed).toBe(2);
    expect(tooltip.props.children.props.disabled).toBe(true);
  });
});
