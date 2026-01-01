import React, {useEffect, useRef, useState} from 'react';
import {Button, Form, message, Modal, Progress, Switch, Table, Tabs, Tag, Tooltip,} from 'antd';
import {
  DeleteOutlined,
  EyeOutlined,
  PlusOutlined,
  ReloadOutlined,
  SettingOutlined,
  StopOutlined
} from '@ant-design/icons';
import {configApi, subscriptionApi} from '../api';
import {fetchTaskStatus, stopTask} from '../utils/taskUtils';
import SubscriptionForm from '../components/SubscriptionForm';
import StatusTag from '../components/StatusTag';
import IntervalSelector from '../components/IntervalSelector';
import {parseCronToSimple} from '../utils/cronUtils';
import {formatDate} from "../utils/timeUtils";

const SubscriptionPage = () => {
  const [subscriptions, setSubscriptions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [modalType, setModalType] = useState('add'); // 'add' 或 'view'
  const [currentSubscription, setCurrentSubscription] = useState(null);
  const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState('1');
  const [taskStatus, setTaskStatus] = useState(null);
  const timerRef = useRef(null);
  const [uploadType, setUploadType] = useState('url');
  const [isMobile, setIsMobile] = useState(window.innerWidth <= 600);
  const [pagination, setPagination] = useState({
    current: 1, pageSize: 10, total: 0,
  });
  const [deletingIds, setDeletingIds] = useState([]);
  const [configModalVisible, setConfigModalVisible] = useState(false);
  const [configLoading, setConfigLoading] = useState(false);
  const [configForm] = Form.useForm();
  const [isCustomConfig, setIsCustomConfig] = useState(false);
  const [intervalMode, setIntervalMode] = useState('simple'); // 'simple' or 'advanced'

  // 获取订阅列表
  const fetchSubscriptions = async (page = pagination.current, pageSize = pagination.pageSize) => {
    try {
      setLoading(true);
      // 构建请求参数
      const params = {
        page: page, pageSize: pageSize
      };

      const data = await subscriptionApi.getSubscriptions({params});
      const items = Array.isArray(data.items) ? data.items : [];

      setSubscriptions(items);
      setPagination(prev => ({
        ...prev, current: page, pageSize: pageSize, total: data.total || items.length,
      }));
    } catch (error) {
      message.error(`获取订阅列表失败: ${error.message || '未知错误'}`);
      console.error(error);
      // 重置到第一页
      setPagination(prev => ({...prev, current: 1}));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSubscriptions()
    fetchTaskStatusHandler();
    // 设置定时器，每3秒获取一次任务状态
    timerRef.current = setInterval(() => {
      fetchTaskStatusHandler();
    }, 3000);

    // 组件卸载时清除定时器
    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, []);

  useEffect(() => {
    const handleResize = () => {
      setIsMobile(window.innerWidth <= 600);
    };
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // 获取任务状态
  const fetchTaskStatusHandler = async () => {
    await fetchTaskStatus("reload_subs", setTaskStatus);
  };

  // 停止任务
  const handleStopTask = async () => {
    await stopTask("reload_subs", setTaskStatus);
  };

  // 打开配置弹窗
  const handleOpenConfig = async (record) => {
    setCurrentSubscription(record);
    setConfigLoading(true);
    try {
      const data = await subscriptionApi.getSubscriptionConfig(record.id);
      let interval = "";
      if (data.is_custom) {
        configForm.setFieldsValue({
          auto_update: data.auto_update, update_interval: data.update_interval, use_proxy: data.use_proxy,
        });
        interval = data.update_interval;
        setIsCustomConfig(true);
      } else {
        // 如果不是自定义配置，拉取全局默认配置
        const globalConfig = await configApi.getConfig();
        const defaultSub = globalConfig.default_sub || {};
        configForm.setFieldsValue({
          auto_update: defaultSub.auto_update, update_interval: defaultSub.interval, use_proxy: defaultSub.use_proxy,
        });
        interval = defaultSub.interval;
        setIsCustomConfig(false);
      }

      const {mode, value, unit} = parseCronToSimple(interval);
      setIntervalMode(mode);
      configForm.setFieldsValue({
        simple_interval_value: value, simple_interval_unit: unit
      });

      setConfigModalVisible(true);
    } catch (error) {
      message.error('获取配置失败: ' + error.message);
    } finally {
      setConfigLoading(false);
    }
  };

  // 保存配置
  const handleSaveConfig = async () => {
    try {
      const values = await configForm.validateFields();
      setConfigLoading(true);
      await subscriptionApi.saveSubscriptionConfig(currentSubscription.id, values);
      message.success('配置已保存');
      setConfigModalVisible(false);
    } catch (error) {
      if (!error.errorFields) {
        message.error('保存配置失败: ' + error.message);
      }
    } finally {
      setConfigLoading(false);
    }
  };

  // 恢复默认配置
  const handleRestoreDefault = async () => {
    try {
      setConfigLoading(true);
      // 获取全局配置
      const globalConfig = await configApi.getConfig();
      const defaultSub = globalConfig.default_sub || {};

      const values = {
        auto_update: defaultSub.auto_update, update_interval: defaultSub.interval, use_proxy: defaultSub.use_proxy,
      };

      await subscriptionApi.saveSubscriptionConfig(currentSubscription.id, values);
      message.success('已恢复为默认配置');
      setConfigModalVisible(false);
    } catch (error) {
      message.error('恢复默认配置失败: ' + error.message);
    } finally {
      setConfigLoading(false);
    }
  };

  // 测试代理
  const handleReloadSubs = async (nodeId) => {
    try {
      const params = {};
      if (nodeId) {
        params.id = nodeId;
      }
      const data = await subscriptionApi.reloadSubs(params);
      if (data.status_code === 200) {
        message.success('任务已启动');
        // 立即获取一次任务状态
        setTimeout(() => {
          fetchTaskStatusHandler();
        }, 500);
      } else {
        message.error('任务执行失败：' + data.status_msg);
      }
    } catch (error) {
      message.error('任务执行失败：' + error.message);
      console.error(error);
    }
  };

  // 监听表单字段变化
  const handleFormValuesChange = (changedValues) => {
    if ('upload_type' in changedValues) {
      setUploadType(changedValues.upload_type);
    }
  };

  // 添加订阅
  const handleAddSubscription = () => {
    setModalType('add');
    setCurrentSubscription(null);
    form.resetFields();
    setUploadType('url');
    form.setFieldsValue({upload_type: 'url', type: 'auto'});
    setModalVisible(true);
  };

  // 查看订阅详情
  const handleViewSubscription = (record) => {
    setModalType('view');
    setCurrentSubscription(record);
    setLoading(true);

    // 调用API获取订阅详情，包含content内容
    subscriptionApi.getSubscriptionDetail(record.id, true)
      .then(data => {
        if (data && data.total > 0) {
          // 使用返回的详细数据
          setCurrentSubscription(data.items[0]);
          form.setFieldsValue(data.items[0]);
        } else {
          message.error('获取订阅详情失败:' + (data.status_msg || '未知错误'));
          // 如果没有返回数据，使用原始记录
          form.setFieldsValue(record);
        }
        setModalVisible(true);
      })
      .catch(error => {
        message.error('获取订阅详情失败');
        console.error(error);
        // 失败时使用原始记录
        form.setFieldsValue(record);
        setModalVisible(true);
      })
      .finally(() => {
        setLoading(false);
      });
  };

  // 提交表单
  const handleSubmit = async () => {
    try {
      const values = await form.validateFields();
      setLoading(true);
      let data

      // 处理表单提交
      if (values.upload_type === 'file' && values.content) {
        // 使用FormData处理文件上传
        const formData = new FormData();
        formData.append('type', values.type);
        formData.append('upload_type', values.upload_type);

        // 将内容转为文件
        const contentBlob = new Blob([values.content], {type: 'text/plain'});
        formData.append('file', contentBlob, 'subscription.txt');

        data = await subscriptionApi.createProxyWithFormData(formData);
      } else if (values.upload_type === 'url_list' && values.url_list_text) {
        // 处理批量URL列表
        const urlList = values.url_list_text
          .split('\n')
          .map(url => url.trim())
          .filter(url => url !== '');

        if (urlList.length === 0) {
          message.error('请输入有效的链接');
          setLoading(false);
          return;
        }

        if (urlList.length > 50) {
          message.error('单次最多支持 50 个订阅链接，请分批导入');
          setLoading(false);
          return;
        }

        data = await subscriptionApi.createProxy({
          type: values.type, upload_type: values.upload_type, url_list: urlList
        });
      } else {
        // 处理URL提交
        data = await subscriptionApi.createProxy({
          type: values.type, upload_type: values.upload_type, url: values.url || '', content: values.content || ''
        });
      }
      if (data.result === 'fail') {
        message.error(`添加订阅失败: ${data?.status_msg || '未知错误'}`);
        return;
      }

      const successMsg = values.upload_type === 'url_list' ? '批量导入任务已启动' : '添加订阅成功';
      message.success(successMsg);
      setModalVisible(false);
      await fetchSubscriptions(pagination.current, pagination.pageSize);
    } catch (error) {
      if (error.errorFields) {
        message.error('请填写必填字段');
      } else {
        message.error(`添加订阅失败: ${error.message || '未知错误'}`);
        console.error(error);
      }
    } finally {
      setLoading(false);
    }
  };

  async function handleDeleteSubs(id) {
    try {
      setDeletingIds(prev => [...prev, id]);
      const data = await subscriptionApi.deleteSubscription(id);
      if (data.status_code === 200) {
        message.success('删除订阅成功');
        await fetchSubscriptions(pagination.current, pagination.pageSize);
      } else {
        message.error('删除订阅失败：' + data.result);
      }
    } catch (error) {
      message.error('删除订阅失败：' + error.message);
      console.error(error);
    } finally {
      setDeletingIds(prev => prev.filter(deletingId => deletingId !== id));
    }
  }

  // 表格列配置
  const columns = [{
    title: '序号', key: 'index', width: 80, render: (_, __, index) => index + 1,
  }, {
    title: '链接', dataIndex: 'url', key: 'url', width: 400, ellipsis: true,
  }, {
    title: '上次拉取状态',
    dataIndex: 'status',
    key: 'status',
    width: 120,
    render: (status) => <StatusTag status={status}/>,
  }, {
    title: <Tooltip title="节点测速结果为正常的节点"><span>状态正常节点数量</span></Tooltip>,
    dataIndex: 'ok_proxy_num',
    key: 'ok_proxy_num',
    width: 140,
    render: (ok_proxy_num) => (ok_proxy_num) ? ok_proxy_num : '-',
  }, {
    title: <Tooltip title="未被禁用的节点数量"><span>生效节点数量</span></Tooltip>,
    dataIndex: 'proxy_num',
    key: 'proxy_num',
    width: 120,
    render: (proxy_num) => (proxy_num) ? proxy_num : '-',
  }, {
    title: <Tooltip title="所有节点数量，包含被禁用的节点"><span>所有节点数量</span></Tooltip>,
    dataIndex: 'all_proxy_num',
    key: 'all_proxy_num',
    width: 120,
    render: (all_proxy_num) => (all_proxy_num) ? all_proxy_num : '-',
  }, {
    title: '上次更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180, render: (text) => formatDate(text),
  }, {
    title: '添加时间', dataIndex: 'created_at', key: 'created_at', width: 180, render: (text) => formatDate(text),
  }, {
    title: '操作', key: 'action', width: 350, fixed: isMobile ? undefined : 'right', render: (_, record) => (<div>
      <Tooltip title="查看内容">
        <Button
          type="text"
          icon={<EyeOutlined/>}
          onClick={() => handleViewSubscription(record)}
        >
          查看
        </Button>
      </Tooltip>
      <Tooltip title="刷新订阅">
        <Button
          type="text"
          icon={<ReloadOutlined/>}
          onClick={() => handleReloadSubs(record.id)}
        >
          刷新
        </Button>
      </Tooltip>
      <Tooltip title="自定义更新配置">
        <Button
          type="text"
          icon={<SettingOutlined/>}
          disabled={!record.url || !record.url.startsWith('http')}
          onClick={() => handleOpenConfig(record)}
        >
          配置
        </Button>
      </Tooltip>
      <Tooltip title="删除订阅">
        <Button
          type="text"
          icon={<DeleteOutlined/>}
          onClick={() => handleDeleteSubs(record.id)}
          loading={deletingIds.includes(record.id)}
        >
          删除
        </Button>
      </Tooltip>
    </div>),
  },];

  return (<div>
    <Tabs
      activeKey={activeTab}
      onChange={setActiveTab}
      tabBarExtraContent={<div className="tab-bar-extra"
                               style={{display: 'flex', alignItems: 'center', gap: 16, flexWrap: 'wrap'}}>
        {taskStatus && taskStatus.State === 0 && (<div style={{display: 'flex', alignItems: 'center'}}>
          <Progress
            type="circle"
            percent={Math.round((taskStatus.Completed / taskStatus.Total) * 100)}
            size="small"
            style={{marginRight: 8}}
          />
          <span style={{marginRight: 8}}>
            处理中: {taskStatus.Completed}/{taskStatus.Total}
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
          icon={<PlusOutlined/>}
          onClick={handleAddSubscription}
          style={{margin: 0}}
        >
          新增
        </Button>
        <Button
          type="primary"
          onClick={() => handleReloadSubs(null)}
          style={{margin: 0}}
        >
          重新获取所有订阅
        </Button>
      </div>}
    >
      <Tabs.TabPane tab="订阅链接" key="1">
        <div style={{overflowX: 'auto', width: '100%'}}>
          <Table
            columns={columns}
            dataSource={subscriptions}
            rowKey="id"
            loading={loading}
            pagination={{
              ...pagination,
              showSizeChanger: true,
              showQuickJumper: false,
              showTotal: (total) => `共 ${total} 条记录`,
              pageSizeOptions: ['10', '20', '50', '100']
            }}
            onChange={(newPagination) => {
              const {current, pageSize} = newPagination;
              fetchSubscriptions(current, pageSize);
            }}
            scroll={{x: 1500}}
            style={{width: '100%'}}
          />
        </div>
      </Tabs.TabPane>
    </Tabs>

    {/* 添加/查看订阅的弹窗 */}
    <Modal
      title={modalType === 'add' ? '添加订阅' : '订阅详情'}
      open={modalVisible}
      onCancel={() => setModalVisible(false)}
      footer={modalType === 'add' ? [<Button key="cancel" onClick={() => setModalVisible(false)}>
        取消
      </Button>, <Button
        key="submit"
        type="primary"
        loading={loading}
        onClick={handleSubmit}
      >
        确定
      </Button>] : [<Button key="close" type="primary" onClick={() => setModalVisible(false)}>
        关闭
      </Button>]}
    >
      <SubscriptionForm
        form={form}
        modalType={modalType}
        uploadType={uploadType}
        currentSubscription={currentSubscription}
        onValuesChange={handleFormValuesChange}
      />
    </Modal>

    {/* 订阅更新配置弹窗 */}
    <Modal
      title="订阅更新配置"
      open={configModalVisible}
      onCancel={() => setConfigModalVisible(false)}
      onOk={handleSaveConfig}
      confirmLoading={configLoading}
      destroyOnClose
    >
      <div style={{marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <Tag color={isCustomConfig ? "blue" : "default"}>
          {isCustomConfig ? "当前使用：自定义配置" : "当前使用：系统默认配置"}
        </Tag>
        {isCustomConfig && (<Button
          type="link"
          size="small"
          onClick={handleRestoreDefault}
          loading={configLoading}
        >
          恢复默认
        </Button>)}
      </div>
      <Form
        form={configForm}
        layout="vertical"
        initialValues={{
          simple_interval_value: 1, simple_interval_unit: 'hours'
        }}
      >
        <Form.Item
          name="auto_update"
          label="自动更新"
          valuePropName="checked"
        >
          <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
        </Form.Item>

        <Form.Item
          noStyle
          shouldUpdate={(prev, current) => prev.auto_update !== current.auto_update}
        >
          {({getFieldValue}) => getFieldValue('auto_update') && (<IntervalSelector
            form={configForm}
            fieldName="update_interval"
            mode={intervalMode}
            setMode={setIntervalMode}
          />)}
        </Form.Item>

        <Form.Item
          name="use_proxy"
          label="使用代理更新"
          valuePropName="checked"
          extra="更新此订阅时是否使用系统设置的代理"
        >
          <Switch checkedChildren="开启" unCheckedChildren="关闭"/>
        </Form.Item>
      </Form>
    </Modal>
  </div>);
};

export default SubscriptionPage;