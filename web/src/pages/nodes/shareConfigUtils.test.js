import {
  buildShareDefaults,
  buildSharePayload,
  joinShareValue,
  normalizeShareMultiValue,
} from './shareConfigUtils';

describe('share config utils', () => {
  describe('normalizeShareMultiValue', () => {
    test('returns empty array for empty values', () => {
      expect(normalizeShareMultiValue(undefined)).toEqual([]);
      expect(normalizeShareMultiValue(null)).toEqual([]);
      expect(normalizeShareMultiValue('')).toEqual([]);
      expect(normalizeShareMultiValue('   ')).toEqual([]);
    });

    test('normalizes comma separated strings', () => {
      expect(normalizeShareMultiValue('1')).toEqual(['1']);
      expect(normalizeShareMultiValue('1,2')).toEqual(['1', '2']);
      expect(normalizeShareMultiValue(' 1, 2 , ,3 ')).toEqual(['1', '2', '3']);
    });

    test('normalizes arrays with mixed scalar types', () => {
      expect(normalizeShareMultiValue([1])).toEqual(['1']);
      expect(normalizeShareMultiValue([1, 2])).toEqual(['1', '2']);
      expect(normalizeShareMultiValue(['1', ' 2 ', '', '3'])).toEqual(['1', '2', '3']);
    });

    test('wraps non-array scalars as a single string value', () => {
      expect(normalizeShareMultiValue(1)).toEqual(['1']);
      expect(normalizeShareMultiValue(true)).toEqual(['true']);
    });
  });

  describe('joinShareValue', () => {
    test('joins arrays and preserves existing scalar fallback behavior', () => {
      expect(joinShareValue(['1', '2'])).toBe('1,2');
      expect(joinShareValue([])).toBe('');
      expect(joinShareValue('1')).toBe('1');
      expect(joinShareValue(undefined)).toBe('');
    });
  });

  describe('buildShareDefaults', () => {
    test('normalizes filter values and keeps expected defaults', () => {
      expect(buildShareDefaults(
        {
          status: [1],
          type: ['trojan'],
          country: ['US'],
          risk: ['low'],
        },
        {
          field: 'ping',
          order: 'ascend',
        },
      )).toEqual({
        name: '节点分享',
        type: 'share_link',
        status: ['1'],
        proxy_type: ['trojan'],
        country_code: ['US'],
        risk_level: ['low'],
        sort: 'ping',
        sort_order: 'ascend',
        limit: 0,
        with_index: true,
      });
    });

    test('falls back to default sort values and empty filters', () => {
      expect(buildShareDefaults()).toEqual({
        name: '节点分享',
        type: 'share_link',
        status: [],
        proxy_type: [],
        country_code: [],
        risk_level: [],
        sort: 'download_speed',
        sort_order: 'descend',
        limit: 0,
        with_index: true,
      });
    });
  });

  describe('buildSharePayload', () => {
    test('serializes multi-select values for API payloads', () => {
      expect(buildSharePayload({
        name: '分享配置',
        type: 'share_link',
        status: ['1', '2'],
        proxy_type: ['trojan'],
        country_code: ['US', 'JP'],
        risk_level: ['low'],
        sort: 'ping',
        sort_order: 'ascend',
        limit: 5,
        with_index: true,
      })).toEqual({
        name: '分享配置',
        type: 'share_link',
        status: '1,2',
        proxy_type: 'trojan',
        country_code: 'US,JP',
        risk_level: 'low',
        sort: 'ping',
        sort_order: 'ascend',
        limit: 5,
        with_index: true,
      });
    });

    test('keeps empty arrays empty and forwards enabled when provided', () => {
      expect(buildSharePayload({
        name: '分享配置',
        type: 'clash',
        status: [],
        proxy_type: [],
        country_code: [],
        risk_level: [],
        sort: 'download_speed',
        sort_order: 'descend',
        limit: undefined,
        with_index: false,
      }, true)).toEqual({
        name: '分享配置',
        type: 'clash',
        status: '',
        proxy_type: '',
        country_code: '',
        risk_level: '',
        sort: 'download_speed',
        sort_order: 'descend',
        limit: 0,
        with_index: false,
        enabled: true,
      });
    });
  });
});
