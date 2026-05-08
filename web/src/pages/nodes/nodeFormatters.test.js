import {formatRisk, formatSpeed, formatTraffic} from './nodeFormatters';

describe('node formatters', () => {
  test('formats speed units', () => {
    expect(formatSpeed(0)).toBe('-');
    expect(formatSpeed(1024)).toBe('1.00KB/s');
    expect(formatSpeed(1024 * 1024)).toBe('1.00MB/s');
  });

  test('formats traffic units', () => {
    expect(formatTraffic(0)).toBe('-');
    expect(formatTraffic(1024)).toBe('1.00KB');
    expect(formatTraffic(1024 * 1024)).toBe('1.00MB');
  });

  test('formats risk labels', () => {
    expect(formatRisk('very_low')).toBe('非常低');
    expect(formatRisk('high')).toBe('高');
    expect(formatRisk('unknown')).toBe('-');
  });
});
