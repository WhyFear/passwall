import React, { useEffect, useState } from 'react';
import { Badge, Button, Card, message, Modal, Table, Tabs, Tag, Tooltip } from 'antd';
import { CopyOutlined, EyeOutlined, ReloadOutlined } from '@ant-design/icons';
import { nodeApi, subscriptionApi } from '../api';

const { TabPane } = Tabs;

const StatusTag = ({ status }) => {
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
  const [sorter, setSorter] = useState({
    field: 'tested_at',
    order: 'descend',
  });

  // 获取所有节点
  const fetchNodes = async (page = pagination.current, pageSize = pagination.pageSize, sort = sorter) => {
    try {
      setLoading(true);
      const data = await subscriptionApi.getProxies({
        params: {
          page: page,
          pageSize: pageSize,
          sortField: sort.field,
          sortOrder: sort.order,
        }
      });
      // 直接使用返回的items数组作为节点列表
      const nodeList = Array.isArray(data.items) ? data.items : [];
      setNodes(nodeList);
      setPagination(prev => ({
        ...prev,
        current: page,
        pageSize: pageSize,
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
  const fetchNodeHistory = async (nodeId, page = historyPagination.current, pageSize = historyPagination.pageSize) => {
    try {
      setHistoryLoading(true);
      const data = await nodeApi.getProxyHistory(nodeId, page, pageSize);
      setNodeHistory(Array.isArray(data) ? data : []);
      setHistoryPagination(prev => ({
        ...prev,
        current: page,
        pageSize: pageSize,
        total: Array.isArray(data) ? data.length : 0,
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
      const data = await nodeApi.getProxyShareUrl(nodeId);
      setCurrentNode(prev => ({
        ...prev,
        share_url: atob(data),
      }));
    } catch (error) {
      message.error('获取节点历史失败');
      console.error(error);
    } finally {
      setHistoryLoading(false);
    }
  };

  // 处理表格分页变化
  const handleTableChange = (newPagination, filters, newSorter) => {
    const sort = newSorter.field ? {
      field: newSorter.field,
      order: newSorter.order || 'descend',
    } : sorter;
    
    setSorter(sort);
    fetchNodes(newPagination.current, newPagination.pageSize, sort);
  };

  // 处理历史记录表格分页变化
  const handleHistoryTableChange = (newPagination) => {
    if (currentNode) {
      fetchNodeHistory(currentNode.id, newPagination.current, newPagination.pageSize);
    }
  };

  useEffect(() => {
    fetchNodes();
  }, []);

  // 查看节点详情
  const handleViewNode = (node) => {
    setCurrentNode(node);
    // 重置历史分页到第一页
    setHistoryPagination(prev => ({
      ...prev,
      current: 1,
      pageSize: 5,
    }));
    fetchNodeHistory(node.id, 1, 5);
    fetchNodeShareUrl(node.id);
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
      render: (status) => <StatusTag status={status} />,
      sorter: true,
    },
    {
      title: 'Ping',
      dataIndex: 'ping',
      key: 'ping',
      render: (ping) => ping ? `${ping}ms` : '-',
      sorter: true,
    },
    {
      title: '下载速度',
      dataIndex: 'download_speed',
      key: 'download_speed',
      render: (speed) => speed ? `${speed}KB/s` : '-',
      sorter: true,
    },
    {
      title: '上传速度',
      dataIndex: 'upload_speed',
      key: 'upload_speed',
      render: (speed) => speed ? `${speed}KB/s` : '-',
      sorter: true,
    },
    {
      title: '测试时间',
      dataIndex: 'tested_at',
      key: 'tested_at', 
      render: (text) => {
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
      },
      sorter: true,
      defaultSortOrder: 'descend',
    },
    {
      title: '操作',
      key: 'action',
      width: 60,
      fixed: 'right',
      render: (_, record) => (
        <div className="table-action">
          <Tooltip title="查看详情">
            <Button
              type="text"
              icon={<EyeOutlined />}
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
          <div style={{ marginBottom: 16 }}>
          </div>
          <Card>
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
              scroll={{ x: 1200 }}
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
            <Card title="基本信息" style={{ marginBottom: 16 }}>
              <p><strong>名称:</strong> {currentNode.name || '未命名'}</p>
              <p><strong>订阅链接:</strong> {currentNode.subscription_url}</p>
              <p><strong>地址:</strong> {currentNode.address}</p>
              <p><strong>状态:</strong> <StatusTag status={currentNode.status} /></p>
              <p><strong>Ping:</strong> {currentNode.ping ? `${currentNode.ping}ms` : '-'}</p>
              <p>
                <strong>下载速度:</strong> {currentNode.download_speed ? `${currentNode.download_speed}KB/s` : '-'}
              </p>
              <p>
                <strong>上传速度:</strong> {currentNode.upload_speed ? `${currentNode.upload_speed}KB/s` : '-'}
              </p>
              <p><strong>最后测试时间:</strong> {currentNode.tested_at || '-'}</p>
              <p><strong>分享链接:</strong>
                {currentNode.share_url ? (
                  <>
                    <div
                      style={{
                        width: '100%',
                        backgroundColor: '#f5f5f5',
                        padding: '8px 12px',
                        borderRadius: '4px',
                        border: '1px solid #e8e8e8',
                        marginBottom: '8px',
                        wordBreak: 'break-all'
                      }}
                    >
                      {currentNode.share_url}
                    </div>
                    <Button
                      type="primary"
                      size="small"
                      icon={<CopyOutlined />}
                      onClick={() => {
                        navigator.clipboard.writeText(currentNode.share_url)
                          .then(() => message.success('分享链接已复制到剪贴板'))
                          .catch(() => message.error('复制失败，请手动复制'));
                      }}
                    >
                      复制
                    </Button>
                  </>
                ) : '-'}
              </p>
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
                    render: (status) => <StatusTag status={status} />
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