export const normalizeShareMultiValue = (value) => {
  if (Array.isArray(value)) {
    return value
      .map(item => String(item).trim())
      .filter(Boolean);
  }

  if (typeof value === 'string') {
    if (!value.trim()) return [];
    return value
      .split(',')
      .map(item => item.trim())
      .filter(Boolean);
  }

  if (value == null) {
    return [];
  }

  const normalized = String(value).trim();
  return normalized ? [normalized] : [];
};

export const joinShareValue = (value) => {
  if (!Array.isArray(value)) return value || '';
  return value.join(',');
};

export const buildShareDefaults = (filters = {}, sorter = {}) => ({
  name: '节点分享',
  type: 'share_link',
  status: normalizeShareMultiValue(filters.status),
  proxy_type: normalizeShareMultiValue(filters.type),
  country_code: normalizeShareMultiValue(filters.country),
  risk_level: normalizeShareMultiValue(filters.risk),
  sort: sorter.field || 'download_speed',
  sort_order: sorter.order || 'descend',
  limit: 0,
  with_index: true,
});

export const buildSharePayload = (values, enabled) => {
  const payload = {
    name: values.name,
    type: values.type,
    status: joinShareValue(values.status),
    proxy_type: joinShareValue(values.proxy_type),
    country_code: joinShareValue(values.country_code),
    risk_level: joinShareValue(values.risk_level),
    sort: values.sort,
    sort_order: values.sort_order,
    limit: values.limit ?? 0,
    with_index: values.with_index,
  };

  if (typeof enabled === 'boolean') {
    payload.enabled = enabled;
  }

  return payload;
};
