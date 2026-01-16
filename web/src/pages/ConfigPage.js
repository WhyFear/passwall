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
  message,
  Row,
  Select,
  Space,
  Switch,
  Tabs,
  Tooltip
} from 'antd';
import {DeleteOutlined, PlusOutlined, QuestionCircleOutlined, SaveOutlined} from '@ant-design/icons';
import {configApi} from '../api';
import IntervalSelector from '../components/IntervalSelector';
import {parseCronToSimple} from '../utils/cronUtils';

const {Panel} = Collapse;

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
      const {mode, value, unit} = parseCronToSimple(data.default_sub.interval);
      setIntervalMode(mode);
      form.setFieldsValue({
        simple_interval_value: value, simple_interval_unit: unit
      });

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

  return (<div>
    <Form
      form={form}
      layout="vertical"
      onFinish={onFinish}
      initialValues={{
        concurrent: 5,
        proxy: {enabled: false},
        ip_check: {
          enable: false,
          concurrent: 10,
          refresh: false,
          app_unlock: {enable: false},
          ip_info: {enable: false}
        },
        clash_api: {enable: false, clients: []},
        cron_jobs: [],
        default_sub: {auto_update: false, interval: "0 0 4 * * *", use_proxy: false},
        simple_interval_value: 1,
        simple_interval_unit: 'hours'
      }}
    >
      <Tabs
        defaultActiveKey="1"
        tabBarExtraContent={<div className="tab-bar-extra"
                                 style={{display: 'flex', alignItems: 'center', gap: 8, flexWrap: 'wrap'}}>
          <Button type="primary" icon={<SaveOutlined/>} onClick={form.submit} loading={loading}>
            保存配置
          </Button>
        </div>}
      >
        <TabPane tab="通用设置" key="1">
          <Card title="基础设置" variant={"borderless"} style={{marginBottom: 16}}>
            <Form.Item
              label="并发数"
              name="concurrent"
              rules={[{required: true, message: '请输入并发数'}]}
              help="全局任务并发限制"
            >
              <InputNumber min={1} max={100}/>
            </Form.Item>
          </Card>

          <Card title="全局代理" variant={"borderless"} style={{marginBottom: 16}}>
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
          <Card title="IP检测配置" variant={"borderless"}>
            <Form.Item name={['ip_check', 'enable']} valuePropName="checked" label="启用IP检测">
              <Switch/>
            </Form.Item>
            <Form.Item name={['ip_check', 'ip_info', 'enable']} valuePropName="checked" label="启用IP风险、地区信息检测">
              <Switch/>
            </Form.Item>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name={['ip_check', 'concurrent']} label="检测并发数">
                  <InputNumber min={1} max={100}/>
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
          <Card title="Clash API 设置" variant={"borderless"}>
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
          <Card title="默认订阅更新配置" variant={"borderless"} style={{marginBottom: 16}}>
            <Form.Item name={['default_sub', 'auto_update']} valuePropName="checked" label="自动更新订阅">
              <Switch/>
            </Form.Item>
            <Form.Item
              noStyle
              shouldUpdate={(prev, current) => prev.default_sub?.auto_update !== current.default_sub?.auto_update}
            >
              {({getFieldValue}) => getFieldValue(['default_sub', 'auto_update']) && (<>
                <IntervalSelector
                  form={form}
                  fieldName={['default_sub', 'interval']}
                  mode={intervalMode}
                  setMode={setIntervalMode}
                />
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

          <Card title="自定义 Cron 任务" variant={"borderless"}>
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
  </div>);
};

export default ConfigPage;