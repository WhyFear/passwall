import React, {useEffect, useRef, useState} from 'react';
import {Button, Form, message, Modal, Progress, Table, Tabs, Tag, Tooltip} from 'antd';
import {EyeOutlined, PlusOutlined, ReloadOutlined, StopOutlined} from '@ant-design/icons';
import {subscriptionApi} from '../api';
import {fetchTaskStatus, stopTask} from '../utils/taskUtils';
import SubscriptionForm from '../components/SubscriptionForm';
import {formatDate} from "../utils/timeUtils";

const StatusTag = ({status}) => {
  let color = 'default';
  let text = '未知';

  if (status === -1) {
    color = 'default';
    text = '新订阅';
  } else if (status === 1) {
    color = 'success';
    text = '拉取成功';
  } else if (status === 2) {
    color = 'error';
    text = '拉取失败';
  } else if (status === 3) {
    color = 'warning';
    text = '未知错误';
  }

  return <Tag color={color}>{text}</Tag>;
};

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
    form.setFieldsValue({upload_type: 'url'});
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

      message.success('添加订阅成功');
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
    title: '生效节点数量',
    dataIndex: 'proxy_num',
    key: 'proxy_num',
    width: 120,
    render: (proxy_num) => (proxy_num) ? proxy_num : '-',
  }, {
    title: '所有节点数量',
    dataIndex: 'all_proxy_num',
    key: 'all_proxy_num',
    width: 120,
    render: (all_proxy_num) => (all_proxy_num) ? all_proxy_num : '-',
  }, {
    title: '上次更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180, render: (text) => formatDate(text),
  }, {
    title: '添加时间', dataIndex: 'created_at', key: 'created_at', width: 180, render: (text) => formatDate(text),
  }, {
    title: '操作', key: 'action', width: 260, fixed: isMobile ? undefined : 'right', render: (_, record) => (<div>
      <Tooltip title="查看内容">
        <Button
          type="text"
          icon={<EyeOutlined/>}
          onClick={() => handleViewSubscription(record)}
        >
          查看内容
        </Button>
      </Tooltip>
      <Tooltip title="刷新订阅">
        <Button
          type="text"
          icon={<ReloadOutlined/>}
          onClick={() => handleReloadSubs(record.id)}
        >
          刷新订阅
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
  </div>);
};

export default SubscriptionPage; 