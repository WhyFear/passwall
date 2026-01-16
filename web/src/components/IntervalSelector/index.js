import React from 'react';
import { Form, Radio, InputNumber, Select, Input, Typography } from 'antd';
import { generateCronFromSimple } from '../../utils/cronUtils';

const { Option } = Select;

const IntervalSelector = ({ form, fieldName, mode, setMode, label = "更新间隔" }) => {
  const handleSimpleChange = () => {
    const value = form.getFieldValue('simple_interval_value') || 1;
    const unit = form.getFieldValue('simple_interval_unit');
    const cron = generateCronFromSimple(value, unit);

    form.setFieldsValue({
      [fieldName]: cron
    });
  };

  return (
    <>
      <Form.Item label="配置模式">
        <Radio.Group
          value={mode}
          onChange={e => {
            setMode(e.target.value);
            if (e.target.value === 'simple') {
              handleSimpleChange();
            }
          }}
        >
          <Radio.Button value="simple">简易模式</Radio.Button>
          <Radio.Button value="advanced">高级模式 (Cron)</Radio.Button>
        </Radio.Group>
      </Form.Item>

      {mode === 'simple' ? (
        <Form.Item label={label}>
          <Input.Group compact>
            <span style={{ padding: '0 8px', lineHeight: '32px' }}>每</span>
            <Form.Item
              name="simple_interval_value"
              noStyle
              rules={[{ required: true }]}
            >
              <InputNumber
                min={1}
                style={{ width: '100px' }}
                onChange={handleSimpleChange}
              />
            </Form.Item>
            <Form.Item
              name="simple_interval_unit"
              noStyle
            >
              <Select
                style={{ width: '100px' }}
                onChange={handleSimpleChange}
              >
                <Option value="seconds">秒</Option>
                <Option value="minutes">分钟</Option>
                <Option value="hours">小时</Option>
                <Option value="days">天</Option>
                <Option value="weeks">周</Option>
                <Option value="months">月</Option>
              </Select>
            </Form.Item>
          </Input.Group>
          {form.getFieldValue('simple_interval_unit') === 'weeks' && (
            <Typography.Text type="secondary" style={{ display: 'block', marginTop: 4 }}>
              注：周模式固定为每周日执行。
            </Typography.Text>
          )}
          {/* 隐藏的真实字段，用于提交 */}
          <Form.Item name={fieldName} noStyle hidden>
            <Input />
          </Form.Item>
        </Form.Item>
      ) : (
        <Form.Item
          name={fieldName}
          label={label}
          rules={[{ required: true, message: '请输入 Cron 表达式' }]}
          help="支持秒级 Cron，例如 '0 0 4 * * *' (每天凌晨4点)"
        >
          <Input />
        </Form.Item>
      )}
    </>
  );
};

export default IntervalSelector;
