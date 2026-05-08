import {buildNodeListParams} from './nodeQueryUtils';

describe('node query utils', () => {
  test('builds list params from table state', () => {
    expect(buildNodeListParams(
      2,
      50,
      {field: 'download_speed', order: 'descend'},
      {
        status: [1, 2],
        type: ['ss', 'trojan'],
        country: ['US', 'JP'],
        risk: ['low', 'high'],
      },
    )).toEqual({
      page: 2,
      pageSize: 50,
      sortField: 'download_speed',
      sortOrder: 'descend',
      status: '1,2',
      type: 'ss,trojan',
      country_code: 'US,JP',
      risk_level: 'low,high',
    });
  });

  test('omits empty filters', () => {
    expect(buildNodeListParams(1, 10, {field: 'id', order: 'ascend'}, {})).toEqual({
      page: 1,
      pageSize: 10,
      sortField: 'id',
      sortOrder: 'ascend',
    });
  });
});
