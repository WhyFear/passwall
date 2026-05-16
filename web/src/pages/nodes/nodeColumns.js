import {
  DeleteOutlined,
  EyeOutlined,
  InfoCircleOutlined,
  PushpinFilled,
  PushpinOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import {Button, Checkbox, Popover, Skeleton, Tooltip} from 'antd';
import {formatDate} from '../../utils/timeUtils';
import {DEFAULT_VISIBLE_COLUMNS, formatRisk, formatSpeed} from './nodeFormatters';
import {AppUnlockStatusTag, StatusTag} from './nodeTags';

export const ALL_NODE_COLUMNS = [
  {key: 'index', title: '序号', fixed: false, hideable: false},
  {key: 'subscription_url', title: '订阅链接', fixed: false, hideable: true},
  {key: 'name', title: '名称', fixed: false, hideable: true},
  {key: 'address', title: '节点', fixed: false, hideable: true},
  {key: 'type', title: '节点类型', fixed: false, hideable: true},
  {key: 'status', title: '状态', fixed: false, hideable: true},
  {key: 'ping', title: 'Ping', fixed: false, hideable: true},
  {key: 'download_speed', title: '下载速度', fixed: false, hideable: true},
  {key: 'upload_speed', title: '上传速度', fixed: false, hideable: true},
  {key: 'latest_test_time', title: '测试时间', fixed: false, hideable: true},
  {key: 'success_rate', title: '成功率', fixed: false, hideable: true},
  {key: 'risk', title: '风险等级', fixed: false, hideable: true},
  {key: 'country_code', title: '国家/地区', fixed: false, hideable: true},
  {key: 'app_unlock', title: 'App 解锁', fixed: false, hideable: true},
  {key: 'action', title: '操作', fixed: true, hideable: false},
];

const defaultTypeFilters = [{text: 'vmess', value: 'vmess'}, {text: 'vless', value: 'vless'}];

const countUnlockedApps = (appUnlock) => new Set(appUnlock
  .filter(item => item?.status === 'unlock' && item?.app_name)
  .map(item => item.app_name)).size;

const AppUnlockDetails = ({appUnlock}) => (
  <div style={{width: 300}}>
    <div style={{
      display: 'flex',
      justifyContent: 'space-between',
      alignItems: 'center',
      marginBottom: 8,
      paddingBottom: 6,
      borderBottom: '1px solid #f0f0f0',
    }}>
      <span style={{fontWeight: 600, color: '#262626'}}>应用解锁详情</span>
      <span style={{fontSize: 12, color: '#8c8c8c'}}>{appUnlock.length} 项检测</span>
    </div>
    <div style={{maxHeight: 360, overflowY: 'auto'}}>
      {appUnlock.map((item, index) => (
        <div
          key={`${item?.app_name || 'app'}-${index}`}
          style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 1fr) 58px 44px',
            alignItems: 'center',
            columnGap: 8,
            padding: '7px 9px',
            marginBottom: index === appUnlock.length - 1 ? 0 : 5,
            background: '#fafafa',
            border: '1px solid #f0f0f0',
            borderRadius: 6,
          }}
        >
          <div style={{
            minWidth: 0,
            color: '#262626',
            fontWeight: 500,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
          }}>
            {item?.app_name || '-'}
          </div>
          <span style={{display: 'inline-flex', justifyContent: 'center'}}>
            <AppUnlockStatusTag status={item?.status} style={{marginInlineEnd: 0}}/>
          </span>
          <span style={{
            display: 'inline-flex',
            justifyContent: 'center',
            color: '#595959',
            fontSize: 12,
            fontWeight: 500,
            fontVariantNumeric: 'tabular-nums',
          }}>
            {item?.region || '-'}
          </span>
        </div>
      ))}
    </div>
  </div>
);

const AppUnlockSummary = ({appUnlock}) => {
  const unlockedCount = countUnlockedApps(appUnlock);

  return (
    <Popover
      trigger="hover"
      placement="topLeft"
      content={<AppUnlockDetails appUnlock={appUnlock}/>}
      overlayInnerStyle={{padding: 12}}
    >
      <div style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 4,
        maxWidth: '100%',
        padding: '2px 8px',
        color: '#1677ff',
        background: '#f0f7ff',
        border: '1px solid #d6e9ff',
        borderRadius: 6,
        cursor: 'default',
        whiteSpace: 'nowrap',
      }}>
        <span>已解锁</span>
        <strong style={{fontVariantNumeric: 'tabular-nums'}}>{unlockedCount}</strong>
        <span style={{color: '#8c8c8c'}}>/ {appUnlock.length}</span>
      </div>
    </Popover>
  );
};

export const createNodeColumns = ({
  visibleColumns,
  nodeTypes,
  countryCodes,
  unlockApps = [],
  isMobile,
  onViewNode,
  onTestProxy,
  onDetectIP,
  onPinProxy,
  onBanProxy,
}) => ([{
  title: '序号',
  key: 'index',
  width: 60,
  render: (_, __, index) => index + 1,
  hidden: !visibleColumns['index']
}, {
  title: '订阅链接',
  dataIndex: 'subscription_url',
  key: 'subscription_url',
  width: 300,
  ellipsis: true,
  hidden: !visibleColumns['subscription_url']
}, {
  title: '名称',
  dataIndex: 'name',
  key: 'name',
  width: 200,
  ellipsis: true,
  hidden: !visibleColumns['name']
}, {
  title: '节点',
  dataIndex: 'address',
  key: 'address',
  width: 200,
  hidden: !visibleColumns['address']
}, {
  title: '节点类型',
  dataIndex: 'type',
  key: 'type',
  width: 120,
  ellipsis: true,
  sorter: true,
  filterMode: 'tree',
  filters: nodeTypes.length > 0 ? nodeTypes : defaultTypeFilters,
  filterMultiple: true,
  hidden: !visibleColumns['type']
}, {
  title: '状态',
  dataIndex: 'status',
  key: 'status',
  width: 100,
  render: (status) => <StatusTag status={status}/>,
  sorter: true,
  filters: [{text: '未测试', value: -1}, {text: '正常', value: 1}, {text: '失败', value: 2}, {
    text: '未知错误', value: 3
  }],
  filterMultiple: true,
  hidden: !visibleColumns['status']
}, {
  title: 'Ping',
  dataIndex: 'ping',
  key: 'ping',
  width: 120,
  render: (ping) => ping ? `${ping}ms` : '-',
  sorter: true,
  hidden: !visibleColumns['ping']
}, {
  title: '下载速度',
  dataIndex: 'download_speed',
  key: 'download_speed',
  width: 120,
  render: (speed) => formatSpeed(speed),
  sorter: true,
  defaultSortOrder: 'descend',
  hidden: !visibleColumns['download_speed']
}, {
  title: '上传速度',
  dataIndex: 'upload_speed',
  key: 'upload_speed',
  width: 110,
  render: (speed) => formatSpeed(speed),
  sorter: true,
  hidden: !visibleColumns['upload_speed']
}, {
  title: '测试时间',
  dataIndex: 'latest_test_time',
  key: 'latest_test_time',
  width: 160,
  render: (text) => formatDate(text),
  sorter: true,
  hidden: !visibleColumns['latest_test_time']
}, {
  title: '成功率',
  dataIndex: 'success_rate',
  key: 'success_rate',
  align: 'right',
  render: (rate, record) => {
    if (record.metadata_loading && rate == null) {
      return <Skeleton.Input active size="small" style={{width: 42, minWidth: 42}}/>;
    }
    return rate == null ? '-' : `${rate}%`;
  },
  width: 80,
  hidden: !visibleColumns['success_rate']
}, {
  title: <Tooltip
    title="风险等级由IPV4及IPV6分别计算，优先展示IPV4的风险等级，可能出现筛选低风险但出现高风险情况"><span>风险等级 <InfoCircleOutlined/></span></Tooltip>,
  dataIndex: ['ip_info', 'risk'],
  key: 'risk',
  width: 110,
  render: (risk, record) => {
    if (record.metadata_loading && risk == null) {
      return <Skeleton.Input active size="small" style={{width: 56, minWidth: 56}}/>;
    }
    return formatRisk(risk);
  },
  filters: [{text: formatRisk('very_low'), value: 'very_low'}, {text: formatRisk('low'), value: 'low'}, {
    text: formatRisk('medium'), value: 'medium'
  }, {text: formatRisk('high'), value: 'high'}, {text: formatRisk('very_high'), value: 'very_high'}],
  filterMultiple: true,
  hidden: !visibleColumns['risk']
}, {
  title: '国家/地区',
  dataIndex: ['ip_info', 'country_code'],
  key: 'country',
  width: 100,
  render: (country, record) => {
    if (record.metadata_loading && country == null) {
      return <Skeleton.Input active size="small" style={{width: 42, minWidth: 42}}/>;
    }
    return country ? country : '-';
  },
  filterMode: 'tree',
  filters: countryCodes.length > 0 ? countryCodes : [],
  filterMultiple: true,
  hidden: !visibleColumns['country_code']
}, {
  title: 'App 解锁',
  dataIndex: ['ip_info', 'app_unlock'],
  key: 'app_unlock',
  width: 110,
  render: (appUnlock, record) => {
    if (record.metadata_loading && appUnlock == null) {
      return <Skeleton.Input active size="small" style={{width: 72, minWidth: 72}}/>;
    }
    if (!Array.isArray(appUnlock) || appUnlock.length === 0) {
      return '-';
    }
    return <AppUnlockSummary appUnlock={appUnlock}/>;
  },
  filters: unlockApps.map(app => (typeof app === 'string' ? {text: app, value: app} : app)),
  filterMultiple: true,
  hidden: !visibleColumns['app_unlock']
}, {
  title: '操作',
  key: 'action',
  width: 130,
  fixed: isMobile ? undefined : 'right',
  render: (_, record) => (<div className="table-action">
    <Tooltip title="查看详情">
      <Button
        type="text"
        icon={<EyeOutlined/>}
        onClick={() => onViewNode(record)}
      />
    </Tooltip>
    <Tooltip title="测速">
      <Button
        type="text"
        icon={<ReloadOutlined/>}
        onClick={() => onTestProxy(record.id)}
      />
    </Tooltip>
    <Tooltip title="检测IP">
      <Button
        type="text"
        icon={<InfoCircleOutlined/>}
        onClick={() => onDetectIP(record.id)}
      />
    </Tooltip>
    <Tooltip title={record.pinned ? '取消置顶' : '置顶'}>
      <Button
        type="text"
        icon={record.pinned ? <PushpinFilled/> : <PushpinOutlined/>}
        onClick={() => onPinProxy(record.id, record.pinned)}
      />
    </Tooltip>
    <Tooltip title="禁用">
      <Button
        type="text"
        icon={<DeleteOutlined/>}
        onClick={() => onBanProxy(record.id)}
      />
    </Tooltip>
  </div>),
  hidden: !visibleColumns['action']
}].filter(column => !column.hidden));

export const createColumnSettingMenu = ({visibleColumns, onColumnVisibilityChange}) => ALL_NODE_COLUMNS.map(column => ({
  key: column.key,
  label: (<Checkbox
    checked={visibleColumns[column.key] ?? DEFAULT_VISIBLE_COLUMNS.includes(column.key)}
    onChange={e => onColumnVisibilityChange(column.key, e.target.checked)}
    disabled={!column.hideable}
    onClick={(e) => e.stopPropagation()}
  >
    {column.title}
  </Checkbox>)
}));
