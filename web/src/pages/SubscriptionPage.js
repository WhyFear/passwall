import React, {useEffect, useState} from 'react';
import {Button, Form, Input, message, Modal, Select, Table, Tabs} from 'antd';
import {CopyOutlined, EyeOutlined, PlusOutlined} from '@ant-design/icons';
import {subscriptionApi} from '../api';
const SubscriptionPage = () => {
  const [subscriptions, setSubscriptions] = useState([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [modalType, setModalType] = useState('add'); // 'add' 或 'view'
  const [currentSubscription, setCurrentSubscription] = useState(null);
  const [form] = Form.useForm();
  const [activeTab, setActiveTab] = useState('1');

  // 获取订阅列表
  const fetchSubscriptions = async () => {
    try {
      setLoading(true);
      const data = await subscriptionApi.getSubscriptions();
      setSubscriptions(Array.isArray(data) ? data : []);
    } catch (error) {
      message.error('获取订阅列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSubscriptions()
  }, []);

  // 添加订阅
  const handleAddSubscription = () => {
    setModalType('add');
    setCurrentSubscription(null);
    form.resetFields();
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
        if (data && data.length > 0) {
          // 使用返回的详细数据
          setCurrentSubscription(data[0]);
          form.setFieldsValue(data[0]);
        } else {
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

      await subscriptionApi.createProxy(values);
      message.success('添加订阅成功');
      setModalVisible(false);
      await fetchSubscriptions();
    } catch (error) {
      message.error('添加订阅失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 表格列配置
  const columns = [{
    title: '序号', key: 'index', width: 80, render: (_, __, index) => index + 1,
  }, {
    title: '链接', dataIndex: 'url', key: 'url', ellipsis: true,
  }, {
    title: '上次更新时间', dataIndex: 'updated_at', key: 'updated_at', width: 180, render: (text) => {
      if (!text) return '-';
      const date = new Date(text);
      return date.toLocaleString('zh-CN', {
        year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit'
      });
    }
  }, {
    title: '操作', key: 'action', width: 120, render: (_, record) => (<Button
      type="link"
      icon={<EyeOutlined/>}
      onClick={() => handleViewSubscription(record)}
    >
      查看内容
    </Button>),
  },];

  return (<div>
    <Tabs activeKey={activeTab} onChange={setActiveTab}>
      <items tab="订阅链接" key="1">
        <div style={{marginBottom: 16}}>
          <Button
            type="primary"
            icon={<PlusOutlined/>}
            onClick={handleAddSubscription}
            style={{marginRight: 8}}
          >
            新增
          </Button>
        </div>
        <Table
          columns={columns}
          dataSource={subscriptions}
          rowKey="id"
          loading={loading}
          pagination={{pageSize: 10}}
        />
      </items>
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
      </Button>,] : [<Button key="close" type="primary" onClick={() => setModalVisible(false)}>
        关闭
      </Button>,]}
    >
      <Form
        form={form}
        layout="vertical"
        disabled={modalType === 'view'}
      >
        <Form.Item
          name="url"
          label="订阅链接"
          rules={[{required: true, message: '请输入订阅链接'}]}
        >
          <Input placeholder="请输入订阅链接"/>
        </Form.Item>
        <Form.Item
          name="type"
          label="类型"
          rules={[{required: true, message: '请选择类型'}]}
        >
          <Select
            style={{width: '100%'}}
            placeholder="请选择订阅类型"
            options={[{value: 'clash', label: 'Clash'}, {value: 'share_url', label: '分享链接'},]}
          />
        </Form.Item>

        {modalType === 'view' && currentSubscription && (<>
          <Form.Item label="创建时间">
            <Input value={currentSubscription.updated_at} disabled/>
          </Form.Item>
          {currentSubscription.content && (<Form.Item label="订阅内容">
            <Button
              type="primary"
              size="small"
              icon={<CopyOutlined/>}
              onClick={() => {
                navigator.clipboard.writeText(currentSubscription.content)
                  .then(() => message.success('分享链接已复制到剪贴板'))
                  .catch(() => message.error('复制失败，请手动复制'));
              }}
              disabled={!currentSubscription.content}
            >
              复制
            </Button>
            <Input.TextArea
              value={currentSubscription.content}
              disabled
              autoSize={{minRows: 3, maxRows: 10}}
            />
          </Form.Item>)}
        </>)}
      </Form>
    </Modal>
  </div>);
};

export default SubscriptionPage; 