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
    baseURL: API_BASE_URL,
    timeout: 10000,
    headers: {
        'Content-Type': 'application/json',
    },
});

// 请求拦截器
api.interceptors.request.use(
    (config) => {
        const token = localStorage.getItem('token');
        if (token) {
            // 确保config.params存在
            if (!config.params) {
                config.params = {};
            }
            // 将token添加到URL参数中
            config.params['token'] = token;
        } else {
            // 如果没有token，可以在这里添加逻辑
            console.warn('请求未携带Token');
            // 触发自定义事件，通知需要显示Token弹窗
            tokenEvents.emitTokenInvalid();
        }
        return config;
    },
    (error) => {
        return Promise.reject(error);
    }
);

// 响应拦截器
api.interceptors.response.use(
    (response) => {
        return response.data;
    },
    (error) => {
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
    }
);

// 订阅相关API
export const subscriptionApi = {
    // 获取所有订阅链接
    getSubscriptions: () => api.get('/subscriptions'),

    // 获取订阅详情
    getSubscriptionDetail: (id, content = true) => api.get(`/subscriptions?id=${id}&content=${content}`),

    // 创建订阅链接
    getProxies: (data) => api.get('/get_proxies', data),
};

// 节点相关API
export const nodeApi = {
    // 获取代理历史
    getProxyHistory: (id) => api.get(`/proxy/${id}/history`),
}; 