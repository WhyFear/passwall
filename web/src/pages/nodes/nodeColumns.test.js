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
      unlockApps: [{text: 'Netflix', value: 'Netflix'}],
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
      'app_unlock',
      'action',
    ]);
    expect(columns.find(column => column.key === 'type').filters).toEqual([{text: 'trojan', value: 'trojan'}]);
    expect(columns.find(column => column.key === 'country').filters).toEqual([{text: 'US', value: 'US'}]);
    expect(columns.find(column => column.key === 'app_unlock').filters).toEqual([{text: 'Netflix', value: 'Netflix'}]);
    expect(columns.find(column => column.key === 'app_unlock').sorter).toBeUndefined();
    expect(columns.find(column => column.key === 'download_speed').defaultSortOrder).toBe('descend');
    expect(columns.find(column => column.key === 'action').fixed).toBe('right');
  });

  test('hides unchecked columns and unfixes actions on mobile', () => {
    const visibleColumns = {...allVisible, ping: false, action: true};

    const columns = createNodeColumns({
      visibleColumns,
      nodeTypes: [],
      countryCodes: [],
      unlockApps: [],
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
      unlockApps: [],
      isMobile: false,
      ...handlers,
    });

    const successColumn = columns.find(column => column.key === 'success_rate');
    const riskColumn = columns.find(column => column.key === 'risk');
    const countryColumn = columns.find(column => column.key === 'country');
    const appUnlockColumn = columns.find(column => column.key === 'app_unlock');

    expect(successColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(successColumn.render(null, {metadata_loading: false})).toBe('-');
    expect(successColumn.render(0, {metadata_loading: false})).toBe('0%');
    expect(successColumn.render(80, {metadata_loading: false})).toBe('80%');
    expect(riskColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(riskColumn.render('low', {metadata_loading: false})).toBe('低');
    expect(countryColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(countryColumn.render('US', {metadata_loading: false})).toBe('US');
    expect(appUnlockColumn.render(null, {metadata_loading: true}).props.active).toBe(true);
    expect(appUnlockColumn.render(undefined, {metadata_loading: false})).toBe('-');
    expect(appUnlockColumn.render([], {metadata_loading: false})).toBe('-');

    const lockedOnlyAppUnlock = appUnlockColumn.render([
      {app_name: 'Netflix', status: 'fail'},
      {app_name: 'OpenAI', status: 'forbidden'},
    ], {metadata_loading: false});
    const lockedOnlyPopover = lockedOnlyAppUnlock.type(lockedOnlyAppUnlock.props);
    expect(lockedOnlyPopover.props.trigger).toBe('hover');
    expect(lockedOnlyPopover.props.children.props.children[1].props.children).toBe(0);
    expect(lockedOnlyPopover.props.children.props.children[2].props.children).toEqual(['/ ', 2]);

    const loadedAppUnlock = appUnlockColumn.render([
      {app_name: 'Netflix', status: 'unlock'},
      {app_name: 'Netflix', status: 'unlock'},
      {app_name: 'OpenAI', status: 'unlock'},
      {app_name: 'Claude', status: 'fail'},
    ], {metadata_loading: false});
    const loadedPopover = loadedAppUnlock.type(loadedAppUnlock.props);
    expect(loadedPopover.props.trigger).toBe('hover');
    expect(loadedPopover.props.placement).toBe('topLeft');
    expect(loadedPopover.props.children.props.children[0].props.children).toBe('已解锁');
    expect(loadedPopover.props.children.props.children[1].props.children).toBe(2);
    expect(loadedPopover.props.children.props.children[2].props.children).toEqual(['/ ', 4]);

    const details = loadedPopover.props.content.type(loadedPopover.props.content.props);
    const detailList = details.props.children[1];
    expect(details.props.children[0].props.children[0].props.children).toBe('应用解锁详情');
    expect(details.props.children[0].props.children[1].props.children).toEqual([4, ' 项检测']);
    const firstAppDetail = detailList.props.children[0];
    expect(firstAppDetail.props.children[0].props.children).toBe('Netflix');
    expect(firstAppDetail.props.style.gridTemplateColumns).toBe('minmax(0, 1fr) 58px 44px');
    expect(firstAppDetail.props.children[1].props.children.props.status).toBe('unlock');
    expect(firstAppDetail.props.children[2].props.children).toBe('-');
  });

  test('renders base cells and action buttons', () => {
    const columns = createNodeColumns({
      visibleColumns: allVisible,
      nodeTypes: [],
      countryCodes: [],
      unlockApps: ['Netflix'],
      isMobile: false,
      ...handlers,
    });

    expect(columns.find(column => column.key === 'index').render(null, null, 3)).toBe(4);
    expect(columns.find(column => column.key === 'status').render(1).props.status).toBe(1);
    expect(columns.find(column => column.key === 'ping').render(25)).toBe('25ms');
    expect(columns.find(column => column.key === 'ping').render(0)).toBe('-');
    expect(columns.find(column => column.key === 'download_speed').render(1024)).toBe('1.00KB/s');
    expect(columns.find(column => column.key === 'upload_speed').render(2048)).toBe('2.00KB/s');
    expect(columns.find(column => column.key === 'latest_test_time').render(null)).toBe('-');

    const action = columns.find(column => column.key === 'action').render(null, {id: 7, pinned: false});
    const buttons = action.props.children.map(tooltip => tooltip.props.children);
    buttons[0].props.onClick();
    buttons[1].props.onClick();
    buttons[2].props.onClick();
    buttons[3].props.onClick();
    buttons[4].props.onClick();

    expect(handlers.onViewNode).toHaveBeenCalledWith({id: 7, pinned: false});
    expect(handlers.onTestProxy).toHaveBeenCalledWith(7);
    expect(handlers.onDetectIP).toHaveBeenCalledWith(7);
    expect(handlers.onPinProxy).toHaveBeenCalledWith(7, false);
    expect(handlers.onBanProxy).toHaveBeenCalledWith(7);
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
    expect(items.find(item => item.key === 'app_unlock').label.props.checked).toBe(false);

    items.find(item => item.key === 'app_unlock').label.props.onChange({
      target: {checked: true},
    });
    const clickEvent = {stopPropagation: jest.fn()};
    items.find(item => item.key === 'app_unlock').label.props.onClick(clickEvent);

    expect(onColumnVisibilityChange).toHaveBeenCalledWith('app_unlock', true);
    expect(clickEvent.stopPropagation).toHaveBeenCalledTimes(1);
  });
});
