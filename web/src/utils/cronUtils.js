/**
 * 解析 Cron 表达式并返回 Simple/Advanced 模式及对应的值
 * @param {string} interval Cron 表达式
 * @returns {object} { mode, value, unit }
 */
export const parseCronToSimple = (interval) => {
  if (!interval) return { mode: 'advanced', value: 1, unit: 'hours' };

  const matchSeconds = interval.match(/^\*\/(\d+)\s+\*\s+\*\s+\*\s+\*\s+\*$/);
  const matchMinutes = interval.match(/^0\s+\*\/(\d+)\s+\*\s+\*\s+\*\s+\*$/);
  const matchHours = interval.match(/^0\s+0\s+\*\/(\d+)\s+\*\s+\*\s+\*$/);
  const matchDays = interval.match(/^0\s+0\s+0\s+\*\/(\d+)\s+\*\s+\*$/);
  const matchMonths = interval.match(/^0\s+0\s+0\s+1\s+\*\/(\d+)\s+\*$/);

  let mode = 'advanced';
  let value = 1;
  let unit = 'hours';

  if (matchSeconds) {
    mode = 'simple';
    value = parseInt(matchSeconds[1]);
    unit = 'seconds';
  } else if (matchMinutes) {
    mode = 'simple';
    value = parseInt(matchMinutes[1]);
    unit = 'minutes';
  } else if (matchHours) {
    mode = 'simple';
    value = parseInt(matchHours[1]);
    unit = 'hours';
  } else if (matchDays) {
    mode = 'simple';
    value = parseInt(matchDays[1]);
    unit = 'days';
  } else if (matchMonths) {
    mode = 'simple';
    value = parseInt(matchMonths[1]);
    unit = 'months';
  } else if (interval === '0 0 0 * * 0') {
    mode = 'simple';
    value = 1;
    unit = 'weeks';
  } else if (interval === '0 0 0 1 * *') {
    mode = 'simple';
    value = 1;
    unit = 'months';
  } else if (interval === '0 0 0 * * *') {
    mode = 'simple';
    value = 1;
    unit = 'days';
  }

  const matchEvery = interval.match(/^@every\s+(\d+)([smh])$/);
  if (matchEvery) {
    mode = 'simple';
    value = parseInt(matchEvery[1]);
    const u = matchEvery[2];
    if (u === 's') unit = 'seconds';
    if (u === 'm') unit = 'minutes';
    if (u === 'h') unit = 'hours';
  }

  return { mode, value, unit };
};

/**
 * 生成 Cron 表达式
 * @param {number} value 数值
 * @param {string} unit 单位
 * @returns {string} Cron 表达式
 */
export const generateCronFromSimple = (value, unit) => {
  const val = value || 1;
  switch (unit) {
    case 'seconds':
      return `*/${val} * * * * *`;
    case 'minutes':
      return `0 */${val} * * * *`;
    case 'hours':
      return `0 0 */${val} * * *`;
    case 'days':
      return val === 1 ? `0 0 0 * * *` : `0 0 0 */${val} * *`;
    case 'weeks':
      return `0 0 0 * * 0`;
    case 'months':
      return val === 1 ? `0 0 0 1 * *` : `0 0 0 1 */${val} *`;
    default:
      return `0 0 4 * * *`;
  }
};
