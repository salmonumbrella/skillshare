import { describe, expect, it } from 'vitest';
import {
  buildSkillDraftStats,
  buildSkillTokenBreakdown,
  parseSkillMarkdown,
  splitSkillMarkdown,
} from './skillMarkdown';

describe('parseSkillMarkdown', () => {
  it('extracts manifest frontmatter fields and returns markdown body for SKILL.md content', () => {
    const markdown = `# Overview

Use this skill for tasks.`;
    const content = `---
name: "My Skill"
description: "A concise skill description."
license: MIT
---
${markdown}`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.manifest).toEqual({
      name: 'My Skill',
      description: 'A concise skill description.',
      license: 'MIT',
    });
    expect(parsed.markdown).toBe(`\n${markdown}`);
  });

  it('preserves markdown body with custom tags, quote, and table', () => {
    const body = `<custom-note level="warning">Careful</custom-note>

> This is a quoted line.

| Col A | Col B |
| ----- | ----- |
| 1     | 2     |`;
    const content = `---
name: custom-format-skill
---
${body}`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.markdown).toBe(`\n${body}`);
  });

  it('parses YAML block scalar descriptions from frontmatter', () => {
    const content = `---
name: yaml-block-skill
description: |
  First line
  Second line
license: MIT
---
Body`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.manifest).toEqual({
      name: 'yaml-block-skill',
      description: 'First line\nSecond line\n',
      license: 'MIT',
    });
  });

  it('returns advanced frontmatter values for the field guide', () => {
    const content = `---
name: advanced-skill
description: Advanced frontmatter example
argument-hint: "[issue-number]"
disable-model-invocation: true
user-invocable: false
allowed-tools:
  - Read
  - Grep
context: fork
agent: explorer
paths:
  - src/**
  - tests/**
shell: bash
---
Body`;

    const parsed = parseSkillMarkdown(content);

    expect(parsed.frontmatter).toMatchObject({
      name: 'advanced-skill',
      description: 'Advanced frontmatter example',
      'argument-hint': '[issue-number]',
      'disable-model-invocation': true,
      'user-invocable': false,
      'allowed-tools': ['Read', 'Grep'],
      context: 'fork',
      agent: 'explorer',
      paths: ['src/**', 'tests/**'],
      shell: 'bash',
    });
  });
});

describe('splitSkillMarkdown', () => {
  it('preserves body whitespace exactly after frontmatter delimiter', () => {
    const body = `\n\n    \`\`\`ts
    const value = 1;
    \`\`\`
`;
    const content = `---
name: preserve-whitespace
---` + body;

    const split = splitSkillMarkdown(content);

    expect(split.frontmatter).toBe('name: preserve-whitespace');
    expect(split.markdown).toBe(body);
  });
});

describe('buildSkillDraftStats', () => {
  it('builds draft stats from full SKILL.md content including frontmatter', () => {
    const content = `---
name: "My Skill"
description: parser helper
license: MIT
---
## Heading
alpha beta

- gamma`;

    expect(buildSkillDraftStats(content)).toEqual({
      wordCount: 16,
      lineCount: 9,
      tokenCount: 25,
    });
  });

  it('uses cl100k-compatible tokenization for draft stats', () => {
    expect(buildSkillDraftStats('tiktoken is great!')).toEqual({
      wordCount: 3,
      lineCount: 1,
      tokenCount: 6,
    });
  });
});

describe('buildSkillTokenBreakdown', () => {
  it('reports full skill tokens separately from the rendered preview tokens', () => {
    const content = `---
name: "My Skill"
description: parser helper
license: MIT
---
## Heading
alpha beta

- gamma`;

    expect(buildSkillTokenBreakdown(content)).toEqual({
      loadTokens: buildSkillDraftStats('My Skill parser helper').tokenCount,
      previewTokens: buildSkillDraftStats(parseSkillMarkdown(content).markdown).tokenCount,
    });
  });
});
