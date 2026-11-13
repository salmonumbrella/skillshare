import { ApiError } from '../api/http';
import type { SharedStatusPayload } from '../api/types/config';
import type { useT } from '../i18n';
import { shortenHome } from './paths';

type T = ReturnType<typeof useT>;
const reasons = new Set(['missing', 'unreadable', 'parse', 'version', 'newer-section', 'machine-key', 'unknown-key', 'privacy', 'entry']);
const shorten = (value: string) => value.replace(/\/(?:Users|home)\/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+/gi, shortenHome);

export function sharedReasonMessage(shared: SharedStatusPayload, t: T): string {
  const finding = shared.findings?.[0];
  const params = { file: shorten(shared.path ?? ''), line: finding?.line ?? '', entry: finding?.entry ?? '', rule: finding?.rule ?? '' };
  const reason = reasons.has(shared.reason ?? '') ? shared.reason : 'unreadable';
  return t(`shared.reason.${reason}`, params);
}

/** Shared refusals use only the server's safe identifiers, never source values. */
export function sharedErrorMessage(error: unknown, t: T): string {
  if (!(error instanceof ApiError) || !['shared_owned', 'shared_unavailable', 'shared_publication'].includes(error.code ?? '')) {
    return shorten(error instanceof Error ? error.message : String(error));
  }
  const p = error.params ?? {};
  const text = (key: string) => typeof p[key] === 'string' ? shorten(p[key] as string) : '';
  const reason = text('reason');
  const detail = reason && reasons.has(reason) ? sharedReasonMessage({
    enabled: true, usable: false, path: text('path'), reason,
    findings: [{ line: Number(p.line) || undefined, entry: text('entry'), rule: text('rule') }],
  }, t) : '';
  return t(`shared.error.${error.code}`, { file: text('path'), name: text('name'), reason: detail });
}
