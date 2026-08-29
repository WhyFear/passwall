import {createRequestGeneration} from './requestGeneration';

test('only the latest request generation remains current', () => {
  const requests = createRequestGeneration();
  const first = requests.begin();
  const second = requests.begin();

  expect(requests.isCurrent(first)).toBe(false);
  expect(requests.isCurrent(second)).toBe(true);

  requests.invalidate();
  expect(requests.isCurrent(second)).toBe(false);
});
