import React, {useEffect, useState} from 'react';
import {Button, Input, message, Modal} from 'antd';
import {getToken, hasToken, isValidTokenFormat, saveToken} from '../utils/tokenUtils';

const TokenModal = ({visible, onClose}) => {
  const [token, setToken] = useState('');
  const [loading, setLoading] = useState(false);

  // 初始化时，如果已经有token，则填充到输入框
  useEffect(() => {
    if (visible) {
      const savedToken = getToken();
      if (savedToken) {
        setToken(savedToken);
      } else {
        setToken('');
      }
    }
  }, [visible]);

  const handleSubmit = () => {
    if (!isValidTokenFormat(token)) {
      message.error('请输入有效的Token');
      return;
    }

    setLoading(true);

    // 保存token到本地存储
    if (saveToken(token)) {
      // 模拟验证过程
      setTimeout(() => {
        setLoading(false);
        message.success('Token已保存');
        onClose(true);
      }, 500);
    } else {
      setLoading(false);
      message.error('Token保存失败');
    }
  };

  // 处理取消操作
  const handleCancel = () => {
    // 如果已经有Token，则可以直接关闭
    if (hasToken()) {
      onClose(false);
    } else {
      // 如果没有Token，提示用户
      Modal.confirm({
        title: '确认取消',
        content: '未设置Token可能导致部分功能无法使用，确定要取消吗？',
        okText: '确定',
        cancelText: '返回设置',
        onOk: () => {
          onClose(false);
        }
      });
    }
  };

  return (
    <Modal
      title="设置Token"
      open={visible}
      onCancel={handleCancel}
      maskClosable={hasToken()} // 只有在已有Token的情况下才允许点击蒙层关闭
      closable={hasToken()} // 只有在已有Token的情况下才显示关闭按钮
      keyboard={hasToken()} // 只有在已有Token的情况下才响应键盘ESC
      footer={[
        <Button key="cancel" onClick={handleCancel}>
          取消
        </Button>,
        <Button
          key="submit"
          type="primary"
          loading={loading}
          onClick={handleSubmit}
        >
          确定
        </Button>
      ]}
    >
      <div style={{marginBottom: 16}}>
        <p>{getToken() ? '更新Token：' : '未检测到有效的Token，请输入Token以继续访问：'}</p>
        <Input.Password
          value={token}
          onChange={(e) => setToken(e.target.value)}
          placeholder="请输入Token"
          autoFocus
          onPressEnter={handleSubmit}
        />
      </div>
    </Modal>
  );
};

export default TokenModal; 