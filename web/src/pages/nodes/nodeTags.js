import {Tag} from 'antd';

export const StatusTag = ({status}) => {
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

export const AppUnlockStatusTag = ({status}) => {
  let color = 'default';
  let text = '未知';

  if (status === 'fail') {
    color = 'error';
    text = '失败';
  } else if (status === 'unlock') {
    color = 'success';
    text = '解锁';
  } else if (status === 'forbidden') {
    color = 'warning';
    text = '屏蔽';
  } else if (status === 'rateLimit') {
    color = 'warning';
    text = '限流';
  }

  return <Tag color={color}>{text}</Tag>;
};

export const InfoItem = ({label, value}) => {
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
