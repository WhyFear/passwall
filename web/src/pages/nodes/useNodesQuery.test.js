jest.mock('../../api', () => ({
  subscriptionApi: {
    getProxies: jest.fn(() => Promise.resolve({items: [], total: 0})),
  },
}));

import React, {createElement} from 'react';
import {createRoot} from 'react-dom/client';
import {act} from 'react-dom/test-utils';
import {useNodesQuery, DEFAULT_NODE_PAGINATION, DEFAULT_NODE_SORTER} from './useNodesQuery';
import {subscriptionApi} from '../../api';

let container;

beforeEach(() => {
  jest.clearAllMocks();
  container = document.createElement('div');
  document.body.appendChild(container);
});

afterEach(() => {
  document.body.removeChild(container);
  container = null;
});

const mountHook = (api) => {
  let hookResult;
  const TestComponent = () => {
    hookResult = useNodesQuery(api);
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
    unmount: () => act(() => root.unmount()),
  };
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
