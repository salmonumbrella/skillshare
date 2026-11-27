import { keepPreviousData } from '@tanstack/react-query';
import { contextRouterApi } from '../../api/contextRouter';
import { queryKeys, staleTimes } from '../../lib/queryKeys';

/**
 * How the Router tab asks for a folder's report.
 *
 * The report is read from files that other pages change (an instruction edit, a sync, a pull), so
 * the tab refetches every time it is opened instead of each of those pages knowing about it. The
 * stale time still spares a refetch while the tab stays open and only re-renders.
 */
export const routerQueryOptions = (root: string) => ({
  queryKey: queryKeys.contextRouter(root),
  queryFn: ({ signal }: { signal: AbortSignal }) => contextRouterApi.get(root, signal),
  staleTime: staleTimes.contextRouter,
  refetchOnMount: 'always' as const,
  // Keep the folder on screen while the next one is measured, instead of a spinner.
  placeholderData: keepPreviousData,
  retry: false,
});
