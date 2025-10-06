import axios from 'axios';
import config from '../config';
import {clearToken} from '../utils/tokenUtils';

const API_BASE_URL = config.apiBaseUrl;

// 创建一个自定义事件，用于通知App组件显示Token弹窗
export const tokenEvents = {
  emitTokenInvalid: () => {
    const event = new CustomEvent('token-invalid');
    window.dispatchEvent(event);
  }
};

// 创建axios实例
const api = axios.create({
  baseURL: API_BASE_URL, timeout: 10000, headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
api.interceptors.request.use((config) => {
  const token = localStorage.getItem('token');
  if (token) {
    // 把token添加到请求头中
    config.headers.Authorization = `Bearer ${token}`;
  } else {
    // 如果没有token，可以在这里添加逻辑
    console.warn('请求未携带Token');
    // 触发自定义事件，通知需要显示Token弹窗
    tokenEvents.emitTokenInvalid();
  }
  return config;
}, (error) => {
  return Promise.reject(error);
});

// 响应拦截器
api.interceptors.response.use((response) => {
  return response.data;
}, (error) => {
  console.error('API请求错误:', error);

  // 如果响应状态码是401（未授权），可能是token无效或过期
  if (error.response && error.response.status === 401) {
    console.error('Token无效或已过期');
    // 清除无效的token
    clearToken();
    // 触发自定义事件，通知需要显示Token弹窗
    tokenEvents.emitTokenInvalid();
  }
  return Promise.reject(error);
});

// 订阅相关API
export const subscriptionApi = {
  // 获取所有订阅链接
  getSubscriptions: (params) => api.get('/subscriptions', params),

  // 获取订阅详情
  getSubscriptionDetail: (id, content = true) => api.get(`/subscriptions?id=${id}&content=${content}`),

  // 获取所有代理节点
  getProxies: (params) => api.get('/get_proxies', params),

  createProxy: (data) => api.post('/create_proxy', data),

  createProxyWithFormData: (formData) => api.post('/create_proxy', formData, {
    headers: {
      'Content-Type': 'multipart/form-data',
    },
  }),

  reloadSubs: (params) => api.post('/reload_subscription', params),

  // 删除订阅
  deleteSubscription: (id) => api.post('/delete_subscription', {id: id}),
};

// 节点相关API
export const nodeApi = {
  // 获取代理历史
  getProxyHistory: (id, page = 1, pageSize = 5) => api.get(`/proxy/${id}/history`, {
    params: {
      page: page, pageSize: pageSize
    }
  }), // 获取代理分享链接
  getProxyShareUrl: (id) => api.get(`/subscribe?type=share_link&id=${id}`),
  getTypes: () => api.get('/get_types'),
  testProxy: (params) => api.post('/test_proxy_server', params),
  pinProxy: (id, pinned) => api.post('/pin_proxy', {id: id, pinned: pinned}),
  banProxy: (params) => api.post(`/ban_proxy`, params),
  detectIP: (params) => api.post(`/detect_ip`, params),
  getIPInfo: (params) => api.post(`/get_ip_info`, params),
  getCountryCodes: () => api.get('/get_country_codes'),
};

// 任务相关API
export const taskApi = {
  // 获取任务状态
  getTaskStatus: (taskType) => api.get('/get_task_status', {params: {task_type: taskType}}), // 停止任务
  stopTask: (taskType) => api.post('/stop_task', {task_type: taskType}),
}; 