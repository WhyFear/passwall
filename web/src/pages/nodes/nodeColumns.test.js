import {ALL_NODE_COLUMNS, createColumnSettingMenu, createNodeColumns} from './nodeColumns';
import {DEFAULT_VISIBLE_COLUMNS} from './nodeFormatters';

const allVisible = ALL_NODE_COLUMNS.reduce((result, column) => ({
  ...result,
  [column.key]: true,
}), {});

const handlers = {
  onViewNode: jest.fn(),
  onTestProxy: jest.fn(),
  onDetectIP: jest.fn(),
  onPinProxy: jest.fn(),
  onBanProxy: jest.fn(),
};

describe('node columns', () => {
  test('builds expected sortable and filterable table columns', () => {
    const columns = createNodeColumns({
      visibleColumns: allVisible,
      nodeTypes: [{text: 'trojan', value: 'trojan'}],
      countryCodes: [{text: 'US', value: 'US'}],
      isMobile: false,
      ...handlers,
    });

    expect(columns.map(column => column.key)).toEqual([
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
      'risk',
      'country',
      'action',
    ]);
    expect(columns.find(column => column.key === 'type').filters).toEqual([{text: 'trojan', value: 'trojan'}]);
    expect(columns.find(column => column.key === 'country').filters).toEqual([{text: 'US', value: 'US'}]);
    expect(columns.find(column => column.key === 'download_speed').defaultSortOrder).toBe('descend');
    expect(columns.find(column => column.key === 'action').fixed).toBe('right');
  });

  test('hides unchecked columns and unfixes actions on mobile', () => {
    const visibleColumns = {...allVisible, ping: false, action: true};

    const columns = createNodeColumns({
      visibleColumns,
      nodeTypes: [],
      countryCodes: [],
      isMobile: true,
      ...handlers,
    });

    expect(columns.some(column => column.key === 'ping')).toBe(false);
    expect(columns.find(column => column.key === 'type').filters).toEqual([
      {text: 'vmess', value: 'vmess'},
      {text: 'vless', value: 'vless'},
    ]);
    expect(columns.find(column => column.key === 'action').fixed).toBeUndefined();
  });

  test('renders metadata cells as loading, empty, and loaded values', () => {
    const columns = createNodeColumns({
      visibleColumns: allVisible,
      nodeTypes: [],
      countryCodes: [],
      isMobile: false,
      ...handlers,
    });

    const successColumn = columns.find(column => column.key === 'success_rate');
    const riskColumn = columns.find(column => column.key === 'risk');
    const countryColumn = columns.find(column => column.key === 'country');

    expect(successColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(successColumn.render(null, {metadata_loading: false})).toBe('-');
    expect(successColumn.render(0, {metadata_loading: false})).toBe('0%');
    expect(successColumn.render(80, {metadata_loading: false})).toBe('80%');
    expect(riskColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(riskColumn.render('low', {metadata_loading: false})).toBe('低');
    expect(countryColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(countryColumn.render('US', {metadata_loading: false})).toBe('US');
  });

  test('builds column setting items from default visibility', () => {
    const onColumnVisibilityChange = jest.fn();

    const items = createColumnSettingMenu({
      visibleColumns: {},
      onColumnVisibilityChange,
    });

    expect(items).toHaveLength(ALL_NODE_COLUMNS.length);
    expect(items.find(item => item.key === 'index').label.props.disabled).toBe(true);
    expect(items.find(item => item.key === 'download_speed').label.props.checked)
      .toBe(DEFAULT_VISIBLE_COLUMNS.includes('download_speed'));
  });
});
