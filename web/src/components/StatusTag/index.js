import React from 'react';
import { Tag } from 'antd';

const StatusTag = ({ status }) => {
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

export default StatusTag;
