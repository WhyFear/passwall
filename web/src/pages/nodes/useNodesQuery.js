import {useCallback, useEffect, useRef, useState} from 'react';
import {message} from 'antd';
import {subscriptionApi} from '../../api';
import {buildNodeListParams} from './nodeQueryUtils';

export const DEFAULT_NODE_PAGINATION = {
  current: 1,
  pageSize: 10,
  total: 0,
};

export const DEFAULT_NODE_SORTER = {
  field: 'download_speed',
  order: 'descend',
};

export const useNodesQuery = (api = subscriptionApi) => {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState(DEFAULT_NODE_PAGINATION);
  const [sorter, setSorter] = useState(DEFAULT_NODE_SORTER);
  const [filters, setFilters] = useState({});
  const abortRef = useRef(null);

  // Cancel in-flight requests on unmount.
  useEffect(() => {
    return () => {
      if (abortRef.current) {
        abortRef.current.abort();
      }
    };
  }, []);

  const fetchNodes = useCallback(async (page, pageSize, sort, filter) => {
    // Cancel any previous in-flight request.
    if (abortRef.current) {
      abortRef.current.abort();
    }
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      setLoading(true);
      const params = buildNodeListParams(page, pageSize, sort, filter);
      const data = await api.getProxies({params, signal: controller.signal});
      if (controller.signal.aborted) return;
      const nodeList = Array.isArray(data.items) ? data.items : [];
      setNodes(nodeList);
      setPagination(prev => ({
        ...prev,
        current: page,
        pageSize: pageSize,
        total: data.total || nodeList.length,
      }));
    } catch (error) {
      if (controller.signal.aborted) return;
      message.error('获取节点列表失败');
      console.error(error);
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, [api]);

  const handleTableChange = useCallback((newPagination, newFilters, newSorter) => {
    const sort = newSorter.field ? {
      field: newSorter.field,
      order: newSorter.order || 'descend',
    } : sorter;

    setSorter(sort);
    setFilters(newFilters);
    fetchNodes(newPagination.current, newPagination.pageSize, sort, newFilters);
  }, [fetchNodes, sorter]);

  return {
    nodes,
    loading,
    pagination,
    sorter,
    filters,
    fetchNodes,
    handleTableChange,
  };
};
