import {EyeOutlined, ReloadOutlined, StopOutlined} from '@ant-design/icons';
import {Button, Card, message, Modal, Progress, Table, Tabs, Tag, Tooltip} from 'antd';
import React, {useEffect, useRef, useState} from 'react';
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

      const data = await subscriptionApi.getProxies({params});

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
      setNodeHistory(Array.isArray(data) ? data : []);
      setHistoryPagination(prev => ({
        ...prev, current: page, pageSize: pageSize, total: Array.isArray(data) ? data.length : 0,
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

  // 处理表格分页变化
  const handleTableChange = (newPagination, newFilters, newSorter) => {
    const sort = newSorter.field ? {
      field: newSorter.field, order: newSorter.order || 'descend',
    } : sorter;

    // 处理筛选条件
    const filter = {};
    if (newFilters.status && newFilters.status.length > 0) {
      filter.status = newFilters.status;
    }

    // 处理节点类型筛选
    if (newFilters.type && newFilters.type.length > 0) {
      filter.type = newFilters.type;
    }

    setSorter(sort);
    setFilters(filter);
    fetchNodes(newPagination.current, newPagination.pageSize, sort, filter);
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

  useEffect(() => {
    fetchNodes();
    fetchNodeTypes();
  }, []);

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

  // 表格列配置
  const columns = [{
    title: '序号', key: 'index', width: 60, render: (_, __, index) => index + 1,
  }, {
    title: '订阅链接', dataIndex: 'subscription_url', key: 'subscription_url', ellipsis: true,
  }, {
    title: '名称', dataIndex: 'name', key: 'name', ellipsis: true,
  }, {
    title: '节点', dataIndex: 'address', key: 'address',
  }, {
    title: '节点类型',
    dataIndex: 'type',
    key: 'type',
    width: 120,
    ellipsis: true,
    sorter: true,
    filters: nodeTypes.length > 0 ? nodeTypes : [{text: 'vmess', value: 'vmess'}, {text: 'vless', value: 'vless'}],
    filterMultiple: true,
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
  }, {
    title: 'Ping', dataIndex: 'ping', key: 'ping', width: 120, render: (ping) => ping ? `${ping}ms` : '-', sorter: true,
  }, {
    title: '下载速度',
    dataIndex: 'download_speed',
    key: 'download_speed',
    width: 120,
    render: (speed) => formatSpeed(speed),
    sorter: true,
    defaultSortOrder: 'descend',
  }, {
    title: '上传速度',
    dataIndex: 'upload_speed',
    key: 'upload_speed',
    width: 110,
    render: (speed) => formatSpeed(speed),
    sorter: true,
  }, {
    title: '测试时间',
    dataIndex: 'latest_test_time',
    key: 'latest_test_time',
    width: 160,
    render: (text) => formatDate(text),
    sorter: true,
  }, {
    title: '操作', key: 'action', width: 100, fixed: 'right', render: (_, record) => (<div className="table-action">
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
    </div>),
  },];

  return (<div>
    <Tabs activeKey={activeTab} onChange={setActiveTab}>
      <items tab="所有节点" key="2">
        <div style={{marginBottom: 16, position: 'relative', display: 'flex', justifyContent: 'flex-end'}}>
          <div style={{display: 'flex', alignItems: 'center', marginRight: 'auto'}}>
            {taskStatus && taskStatus.State === 0 && (
              <div style={{display: 'flex', alignItems: 'center', marginRight: 16}}>
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
                >
                  停止任务
                </Button>
              </div>)}
          </div>
          <Button
            type="primary"
            onClick={() => {
              handleTestProxy(null)
            }}
            style={{marginRight: 16}}
          >
            按当前参数进行测速
          </Button>
          <Button
            type="primary"
            onClick={() => {
              handleExportSubscriptionUrl()
            }}
          >
            按当前参数导出订阅链接
          </Button>
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
      </items>
    </Tabs>

    {/* 节点详情弹窗 */}
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
        <Card title="基本信息" style={{marginBottom: 16}}>
          <InfoItem label="名称" value={currentNode.name || '未命名'}/>
          <InfoItem label="订阅链接" value={currentNode.subscription_url}/>
          <InfoItem label="地址" value={currentNode.address}/>
          <InfoItem label="状态" value={<StatusTag status={currentNode.status}/>}/>
          <InfoItem label="Ping" value={currentNode.ping ? `${currentNode.ping}ms` : '-'}/>
          <InfoItem label="下载速度" value={formatSpeed(currentNode.download_speed)}/>
          <InfoItem label="上传速度" value={formatSpeed(currentNode.upload_speed)}/>
          <InfoItem label="最近测试时间" value={formatDate(currentNode.latest_test_time)}/>
          <InfoItem label="分享链接" value={currentNode.share_url} isMultiLine={true}/>
        </Card>

        <Card title="历史记录">
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
        </Card>
      </div>)}
    </Modal>
  </div>);
};

export default NodesPage; 