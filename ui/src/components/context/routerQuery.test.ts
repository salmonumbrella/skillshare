import { keepPreviousData } from '@tanstack/react-query';
import { expect, it } from 'vitest';
import { staleTimes } from '../../lib/queryKeys';
import { routerQueryOptions } from './routerQuery';

it('refetches whenever the tab is opened, since other pages change the files it reads', () => {
  const options = routerQueryOptions('/a');
  expect(options.refetchOnMount).toBe('always');
  expect(options.staleTime).toBe(staleTimes.contextRouter);
});

it('keeps the previous folder while the next loads and does not retry a refused folder', () => {
  const options = routerQueryOptions('/a');
  expect(options.placeholderData).toBe(keepPreviousData);
  expect(options.retry).toBe(false);
});

it('caches each folder apart, the default folder included', () => {
  expect(routerQueryOptions('/a').queryKey).not.toEqual(routerQueryOptions('/b').queryKey);
  expect(routerQueryOptions('').queryKey).toEqual(['context-router', '']);
});
