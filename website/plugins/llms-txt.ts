import fs from 'node:fs/promises';
import path from 'node:path';
import type {LoadContext, Plugin} from '@docusaurus/types';
import type {LoadedContent, DocMetadata} from '@docusaurus/plugin-content-docs';

// Emits /llms.txt (a link index following https://llmstxt.org) and /llms-full.txt
// (every English doc concatenated) from the docs plugin's loaded content, so both
// stay in sync with the sidebar without being maintained by hand.

type Section = {title: string; docs: DocMetadata[]};

type SidebarItem = {
  type: string;
  id?: string;
  label?: string;
  items?: SidebarItem[];
  link?: {type: string; id?: string};
};

function collectDocIds(items: SidebarItem[], ids: string[] = []): string[] {
  for (const item of items) {
    if (item.type === 'doc' && item.id) ids.push(item.id);
    if (item.type === 'category') {
      if (item.link?.type === 'doc' && item.link.id) ids.push(item.link.id);
      collectDocIds(item.items ?? [], ids);
    }
  }
  return ids;
}

function buildSections(content: LoadedContent): Section[] {
  const version = content.loadedVersions[0];
  const byId = new Map(version.docs.map((doc) => [doc.id, doc]));
  const pick = (ids: string[]) =>
    ids.map((id) => byId.get(id)).filter((doc): doc is DocMetadata => !!doc && !doc.unlisted);

  const sections: Section[] = [];
  const overview: string[] = [];
  for (const sidebar of Object.values(version.sidebars)) {
    for (const item of sidebar as unknown as SidebarItem[]) {
      if (item.type === 'category') {
        sections.push({title: item.label ?? 'Docs', docs: pick(collectDocIds([item]))});
      } else {
        overview.push(...collectDocIds([item]));
      }
    }
  }
  if (overview.length) sections.unshift({title: 'Overview', docs: pick(overview)});
  return sections;
}

// Auto-derived descriptions are the whole first paragraph, sometimes cut mid-line;
// the first sentence keeps the index short.
function firstSentence(text: string): string {
  return text.split(/(?<=[.!?])\s/)[0].trim();
}

async function readBody(siteDir: string, doc: DocMetadata): Promise<string> {
  const file = path.join(siteDir, doc.source.replace(/^@site\//, ''));
  const raw = await fs.readFile(file, 'utf8');
  return raw
    .replace(/^---\n[\s\S]*?\n---\n/, '')
    .replace(/^\s*# .*\n/, '')
    .trim();
}

export default function llmsTxtPlugin(context: LoadContext): Plugin {
  const {siteConfig, siteDir, i18n} = context;
  const siteUrl = siteConfig.url.replace(/\/$/, '');
  let sections: Section[] = [];

  return {
    name: 'llms-txt',

    allContentLoaded({allContent}) {
      const docs = allContent['docusaurus-plugin-content-docs']?.default as LoadedContent;
      sections = buildSections(docs);
    },

    async postBuild({outDir}) {
      // Docs are English in every locale; emit once at the site root.
      if (i18n.currentLocale !== i18n.defaultLocale) return;

      const header = [`# ${siteConfig.title}`, '', `> ${siteConfig.tagline}`, ''];

      const index = [...header];
      for (const section of sections) {
        index.push(`## ${section.title}`, '');
        for (const doc of section.docs) {
          const desc = doc.description ? `: ${firstSentence(doc.description)}` : '';
          index.push(`- [${doc.title}](${siteUrl}${doc.permalink})${desc}`);
        }
        index.push('');
      }
      index.push(
        '## Optional',
        '',
        `- [Full documentation](${siteUrl}/llms-full.txt): Every page above in one Markdown file`,
        '',
      );

      const full = [...header];
      for (const section of sections) {
        for (const doc of section.docs) {
          full.push(
            '---',
            '',
            `# ${doc.title}`,
            '',
            `Source: ${siteUrl}${doc.permalink}`,
            '',
            await readBody(siteDir, doc),
            '',
          );
        }
      }

      await fs.writeFile(path.join(outDir, 'llms.txt'), index.join('\n'));
      await fs.writeFile(path.join(outDir, 'llms-full.txt'), full.join('\n'));
    },
  };
}
