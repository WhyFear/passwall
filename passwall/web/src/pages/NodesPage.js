import React, {useEffect, useState} from 'react';
import {Badge, Button, Card, message, Modal, Table, Tabs, Tag, Tooltip} from 'antd';
import {CopyOutlined, EyeOutlined, ReloadOutlined} from '@ant-design/icons';
import {nodeApi, subscriptionApi} from '../api';

const {TabPane} = Tabs;

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

const NodesPage = () => {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [activeTab, setActiveTab] = useState('2');
  const [modalVisible, setModalVisible] = useState(false);
  const [currentNode, setCurrentNode] = useState(null);
  const [nodeHistory, setNodeHistory] = useState([]);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
  });
  const [historyPagination, setHistoryPagination] = useState({
    current: 1,
    pageSize: 5,
    total: 0,
  });

  // 获取所有节点
  const fetchNodes = async () => {
    try {
      setLoading(true);
      const data = await subscriptionApi.getProxies();
      // 直接使用返回的items数组作为节点列表
      const nodeList = Array.isArray(data.items) ? data.items : [];
      setNodes(nodeList);
      setPagination(prev => ({
        ...prev,
        total: data.total || nodeList.length,
      }));
    } catch (error) {
      message.error('获取节点列表失败');
      console.error(error);
    } finally {
      setLoading(false);
    }
  };

  // 获取节点历史
  const fetchNodeHistory = async (nodeId) => {
    try {
      setHistoryLoading(true);
      const data = await nodeApi.getProxyHistory(nodeId);
      setNodeHistory(Array.isArray(data) ? data : []);
      setHistoryPagination(prev => ({
        ...prev,
        total: Array.isArray(data) ? data.length : 0,
        current: 1, // 重置为第一页
      }));
    } catch (error) {
      message.error('获取节点历史失败');
      console.error(error);
    } finally {
      setHistoryLoading(false);
    }
  };

  // 处理表格分页变化
  const handleTableChange = (newPagination) => {
    setPagination(prev => ({
      ...prev,
      current: newPagination.current,
      pageSize: newPagination.pageSize,
    }));
  };

  // 处理历史记录表格分页变化
  const handleHistoryTableChange = (newPagination) => {
    setHistoryPagination(prev => ({
      ...prev,
      current: newPagination.current,
      pageSize: newPagination.pageSize,
    }));
  };

  useEffect(() => {
    fetchNodes();
  }, []);

  // 复制节点信息
  const handleCopyNode = (node) => {
    const textToCopy = `${node.name || '未命名节点'} - ${node.address}`;
    navigator.clipboard.writeText(textToCopy)
      .then(() => message.success('复制成功'))
      .catch(() => message.error('复制失败'));
  };

  // 查看节点详情
  const handleViewNode = (node) => {
    setCurrentNode(node);
    fetchNodeHistory(node.id);
    setModalVisible(true);
  };

  // 表格列配置
  const columns = [
    {
      title: '序号', key: 'index', width: 80, render: (_, __, index) => index + 1,
    },
    {
      title: '订阅链接',
      dataIndex: 'subscription_url',
      key: 'subscription_url',
      ellipsis: true,
    },
    {
      title: '名称',
      dataIndex: 'name',
      key: 'name',
      ellipsis: true,
    },
    {
      title: '节点',
      dataIndex: 'address',
      key: 'address',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status) => <StatusTag status={status}/>,
    },
    {
      title: 'Ping',
      dataIndex: 'ping',
      key: 'ping',
      render: (ping) => ping ? `${ping}ms` : '-',
    },
    {
      title: '下载速度',
      dataIndex: 'download_speed',
      key: 'download_speed',
      render: (speed) => speed ? `${speed}KB/s` : '-',
    },
    {
      title: '上传速度',
      dataIndex: 'upload_speed',
      key: 'upload_speed',
      render: (speed) => speed ? `${speed}KB/s` : '-',
    },
    {
      title: '测试时间',
      dataIndex: 'tested_at',
      key: 'tested_at', render: (text) => {
        if (!text) return '-';
        const date = new Date(text);
        return date.toLocaleString('zh-CN', {
          year: 'numeric',
          month: '2-digit',
          day: '2-digit',
          hour: '2-digit',
          minute: '2-digit',
          second: '2-digit'
        });
      }
    },
    {
      title: '操作',
      key: 'action',
      width: 180,
      render: (_, record) => (
        <div className="table-action">
          <Tooltip title="复制">
            <Button
              type="text"
              icon={<CopyOutlined/>}
              onClick={() => handleCopyNode(record)}
            />
          </Tooltip>
          <Tooltip title="查看详情">
            <Button
              type="text"
              icon={<EyeOutlined/>}
              onClick={() => handleViewNode(record)}
            />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <div>
      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab="所有节点" key="2">
          <div style={{marginBottom: 16}}>
            <Button
              icon={<ReloadOutlined/>}
              onClick={fetchNodes}
              loading={loading}
            >
              刷新节点
            </Button>
          </div>
          <Card>
            <div style={{marginBottom: 16, display: 'flex', gap: 16}}>
              <Badge status="new" text={`未测试: ${nodes.filter(n => n.status === -1).length}`}/>
              <Badge status="success" text={`正常: ${nodes.filter(n => n.status === 1).length}`}/>
              <Badge status="error" text={`失败: ${nodes.filter(n => n.status === 2).length}`}/>
              <Badge status="warning" text={`未知: ${nodes.filter(n => n.status === 3).length}`}/>
              <Badge status="default" text={`总计: ${nodes.length}`}/>
            </div>
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
              scroll={{x: 1200}}
            />
          </Card>
        </TabPane>
      </Tabs>

      {/* 节点详情弹窗 */}
      <Modal
        title="节点详情"
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={[
          <Button key="close" type="primary" onClick={() => setModalVisible(false)}>
            关闭
          </Button>,
        ]}
        width={800}
      >
        {currentNode && (
          <div>
            <Card title="基本信息" style={{marginBottom: 16}}>
              <p><strong>名称:</strong> {currentNode.name || '未命名'}</p>
              <p><strong>订阅链接:</strong> {currentNode.subscription_url}</p>
              <p><strong>地址:</strong> {currentNode.address}</p>
              <p><strong>状态:</strong> <StatusTag status={currentNode.status}/></p>
              <p><strong>Ping:</strong> {currentNode.ping ? `${currentNode.ping}ms` : '-'}</p>
              <p>
                <strong>下载速度:</strong> {currentNode.download_speed ? `${currentNode.download_speed}KB/s` : '-'}
              </p>
              <p>
                <strong>上传速度:</strong> {currentNode.upload_speed ? `${currentNode.upload_speed}KB/s` : '-'}
              </p>
              <p><strong>最后测试时间:</strong> {currentNode.tested_at || '-'}</p>
            </Card>

            <Card title="历史记录">
              <Table
                columns={[
                  {
                    title: '测试时间', dataIndex: 'tested_at', key: 'tested_at', render: (text) => {
                      if (!text) return '-';
                      const date = new Date(text);
                      return date.toLocaleString('zh-CN', {
                        year: 'numeric',
                        month: '2-digit',
                        day: '2-digit',
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit'
                      });
                    }
                  },
                  {
                    title: '状态',
                    dataIndex: 'status',
                    key: 'status',
                    render: (status) => <StatusTag status={status}/>
                  },
                  {
                    title: 'Ping',
                    dataIndex: 'ping',
                    key: 'ping',
                    render: (ping) => ping ? `${ping}ms` : '-'
                  },
                  {
                    title: '下载速度',
                    dataIndex: 'download_speed',
                    key: 'download_speed',
                    render: (speed) => speed ? `${speed}KB/s` : '-'
                  },
                  {
                    title: '上传速度',
                    dataIndex: 'upload_speed',
                    key: 'upload_speed',
                    render: (speed) => speed ? `${speed}KB/s` : '-'
                  },
                ]}
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
            </Card>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default NodesPage; 