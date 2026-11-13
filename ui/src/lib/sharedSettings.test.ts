import { describe, expect, it } from 'vitest';
import { ApiError } from '../api/client';
import { messagesByLocale, supportedLocales, translate } from '../i18n';
import { sharedErrorMessage, sharedReasonMessage } from './sharedSettings';

const t = (key: string, params?: Parameters<typeof translate>[2], fallback?: string) => translate('en', key, params, fallback);
const reasons = ['missing', 'unreadable', 'parse', 'version', 'newer-section', 'machine-key', 'unknown-key', 'privacy', 'entry'];

describe('shared settings messages', () => {
  it('translates all refusals and reasons with safe location details', () => {
    for (const reason of reasons) {
      const shared = { enabled: true, usable: false, path: '/home/test/team.yaml', reason,
        findings: [{ line: 7, entry: 'rules', rule: 'secret' }] };
      const message = sharedReasonMessage(shared, t);
      expect(message).toContain('~/team.yaml');
      expect(message).not.toContain('shared.reason.');
      expect(sharedErrorMessage(new ApiError(409, 'fallback', { code: 'shared_unavailable',
        params: { path: shared.path, reason, line: 7, entry: 'rules', rule: 'secret' } }), t)).toContain(message);
    }
    for (const code of ['shared_owned', 'shared_publication']) {
      const message = sharedErrorMessage(new ApiError(409, 'fallback', { code, params: { path: '/team.yaml', name: 'rules' } }), t);
      expect(message).not.toBe('fallback');
    }
  });
  it('preserves unknown error diagnostics', () => {
    expect(sharedErrorMessage(new Error('offline'), t)).toBe('offline');
    expect(sharedErrorMessage(new ApiError(500, 'future diagnostic', { code: 'future' }), t)).toBe('future diagnostic');
  });
  it('has all twelve shared keys in all eleven locales', () => {
    const keys = ['shared_owned', 'shared_unavailable', 'shared_publication'].map((code) => `shared.error.${code}`)
      .concat(reasons.map((reason) => `shared.reason.${reason}`));
    for (const { code: locale } of supportedLocales) for (const key of keys) {
      expect(messagesByLocale[locale][key], `${locale}: ${key}`).toBeTruthy();
    }
  });
});
