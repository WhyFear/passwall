import {
  DeleteOutlined,
  EditOutlined,
  LinkOutlined
} from '@ant-design/icons';
import {
  Alert,
  Button,
  Card,
  Form,
  Input,
  InputNumber,
  message,
  Modal,
  Select,
  Space,
  Switch,
  Table,
  Tabs,
  Tag
} from 'antd';
import {useCallback, useEffect, useRef, useState} from 'react';
import {nodeApi, shareConfigApi, subscriptionApi} from '../api';
import {fetchTaskStatus, stopTask} from '../utils/taskUtils';
import {formatDate} from '../utils/timeUtils';
import {DEFAULT_VISIBLE_COLUMNS, formatRisk} from './nodes/nodeFormatters';
import NodeBatchActions from './nodes/NodeBatchActions';
import NodeDetailModal from './nodes/NodeDetailModal';
import {createColumnSettingMenu, createNodeColumns} from './nodes/nodeColumns';
import {buildShareDefaults, buildSharePayload, normalizeShareMultiValue} from './nodes/shareConfigUtils';
import {DEFAULT_NODE_PAGINATION, DEFAULT_NODE_SORTER, useNodesQuery} from './nodes/useNodesQuery';

const NodesPage = () => {
  const [shareForm] = Form.useForm();
  const [quickWakeForm] = Form.useForm();
  const [activeTab, setActiveTab] = useState('2');
  const [modalVisible, setModalVisible] = useState(false);
  const [currentNode, setCurrentNode] = useState(null);
  const [nodeHistory, setNodeHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [historyPagination, setHistoryPagination] = useState({
    current: 1, pageSize: 5, total: 0,
  });
  const [nodeTypes, setNodeTypes] = useState([]);
  const [taskStatus, setTaskStatus] = useState(null);
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 600);
  const [countryCodes, setCountryCodes] = useState([]);
  const [unlockApps, setUnlockApps] = useState([]);
  const [shareModalVisible, setShareModalVisible] = useState(false);
  const [shareConfigs, setShareConfigs] = useState([]);
  const [shareLoading, setShareLoading] = useState(false);
  const [quickWakeModalVisible, setQuickWakeModalVisible] = useState(false);
  const [quickWakeLoading, setQuickWakeLoading] = useState(false);
  const [quickWakeTaskStatus, setQuickWakeTaskStatus] = useState(null);
  const [editingShareConfig, setEditingShareConfig] = useState(null);
  const [visibleColumns, setVisibleColumns] = useState({});
  const nodeMetadataIncludes = visibleColumns.risk || visibleColumns.country_code || visibleColumns.app_unlock
    ? ['success_rate', 'ip_info']
    : ['success_rate'];
  const {
    nodes,
    loading,
    pagination,
    sorter,
    filters,
    fetchNodes,
    handleTableChange,
  } = useNodesQuery(subscriptionApi, {metadataIncludes: nodeMetadataIncludes});

  const timerRef = useRef(null);

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

  const fetchNodeDetails = async (nodeId) => {
    try {
      const data = await nodeApi.getProxyDetails(nodeId);
      setCurrentNode(prev => {
        if (!prev || prev.id !== nodeId) return prev;
        return {
          ...prev,
          download_total: data?.traffic?.download_total,
          upload_total: data?.traffic?.upload_total,
          ip_info: data?.ip_info || prev.ip_info,
        };
      });
    } catch (error) {
      message.error('获取节点详情失败');
      console.error(error);
    }
  };

  // 获取节点类型
  const fetchNodeTypes = useCallback(async () => {
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
  }, []);

  // 获取国家代码
  const fetchCountryCodes = useCallback(async () => {
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
      setCountryCodes([]);
      console.error(error);
    }
  }, []);

  const fetchUnlockApps = useCallback(async () => {
    try {
      const data = await nodeApi.getUnlockApps();
      if (Array.isArray(data?.data)) {
        setUnlockApps(data.data.map(app => ({
          text: app, value: app
        })));
      }
    } catch (error) {
      setUnlockApps([]);
      console.error(error);
    }
  }, []);
  // 获取任务状态
  const fetchTaskStatusHandler = useCallback(async () => {
    await fetchTaskStatus("speed_test", setTaskStatus);
  }, []);
  const fetchQuickWakeTaskStatusHandler = useCallback(async () => {
    await fetchTaskStatus("quick_wake", setQuickWakeTaskStatus);
  }, []);

  // 停止任务
  const handleStopTask = async () => {
    await stopTask("speed_test", setTaskStatus);
  };
  const handleStopQuickWake = async () => {
    await stopTask("quick_wake", setQuickWakeTaskStatus);
  };

  // 启动定时器
  useEffect(() => {
    // 初始获取一次任务状态
    fetchTaskStatusHandler();
    fetchQuickWakeTaskStatusHandler();
    fetchNodes(DEFAULT_NODE_PAGINATION.current, DEFAULT_NODE_PAGINATION.pageSize, DEFAULT_NODE_SORTER, {});
    fetchNodeTypes();
    fetchCountryCodes();
    fetchUnlockApps();

    // 设置定时器，每3秒获取一次任务状态
    timerRef.current = setInterval(() => {
      fetchTaskStatusHandler();
      fetchQuickWakeTaskStatusHandler();
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
  }, [fetchNodes, fetchNodeTypes, fetchCountryCodes, fetchUnlockApps, fetchTaskStatusHandler, fetchQuickWakeTaskStatusHandler]);


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
    DEFAULT_VISIBLE_COLUMNS.forEach(key => {
      initialColumns[key] = true;
    });
    setVisibleColumns(initialColumns);
  }, []);

  // 处理历史记录表格分页变化
  const handleHistoryTableChange = (newPagination) => {
    if (currentNode) {
      fetchNodeHistory(currentNode.id, newPagination.current, newPagination.pageSize);
    }
  };

  const getShareUrl = (slug) => `${window.location.origin}/s/${slug}`;

  const copyText = async (text, successMessage = '已复制到剪贴板') => {
    if (window.isSecureContext && navigator.clipboard) {
      await navigator.clipboard.writeText(text);
      message.success(successMessage);
      return;
    }

    const textArea = document.createElement("textarea");
    textArea.value = text;
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    try {
      document.execCommand('copy');
      message.success(successMessage);
    } catch (err) {
      message.error('复制失败，请手动复制\n' + text);
    } finally {
      document.body.removeChild(textArea);
    }
  };

  const fetchShareConfigs = async () => {
    try {
      setShareLoading(true);
      const data = await shareConfigApi.list();
      setShareConfigs(Array.isArray(data) ? data : []);
    } catch (error) {
      message.error('获取分享配置失败');
      console.error(error);
    } finally {
      setShareLoading(false);
    }
  };

  const openShareModal = async () => {
    setEditingShareConfig(null);
    shareForm.setFieldsValue(buildShareDefaults(filters, sorter));
    setShareModalVisible(true);
    await fetchShareConfigs();
  };

  const handleEditShareConfig = (record) => {
    setEditingShareConfig(record);
    shareForm.setFieldsValue({
      ...record,
      status: normalizeShareMultiValue(record.status),
      proxy_type: normalizeShareMultiValue(record.proxy_type),
      country_code: normalizeShareMultiValue(record.country_code),
      risk_level: normalizeShareMultiValue(record.risk_level),
      app_unlock: normalizeShareMultiValue(record.app_unlock),
    });
  };

  const handleResetShareForm = () => {
    setEditingShareConfig(null);
    shareForm.setFieldsValue(buildShareDefaults(filters, sorter));
  };

  const handleSaveShareConfig = async () => {
    try {
      const values = await shareForm.validateFields();
      const payload = buildSharePayload(values);

      let saved;
      if (editingShareConfig) {
        saved = await shareConfigApi.update(editingShareConfig.id, payload);
        message.success('分享配置已更新');
      } else {
        saved = await shareConfigApi.create(payload);
        message.success('分享链接已创建');
      }
      await fetchShareConfigs();
      setEditingShareConfig(saved);
      if (saved?.slug) {
        await copyText(getShareUrl(saved.slug), '分享链接已复制到剪贴板');
      }
    } catch (error) {
      if (error?.errorFields) return;
      message.error('保存分享配置失败: ' + (error?.message || '未知错误'));
      console.error(error);
    }
  };

  const handleEnableShareConfig = async (record) => {
    try {
      await shareConfigApi.update(record.id, buildSharePayload(record, true));
      message.success('分享链接已启用');
      await fetchShareConfigs();
    } catch (error) {
      message.error('启用分享链接失败');
      console.error(error);
    }
  };

  const handleDisableShareConfig = async (record) => {
    try {
      await shareConfigApi.disable(record.id);
      message.success('分享链接已失效');
      await fetchShareConfigs();
    } catch (error) {
      message.error('失效分享链接失败');
      console.error(error);
    }
  };

  const handleDeleteShareConfig = async (record) => {
    Modal.confirm({
      title: '删除分享链接',
      content: `确认删除「${record.name}」？删除后短链将不可访问。`,
      okText: '删除',
      okButtonProps: {danger: true},
      cancelText: '取消',
      onOk: async () => {
        try {
          await shareConfigApi.delete(record.id);
          message.success('分享链接已删除');
          if (editingShareConfig?.id === record.id) {
            handleResetShareForm();
          }
          await fetchShareConfigs();
        } catch (error) {
          message.error('删除分享链接失败');
          console.error(error);
        }
      }
    });
  };

  const shareConfigColumns = [{
    title: '名称',
    dataIndex: 'name',
    key: 'name',
  }, {
    title: '类型',
    dataIndex: 'type',
    key: 'type',
  }, {
    title: '状态',
    dataIndex: 'enabled',
    key: 'enabled',
    render: (enabled) => enabled ? <Tag color="success">启用</Tag> : <Tag color="default">已失效</Tag>,
  }, {
    title: '更新时间',
    dataIndex: 'updated_at',
    key: 'updated_at',
    render: (time) => time ? formatDate(time) : '-',
  }, {
    title: '操作',
    key: 'action',
    width: 260,
    render: (_, record) => (<Space wrap>
      <Button size="small" icon={<LinkOutlined/>} onClick={() => copyText(getShareUrl(record.slug), '分享链接已复制到剪贴板')}>
        复制
      </Button>
      <Button size="small" icon={<EditOutlined/>} onClick={() => handleEditShareConfig(record)}>
        编辑
      </Button>
      {record.enabled ? (
        <Button size="small" onClick={() => handleDisableShareConfig(record)}>
          失效
        </Button>
      ) : (
        <Button size="small" onClick={() => handleEnableShareConfig(record)}>
          启用
        </Button>
      )}
      <Button size="small" danger icon={<DeleteOutlined/>} onClick={() => handleDeleteShareConfig(record)}>
        删除
      </Button>
    </Space>),
  }];

  // 打开分享配置管理
  const handleExportSubscriptionUrl = async () => {
    try {
      await openShareModal();
    } catch (error) {
      message.error('打开分享管理失败');
      console.error(error);
    }
  };

  const openQuickWakeModal = () => {
    quickWakeForm.setFieldsValue({
      concurrent: 50,
      type: [],
    });
    setQuickWakeModalVisible(true);
  };

  const handleQuickWake = async () => {
    try {
      const values = await quickWakeForm.validateFields();
      setQuickWakeLoading(true);
      const data = await nodeApi.quickWakeProxies({
        concurrent: values.concurrent || 50,
        type: values.type || [],
      });
      if (data.status_code === 200 && data.result === 'success') {
        message.success('快速唤醒任务已启动');
        setQuickWakeModalVisible(false);
        setTimeout(() => {
          fetchQuickWakeTaskStatusHandler();
        }, 500);
      } else {
        message.error('快速唤醒失败：' + data.status_msg);
      }
    } catch (error) {
      if (error?.errorFields) return;
      message.error('快速唤醒失败：' + error.message);
      console.error(error);
    } finally {
      setQuickWakeLoading(false);
    }
  };

  const handleTestProxy = async (nodeId) => {
    try {
      const params = {};

      if (filters.status && Array.isArray(filters.status) && filters.status.length > 0) {
        params.status = filters.status.join(',');
      }
      if (filters.type && Array.isArray(filters.type) && filters.type.length > 0) {
        params.type = filters.type.join(',');
      }
      if (filters.country && filters.country.length > 0) {
        params.country_code = filters.country.join(',');
      }
      if (filters.risk && filters.risk.length > 0) {
        params.risk_level = filters.risk.join(',');
      }
      if (filters.app_unlock && filters.app_unlock.length > 0) {
        params.app_unlock = filters.app_unlock.join(',');
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
    fetchNodeDetails(node.id);
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

  const columns = createNodeColumns({
    visibleColumns,
    nodeTypes,
    countryCodes,
    unlockApps,
    isMobile,
    onViewNode: handleViewNode,
    onTestProxy: handleTestProxy,
    onDetectIP: handleDetectIP,
    onPinProxy: handlePinProxy,
    onBanProxy: handleBanProxy,
  });

  const columnSettingMenu = createColumnSettingMenu({
    visibleColumns,
    onColumnVisibilityChange: handleColumnVisibilityChange,
  });

  return (<div>
    <Tabs
      activeKey={activeTab}
      onChange={setActiveTab}
      tabBarExtraContent={<NodeBatchActions
        taskStatus={taskStatus}
        quickWakeTaskStatus={quickWakeTaskStatus}
        onStopTask={handleStopTask}
        onStopQuickWake={handleStopQuickWake}
        onBanProxy={handleBanProxy}
        onTestProxy={handleTestProxy}
        onExportSubscriptionUrl={handleExportSubscriptionUrl}
        onQuickWake={openQuickWakeModal}
        columnSettingMenu={columnSettingMenu}
      />}
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

    <Modal
      title="分享管理"
      open={shareModalVisible}
      onCancel={() => setShareModalVisible(false)}
      footer={null}
      width={1100}
    >
      <Card
        title={editingShareConfig ? '编辑分享配置' : '创建分享配置'}
        size="small"
        style={{marginBottom: 16}}
        extra={<Space>
          <Button onClick={handleResetShareForm}>使用当前筛选新建</Button>
          <Button type="primary" onClick={handleSaveShareConfig} loading={shareLoading}>
            {editingShareConfig ? '保存修改' : '创建并复制链接'}
          </Button>
        </Space>}
      >
        <Form form={shareForm} layout="vertical">
          <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: 12}}>
            <Form.Item name="name" label="名称" rules={[{required: true, message: '请输入名称'}]}>
              <Input placeholder="节点分享"/>
            </Form.Item>
            <Form.Item name="type" label="订阅类型" rules={[{required: true, message: '请选择订阅类型'}]}>
              <Select options={[
                {label: '分享链接', value: 'share_link'},
                {label: 'Clash', value: 'clash'},
              ]}/>
            </Form.Item>
            <Form.Item name="limit" label="返回数量" help="0 表示不限制">
              <InputNumber min={0} style={{width: '100%'}}/>
            </Form.Item>
            <Form.Item name="sort" label="排序字段">
              <Select allowClear options={[
                {label: '下载速度', value: 'download_speed'},
                {label: '上传速度', value: 'upload_speed'},
                {label: '延迟', value: 'ping'},
                {label: '最近测试时间', value: 'latest_test_time'},
                {label: '创建时间', value: 'created_at'},
                {label: 'ID', value: 'id'},
              ]}/>
            </Form.Item>
            <Form.Item name="sort_order" label="排序方向">
              <Select options={[
                {label: '降序', value: 'descend'},
                {label: '升序', value: 'ascend'},
              ]}/>
            </Form.Item>
            <Form.Item name="status" label="状态">
              <Select mode="multiple" allowClear options={[
                {label: '未测试', value: '-1'},
                {label: '正常', value: '1'},
                {label: '失败', value: '2'},
                {label: '未知错误', value: '3'},
              ]}/>
            </Form.Item>
            <Form.Item name="proxy_type" label="节点类型">
              <Select mode="multiple" allowClear options={nodeTypes.map(item => ({label: item.text, value: item.value}))}/>
            </Form.Item>
            <Form.Item name="country_code" label="国家/地区">
              <Select
                mode="multiple"
                allowClear
                showSearch
                options={countryCodes.map(item => ({label: item.text, value: item.value}))}
              />
            </Form.Item>
            <Form.Item name="risk_level" label="风险等级">
              <Select mode="multiple" allowClear options={[
                {label: formatRisk('very_low'), value: 'very_low'},
                {label: formatRisk('low'), value: 'low'},
                {label: formatRisk('medium'), value: 'medium'},
                {label: formatRisk('high'), value: 'high'},
                {label: formatRisk('very_high'), value: 'very_high'},
              ]}/>
            </Form.Item>
            <Form.Item name="app_unlock" label="App 解锁">
              <Select
                mode="multiple"
                allowClear
                showSearch
                options={unlockApps.map(item => ({label: item.text, value: item.value}))}
              />
            </Form.Item>
            <Form.Item name="with_index" label="节点名前加序号" valuePropName="checked">
              <Switch/>
            </Form.Item>
          </div>
        </Form>
      </Card>

      <Table
        columns={shareConfigColumns}
        dataSource={shareConfigs}
        rowKey="id"
        loading={shareLoading}
        pagination={{pageSize: 5}}
        scroll={{x: 900}}
      />
    </Modal>

    <Modal
      title="快速唤醒"
      open={quickWakeModalVisible}
      onCancel={() => setQuickWakeModalVisible(false)}
      onOk={handleQuickWake}
      confirmLoading={quickWakeLoading}
      okText="开始唤醒"
      cancelText="取消"
      width={560}
    >
      <Alert
        type="info"
        showIcon
        message="快速唤醒只检测已封禁节点。延迟探测成功后，节点状态会从已封禁改为待测试。"
        style={{marginBottom: 16}}
      />
      <Form form={quickWakeForm} layout="vertical">
        <Form.Item
          name="concurrent"
          label="线程数"
          rules={[{required: true, message: '请输入线程数'}]}
        >
          <InputNumber min={1} precision={0} style={{width: '100%'}}/>
        </Form.Item>
        <Form.Item name="type" label="节点类型" help="不选择任何类型时，默认处理全部类型。">
          <Select
            mode="multiple"
            allowClear
            options={nodeTypes.map(item => ({label: item.text, value: item.value}))}
          />
        </Form.Item>
      </Form>
    </Modal>

    <NodeDetailModal
      open={modalVisible}
      node={currentNode}
      nodeHistory={nodeHistory}
      historyLoading={historyLoading}
      historyPagination={historyPagination}
      onClose={() => setModalVisible(false)}
      onHistoryTableChange={handleHistoryTableChange}
    />
  </div>);
};

export default NodesPage;
