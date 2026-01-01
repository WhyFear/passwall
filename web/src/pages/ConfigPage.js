import React, {useEffect, useState} from 'react';
import {
  Button,
  Card,
  Checkbox,
  Col,
  Collapse,
  Form,
  Input,
  InputNumber,
  Layout,
  message,
  Radio,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Tooltip,
  Typography
} from 'antd';
import {DeleteOutlined, PlusOutlined, QuestionCircleOutlined, SaveOutlined} from '@ant-design/icons';
import {configApi} from '../api';

const {Panel} = Collapse;

const {Content} = Layout;
const {Title} = Typography;
const {TabPane} = Tabs;
const {Option} = Select;

const ConfigPage = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [intervalMode, setIntervalMode] = useState('simple'); // 'simple' or 'advanced'
  const [initialData, setInitialData] = useState({});

  useEffect(() => {
    loadConfig();
  }, []);

  const loadConfig = async () => {
    try {
      setLoading(true);
      const data = await configApi.getConfig();
      // 数据处理，确保默认值
      if (!data.default_sub) {
        data.default_sub = {auto_update: false, interval: "0 0 4 * * *", use_proxy: false};
      }
      form.setFieldsValue(data);
      setInitialData(data); // 保存初始数据

      // 解析 Interval，设置模式
      parseInterval(data.default_sub.interval);

    } catch (error) {
      message.error('加载配置失败: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  const onFinish = async (values) => {
    try {
      setLoading(true);

      // 计算变更字段
      const changedValues = {};
      const keys = ['concurrent', 'proxy', 'ip_check', 'clash_api', 'cron_jobs', 'default_sub'];

      let hasChanges = false;
      keys.forEach(key => {
        // 使用 JSON.stringify 进行简单深度比较
        // 注意：values[key] 可能包含表单自动注入的 undefined 属性，需要小心
        // initialData[key] 来自后端，可能为 null/undefined

        const currentVal = values[key];
        const initialVal = initialData[key];

        if (JSON.stringify(currentVal) !== JSON.stringify(initialVal)) {
          changedValues[key] = currentVal;
          hasChanges = true;
        }
      });

      if (!hasChanges) {
        message.info('配置未修改');
        setLoading(false);
        return;
      }

      await configApi.updateConfig(changedValues);
      message.success('配置保存成功');
      loadConfig(); // 重新加载以确认
    } catch (error) {
      message.error('保存配置失败: ' + error.message);
    } finally {
      setLoading(false);
    }
  };

  // 解析 Cron 表达式并设置 Simple/Advanced 模式及对应的值
  const parseInterval = (interval) => {
    if (!interval) return;

    // 尝试匹配标准间隔 Cron 表达式
    // 秒: */x * * * * *
    // 分: 0 */x * * * *
    // 时: 0 0 */x * * *
    // 天: 0 0 0 */x * *
    // 周: 0 0 0 * * 0 (每周)
    // 月: 0 0 0 1 */x *

    const matchSeconds = interval.match(/^\*\/(\d+)\s+\*\s+\*\s+\*\s+\*\s+\*$/);
    const matchMinutes = interval.match(/^0\s+\*\/(\d+)\s+\*\s+\*\s+\*\s+\*$/);
    const matchHours = interval.match(/^0\s+0\s+\*\/(\d+)\s+\*\s+\*\s+\*$/);
    const matchDays = interval.match(/^0\s+0\s+0\s+\*\/(\d+)\s+\*\s+\*$/);
    const matchMonths = interval.match(/^0\s+0\s+0\s+1\s+\*\/(\d+)\s+\*$/);

    // 特殊处理 "1" 的情况 (*/1 经常简写为 *)，这里简化处理，假设后端或生成器都用 */n
    // 如果是单星号，通常是每秒/分/时...

    let mode = 'advanced';
    let value = 1;
    let unit = 'hours'; // 默认

    if (matchSeconds) {
      mode = 'simple';
      value = parseInt(matchSeconds[1]);
      unit = 'seconds';
    } else if (matchMinutes) {
      mode = 'simple';
      value = parseInt(matchMinutes[1]);
      unit = 'minutes';
    } else if (matchHours) {
      mode = 'simple';
      value = parseInt(matchHours[1]);
      unit = 'hours';
    } else if (matchDays) {
      mode = 'simple';
      value = parseInt(matchDays[1]);
      unit = 'days';
    } else if (matchMonths) {
      mode = 'simple';
      value = parseInt(matchMonths[1]);
      unit = 'months';
    } else if (interval === '0 0 0 * * 0') {
      mode = 'simple';
      value = 1;
      unit = 'weeks';
    } else if (interval === '0 0 0 1 * *') { // 每月1号
      mode = 'simple';
      value = 1;
      unit = 'months';
    } else if (interval === '0 0 0 * * *') { // 每天0点
      mode = 'simple';
      value = 1;
      unit = 'days';
    }

    // 如果是 @every 格式
    const matchEvery = interval.match(/^@every\s+(\d+)([smh])$/);
    if (matchEvery) {
      // @every 也是一种 Simple 模式，但为了统一，如果之前存的是 Cron，我们尽量保留。
      // 这里如果识别到 @every，也可以切到 Simple。
      // 但为了避免混淆，暂时把 @every 归为 Advanced，除非我们需要完全接管。
      // 需求是 "用户选择每 X ...", 所以 @every 也是有效实现。
      // 让我们把 @every 也映射回来。
      mode = 'simple';
      value = parseInt(matchEvery[1]);
      const u = matchEvery[2];
      if (u === 's') unit = 'seconds';
      if (u === 'm') unit = 'minutes';
      if (u === 'h') unit = 'hours';
    }

    setIntervalMode(mode);
    if (mode === 'simple') {
      form.setFieldsValue({
        simple_interval_value: value, simple_interval_unit: unit
      });
    }
  };

  // 生成 Cron 表达式并更新 Form
  const handleSimpleIntervalChange = () => {
    const value = form.getFieldValue('simple_interval_value') || 1;
    const unit = form.getFieldValue('simple_interval_unit');

    let cron = "";
    switch (unit) {
      case 'seconds':
        cron = `*/${value} * * * * *`;
        break;
      case 'minutes':
        cron = `0 */${value} * * * *`;
        break;
      case 'hours':
        cron = `0 0 */${value} * * *`;
        break;
      case 'days':
        cron = `0 0 0 */${value} * *`;
        if (value === 1) cron = `0 0 0 * * *`;
        break;
      case 'weeks':
        // 仅支持每周一次
        cron = `0 0 0 * * 0`;
        break;
      case 'months':
        cron = `0 0 0 1 */${value} *`;
        if (value === 1) cron = `0 0 0 1 * *`;
        break;
      default:
        cron = `0 0 4 * * *`;
    }

    form.setFieldsValue({
      default_sub: {
        ...form.getFieldValue('default_sub'), interval: cron
      }
    });
  };

  return (<Layout>
    <Content style={{padding: '24px'}}>
      <div style={{display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 24}}>
        <Title level={2}>系统配置</Title>
        <Button type="primary" icon={<SaveOutlined/>} onClick={form.submit} loading={loading}>
          保存配置
        </Button>
      </div>

      <Form
        form={form}
        layout="vertical"
        onFinish={onFinish}
        initialValues={{
          concurrent: 5,
          proxy: {enabled: false},
          ip_check: {enable: false},
          clash_api: {enable: false, clients: []},
          cron_jobs: [],
          default_sub: {auto_update: false, interval: "0 0 4 * * *", use_proxy: false},
          simple_interval_value: 1,
          simple_interval_unit: 'hours'
        }}
      >
        <Tabs defaultActiveKey="1">
          <TabPane tab="通用设置" key="1">
            <Card title="基础设置" bordered={false} style={{marginBottom: 16}}>
              <Form.Item
                label="并发数"
                name="concurrent"
                rules={[{required: true, message: '请输入并发数'}]}
                help="全局任务并发限制"
              >
                <InputNumber min={1} max={100}/>
              </Form.Item>
            </Card>

            <Card title="全局代理" bordered={false} style={{marginBottom: 16}}>
              <Form.Item name={['proxy', 'enabled']} valuePropName="checked" label="启用代理">
                <Switch/>
              </Form.Item>
              <Form.Item
                noStyle
                shouldUpdate={(prev, current) => prev.proxy?.enabled !== current.proxy?.enabled}
              >
                {({getFieldValue}) => getFieldValue(['proxy', 'enabled']) && (<Form.Item
                  label="代理地址"
                  name={['proxy', 'url']}
                  rules={[{required: true, message: '请输入代理地址'}]}
                  help="例如: http://127.0.0.1:7890 或 socks5://127.0.0.1:1080"
                >
                  <Input placeholder="http://127.0.0.1:7890"/>
                </Form.Item>)}
              </Form.Item>
            </Card>
          </TabPane>

          <TabPane tab="IP检测" key="2">
            <Card title="IP检测配置" bordered={false}>
              <Form.Item name={['ip_check', 'enable']} valuePropName="checked" label="启用IP检测">
                <Switch/>
              </Form.Item>
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name={['ip_check', 'concurrent']} label="检测并发数">
                    <InputNumber min={1} max={50}/>
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name={['ip_check', 'refresh']} valuePropName="checked" label="强制刷新">
                    <Switch/>
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item name={['ip_check', 'app_unlock', 'enable']} valuePropName="checked" label="启用应用解锁检测">
                <Switch/>
              </Form.Item>
            </Card>
          </TabPane>

          <TabPane tab="Clash API" key="3">
            <Card title="Clash API 设置" bordered={false}>
              <Form.Item name={['clash_api', 'enable']} valuePropName="checked" label="启用 Clash API">
                <Switch/>
              </Form.Item>

              <Form.List name={['clash_api', 'clients']}>
                {(fields, {add, remove}) => (<>
                  {fields.map(({key, name, ...restField}) => (
                    <Space key={key} style={{display: 'flex', marginBottom: 8}} align="baseline">
                      <Form.Item
                        {...restField}
                        name={[name, 'url']}
                        rules={[{required: true, message: 'Missing URL'}]}
                      >
                        <Input placeholder="API URL"/>
                      </Form.Item>
                      <Form.Item
                        {...restField}
                        name={[name, 'secret']}
                      >
                        <Input placeholder="Secret (Optional)"/>
                      </Form.Item>
                      <DeleteOutlined onClick={() => remove(name)}/>
                    </Space>))}
                  <Form.Item>
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined/>}>
                      添加客户端
                    </Button>
                  </Form.Item>
                </>)}
              </Form.List>
            </Card>
          </TabPane>

          <TabPane tab="定时任务" key="4">
            <Card title="默认订阅更新配置" bordered={false} style={{marginBottom: 16}}>
              <Form.Item name={['default_sub', 'auto_update']} valuePropName="checked" label="自动更新订阅">
                <Switch/>
              </Form.Item>
              <Form.Item
                noStyle
                shouldUpdate={(prev, current) => prev.default_sub?.auto_update !== current.default_sub?.auto_update}
              >
                {({getFieldValue}) => getFieldValue(['default_sub', 'auto_update']) && (<>
                  <Form.Item label="配置模式">
                    <Radio.Group
                      value={intervalMode}
                      onChange={e => {
                        setIntervalMode(e.target.value);
                        // 切换时如果切到 simple，立即用当前 simple 值更新 Cron
                        if (e.target.value === 'simple') {
                          handleSimpleIntervalChange();
                        }
                      }}
                    >
                      <Radio.Button value="simple">简易模式</Radio.Button>
                      <Radio.Button value="advanced">高级模式 (Cron)</Radio.Button>
                    </Radio.Group>
                  </Form.Item>

                  {intervalMode === 'simple' ? (<Form.Item label="更新间隔">
                    <Input.Group compact>
                      <Form.Item
                        name="simple_interval_value"
                        noStyle
                        rules={[{required: true}]}
                      >
                        <InputNumber
                          min={1}
                          style={{width: '100px'}}
                          onChange={handleSimpleIntervalChange}
                        />
                      </Form.Item>
                      <span style={{padding: '0 8px', lineHeight: '32px'}}>每</span>
                      <Form.Item
                        name="simple_interval_unit"
                        noStyle
                      >
                        <Select
                          style={{width: '100px'}}
                          onChange={handleSimpleIntervalChange}
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
                      <Typography.Text type="secondary" style={{display: 'block', marginTop: 4}}>
                        注：周模式固定为每周日执行。
                      </Typography.Text>)}
                    {/* 隐藏的真实字段，用于提交 */}
                    <Form.Item name={['default_sub', 'interval']} noStyle hidden>
                      <Input/>
                    </Form.Item>
                  </Form.Item>) : (<Form.Item
                    name={['default_sub', 'interval']}
                    label="Cron 表达式"
                    rules={[{required: true}]}
                    help="支持秒级 Cron，例如 '0 0 4 * * *' (每天凌晨4点)"
                  >
                    <Input/>
                  </Form.Item>)}

                  <Form.Item
                    name={['default_sub', 'use_proxy']}
                    valuePropName="checked"
                    label={<span>
                              使用代理更新&nbsp;
                      <Tooltip title="如果不勾选，将直连更新。如果勾选但全局代理未配置，此选项不生效。">
                                <QuestionCircleOutlined/>
                              </Tooltip>
                            </span>}
                  >
                    <Checkbox>使用全局代理</Checkbox>
                  </Form.Item>
                </>)}
              </Form.Item>
            </Card>

            <Card title="自定义 Cron 任务" bordered={false}>
              <Form.List name="cron_jobs">
                {(fields, {add, remove}) => (<>
                  {fields.map(({key, name, ...restField}) => (<Card key={key} size="small" style={{marginBottom: 16}}
                                                                    extra={<DeleteOutlined
                                                                      onClick={() => remove(name)}/>}>
                    <Row gutter={16}>
                      <Col span={12}>
                        <Form.Item
                          {...restField}
                          name={[name, 'name']}
                          label="任务名称"
                          rules={[{required: true}]}
                        >
                          <Input/>
                        </Form.Item>
                      </Col>
                      <Col span={12}>
                        <Form.Item
                          {...restField}
                          name={[name, 'schedule']}
                          label="Cron 表达式"
                          rules={[{required: true}]}
                        >
                          <Input/>
                        </Form.Item>
                      </Col>
                    </Row>
                    <Collapse ghost>
                      <Panel header="测速配置 (TestProxy)" key="test_proxy">
                        <Row gutter={16}>
                          <Col span={4}>
                            <Form.Item {...restField} name={[name, 'test_proxy', 'enable']}
                                       valuePropName="checked" label="启用">
                              <Switch size="small"/>
                            </Form.Item>
                          </Col>
                          <Col span={10}>
                            <Form.Item {...restField} name={[name, 'test_proxy', 'status']} label="状态过滤"
                                       help="逗号分隔，如 0,1">
                              <Input placeholder="0,1"/>
                            </Form.Item>
                          </Col>
                          <Col span={10}>
                            <Form.Item {...restField} name={[name, 'test_proxy', 'concurrent']} label="并发数">
                              <InputNumber min={1}/>
                            </Form.Item>
                          </Col>
                        </Row>
                      </Panel>

                      <Panel header="自动封禁 (AutoBan)" key="auto_ban">
                        <Form.Item {...restField} name={[name, 'auto_ban', 'enable']} valuePropName="checked"
                                   label="启用自动封禁">
                          <Switch size="small"/>
                        </Form.Item>
                        <Row gutter={8}>
                          <Col span={8}>
                            <Form.Item {...restField} name={[name, 'auto_ban', 'success_rate_threshold']}
                                       label="成功率阈值">
                              <InputNumber step={0.1} min={0} max={1} placeholder="0.5" style={{width: '100%'}}/>
                            </Form.Item>
                          </Col>
                          <Col span={8}>
                            <Form.Item {...restField} name={[name, 'auto_ban', 'download_speed_threshold']}
                                       label="下载阈值(KB/s)">
                              <InputNumber min={0} style={{width: '100%'}}/>
                            </Form.Item>
                          </Col>
                          <Col span={8}>
                            <Form.Item {...restField} name={[name, 'auto_ban', 'upload_speed_threshold']}
                                       label="上传阈值(KB/s)">
                              <InputNumber min={0} style={{width: '100%'}}/>
                            </Form.Item>
                          </Col>
                          <Col span={8}>
                            <Form.Item {...restField} name={[name, 'auto_ban', 'ping_threshold']}
                                       label="延迟阈值(ms)">
                              <InputNumber min={0} style={{width: '100%'}}/>
                            </Form.Item>
                          </Col>
                          <Col span={8}>
                            <Form.Item {...restField} name={[name, 'auto_ban', 'test_times']} label="测试次数">
                              <InputNumber min={1} style={{width: '100%'}}/>
                            </Form.Item>
                          </Col>
                        </Row>
                      </Panel>

                      <Panel header="IP 检测配置 (IPCheck)" key="ip_check">
                        <Form.Item {...restField} name={[name, 'ip_check', 'enable']} valuePropName="checked"
                                   label="启用检测">
                          <Switch size="small"/>
                        </Form.Item>
                        <Row gutter={8}>
                          <Col span={6}>
                            <Form.Item {...restField} name={[name, 'ip_check', 'ip_info', 'enable']}
                                       valuePropName="checked" label="IP信息">
                              <Checkbox/>
                            </Form.Item>
                          </Col>
                          <Col span={6}>
                            <Form.Item {...restField} name={[name, 'ip_check', 'app_unlock', 'enable']}
                                       valuePropName="checked" label="解锁检测">
                              <Checkbox/>
                            </Form.Item>
                          </Col>
                          <Col span={6}>
                            <Form.Item {...restField} name={[name, 'ip_check', 'refresh']} valuePropName="checked"
                                       label="强制刷新">
                              <Checkbox/>
                            </Form.Item>
                          </Col>
                          <Col span={6}>
                            <Form.Item {...restField} name={[name, 'ip_check', 'concurrent']} label="并发">
                              <InputNumber min={1} size="small"/>
                            </Form.Item>
                          </Col>
                        </Row>
                      </Panel>

                      <Panel header="Webhook 通知" key="webhooks">
                        <Form.List name={[name, 'webhook']}>
                          {(whFields, {add: addWh, remove: removeWh}) => (<>
                            {whFields.map((whField) => (<Card size="small" type="inner" key={whField.key}
                                                              title={`Webhook #${whField.name + 1}`}
                                                              style={{marginBottom: 8}}
                                                              extra={<DeleteOutlined
                                                                onClick={() => removeWh(whField.name)}/>}>
                              <Row gutter={8}>
                                <Col span={12}>
                                  <Form.Item {...whField} name={[whField.name, 'name']} label="名称"
                                             rules={[{required: true}]}>
                                    <Input/>
                                  </Form.Item>
                                </Col>
                                <Col span={12}>
                                  <Form.Item {...whField} name={[whField.name, 'method']} label="方法"
                                             rules={[{required: true}]}>
                                    <Select>
                                      <Option value="GET">GET</Option>
                                      <Option value="POST">POST</Option>
                                      <Option value="PUT">PUT</Option>
                                    </Select>
                                  </Form.Item>
                                </Col>
                                <Col span={24}>
                                  <Form.Item {...whField} name={[whField.name, 'url']} label="URL"
                                             rules={[{required: true}]}>
                                    <Input/>
                                  </Form.Item>
                                </Col>
                                <Col span={24}>
                                  <Form.Item {...whField} name={[whField.name, 'header']}
                                             label="Header (JSON 字符串)">
                                    <Input.TextArea rows={2}
                                                    placeholder='{"Content-Type": "application/json"}'/>
                                  </Form.Item>
                                </Col>
                                <Col span={24}>
                                  <Form.Item {...whField} name={[whField.name, 'body']}
                                             label="Body (模板字符串)">
                                    <Input.TextArea rows={3}/>
                                  </Form.Item>
                                </Col>
                              </Row>
                            </Card>))}
                            <Button type="dashed" onClick={() => addWh()} block icon={<PlusOutlined/>}>添加
                              Webhook</Button>
                          </>)}
                        </Form.List>
                      </Panel>
                    </Collapse>
                  </Card>))}
                  <Form.Item>
                    <Button type="dashed" onClick={() => add()} block icon={<PlusOutlined/>}>
                      添加 Cron 任务
                    </Button>
                  </Form.Item>
                </>)}
              </Form.List>
            </Card>
          </TabPane>
        </Tabs>
      </Form>
    </Content>
  </Layout>);
};

export default ConfigPage;