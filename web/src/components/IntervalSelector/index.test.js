import IntervalSelector from './index';

test.each([
  'update_interval',
  ['default_sub', 'interval'],
])('simple mode updates the exact field path %#j', fieldName => {
  const form = {
    getFieldValue: jest.fn(name => name === 'simple_interval_value' ? 2 : 'hours'),
    setFieldValue: jest.fn(),
  };
  const element = IntervalSelector({form, fieldName, mode: 'advanced', setMode: jest.fn()});
  const radioGroup = element.props.children[0].props.children;

  radioGroup.props.onChange({target: {value: 'simple'}});

  expect(form.setFieldValue).toHaveBeenCalledWith(fieldName, '0 0 */2 * * *');
});
