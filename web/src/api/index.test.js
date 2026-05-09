const mockRequestUse = jest.fn();
const mockResponseUse = jest.fn();
const mockApi = {
  get: jest.fn(),
  post: jest.fn(),
  put: jest.fn(),
  interceptors: {
    request: {use: mockRequestUse},
    response: {use: mockResponseUse},
  },
};

jest.mock('axios', () => ({
  create: jest.fn(() => mockApi),
}));

jest.mock('../utils/tokenUtils', () => ({
  clearToken: jest.fn(),
}));

describe('api client', () => {
  let axios;
  let nodeApi;
  let subscriptionApi;
  let tokenEvents;
  let clearToken;
  let warnSpy;
  let errorSpy;

  beforeEach(() => {
    jest.clearAllMocks();
    jest.resetModules();
    localStorage.clear();
    warnSpy = jest.spyOn(console, 'warn').mockImplementation(() => {});
    errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});

    axios = require('axios');
    clearToken = require('../utils/tokenUtils').clearToken;
    const apiModule = require('./index');
    nodeApi = apiModule.nodeApi;
    subscriptionApi = apiModule.subscriptionApi;
    tokenEvents = apiModule.tokenEvents;
  });

  afterEach(() => {
    warnSpy.mockRestore();
    errorSpy.mockRestore();
  });

  test('uses configured API base URL', () => {
    const config = require('../config').default;

    expect(axios.create).toHaveBeenCalledWith(expect.objectContaining({
      baseURL: config.apiBaseUrl,
      timeout: 10000,
    }));
  });

  test('adds bearer token to outgoing requests', () => {
    localStorage.setItem('token', 'secret');
    const interceptor = mockRequestUse.mock.calls[0][0];

    const request = interceptor({headers: {}});

    expect(request.headers.Authorization).toBe('Bearer secret');
  });

  test('emits token invalid event when token is missing', () => {
    const listener = jest.fn();
    window.addEventListener('token-invalid', listener);
    const interceptor = mockRequestUse.mock.calls[0][0];

    interceptor({headers: {}});

    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener('token-invalid', listener);
  });

  test('clears token and emits event on unauthorized response', async () => {
    const listener = jest.fn();
    window.addEventListener('token-invalid', listener);
    const errorInterceptor = mockResponseUse.mock.calls[0][1];

    await expect(errorInterceptor({response: {status: 401}})).rejects.toEqual({response: {status: 401}});

    expect(clearToken).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledTimes(1);
    window.removeEventListener('token-invalid', listener);
  });

  test('keeps node IP info endpoint aligned with backend route', () => {
    nodeApi.getIPInfo({proxy_id: 7});

    expect(mockApi.get).toHaveBeenCalledWith('/get_ip_info', {params: {proxy_id: 7}});
  });

  test('passes subscription pagination as axios params object', () => {
    subscriptionApi.getSubscriptions({params: {page: 2, pageSize: 20}});

    expect(mockApi.get).toHaveBeenCalledWith('/subscriptions', {params: {page: 2, pageSize: 20}});
  });
});
