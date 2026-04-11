import { Component, useEffect, useRef, useState, type ReactNode } from 'react';
import {
  MDXEditor,
  codeBlockPlugin,
  codeMirrorPlugin,
  frontmatterPlugin,
  headingsPlugin,
  linkPlugin,
  listsPlugin,
  markdownShortcutPlugin,
  quotePlugin,
  tablePlugin,
  thematicBreakPlugin,
  type MDXEditorMethods,
} from '@mdxeditor/editor';
import '@mdxeditor/editor/style.css';
import Button from './Button';
import { Textarea } from './Input';

export type SkillMarkdownEditorSurface = 'rich' | 'raw';
export type SkillMarkdownEditorMode = 'edit' | 'split';

type SkillMarkdownEditorProps = {
  value: string;
  onChange: (value: string) => void;
  onSave: (value: string) => void;
  onDiscard: () => void;
  onSurfaceChange: (surface: SkillMarkdownEditorSurface) => void;
  surface: SkillMarkdownEditorSurface;
  mode: SkillMarkdownEditorMode;
  isDirty: boolean;
};

const editorPlugins = [
  frontmatterPlugin(),
  headingsPlugin({ allowedHeadingLevels: [1, 2, 3, 4, 5, 6] }),
  listsPlugin(),
  quotePlugin(),
  thematicBreakPlugin(),
  linkPlugin(),
  tablePlugin(),
  codeBlockPlugin({ defaultCodeBlockLanguage: 'txt' }),
  codeMirrorPlugin({
    codeBlockLanguages: {
      txt: 'Plain text',
      md: 'Markdown',
      yaml: 'YAML',
      json: 'JSON',
      bash: 'Bash',
      ts: 'TypeScript',
      js: 'JavaScript',
    },
  }),
  markdownShortcutPlugin(),
];

class RichEditorBoundary extends Component<{
  children: ReactNode;
  onError: (message?: string) => void;
  resetKey: string;
}, { hasError: boolean }> {
  state = { hasError: false };

  static getDerivedStateFromError() {
    return { hasError: true };
  }

  componentDidCatch(error: unknown) {
    const message = error instanceof Error ? error.message : undefined;
    this.props.onError(message);
  }

  componentDidUpdate(prevProps: Readonly<{ children: ReactNode; onError: (message?: string) => void; resetKey: string }>) {
    if (prevProps.resetKey !== this.props.resetKey && this.state.hasError) {
      this.setState({ hasError: false });
    }
  }

  render() {
    if (this.state.hasError) {
      return null;
    }

    return this.props.children;
  }
}

export default function SkillMarkdownEditor({
  value,
  onChange,
  onSave,
  onDiscard,
  onSurfaceChange,
  surface,
  mode,
  isDirty,
}: SkillMarkdownEditorProps) {
  const editorRef = useRef<MDXEditorMethods | null>(null);
  const lastLocalValueRef = useRef<string | null>(null);
  const previousValueRef = useRef(value);
  const [richError, setRichError] = useState<string | null>(null);
  const [richResetVersion, setRichResetVersion] = useState(0);
  const isSplitMode = mode === 'split';
  const editorHeightClass = isSplitMode ? 'h-full min-h-[20rem]' : 'min-h-[28rem]';
  const richEditorResetKey = `${surface}:${richResetVersion}`;

  useEffect(() => {
    const previousValue = previousValueRef.current;
    previousValueRef.current = value;

    if (lastLocalValueRef.current === value) {
      lastLocalValueRef.current = null;
      return;
    }

    if (surface !== 'rich' || previousValue === value) {
      return;
    }

    if (previousValue.trim() === value.trim()) {
      setRichResetVersion((current) => current + 1);
      return;
    }

    if (editorRef.current) {
      editorRef.current.setMarkdown(value);
    }
  }, [surface, value]);

  useEffect(() => {
    if (surface === 'rich') {
      setRichError(null);
    }
  }, [surface]);

  function handleRichFailure(message?: string) {
    setRichError(message ?? 'Rich editor unavailable for this document. Raw mode is active.');
    onSurfaceChange('raw');
  }

  return (
    <section
      className={`ss-skill-markdown-editor flex flex-col gap-3${isSplitMode ? ' h-full min-h-0' : ''}`}
      data-mode={mode}
      data-surface={surface}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2 text-sm text-pencil-light">
          <span className="rounded-full border border-muted-dark bg-muted/40 px-3 py-1 font-medium text-pencil">
            {surface === 'rich' ? 'Rich editor' : 'Raw markdown'}
          </span>
          {isDirty ? (
            <span className="rounded-full border border-pencil bg-paper px-3 py-1 font-medium text-pencil">
              Unsaved changes
            </span>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          {surface === 'rich' ? (
            <Button variant="secondary" size="sm" onClick={() => onSurfaceChange('raw')}>
              Open Raw
            </Button>
          ) : null}
          <Button variant="secondary" size="sm" onClick={onDiscard} disabled={!isDirty}>
            Discard
          </Button>
          <Button size="sm" onClick={() => onSave(value)} disabled={!isDirty}>
            Save
          </Button>
        </div>
      </div>

      {richError ? (
        <div
          role="alert"
          className="rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
        >
          {richError}
        </div>
      ) : null}

      <div className={`ss-skill-markdown-editor-surface rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-3${isSplitMode ? ' flex min-h-0 flex-1 flex-col' : ''}`}>
        {surface === 'rich' ? (
          <RichEditorBoundary onError={handleRichFailure} resetKey={richEditorResetKey}>
            <MDXEditor
              key={richEditorResetKey}
              ref={editorRef}
              markdown={value}
              trim={false}
              onChange={(next, initialMarkdownNormalize) => {
                if (initialMarkdownNormalize) {
                  return;
                }
                lastLocalValueRef.current = next;
                onChange(next);
              }}
              onError={({ error }) => handleRichFailure(error)}
              plugins={editorPlugins}
              className={`ss-skill-markdown-editor-instance ${editorHeightClass}`}
              contentEditableClassName={`ss-skill-markdown-editor-content prose-hand max-w-none ${editorHeightClass} px-4 py-3 focus:outline-none`}
              placeholder="Write markdown content"
            />
          </RichEditorBoundary>
        ) : (
          <Textarea
            aria-label="Raw markdown editor"
            value={value}
            onChange={(event) => {
              const next = event.currentTarget.value;
              lastLocalValueRef.current = next;
              onChange(next);
            }}
            spellCheck={false}
            wrapperClassName={isSplitMode ? 'flex h-full min-h-0 flex-1 flex-col' : undefined}
            className={`ss-skill-markdown-editor-input prose-hand w-full ${editorHeightClass} font-mono`}
          />
        )}
      </div>
    </section>
  );
}
