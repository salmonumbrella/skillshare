import type { SkillStats } from '../api/client';
import { getEncoding, type Tiktoken } from 'js-tiktoken';
import { parse as parseYaml } from 'yaml';

export type SkillManifest = {
  name?: string;
  description?: string;
  license?: string;
};

export type SkillFrontmatter = Record<string, unknown>;

export type SkillMarkdownParts = {
  frontmatter?: string;
  markdown: string;
};

export function splitSkillMarkdown(content: string): SkillMarkdownParts {
  if (!content) return { markdown: '' };

  const match = content.match(/^---[ \t]*\r?\n([\s\S]*?)\r?\n---[ \t]*(?=\r?\n|$)/);
  if (!match) return { markdown: content };

  return {
    frontmatter: match[1],
    markdown: content.slice(match[0].length),
  };
}

export function parseSkillMarkdown(content: string): {
  manifest: SkillManifest;
  frontmatter: SkillFrontmatter;
  markdown: string;
} {
  const { frontmatter, markdown } = splitSkillMarkdown(content);
  if (!frontmatter) return { manifest: {}, frontmatter: {}, markdown };

  let parsed: unknown;
  try {
    parsed = parseYaml(frontmatter);
  } catch {
    return { manifest: {}, frontmatter: {}, markdown };
  }

  const source = (typeof parsed === 'object' && parsed !== null) ? parsed as Record<string, unknown> : {};
  return {
    manifest: {
      name: typeof source.name === 'string' ? source.name : undefined,
      description: typeof source.description === 'string' ? source.description : undefined,
      license: typeof source.license === 'string' ? source.license : undefined,
    },
    frontmatter: source,
    markdown,
  };
}

export function buildSkillDraftStats(content: string): SkillStats {
  const wordCount = countWords(content);
  const lineCount = countLines(content);
  const tokenCount = countTokens(content);

  return { wordCount, lineCount, tokenCount };
}

export function buildSkillTokenBreakdown(content: string): {
  loadTokens: number;
  previewTokens: number;
} {
  const parsed = parseSkillMarkdown(content);
  const renderedMarkdown = parsed.markdown.trim() ? parsed.markdown : content;
  const alwaysLoadedContext = [parsed.manifest.name, parsed.manifest.description]
    .filter((value): value is string => Boolean(value && value.trim()))
    .join(' ');

  return {
    loadTokens: countTokens(alwaysLoadedContext),
    previewTokens: countTokens(renderedMarkdown),
  };
}

function countWords(content: string): number {
  const trimmed = content.trim();
  if (!trimmed) return 0;
  return trimmed.split(/\s+/).length;
}

function countLines(content: string): number {
  const trimmed = content.trim();
  if (!trimmed) return 0;
  return trimmed.replaceAll('\r\n', '\n').split('\n').length;
}

let cl100kEncoder: Tiktoken | null = null;
let cl100kEncoderLoadFailed = false;

function getCL100KEncoder(): Tiktoken | null {
  if (cl100kEncoder) return cl100kEncoder;
  if (cl100kEncoderLoadFailed) return null;
  try {
    cl100kEncoder = getEncoding('cl100k_base');
    return cl100kEncoder;
  } catch {
    cl100kEncoderLoadFailed = true;
    return null;
  }
}

function countTokens(content: string): number {
  const encoder = getCL100KEncoder();
  if (!encoder) return 0;
  return encoder.encode(content).length;
}
