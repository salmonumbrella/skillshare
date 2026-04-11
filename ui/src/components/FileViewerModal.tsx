import { Component, Suspense, lazy, useEffect, useMemo, useState, type ReactNode } from 'react';
import { X } from 'lucide-react';
import Markdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import CodeMirror from '@uiw/react-codemirror';
import { json } from '@codemirror/lang-json';
import { yaml } from '@codemirror/lang-yaml';
import { python } from '@codemirror/lang-python';
import { javascript } from '@codemirror/lang-javascript';
import { EditorView } from '@codemirror/view';
import CopyButton from './CopyButton';
import Button from './Button';
import IconButton from './IconButton';
import Spinner from './Spinner';
import DialogShell from './DialogShell';
import SegmentedControl from './SegmentedControl';
import ConfirmDialog from './ConfirmDialog';
import { createSkillMarkdownComponents } from './SkillMarkdownComponents';
import { api, type SkillFileContent } from '../api/client';
import { handTheme } from '../lib/codemirror-theme';
import { Textarea } from './Input';
import type { SkillMarkdownEditorSurface } from './SkillMarkdownEditor';

const SkillMarkdownEditor = lazy(() => import('./SkillMarkdownEditor'));

type FileViewerMode = 'read' | 'edit' | 'split';

interface FileViewerModalProps {
  skillName: string;
  filepath: string;
  sourcePath?: string;
  onClose: () => void;
}

export default function FileViewerModal({ skillName, filepath, sourcePath, onClose }: FileViewerModalProps) {
  const fullPath = sourcePath ? `${sourcePath}/${filepath}` : filepath;
  const [data, setData] = useState<SkillFileContent | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [savedContent, setSavedContent] = useState('');
  const [draftContent, setDraftContent] = useState('');
  const [mode, setMode] = useState<FileViewerMode>('read');
  const [editorSurface, setEditorSurface] = useState<SkillMarkdownEditorSurface>('rich');
  const [editorLoadError, setEditorLoadError] = useState<string | null>(null);
  const [saveError, setSaveError] = useState<string | null>(null);
  const [isSaving, setIsSaving] = useState(false);
  const [showDiscardDialog, setShowDiscardDialog] = useState(false);
  const isMarkdownFile = filepath.toLowerCase().endsWith('.md');
  const isDirty = draftContent !== savedContent;

  useEffect(() => {
    setLoading(true);
    setError(null);
    setSaveError(null);
    setMode('read');
    setEditorSurface('rich');
    setEditorLoadError(null);
    setShowDiscardDialog(false);
    api
      .getSkillFile(skillName, filepath)
      .then((response) => {
        setData(response);
        setSavedContent(response.content);
        setDraftContent(response.content);
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => setLoading(false));
  }, [skillName, filepath]);

  const cmExtensions = useMemo(() => {
    if (!data) return [];
    const exts = [EditorView.lineWrapping, EditorView.editable.of(false), ...handTheme];
    if (data.contentType === 'application/json') exts.push(json());
    else if (data.contentType === 'text/yaml') exts.push(yaml());
    // Infer language from filename extension
    const ext = filepath.split('.').pop()?.toLowerCase();
    if (ext === 'py') exts.push(python());
    else if (ext === 'js' || ext === 'mjs' || ext === 'cjs') exts.push(javascript());
    else if (ext === 'ts' || ext === 'mts' || ext === 'cts') exts.push(javascript({ typescript: true }));
    else if (ext === 'jsx') exts.push(javascript({ jsx: true }));
    else if (ext === 'tsx') exts.push(javascript({ jsx: true, typescript: true }));
    return exts;
  }, [data, filepath]);
  const markdownComponents = useMemo(() => createSkillMarkdownComponents(), []);

  const renderedMarkdown = isMarkdownFile ? draftContent : data?.content ?? '';

  const handleDiscardDraft = () => {
    setDraftContent(savedContent);
    setSaveError(null);
  };

  const handleSave = async (nextContent = draftContent) => {
    if (!isMarkdownFile || isSaving) return;

    setIsSaving(true);
    setSaveError(null);
    try {
      const response = await api.saveSkillFile(skillName, filepath, nextContent);
      setSavedContent(response.content);
      setDraftContent(response.content);
      setData((current) => (current ? { ...current, content: response.content } : current));
    } catch (e: unknown) {
      setSaveError((e as Error).message);
    } finally {
      setIsSaving(false);
    }
  };

  const requestClose = () => {
    if (isSaving) {
      return;
    }

    if (!isDirty) {
      onClose();
      return;
    }

    setShowDiscardDialog(true);
  };

  return (
    <>
      <DialogShell
        open={true}
        onClose={requestClose}
        maxWidth="3xl"
        padding="none"
        preventClose={isSaving}
        className="max-h-[85vh] flex flex-col overflow-hidden"
      >
        <div className="flex items-center justify-between mb-3 px-6 pt-6">
          <h3
            className="font-bold text-pencil truncate font-mono flex items-center gap-1.5"
            style={{ fontSize: '0.95rem' }}
          >
            {filepath}
            <CopyButton
              value={fullPath}
              title="Copy file path"
              copiedLabelClassName="text-xs font-normal"
            />
          </h3>
          <IconButton
            icon={<X size={16} strokeWidth={2.5} />}
            label="Close"
            size="md"
            onClick={requestClose}
            disabled={isSaving}
            className="shrink-0 ml-2"
          />
        </div>

        <div className="overflow-auto flex-1 min-h-0 px-6 pb-6">
          {loading && (
            <div className="py-12 flex justify-center">
              <Spinner size="md" />
            </div>
          )}

          {error && (
            <div className="py-8 text-center">
              <p className="text-danger">
                {error}
              </p>
            </div>
          )}

          {data && !loading && (
            <>
              {isMarkdownFile ? (
                <>
                  <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
                    <SegmentedControl
                      value={mode}
                      onChange={setMode}
                      options={[
                        { value: 'read', label: 'Read' },
                        { value: 'edit', label: 'Edit' },
                        { value: 'split', label: 'Split' },
                      ]}
                    />
                    <div className="flex flex-wrap items-center gap-2">
                      {isSaving ? (
                        <span className="rounded-full border border-blue bg-blue/10 px-3 py-1 text-sm font-medium text-blue inline-flex items-center gap-1.5">
                          <Spinner size="sm" />
                          Saving...
                        </span>
                      ) : null}
                      {isDirty ? (
                        <span className="rounded-full border border-warning bg-warning-light px-3 py-1 text-sm font-medium text-warning">
                          Unsaved changes
                        </span>
                      ) : null}
                    </div>
                  </div>

                  {saveError ? (
                    <div
                      role="alert"
                      className="mb-4 rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
                    >
                      {saveError}
                    </div>
                  ) : null}

                  {mode === 'read' ? (
                    <div className="prose-hand">
                      <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                        {renderedMarkdown}
                      </Markdown>
                    </div>
                  ) : mode === 'edit' ? (
                    <div className={editorShellClassName(isSaving)} aria-busy={isSaving}>
                      <MarkdownEditorPanel
                        mode="edit"
                        value={draftContent}
                        surface={editorSurface}
                        isDirty={isDirty}
                        isSaving={isSaving}
                        editorLoadError={editorLoadError}
                        onChange={(next) => {
                          setDraftContent(next);
                          setSaveError(null);
                        }}
                        onSave={(next) => void handleSave(next)}
                        onDiscard={handleDiscardDraft}
                        onSurfaceChange={setEditorSurface}
                        onEditorLoadError={setEditorLoadError}
                      />
                    </div>
                  ) : (
                    <div className="grid grid-cols-1 items-stretch gap-4 xl:grid-cols-2 xl:auto-rows-fr">
                      <div className={editorShellClassName(isSaving, true)} aria-busy={isSaving}>
                        <MarkdownEditorPanel
                          mode="split"
                          value={draftContent}
                          surface={editorSurface}
                          isDirty={isDirty}
                          isSaving={isSaving}
                          editorLoadError={editorLoadError}
                          onChange={(next) => {
                            setDraftContent(next);
                            setSaveError(null);
                          }}
                          onSave={(next) => void handleSave(next)}
                          onDiscard={handleDiscardDraft}
                          onSurfaceChange={setEditorSurface}
                          onEditorLoadError={setEditorLoadError}
                        />
                      </div>
                      <div className="flex h-full min-h-0 flex-col rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-4">
                        <div className="prose-hand max-w-none min-h-0 flex-1">
                          <Markdown remarkPlugins={[remarkGfm]} components={markdownComponents}>
                            {renderedMarkdown}
                          </Markdown>
                        </div>
                      </div>
                    </div>
                  )}
                </>
              ) : (
                <CodeMirror
                  value={data.content}
                  extensions={cmExtensions}
                  theme="none"
                  readOnly
                  editable={false}
                  basicSetup={{
                    lineNumbers: true,
                    foldGutter: true,
                    highlightActiveLine: false,
                    bracketMatching: true,
                    autocompletion: false,
                  }}
                />
              )}
            </>
          )}
        </div>
      </DialogShell>

      <ConfirmDialog
        open={showDiscardDialog}
        onConfirm={() => {
          setShowDiscardDialog(false);
          onClose();
        }}
        onCancel={() => setShowDiscardDialog(false)}
        title="Discard changes?"
        message="You have unsaved changes in this file. Discard them and close the modal?"
        confirmText="Discard Changes"
        cancelText="Keep Editing"
        variant="danger"
      />
    </>
  );
}

type MarkdownEditorPanelProps = {
  editorLoadError: string | null;
  isDirty: boolean;
  isSaving: boolean;
  mode: Exclude<FileViewerMode, 'read'>;
  onChange: (value: string) => void;
  onDiscard: () => void;
  onEditorLoadError: (message: string) => void;
  onSave: (value: string) => void;
  onSurfaceChange: (surface: SkillMarkdownEditorSurface) => void;
  surface: SkillMarkdownEditorSurface;
  value: string;
};

function MarkdownEditorPanel({
  editorLoadError,
  isDirty,
  isSaving,
  mode,
  onChange,
  onDiscard,
  onEditorLoadError,
  onSave,
  onSurfaceChange,
  surface,
  value,
}: MarkdownEditorPanelProps) {
  if (editorLoadError) {
    return (
      <RawMarkdownFallback
        errorMessage={editorLoadError}
        isDirty={isDirty}
        isSaving={isSaving}
        mode={mode}
        onChange={onChange}
        onDiscard={onDiscard}
        onSave={onSave}
        value={value}
      />
    );
  }

  return (
    <EditorLoadBoundary
      onError={(message) => onEditorLoadError(message ?? 'Markdown editor unavailable. Raw mode is active.')}
      resetKey={mode}
    >
      <Suspense fallback={<EditorFallback />}>
        <SkillMarkdownEditor
          value={value}
          onChange={onChange}
          onSave={onSave}
          onDiscard={onDiscard}
          onSurfaceChange={onSurfaceChange}
          surface={surface}
          mode={mode}
          isDirty={isDirty}
        />
      </Suspense>
    </EditorLoadBoundary>
  );
}

type RawMarkdownFallbackProps = {
  errorMessage: string;
  isDirty: boolean;
  isSaving: boolean;
  mode: Exclude<FileViewerMode, 'read'>;
  onChange: (value: string) => void;
  onDiscard: () => void;
  onSave: (value: string) => void;
  value: string;
};

function RawMarkdownFallback({
  errorMessage,
  isDirty,
  isSaving,
  mode,
  onChange,
  onDiscard,
  onSave,
  value,
}: RawMarkdownFallbackProps) {
  const isSplitMode = mode === 'split';
  const editorHeightClass = isSplitMode ? 'h-full min-h-[20rem]' : 'min-h-[28rem]';

  return (
    <section className={`flex flex-col gap-3${isSplitMode ? ' h-full min-h-0' : ''}`}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2 text-sm text-pencil-light">
          <span className="rounded-full border border-muted-dark bg-muted/40 px-3 py-1 font-medium text-pencil">
            Raw markdown
          </span>
          {isDirty ? (
            <span className="rounded-full border border-pencil bg-paper px-3 py-1 font-medium text-pencil">
              Unsaved changes
            </span>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" size="sm" onClick={onDiscard} disabled={!isDirty || isSaving}>
            Discard
          </Button>
          <Button size="sm" onClick={() => onSave(value)} disabled={!isDirty || isSaving}>
            Save
          </Button>
        </div>
      </div>

      <div
        role="alert"
        className="rounded-[var(--radius-md)] border border-danger/40 bg-danger/5 px-4 py-3 text-sm text-danger"
      >
        {errorMessage}
      </div>

      <div className={`rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-3${isSplitMode ? ' flex min-h-0 flex-1 flex-col' : ''}`}>
        <Textarea
          aria-label="Raw markdown editor"
          value={value}
          onChange={(event) => onChange(event.currentTarget.value)}
          spellCheck={false}
          wrapperClassName={isSplitMode ? 'flex h-full min-h-0 flex-1 flex-col' : undefined}
          className={`prose-hand w-full ${editorHeightClass} font-mono`}
        />
      </div>
    </section>
  );
}

class EditorLoadBoundary extends Component<{
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

function editorShellClassName(isSaving: boolean, stretch = false) {
  return [
    'transition-opacity duration-150',
    stretch ? 'h-full min-h-0' : '',
    isSaving ? 'opacity-70 pointer-events-none' : '',
  ].filter(Boolean).join(' ');
}

function EditorFallback() {
  return (
    <div className="rounded-[var(--radius-lg)] border-2 border-muted bg-surface p-6">
      <div className="flex items-center justify-center py-10">
        <Spinner size="md" />
      </div>
    </div>
  );
}
