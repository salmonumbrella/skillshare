import { ApiError } from '../../api/client';
import type { useT } from '../../i18n';

/** A plugin failure the backend keyed, such as an unreachable source, in the reader's language; otherwise its message. */
export function pluginErrorMessage(error: unknown, t: ReturnType<typeof useT>) {
  const message = (error as Error).message;
  if (error instanceof ApiError && error.code?.startsWith('plugins.')) {
    return t(error.code, error.params as Record<string, string> | undefined, message);
  }
  return message;
}
