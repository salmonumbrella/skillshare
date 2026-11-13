import { describe, expect, it } from 'vitest';
import { folderOf, formatAgentDisplayName, formatSkillDisplayName } from './resourceNames';

describe('resourceNames', () => {
  it('formats nested agent flat names as slash paths without markdown suffix', () => {
    expect(formatAgentDisplayName('demo__code-archaeologist.md')).toBe('demo/code-archaeologist');
  });

  it('formats top-level agent flat names without markdown suffix', () => {
    expect(formatAgentDisplayName('reviewer.md')).toBe('reviewer');
  });

  it('formats nested skill flat names as slash paths', () => {
    expect(formatSkillDisplayName('_team__frontend__ui')).toBe('_team/frontend/ui');
  });

  it('puts an item inside a tracked repo in the repo folder', () => {
    expect(folderOf({ relPath: '_team__repo/skills/gamma', isInRepo: true })).toBe('_team__repo');
  });

  it('puts a nested item in its full parent path', () => {
    expect(folderOf({ relPath: 'frontend/react/hooks', isInRepo: false })).toBe('frontend/react');
  });

  it('puts an item at the source root in the empty folder', () => {
    expect(folderOf({ relPath: 'solo', isInRepo: false })).toBe('');
  });
});
