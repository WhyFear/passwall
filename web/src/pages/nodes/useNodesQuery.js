import {useCallback, useEffect, useMemo, useRef, useState} from 'react';
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

const normalizeMetadataIncludes = (includes = ['success_rate']) => {
  const normalized = Array.isArray(includes) ? includes.filter(Boolean) : ['success_rate'];
  return [...new Set(normalized)].sort();
};

const withMetadataLoading = (items, hasMetadataRequest) => {
  if (!hasMetadataRequest) return items;
  return items.map(item => ({
    ...item,
    metadata_loading: true,
    metadata_error: false,
  }));
};

export const useNodesQuery = (api = subscriptionApi, options = {}) => {
  const [nodes, setNodes] = useState([]);
  const [loading, setLoading] = useState(false);
  const [pagination, setPagination] = useState(DEFAULT_NODE_PAGINATION);
  const [sorter, setSorter] = useState(DEFAULT_NODE_SORTER);
  const [filters, setFilters] = useState({});
  const abortRef = useRef(null);
  const metadataAbortRef = useRef(null);
  const nodesRef = useRef([]);
  const metadataIncludeKey = normalizeMetadataIncludes(options.metadataIncludes).join(',');
  const metadataIncludes = useMemo(
    () => metadataIncludeKey ? metadataIncludeKey.split(',') : [],
    [metadataIncludeKey],
  );
  const metadataIncludesRef = useRef(metadataIncludes);

  // Cancel in-flight requests on unmount.
  useEffect(() => {
    return () => {
      if (abortRef.current) {
        abortRef.current.abort();
      }
      if (metadataAbortRef.current) {
        metadataAbortRef.current.abort();
      }
    };
  }, []);

  useEffect(() => {
    nodesRef.current = nodes;
  }, [nodes]);

  useEffect(() => {
    metadataIncludesRef.current = metadataIncludes;
  }, [metadataIncludes]);

  const fetchNodeMetadata = useCallback(async (nodeList, includes = metadataIncludesRef.current) => {
    if (!api.getProxyMetadata || !Array.isArray(nodeList) || nodeList.length === 0 || includes.length === 0) {
      return;
    }

    if (metadataAbortRef.current) {
      metadataAbortRef.current.abort();
    }
    const controller = new AbortController();
    metadataAbortRef.current = controller;

    const ids = nodeList.map(node => node.id).filter(Boolean);
    if (ids.length === 0) {
      return;
    }

    try {
      const data = await api.getProxyMetadata({
        params: {
          proxy_ids: ids.join(','),
          include: includes.join(','),
        },
        signal: controller.signal,
      });
      if (controller.signal.aborted) return;

      const metadataItems = data?.items || {};
      setNodes(prevNodes => prevNodes.map(node => {
        if (!ids.includes(node.id)) return node;
        const metadata = metadataItems[String(node.id)] || {};
        const nextNode = {
          ...node,
          metadata_loading: false,
          metadata_error: false,
        };
        if (includes.includes('success_rate')) {
          nextNode.success_rate = metadata.success_rate;
        }
        if (includes.includes('ip_info')) {
          nextNode.ip_info = metadata.ip_info;
        }
        return nextNode;
      }));
    } catch (error) {
      if (controller.signal.aborted) return;
      message.error('附加数据加载失败');
      console.error(error);
      setNodes(prevNodes => prevNodes.map(node => (
        ids.includes(node.id)
          ? {...node, metadata_loading: false, metadata_error: true}
          : node
      )));
    }
  }, [api]);

  const fetchNodes = useCallback(async (page, pageSize, sort, filter) => {
    // Cancel any previous in-flight request.
    if (abortRef.current) {
      abortRef.current.abort();
    }
    if (metadataAbortRef.current) {
      metadataAbortRef.current.abort();
    }
    const controller = new AbortController();
    abortRef.current = controller;

    try {
      setLoading(true);
      const params = buildNodeListParams(page, pageSize, sort, filter);
      const data = await api.getProxies({params, signal: controller.signal});
      if (controller.signal.aborted) return;
      const nodeList = Array.isArray(data.items) ? data.items : [];
      const includes = metadataIncludesRef.current;
      const hasMetadataRequest = Boolean(api.getProxyMetadata) && nodeList.length > 0 && includes.length > 0;
      setNodes(withMetadataLoading(nodeList, hasMetadataRequest));
      setPagination(prev => ({
        ...prev,
        current: page,
        pageSize: pageSize,
        total: data.total || nodeList.length,
      }));
      if (hasMetadataRequest) {
        fetchNodeMetadata(nodeList, includes);
      }
    } catch (error) {
      if (controller.signal.aborted) return;
      message.error('获取节点列表失败');
      console.error(error);
    } finally {
      if (!controller.signal.aborted) {
        setLoading(false);
      }
    }
  }, [api, fetchNodeMetadata]);

  useEffect(() => {
    const currentNodes = nodesRef.current;
    if (!currentNodes.length || !api.getProxyMetadata) return;
    fetchNodeMetadata(currentNodes, metadataIncludes);
  }, [api.getProxyMetadata, fetchNodeMetadata, metadataIncludes]);

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
    fetchNodeMetadata,
    handleTableChange,
  };
};
