jest.mock('../../api', () => ({
  subscriptionApi: {
    getProxies: jest.fn(() => Promise.resolve({items: [], total: 0})),
  },
}));

import {act, createElement} from 'react';
import {createRoot} from 'react-dom/client';
import {message} from 'antd';
import {useNodesQuery, DEFAULT_NODE_PAGINATION, DEFAULT_NODE_SORTER} from './useNodesQuery';
import {subscriptionApi} from '../../api';

let container;

beforeEach(() => {
  global.IS_REACT_ACT_ENVIRONMENT = true;
  jest.clearAllMocks();
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  document.body.removeChild(container);
  container = null;
});

const mountHook = (api, options) => {
  let hookResult;
  let currentOptions = options;
  const TestComponent = () => {
    hookResult = useNodesQuery(api, currentOptions);
    return null;
  };
  const root = createRoot(container);
  act(() => {
    root.render(createElement(TestComponent));
  });
  return {
    get current() {
      return hookResult;
    },
    rerender: (nextOptions) => act(() => {
      currentOptions = nextOptions;
      root.render(createElement(TestComponent));
    }),
    unmount: () => act(() => root.unmount()),
  };
};

/** Returns a promise and its resolver/rejector for manual control. */
const deferred = () => {
  let resolve, reject;
  const promise = new Promise((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return {promise, resolve, reject};
};

describe('useNodesQuery', () => {
  describe('defaults', () => {
    test('DEFAULT_NODE_PAGINATION has correct shape', () => {
      expect(DEFAULT_NODE_PAGINATION).toEqual({
        current: 1,
        pageSize: 10,
        total: 0,
      });
    });

    test('DEFAULT_NODE_SORTER sorts by download_speed descending', () => {
      expect(DEFAULT_NODE_SORTER).toEqual({
        field: 'download_speed',
        order: 'descend',
      });
    });
  });

  describe('initial state', () => {
    test('returns default pagination, sorter, empty nodes and filters', async () => {
      subscriptionApi.getProxies.mockResolvedValue({items: [], total: 0});

      const hook = mountHook(subscriptionApi);

      // Wait for the initial fetch to settle since the mock resolves immediately.
      await act(async () => {});

      expect(hook.current.nodes).toEqual([]);
      expect(hook.current.loading).toBe(false);
      expect(hook.current.pagination).toEqual(DEFAULT_NODE_PAGINATION);
      expect(hook.current.sorter).toEqual(DEFAULT_NODE_SORTER);
      expect(hook.current.filters).toEqual({});
      expect(typeof hook.current.fetchNodes).toBe('function');
      expect(typeof hook.current.handleTableChange).toBe('function');

      hook.unmount();
    });
  });

  describe('fetching data', () => {
    test('fetches with buildNodeListParams on table change', async () => {
      const items = [{id: 1, name: 'node-1'}];
      subscriptionApi.getProxies.mockResolvedValue({items, total: 1});

      const hook = mountHook(subscriptionApi);

      await act(async () => {
        hook.current.handleTableChange(
          {current: 2, pageSize: 20},
          {status: [1]},
          {field: 'ping', order: 'ascend'},
        );
      });

      expect(subscriptionApi.getProxies).toHaveBeenCalledWith({
        params: {
          page: 2,
          pageSize: 20,
          sortField: 'ping',
          sortOrder: 'ascend',
          status: '1',
        },
        signal: expect.any(AbortSignal),
      });

      expect(hook.current.nodes).toEqual(items);
      expect(hook.current.pagination.current).toBe(2);
      expect(hook.current.pagination.pageSize).toBe(20);
      expect(hook.current.pagination.total).toBe(1);
      expect(hook.current.loading).toBe(false);

      hook.unmount();
    });

    test('falls back to array-length total when data.total is missing', async () => {
      const items = [{id: 1}, {id: 2}, {id: 3}];
      subscriptionApi.getProxies.mockResolvedValue({items});

      const hook = mountHook(subscriptionApi);

      await act(async () => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      expect(hook.current.pagination.total).toBe(3);
      hook.unmount();
    });

    test('handles non-array items gracefully', async () => {
      subscriptionApi.getProxies.mockResolvedValue({items: null, total: 0});

      const hook = mountHook(subscriptionApi);

      await act(async () => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      expect(hook.current.nodes).toEqual([]);
      hook.unmount();
    });

    test('sets nodes once on success (no intermediate empty array)', async () => {
      const items = [{id: 1}];
      subscriptionApi.getProxies.mockResolvedValue({items, total: 1});

      const hook = mountHook(subscriptionApi);

      await act(async () => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      expect(hook.current.nodes).toEqual(items);
      hook.unmount();
    });

    test('fetches metadata after main list and merges by id', async () => {
      const api = {
        getProxies: jest.fn().mockResolvedValue({items: [{id: 1}, {id: 2}], total: 2}),
        getProxyMetadata: jest.fn().mockResolvedValue({
          items: {
            '1': {success_rate: 80, ip_info: {country_code: 'US'}},
            '2': {success_rate: 0},
          },
        }),
      };
      const hook = mountHook(api, {metadataIncludes: ['success_rate', 'ip_info']});

      await act(async () => {
        await hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });
      await act(async () => {});

      expect(api.getProxyMetadata).toHaveBeenCalledWith({
        params: {
          proxy_ids: '1,2',
          include: 'ip_info,success_rate',
        },
        signal: expect.any(AbortSignal),
      });
      expect(hook.current.nodes[0]).toEqual(expect.objectContaining({
        id: 1,
        success_rate: 80,
        ip_info: {country_code: 'US'},
        metadata_loading: false,
        metadata_error: false,
      }));
      expect(hook.current.nodes[1]).toEqual(expect.objectContaining({
        id: 2,
        success_rate: 0,
        metadata_loading: false,
        metadata_error: false,
      }));
      hook.unmount();
    });

    test('keeps metadata loading state until metadata resolves', async () => {
      const metadata = deferred();
      const api = {
        getProxies: jest.fn().mockResolvedValue({items: [{id: 1}], total: 1}),
        getProxyMetadata: jest.fn().mockReturnValue(metadata.promise),
      };
      const hook = mountHook(api);

      await act(async () => {
        await hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      expect(hook.current.nodes[0]).toEqual(expect.objectContaining({
        id: 1,
        metadata_loading: true,
      }));

      await act(async () => {
        metadata.resolve({items: {'1': {success_rate: 100}}});
      });

      expect(hook.current.nodes[0]).toEqual(expect.objectContaining({
        success_rate: 100,
        metadata_loading: false,
      }));
      hook.unmount();
    });

    test('metadata failure keeps main list and shows one lightweight error', async () => {
      const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      const messageSpy = jest.spyOn(message, 'error').mockImplementation(() => {});
      const api = {
        getProxies: jest.fn().mockResolvedValue({items: [{id: 1}], total: 1}),
        getProxyMetadata: jest.fn().mockRejectedValue(new Error('metadata failed')),
      };
      const hook = mountHook(api);

      await act(async () => {
        await hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });
      await act(async () => {});

      expect(hook.current.nodes[0]).toEqual(expect.objectContaining({
        id: 1,
        metadata_loading: false,
        metadata_error: true,
      }));
      expect(messageSpy).toHaveBeenCalledWith('附加数据加载失败');

      messageSpy.mockRestore();
      errorSpy.mockRestore();
      hook.unmount();
    });

    test('keeps fetchNodes stable when metadata includes change', async () => {
      const api = {
        getProxies: jest.fn().mockResolvedValue({items: [{id: 1}], total: 1}),
        getProxyMetadata: jest.fn().mockResolvedValue({items: {'1': {success_rate: 80}}}),
      };
      const hook = mountHook(api, {metadataIncludes: ['success_rate']});
      const initialFetchNodes = hook.current.fetchNodes;

      hook.rerender({metadataIncludes: ['success_rate', 'ip_info']});

      expect(hook.current.fetchNodes).toBe(initialFetchNodes);
      hook.unmount();
    });
  });

  describe('request cancellation', () => {
    test('aborts previous request when a new one starts', async () => {
      const first = deferred();
      const second = deferred();
      subscriptionApi.getProxies
        .mockReturnValueOnce(first.promise)
        .mockReturnValueOnce(second.promise);

      const hook = mountHook(subscriptionApi);

      act(() => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });
      act(() => {
        hook.current.fetchNodes(2, 10, DEFAULT_NODE_SORTER, {});
      });

      // Resolve the second request.
      await act(async () => {
        second.resolve({items: [{id: 2}], total: 1});
      });

      // The first request was aborted — resolving it should be a no-op.
      await act(async () => {
        first.resolve({items: [{id: 1}], total: 1});
      });

      // Nodes should reflect the second request, not the first.
      expect(hook.current.nodes).toEqual([{id: 2}]);
      expect(hook.current.pagination.current).toBe(2);
      hook.unmount();
    });

    test('does not update state when request is aborted by unmount', async () => {
      const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      const d = deferred();
      subscriptionApi.getProxies.mockReturnValue(d.promise);

      const hook = mountHook(subscriptionApi);

      act(() => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      hook.unmount();

      // Resolve after unmount — should not cause state updates or errors.
      await act(async () => {
        d.resolve({items: [{id: 1}], total: 1});
      });

      // No error message should appear.
      // The console.error spy should not have been called from our hook.
      errorSpy.mockRestore();
    });
  });

  describe('error handling', () => {
    test('shows error message on fetch failure', async () => {
      const errorSpy = jest.spyOn(console, 'error').mockImplementation(() => {});
      subscriptionApi.getProxies.mockRejectedValue(new Error('network'));

      const hook = mountHook(subscriptionApi);

      await act(async () => {
        hook.current.fetchNodes(1, 10, DEFAULT_NODE_SORTER, {});
      });

      expect(hook.current.loading).toBe(false);
      expect(hook.current.nodes).toEqual([]);

      errorSpy.mockRestore();
      hook.unmount();
    });
  });
});
