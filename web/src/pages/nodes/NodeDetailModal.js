import {Button, Card, Modal, Table} from 'antd';
import {formatDate} from '../../utils/timeUtils';
import {formatSpeed, formatTraffic} from './nodeFormatters';
import {AppUnlockStatusTag, InfoItem, StatusTag} from './nodeTags';

const NodeDetailModal = ({
  open,
  node,
  nodeHistory,
  historyLoading,
  historyPagination,
  onClose,
  onHistoryTableChange,
}) => (
  <Modal
    title="节点详情"
    open={open}
    onCancel={onClose}
    footer={[<Button key="close" type="primary" onClick={onClose}>
      关闭
    </Button>]}
    width={800}
  >
    {node && (<div>
      <Card title="基本信息" style={{marginBottom: 5}}>
        <InfoItem label="名称" value={node.name || '未命名'}/>
        <InfoItem label="订阅链接" value={node.subscription_url}/>
        <InfoItem label="地址" value={node.address}/>
        <InfoItem label="节点类型" value={node.type}/>
        <InfoItem label="状态" value={<StatusTag status={node.status}/>}/>
        <InfoItem label="Ping" value={node.ping ? `${node.ping}ms` : '-'}/>
        <InfoItem label="下载速度" value={formatSpeed(node.download_speed)}/>
        <InfoItem label="上传速度" value={formatSpeed(node.upload_speed)}/>
        <InfoItem label="节点创建时间" value={formatDate(node.created_at)}/>
        <InfoItem label="最近测试时间" value={formatDate(node.latest_test_time)}/>
        <InfoItem label="分享链接" value={node.share_url}/>
        <InfoItem label="总计下载流量" value={formatTraffic(node.download_total)}/>
        <InfoItem label="总计上传流量" value={formatTraffic(node.upload_total)}/>
        {node.ip_info?.ipv4 && (<InfoItem label="IPV4地址" value={node.ip_info?.ipv4}/>)}
        {node.ip_info?.ipv6 && (<InfoItem label="IPV6地址" value={node.ip_info?.ipv6}/>)}
        {node.ip_info?.risk && (<InfoItem label="风险等级" value={node.ip_info?.risk}/>)}
        {node.ip_info?.country_code && (
          <InfoItem label="国家/地区代码" value={node.ip_info?.country_code}/>)}
      </Card>

      {node?.ip_info?.app_unlock && node.ip_info?.app_unlock.length > 0 && (
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
            dataSource={node.ip_info?.app_unlock || []}
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
          }]}
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
          onChange={onHistoryTableChange}
        />
      </Card>}
    </div>)}
  </Modal>
);

export default NodeDetailModal;
