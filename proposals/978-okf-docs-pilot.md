# Feature Proposal: Pilot OKF v0.3 on Skillshare's website docs

Related issue: [#978](https://github.com/runkids/skillshare/issues/978).
Prior art: [Kenn Kit #378](https://github.com/kenn-io/kit/issues/378).
Spec: [OKF v0.3, pinned to 527e81af](https://github.com/open-knowledge-format/open-knowledge-format/blob/527e81af/SPEC.md).

## Problem

Docusaurus gives readers and search engines a useful view of Skillshare's docs. An agent reading the same Markdown directly can find the heading and text, but lacks a consistent way to distinguish tutorials from reference pages or see who last changed a page and when. The goal is to make that information available in the source files without changing how the site works.

## Proposed Solution

Pilot on two current English pages: `website/docs/getting-started/first-sync.md` and `website/docs/reference/commands/status.md`. Extend their existing YAML frontmatter instead of adding a second block. Keep existing Docusaurus fields and Markdown bodies, and use the page's existing title and first sentence to minimize presentation changes. No CLI, dashboard, runtime dependency, or importer changes.

Example headers for the proposed pilot (illustrative only):

```yaml
---
sidebar_position: 2
type: Tutorial
title: First Sync
description: A complete first-time setup, in order.
---
```

```yaml
---
sidebar_position: 7
type: Reference
title: status
description: "Show the current state of skillshare: source, tracked repositories, targets, and versions."
---
```

Only `type` is required by OKF. Prefer `title` and `description` for these pages. When the actual editor and time of a substantive edit are known, add `generated: { by: "<actual actor>", at: "<RFC 3339 UTC date-time>" }`. For agent edits use the actual harness/model, and for human edits use `human:<id>`. Do not backfill guessed provenance or update it for reformatting. Keep `status` optional and don't add tags, source entries or backlinks just to fill fields.

This is a concept-frontmatter pilot, not an assertion that all `website/docs` is a conformant bundle. Existing nested `index.md` pages have Docusaurus metadata whereas OKF reserves nested indexes for listings without frontmatter. Leave those files, sidebars, MDX, locales, routes, and existing locale fallback rules alone. Do not add a bundle root, indexes, logs, or runtime validation in this pass. Do not rewrite any user or bundled `SKILL.md`, README, changelog, or historical file. The developer wiki could be a later, separate pilot; keep its existing router as the source of truth.

Acceptance checks for a pilot implementation:

1. Parse both headers as YAML 1.2 without duplicate keys; confirm that both have a type and that existing fields and bodies are intact. Check `generated` only if supplied. Read temporary copies of the two pages with an OKF v0.3 reader in a disposable bundle (with its own `index.md` version declaration). Confirm the expected types and descriptions; this checks those pages, not the full Docusaurus tree.
2. Run `make docs-check` and the production Docusaurus build inside the devcontainer. Check existing English and locale fallback routes, sidebar labels and order, headings/anchors, and broken links. If a page has translated copies, verify those still render on their existing routes without requiring a migration.
3. Compare `llms.txt` and `llms-full.txt` before and after the build. Preserve the links, order, and body text, and check that the new YAML does not leak into the full-text export. Review any change to the index description: the current plugin reads Docusaurus's description metadata, so a change there is possible even if the Markdown body is unchanged.

## Alternatives Considered

- **Keep Docusaurus metadata only.** This is the smallest option but does not provide classification or edit provenance to tools reading the Markdown directly.
- **Migrate the whole docs tree or add an export now.** Either is more work and introduces navigation or maintenance risks before the metadata scheme has demonstrated value. A separate export may make sense if Docusaurus's index conventions prove incompatible with a strict OKF bundle.

## Scope

- [x] Small (1-3 files, < 200 lines)
- [ ] Medium (3-10 files, 200-500 lines)
- [ ] Large (10+ files, 500+ lines)

This proposal changes only this file. If accepted, the first implementation would change only the two page headers.

## Open Questions

- Would these two pages be representative enough, and do `Tutorial` and `Reference` fit the existing docs taxonomy?
- Is page-level compatibility sufficient, or is a separate machine-readable bundle worth a later proposal? Should lightweight validation remain manual until more pages adopt the fields?
