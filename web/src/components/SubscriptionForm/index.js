import React from 'react';
import {Form, Input, Radio, Select} from 'antd';

/**
 * 订阅表单组件
 */
const SubscriptionForm = ({
                            form, uploadType, onValuesChange
                          }) => {
  return (<Form
    form={form}
    layout="vertical"
    onValuesChange={onValuesChange}
  >
    {/* 类型选择 */}
    <Form.Item
      name="type"
      label="类型"
      rules={[{required: true, message: '请选择类型'}]}
    >
      <Select
        style={{width: '100%'}}
        placeholder="请选择订阅类型"
        options={[{value: 'auto', label: '自动识别'}, {value: 'clash', label: 'Clash'}, {
          value: 'share_url', label: '分享链接'
        },]}
        defaultValue={'auto'}
      />
    </Form.Item>

    {/* 上传方式 */}
    <Form.Item
      name="upload_type"
      label="上传方式"
      rules={[{required: true, message: '请选择上传方式'}]}
    >
      <Radio.Group>
        <Radio value="url">链接</Radio>
        <Radio value="url_list">批量链接</Radio>
        <Radio value="file">上传</Radio>
      </Radio.Group>
    </Form.Item>

    {/* 动态表单项 */}
    {uploadType === 'url' && (
      <Form.Item
        name="url"
        label="订阅链接"
        rules={[{required: true, message: '请输入订阅链接'}]}
      >
        <Input placeholder="请输入订阅链接"/>
      </Form.Item>
    )}
    {uploadType === 'url_list' && (
      <Form.Item
        name="url_list_text"
        label="批量链接"
        rules={[{required: true, message: '请输入批量订阅链接'}]}
      >
        <Input.TextArea
          placeholder="请输入订阅链接，一行一个"
          autoSize={{minRows: 3, maxRows: 10}}
        />
      </Form.Item>
    )}
    {uploadType === 'file' && (
      <Form.Item
        name="content"
        label="订阅内容"
        rules={[{required: true, message: '请输入订阅内容'}]}
      >
        <Input.TextArea
          placeholder="请粘贴订阅内容"
          autoSize={{minRows: 3, maxRows: 10}}
        />
      </Form.Item>
    )}
  </Form>);
};

export default SubscriptionForm;
