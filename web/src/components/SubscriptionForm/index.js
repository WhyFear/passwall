import React from 'react';
import {Button, Form, Input, message, Radio, Select} from 'antd';
import {CopyOutlined} from '@ant-design/icons';
import {formatDate} from '../../utils/timeUtils';

/**
 * 订阅表单组件 - 用于添加和查看订阅
 */
const SubscriptionForm = ({
                            form, modalType, uploadType, currentSubscription, onValuesChange
                          }) => {
  const isViewMode = modalType === 'view';

  return (<Form
    form={form}
    layout="vertical"
    disabled={isViewMode}
    onValuesChange={onValuesChange}
  >
    {/* 类型选择 */}
    <Form.Item
      name="type"
      label="类型"
      rules={[{required: true, message: '请选择类型'}]}
      style={isViewMode ? {display: 'none'} : {}}
    >
      <Select
        style={{width: '100%'}}
        placeholder="请选择订阅类型"
        options={[{value: 'auto', label: '自动识别'}, {value: 'clash', label: 'Clash'}, {
          value: 'share_url',
          label: '分享链接'
        },]}
      />
    </Form.Item>

    {/* 上传方式 */}
    <Form.Item
      name="upload_type"
      label="上传方式"
      rules={[{required: true, message: '请选择上传方式'}]}
      style={isViewMode ? {display: 'none'} : {}}
    >
      <Radio.Group>
        <Radio value="url">链接</Radio>
        <Radio value="file">上传</Radio>
      </Radio.Group>
    </Form.Item>

    {/* 动态表单项 */}
    {!isViewMode ? (// 添加模式
      uploadType === 'url' ? (<Form.Item
        name="url"
        label="订阅链接"
        rules={[{required: true, message: '请输入订阅链接'}]}
      >
        <Input placeholder="请输入订阅链接"/>
      </Form.Item>) : (<Form.Item
        name="content"
        label="订阅内容"
        rules={[{required: true, message: '请输入订阅内容'}]}
      >
        <Input.TextArea
          placeholder="请粘贴订阅内容"
          autoSize={{minRows: 3, maxRows: 10}}
        />
      </Form.Item>)) : (// 查看模式
      <>
        <Form.Item
          name="url"
          label="订阅链接"
        >
          <Input/>
        </Form.Item>
        <Form.Item label="创建时间">
          <Input value={formatDate(currentSubscription?.created_at)} disabled/>
        </Form.Item>
        <Form.Item label="更新时间">
          <Input value={formatDate(currentSubscription?.updated_at)} disabled/>
        </Form.Item>
        {currentSubscription?.content && (<Form.Item label="订阅内容">
          <Button
            type="primary"
            size="small"
            icon={<CopyOutlined/>}
            disabled={false}
            onClick={() => {
              navigator.clipboard.writeText(currentSubscription.content)
                .then(() => message.success('分享链接已复制到剪贴板'))
                .catch(() => message.error('复制失败，请手动复制'));
            }}
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
  </Form>);
};

export default SubscriptionForm; 