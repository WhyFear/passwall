export const buildNodeListParams = (page, pageSize, sort, filter = {}) => {
  const params = {
    page,
    pageSize,
    sortField: sort.field,
    sortOrder: sort.order,
  };

  if (filter.status && filter.status.length > 0) {
    params.status = filter.status.join(',');
  }

  if (filter.type && filter.type.length > 0) {
    params.type = filter.type.join(',');
  }

  if (filter.country && filter.country.length > 0) {
    params.country_code = filter.country.join(',');
  }

  if (filter.risk && filter.risk.length > 0) {
    params.risk_level = filter.risk.join(',');
  }

  if (filter.app_unlock && filter.app_unlock.length > 0) {
    params.app_unlock = filter.app_unlock.join(',');
  }

  return params;
};
