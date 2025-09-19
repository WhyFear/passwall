import {
  DeleteOutlined,
  EyeOutlined,
  InfoCircleOutlined,
  PushpinFilled,
  PushpinOutlined,
  ReloadOutlined,
  SettingOutlined,
  StopOutlined
} from '@ant-design/icons';
import {Button, Card, Checkbox, Dropdown, InputNumber, message, Modal, Progress, Table, Tabs, Tag, Tooltip} from 'antd';
import {useEffect, useRef, useState} from 'react';
import {nodeApi, subscriptionApi} from '../api';
import {fetchTaskStatus, stopTask} from '../utils/taskUtils';
import {formatDate} from '../utils/timeUtils';

// 状态标签组件
const StatusTag = ({status}) => {
  let color = 'default';
  let text = '未知';

  if (status === -1) {
    color = 'default';
    text = '未测试';
  } else if (status === 1) {
    color = 'success';
    text = '正常';
  } else if (status === 2) {
    color = 'error';
    text = '失败';
  } else if (status === 3) {
    color = 'warning';
    text = '未知错误';
  }

  return <Tag color={color}>{text}</Tag>;
};

const AppUnlockStatusTag = ({status}) => {
  let color = 'default';
  let text = '未知';
  // fail unlock forbidden
  if (status === "fail") {
    color = 'error';
    text = '失败';
  } else if (status === "unlock") {
    color = 'success';
    text = '解锁';
  } else if (status === "forbidden") {
    color = 'warning';
    text = '屏蔽';
  } else if (status === "rateLimit") {
    color = 'warning';
    text = '限流';
  }

  return <Tag color={color}>{text}</Tag>;
};

// 信息项组件，用于显示节点详情中的每一项
const InfoItem = ({label, value}) => {
  return (<div style={{display: 'flex', alignItems: 'center', marginBottom: '8px'}}>
    <strong style={{width: '100px', textAlign: 'right', marginRight: '8px'}}>{label}:</strong>
    <span style={{
      flex: 1,
      backgroundColor: '#f5f5f5',
      padding: '4px 8px',
      borderRadius: '4px',
      border: '1px solid #e8e8e8',
      wordBreak: 'break-all'
    }}>{value}</span>
  </div>);
};

// 格式化速度的函数
const formatSpeed = (bytesPerSecond) => {
  if (!bytesPerSecond) return '-';

  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'];
  let unit = 0;
  let speed = bytesPerSecond;

  while (speed >= 1024 && unit < units.length - 1) {
    speed /= 1024;
    unit++;
  }

  return `${speed.toFixed(2)}${units[unit]}`;
};

const formatRisk = (risk) => {
  if (!risk) return '-';
  switch (risk) {
    case 'very_low':
      return '非常低';
    case 'low':
      return '低';
    case 'medium':
      return '中';
    case 'high':
      return '高';
    case 'very_high':
      return '非常高';
    default:
      return '-';
  }
}

// 格式化流量的函数
const formatTraffic = (bytes) => {
  if (!bytes) return '-';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let unit = 0;
  let traffic = bytes;

  while (traffic >= 1024 && unit < units.length - 1) {
    traffic /= 1024;
    unit++;
  }

  return `${traffic.toFixed(2)}${units[unit]}`;
};

const NodesPage = () => {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('2');
  const [modalVisible, setModalVisible] = useState(false);
  const [currentNode, setCurrentNode] = useState(null);
  const [nodeHistory, setNodeHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1, pageSize: 10, total: 0,
  });
  const [historyPagination, setHistoryPagination] = useState({
    current: 1, pageSize: 5, total: 0,
  });
  const [sorter, setSorter] = useState({
    field: 'download_speed', order: 'descend',
  });
  const [filters, setFilters] = useState({});
  const [nodeTypes, setNodeTypes] = useState([]);
  const [taskStatus, setTaskStatus] = useState(null);
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 600);
  const [countryCodes, setCountryCodes] = useState({});

  const timerRef = useRef(null);

  // 获取所有节点
  const fetchNodes = async (page = pagination.current, pageSize = pagination.pageSize, sort = sorter, filter = filters) => {
    try {
      setLoading(true);

      // 构建请求参数
      const params = {
        page: page, pageSize: pageSize, sortField: sort.field, sortOrder: sort.order,
      };

      // 处理状态筛选
      if (filter.status && filter.status.length > 0) {
        // 按status=1,2,3拼接
        params.status = filter.status.join(',');
      }

      // 处理节点类型筛选
      if (filter.type && filter.type.length > 0) {
        params.type = filter.type.join(',');
      }

      if (filter.country) {
        params.country_code = filter.country.join(',');
      }

      if (filter.risk) {
        params.risk_level = filter.risk.join(',');
      }

      const data = await subscriptionApi.getProxies({params});
      // 在开始加载时立即清空节点列表，避免分页时数据乱序
      setNodes([]);

      // 直接使用返回的items数组作为节点列表
      const nodeList = Array.isArray(data.items) ? data.items : [];
      setNodes(nodeList);
      setPagination(prev => ({
        ...prev, current: page, pageSize: pageSize, total: data.total || nodeList.length,
      }));
    } catch (error) {
      message.error('获取节点列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 获取节点历史
  const fetchNodeHistory = async (nodeId, page = historyPagination.current, pageSize = historyPagination.pageSize) => {
    try {
      setHistoryLoading(true);
      const data = await nodeApi.getProxyHistory(nodeId, page, pageSize);
      setNodeHistory(Array.isArray(data?.items) ? data.items : []);
      setHistoryPagination(prev => ({
        ...prev, current: page, pageSize: pageSize, total: data?.total || 0,
      }));
    } catch (error) {
      message.error('获取节点历史失败');
      console.error(error);
    } finally {
      setHistoryLoading(false);
    }
  };

  // 获取节点分享链接
  const fetchNodeShareUrl = async (nodeId) => {
    try {
      setHistoryLoading(true);
      let data = await nodeApi.getProxyShareUrl(nodeId);
      // 先判断data是否有status_code字段，如果有，说明是错误信息
      if (data.status_code) {
        message.error('获取节点分享链接失败：' + data.status_msg);
        return;
      }
      setCurrentNode(prev => ({
        ...prev, share_url: atob(data),
      }));
    } catch (error) {
      message.error('获取节点分享链接失败：' + error.message);
      console.error(error);
    } finally {
      setHistoryLoading(false);
    }
  };

  // 获取节点类型
  const fetchNodeTypes = async () => {
    try {
      const data = await nodeApi.getTypes();
      if (Array.isArray(data)) {
        setNodeTypes(data.map(type => ({
          text: type, value: type
        })));
      }
    } catch (error) {
      message.error('获取节点类型失败');
      console.error(error);
    }
  };

  // 获取国家代码
  const fetchCountryCodes = async () => {
    try {
      const data = await nodeApi.getCountryCodes();
      if (Array.isArray(data?.data)) {
        const list = data?.data;
        setCountryCodes(list.map(code => ({
          text: code, value: code
        })));
      }
    } catch (error) {
      console.error('获取国家代码失败');
      setCountryCodes({});
      console.error(error);
    }
  };
  // 获取任务状态
  const fetchTaskStatusHandler = async () => {
    await fetchTaskStatus("speed_test", setTaskStatus);
  };

  // 停止任务
  const handleStopTask = async () => {
    await stopTask("speed_test", setTaskStatus);
  };

  // 启动定时器
  useEffect(() => {
    // 初始获取一次任务状态
    fetchTaskStatusHandler();
    fetchNodes();
    fetchNodeTypes();
    fetchCountryCodes();

    // 设置定时器，每3秒获取一次任务状态
    timerRef.current = setInterval(() => {
      fetchTaskStatusHandler();
    }, 3000);

    const handleResize = () => {
      setIsMobile(window.innerWidth <= 600);
    };
    window.addEventListener('resize', handleResize);

    // 组件卸载时清除定时器
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
      window.removeEventListener('resize', handleResize);
    };
  }, []);


  // 初始化列显示状态
  useEffect(() => {
    const savedColumns = localStorage.getItem('nodeTableColumns');
    if (savedColumns) {
      try {
        setVisibleColumns(JSON.parse(savedColumns));
        return
      } catch (e) {
        console.error('Failed to parse saved column settings', e);
      }
    }
    // 其他情况，比如首次加载或者配置错误时使用默认设置
    const initialColumns = {};
    defaultVisibleColumns.forEach(key => {
      initialColumns[key] = true;
    });
    setVisibleColumns(initialColumns);
  }, []);

// 处理表格分页变化
  const handleTableChange = (newPagination, newFilters, newSorter) => {
    const sort = newSorter.field ? {
      field: newSorter.field, order: newSorter.order || 'descend',
    } : sorter;

    // 先更新状态，然后使用最新的状态调用 fetchNodes
    setSorter(sort);
    setFilters(newFilters);

    // 直接使用新的参数值，而不是依赖异步更新的状态
    fetchNodes(newPagination.current, newPagination.pageSize, sort, newFilters);
  };

  // 处理历史记录表格分页变化
  const handleHistoryTableChange = (newPagination) => {
    if (currentNode) {
      fetchNodeHistory(currentNode.id, newPagination.current, newPagination.pageSize);
    }
  };

  // 导出订阅链接
  const handleExportSubscriptionUrl = async () => {
    try {
      const params = {
        sort: sorter.field, sortOrder: sorter.order,
      };
      if (filters.status && filters.status.length > 0) {
        params.status = filters.status.join(',');
      }
      if (filters.type && filters.type.length > 0) {
        params.proxy_type = filters.type.join(',');
      }
      params.token = localStorage.getItem('token');
      params.with_index = 1;
      params.type = 'share_link';

      const url = `${window.location.host}/api/subscribe`;
      const urlParams = new URLSearchParams(params);
      const export_url = url + `?${urlParams.toString()}`;

      // 复制功能
      if (window.isSecureContext && navigator.clipboard) {
        navigator.clipboard.writeText(export_url)
          .then(() => message.success('订阅链接已复制到剪贴板'))
          .catch(() => message.error('复制失败，请手动复制'));
      } else {
        // 非https环境下无法使用navigator.clipboard，使用textarea模拟复制
        const textArea = document.createElement("textarea");
        textArea.value = export_url;
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
          document.execCommand('copy');
        } catch (err) {
          message.error('复制失败，请手动复制\n' + export_url);
        }
        document.body.removeChild(textArea);
      }
    } catch (error) {
      message.error('获取订阅链接失败');
      console.error(error);
    }
  };

  const handleTestProxy = async (nodeId) => {
    try {
      const params = {};

      // 检查 filters.status 是否存在且是数组
      if (filters.status && Array.isArray(filters.status) && filters.status.length > 0) {
        params.status = filters.status.join(',');
      }

      // 检查 filters.type 是否存在且是数组
      if (filters.type && Array.isArray(filters.type) && filters.type.length > 0) {
        params.type = filters.type.join(',');
      }
      if (nodeId) {
        params.id = nodeId;
      }
      const data = await nodeApi.testProxy(params);
      if (data.status_code === 200) {
        message.success('测速任务已启动');
        // 立即获取一次任务状态
        setTimeout(() => {
          fetchTaskStatusHandler();
        }, 500);
      } else {
        message.error('测速失败：' + data.status_msg);
      }
    } catch (error) {
      message.error('测速失败：' + error.message);
      console.error(error);
    }
  };
  // 处理IP检测
  const handleDetectIP = async (nodeId) => {
    try {
      const params = {
        proxy_id: nodeId,
      };
      const data = await nodeApi.detectIP(params);
      if (data.status_code === 200) {
        message.success('IP检测任务已启动');
      } else {
        message.error('IP检测失败：' + data.status_msg);
      }
    } catch (error) {
      message.error('IP检测失败：' + error.message);
      console.error(error);
    }
  };

  // 处理节点置顶
  const handlePinProxy = async (nodeId, currentPinned) => {
    try {
      // 调用置顶接口，传递相反的状态以切换
      const newPinned = !currentPinned;
      const data = await nodeApi.pinProxy(nodeId, newPinned);
      if (data && data.status_code === 200) {
        message.success(newPinned ? '节点已置顶' : '节点已取消置顶');
        // 刷新节点列表
        fetchNodes(pagination.current, pagination.pageSize, sorter, filters);
      } else {
        message.error((newPinned ? '置顶' : '取消置顶') + '失败：' + (data?.status_msg || '未知错误'));
      }
    } catch (error) {
      message.error((currentPinned ? '取消置顶' : '置顶') + '失败：' + error.message);
      console.error(error);
    }
  };

  // 查看节点详情
  const handleViewNode = (node) => {
    setCurrentNode(node);
    // 重置历史分页到第一页
    setHistoryPagination(prev => ({
      ...prev, current: 1, pageSize: 5,
    }));
    fetchNodeHistory(node.id, 1, 5);
    fetchNodeShareUrl(node.id);
    setModalVisible(true);
  };

  const handleBanProxy = async (id = null) => {
    try {
      // 创建禁用参数对象
      let banParams = {
        success_rate_threshold: 0,
        download_speed_threshold: 0,
        upload_speed_threshold: 0,
        ping_threshold: 0,
        test_times: 5,
      };

      // 如果有ID，则添加到参数中
      if (id) {
        banParams.id = id;
      }

      const content = '除非节点配置有更新，否则节点将被永久禁用无法恢复，确认继续吗？';
      // 弹出确认对话框
      Modal.confirm({
        title: '确认禁用', content: id ? (content) : (<div>
          <p>{content}</p>
          <div style={{marginTop: '15px'}}>
            <div style={{marginBottom: '10px'}}>
              <span style={{display: 'inline-block', width: '180px'}}>成功率阈值(%)：</span>
              <InputNumber
                min={0}
                max={100}
                defaultValue={0}
                step={10}
                onChange={(value) => banParams.success_rate_threshold = value}
              />
            </div>
            <div style={{marginBottom: '10px'}}>
              <span style={{display: 'inline-block', width: '180px'}}>下载速度阈值(B/s)：</span>
              <InputNumber
                min={0}
                defaultValue={0}
                precision={0}
                onChange={(value) => banParams.download_speed_threshold = value}
              />
            </div>
            <div style={{marginBottom: '10px'}}>
              <span style={{display: 'inline-block', width: '180px'}}>上传速度阈值(B/s)：</span>
              <InputNumber
                min={0}
                defaultValue={0}
                precision={0}
                onChange={(value) => banParams.upload_speed_threshold = value}
              />
            </div>
            <div style={{marginBottom: '10px'}}>
              <span style={{display: 'inline-block', width: '180px'}}>Ping阈值(ms)：</span>
              <InputNumber
                min={0}
                defaultValue={0}
                precision={0}
                onChange={(value) => banParams.ping_threshold = value}
              />
            </div>
            <div style={{marginBottom: '10px'}}>
              <span style={{display: 'inline-block', width: '180px'}}>测试次数：</span>
              <InputNumber
                min={1}
                defaultValue={5}
                precision={0}
                onChange={(value) => banParams.test_times = value}
              />
            </div>
          </div>
        </div>), okText: '确认', cancelText: '取消', width: id ? 400 : 500, onOk: async () => {
          try {
            const data = await nodeApi.banProxy(banParams);
            if (data.status_code === 200) {
              message.success('任务提交成功');
              // 刷新节点列表
              fetchNodes(pagination.current, pagination.pageSize, sorter, filters);
            } else {
              message.error('任务提交失败：' + data.status_msg);
            }
          } catch (error) {
            message.error('任务提交失败失败：' + error.message);
            console.error(error);
          }
        }
      });
    } catch (error) {
      message.error('打开对话框失败：' + error.message);
      console.error(error);
    }
  };


  const [visibleColumns, setVisibleColumns] = useState({});

  // 定义所有列的配置
  const allColumns = [{key: 'index', title: '序号', fixed: false, hideable: false}, {
    key: 'subscription_url', title: '订阅链接', fixed: false, hideable: true
  }, {
    key: 'name', title: '名称', fixed: false, hideable: true
  }, {
    key: 'address', title: '节点', fixed: false, hideable: true
  }, {
    key: 'type', title: '节点类型', fixed: false, hideable: true
  }, {
    key: 'status', title: '状态', fixed: false, hideable: true
  }, {key: 'ping', title: 'Ping', fixed: false, hideable: true}, {
    key: 'download_speed', title: '下载速度', fixed: false, hideable: true
  }, {key: 'upload_speed', title: '上传速度', fixed: false, hideable: true}, {
    key: 'latest_test_time', title: '测试时间', fixed: false, hideable: true
  }, {key: 'success_rate', title: '成功率', fixed: false, hideable: true}, {
    key: 'risk', title: '风险等级', fixed: false, hideable: true
  }, {key: 'country_code', title: '国家/地区', fixed: false, hideable: true}, {
    key: 'action', title: '操作', fixed: true, hideable: false
  }];

  // 默认显示的列（不可隐藏的列+一些默认显示的可隐藏列）
  const defaultVisibleColumns = ['index', 'subscription_url', 'name', 'address', 'type', 'status', 'ping', 'download_speed', 'upload_speed', 'latest_test_time', 'success_rate', 'action'];


  // 保存列设置到本地存储
  const saveColumnSettings = (newSettings) => {
    try {
      localStorage.setItem('nodeTableColumns', JSON.stringify(newSettings));
    } catch (e) {
      console.error('Failed to save column settings', e);
    }
  };

  // 处理列显示变化
  const handleColumnVisibilityChange = (key, checked) => {
    const newSettings = {...visibleColumns, [key]: checked};
    setVisibleColumns(newSettings);
    saveColumnSettings(newSettings);
  };

  // 表格列配置
  const columns = [{
    title: '序号', key: 'index', width: 60, render: (_, __, index) => index + 1, hidden: !visibleColumns['index']
  }, {
    title: '订阅链接',
    dataIndex: 'subscription_url',
    key: 'subscription_url',
    width: 300,
    ellipsis: true,
    hidden: !visibleColumns['subscription_url']
  }, {
    title: '名称', dataIndex: 'name', key: 'name', width: 200, ellipsis: true, hidden: !visibleColumns['name']
  }, {
    title: '节点', dataIndex: 'address', key: 'address', width: 200, hidden: !visibleColumns['address']
  }, {
    title: '节点类型',
    dataIndex: 'type',
    key: 'type',
    width: 120,
    ellipsis: true,
    sorter: true,
    filterMode: 'tree',
    filters: nodeTypes.length > 0 ? nodeTypes : [{text: 'vmess', value: 'vmess'}, {text: 'vless', value: 'vless'}],
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
    render: (rate) => `${rate}%`,
    width: 80,
    hidden: !visibleColumns['success_rate']
  }, {
    title: <Tooltip
      title="风险等级由IPV4及IPV6分别计算，优先展示IPV4的风险等级，可能出现筛选低风险但出现高风险情况"><span>风险等级 <InfoCircleOutlined/></span></Tooltip>,
    dataIndex: ['ip_info', 'risk'],
    key: 'risk',
    width: 110,
    render: (risk) => formatRisk(risk),
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
    render: (country) => country ? country : "-",
    filterMode: 'tree',
    filters: countryCodes.length > 0 ? countryCodes : [],
    filterMultiple: true,
    hidden: !visibleColumns['country_code']
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
          onClick={() => handleViewNode(record)}
        />
      </Tooltip>
      <Tooltip title="测速">
        <Button
          type="text"
          icon={<ReloadOutlined/>}
          onClick={() => handleTestProxy(record.id)}
        />
      </Tooltip>
      <Tooltip title="检测IP">
        <Button
          type="text"
          icon={<InfoCircleOutlined/>}
          onClick={() => handleDetectIP(record.id)}
        />
      </Tooltip>
      <Tooltip title={record.pinned ? "取消置顶" : "置顶"}>
        <Button
          type="text"
          icon={record.pinned ? <PushpinFilled/> : <PushpinOutlined/>}
          onClick={() => handlePinProxy(record.id, record.pinned)}
        />
      </Tooltip>
      <Tooltip title="禁用">
        <Button
          type="text"
          icon={<DeleteOutlined/>}
          onClick={() => handleBanProxy(record.id)}
        />
      </Tooltip>
    </div>),
    hidden: !visibleColumns['action']
  }].filter(column => !column.hidden);

  // 列设置菜单
  const columnSettingMenu = allColumns.map(column => ({
    key: column.key, label: (<Checkbox
      checked={visibleColumns[column.key] ?? defaultVisibleColumns.includes(column.key)}
      onChange={e => handleColumnVisibilityChange(column.key, e.target.checked)}
      disabled={!column.hideable}
      onClick={(e) => e.stopPropagation()}
    >
      {column.title}
    </Checkbox>)
  }));

  return (<div>
    <Tabs
      activeKey={activeTab}
      onChange={setActiveTab}
      tabBarExtraContent={<div className="tab-bar-extra"
                               style={{display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap'}}>
        {taskStatus && taskStatus.State === 0 && (<div style={{display: 'flex', alignItems: 'center'}}>
          <Progress
            type="circle"
            percent={Math.round((taskStatus.Completed / taskStatus.Total) * 100)}
            size="small"
            style={{marginRight: 8}}
          />
          <span style={{marginRight: 8}}>
            测速进行中: {taskStatus.Completed}/{taskStatus.Total}
          </span>
          <Button
            type="primary"
            danger
            icon={<StopOutlined/>}
            onClick={handleStopTask}
            style={{margin: 0}}
          >
            停止任务
          </Button>
        </div>)}
        <Button
          type="primary"
          danger
          onClick={() => handleBanProxy(null)}
          style={{margin: 0}}
        >
          批量禁用节点
        </Button>
        <Button
          type="primary"
          onClick={() => handleTestProxy(null)}
          style={{margin: 0}}
        >
          按当前参数进行测速
        </Button>
        <Button
          type="primary"
          onClick={handleExportSubscriptionUrl}
          style={{margin: 0}}
        >
          按当前参数导出订阅链接
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
      </div>}
    >
      <Tabs.TabPane tab="所有节点" key="2">
        <div style={{overflowX: 'auto', width: '100%'}}>
          <Table
            columns={columns}
            dataSource={nodes}
            rowKey="id"
            loading={loading}
            pagination={{
              ...pagination,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 条记录`,
              pageSizeOptions: ['10', '20', '50', '100']
            }}
            onChange={handleTableChange}
            scroll={{x: 1500}}
            style={{width: '100%'}}
          />
        </div>
      </Tabs.TabPane>
    </Tabs>

    {/* 节点详情弹窗，这两个弹窗写的跟shit一样 */}
    <Modal
      title="节点详情"
      open={modalVisible}
      onCancel={() => setModalVisible(false)}
      footer={[<Button key="close" type="primary" onClick={() => setModalVisible(false)}>
        关闭
      </Button>,]}
      width={800}
    >
      {currentNode && (<div>
        <Card title="基本信息" style={{marginBottom: 5}}>
          <InfoItem label="名称" value={currentNode.name || '未命名'}/>
          <InfoItem label="订阅链接" value={currentNode.subscription_url}/>
          <InfoItem label="地址" value={currentNode.address}/>
          <InfoItem label="节点类型" value={currentNode.type}/>
          <InfoItem label="状态" value={<StatusTag status={currentNode.status}/>}/>
          <InfoItem label="Ping" value={currentNode.ping ? `${currentNode.ping}ms` : '-'}/>
          <InfoItem label="下载速度" value={formatSpeed(currentNode.download_speed)}/>
          <InfoItem label="上传速度" value={formatSpeed(currentNode.upload_speed)}/>
          <InfoItem label="节点创建时间" value={formatDate(currentNode.created_at)}/>
          <InfoItem label="最近测试时间" value={formatDate(currentNode.latest_test_time)}/>
          <InfoItem label="分享链接" value={currentNode.share_url} isMultiLine={true}/>
          <InfoItem label="总计下载流量" value={formatTraffic(currentNode.download_total)}/>
          <InfoItem label="总计上传流量" value={formatTraffic(currentNode.upload_total)}/>
          {currentNode.ip_info?.ipv4 && (<InfoItem label="IPV4地址" value={currentNode.ip_info?.ipv4}/>)}
          {currentNode.ip_info?.ipv6 && (<InfoItem label="IPV6地址" value={currentNode.ip_info?.ipv6}/>)}
          {currentNode.ip_info?.risk && (<InfoItem label="风险等级" value={currentNode.ip_info?.risk}/>)}
          {currentNode.ip_info?.country_code && (
            <InfoItem label="国家/地区代码" value={currentNode.ip_info?.country_code}/>)}
        </Card>

        {currentNode?.ip_info?.app_unlock && currentNode.ip_info?.app_unlock.length > 0 && (
          <Card title="应用解锁" style={{marginBottom: 5}}>
            <Table
              columns={[{
                title: '应用名称', dataIndex: 'app_name', key: 'app_name'
              }, {
                title: '解锁状态',
                dataIndex: 'status',
                key: 'status',
                render: (status) => <AppUnlockStatusTag status={status}/>
              }, {
                title: '地区', dataIndex: 'region', key: 'region', render: (region) => region || '-'
              }]}
              dataSource={currentNode.ip_info?.app_unlock || []}
              pagination={false}
            />
          </Card>)}

        {nodeHistory && nodeHistory.length > 0 && <Card title="历史记录" style={{marginBottom: 5}}>
          <Table
            columns={[{
              title: '测试时间', dataIndex: 'tested_at', key: 'tested_at', render: (text) => formatDate(text)
            }, {
              title: 'Ping', dataIndex: 'ping', key: 'ping', render: (ping) => ping ? `${ping}ms` : '-'
            }, {
              title: '下载速度',
              dataIndex: 'download_speed',
              key: 'download_speed',
              render: (speed) => formatSpeed(speed)
            }, {
              title: '上传速度', dataIndex: 'upload_speed', key: 'upload_speed', render: (speed) => formatSpeed(speed)
            },]}
            dataSource={nodeHistory}
            rowKey="id"
            loading={historyLoading}
            pagination={{
              ...historyPagination,
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 条记录`,
              pageSizeOptions: ['5', '10', '20']
            }}
            onChange={handleHistoryTableChange}
          />
        </Card>}
      </div>)}
    </Modal>
  </div>);
};

export default NodesPage;