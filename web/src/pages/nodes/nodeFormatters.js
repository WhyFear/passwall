export const DEFAULT_VISIBLE_COLUMNS = [
  'index',
  'subscription_url',
  'name',
  'address',
  'type',
  'status',
  'ping',
  'download_speed',
  'upload_speed',
  'latest_test_time',
  'success_rate',
  'action',
];

export const formatSpeed = (bytesPerSecond) => {
  if (!bytesPerSecond) return '-';

  const units = ['B/s', 'KB/s', 'MB/s', 'GB/s', 'TB/s'];
  let unit = 0;
  let speed = bytesPerSecond;

  while (speed >= 1024 && unit < units.length - 1) {
    speed /= 1024;
    unit++;
  }

  return `${speed.toFixed(2)}${units[unit]}`;
};

export const formatRisk = (risk) => {
  if (!risk) return '-';
  switch (risk) {
    case 'very_low':
      return '非常低';
    case 'low':
      return '低';
    case 'medium':
      return '中';
    case 'high':
      return '高';
    case 'very_high':
      return '非常高';
    default:
      return '-';
  }
};

export const formatTraffic = (bytes) => {
  if (!bytes) return '-';

  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let unit = 0;
  let traffic = bytes;

  while (traffic >= 1024 && unit < units.length - 1) {
    traffic /= 1024;
    unit++;
  }

  return `${traffic.toFixed(2)}${units[unit]}`;
};
