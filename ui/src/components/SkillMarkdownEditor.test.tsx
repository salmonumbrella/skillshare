import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import SkillMarkdownEditor, {
  type SkillMarkdownEditorMode,
  type SkillMarkdownEditorSurface,
} from './SkillMarkdownEditor';

const mdxEditorState = vi.hoisted(() => ({
  shouldThrow: false,
  forceInitialNormalizeCallback: false,
}));

vi.mock('@mdxeditor/editor', async () => {
  const React = await vi.importActual<typeof import('react')>('react');

  const MockMDXEditor = React.forwardRef<
    {
      setMarkdown: (value: string) => void;
      insertMarkdown: (value: string) => void;
      focus: () => void;
      getContentEditableHTML: () => string;
      getSelectionMarkdown: () => string;
    },
    {
      markdown: string;
      trim?: boolean;
      onChange?: (next: string, initialMarkdownNormalize: boolean) => void;
      onError?: (payload: { error: string; source: string }) => void;
    }
  >(function MockMDXEditor(props, ref) {
    const [markdown, setMarkdown] = React.useState(() => {
      return props.trim === false ? props.markdown : props.markdown.trim();
    });

    React.useEffect(() => {
      const normalized = props.trim === false ? props.markdown : props.markdown.trim();
      if (mdxEditorState.forceInitialNormalizeCallback) {
        props.onChange?.(`${props.markdown}::normalized`, true);
        return;
      }
      if (props.trim !== false && normalized !== props.markdown) {
        props.onChange?.(normalized, true);
      }
    }, []);

    React.useImperativeHandle(ref, () => ({
      setMarkdown(value: string) {
        if (value.trim() === markdown.trim()) {
          return;
        }
        setMarkdown(value);
      },
      insertMarkdown() {},
      focus() {},
      getContentEditableHTML() {
        return markdown;
      },
      getSelectionMarkdown() {
        return markdown;
      },
    }), [markdown]);

    if (mdxEditorState.shouldThrow) {
      throw new Error('rich editor failed to initialize');
    }

    return (
      <div data-testid="mock-mdx-editor">
        <label htmlFor="mock-mdx-editor-input">Rich markdown editor</label>
        <textarea
          id="mock-mdx-editor-input"
          aria-label="Rich markdown editor"
          value={markdown}
          onChange={(event) => {
            const next = event.currentTarget.value;
            setMarkdown(next);
            props.onChange?.(next, false);
          }}
        />
      </div>
    );
  });

  const plugin = (name: string) => () => ({ name });

  return {
    MDXEditor: MockMDXEditor,
    headingsPlugin: plugin('headings'),
    listsPlugin: plugin('lists'),
    quotePlugin: plugin('quote'),
    thematicBreakPlugin: plugin('thematic-break'),
    markdownShortcutPlugin: plugin('markdown-shortcut'),
    frontmatterPlugin: plugin('frontmatter'),
    tablePlugin: plugin('table'),
    linkPlugin: plugin('link'),
    codeBlockPlugin: plugin('code-block'),
    codeMirrorPlugin: plugin('code-mirror'),
  };
});

type HarnessProps = {
  initialValue?: string;
  initialSurface?: SkillMarkdownEditorSurface;
  mode?: SkillMarkdownEditorMode;
  onSave?: (value: string) => void;
  onDiscard?: () => void;
  onSurfaceChange?: (surface: SkillMarkdownEditorSurface) => void;
  onChangeSpy?: (value: string) => void;
};

function ControlledHarness({
  initialValue = '---\nname: sample\n---\n# Heading\n',
  initialSurface = 'rich',
  mode = 'edit',
  onSave = vi.fn(),
  onDiscard = vi.fn(),
  onSurfaceChange = vi.fn(),
  onChangeSpy,
}: HarnessProps) {
  const [value, setValue] = useState(initialValue);
  const [savedValue, setSavedValue] = useState(initialValue);
  const [surface, setSurface] = useState<SkillMarkdownEditorSurface>(initialSurface);

  return (
    <SkillMarkdownEditor
      value={value}
      mode={mode}
      surface={surface}
      isDirty={value !== savedValue}
      onChange={(next) => {
        onChangeSpy?.(next);
        setValue(next);
      }}
      onSave={(next) => {
        onSave(next);
        setSavedValue(next);
      }}
      onDiscard={() => {
        onDiscard();
      }}
      onSurfaceChange={(nextSurface) => {
        onSurfaceChange(nextSurface);
        setSurface(nextSurface);
      }}
    />
  );
}

function ExternalResetHarness() {
  const [value, setValue] = useState('Hello');
  const [surface, setSurface] = useState<SkillMarkdownEditorSurface>('rich');

  return (
    <div>
      <button type="button" onClick={() => setValue('Hello\n')}>
        Apply trailing newline
      </button>
      <button type="button" onClick={() => setSurface('raw')}>
        Raw surface
      </button>
      <SkillMarkdownEditor
        value={value}
        mode="edit"
        surface={surface}
        isDirty={false}
        onChange={setValue}
        onSave={() => {}}
        onDiscard={() => {}}
        onSurfaceChange={setSurface}
      />
    </div>
  );
}

describe('SkillMarkdownEditor', () => {
  beforeEach(() => {
    mdxEditorState.shouldThrow = false;
    mdxEditorState.forceInitialNormalizeCallback = false;
    vi.spyOn(console, 'error').mockImplementation(() => {});
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders the controlled rich editor surface when the document loads', () => {
    render(<ControlledHarness />);

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Raw markdown editor' })).not.toBeInTheDocument();
  });

  it('does not dirty parent state on mount-time normalization callbacks', () => {
    const onChangeSpy = vi.fn();

    render(<ControlledHarness initialValue={'# Heading\n'} onChangeSpy={onChangeSpy} />);

    expect(onChangeSpy).not.toHaveBeenCalled();
    expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('# Heading\n');
  });

  it('ignores normalization-only rich editor callbacks even if they fire', () => {
    mdxEditorState.forceInitialNormalizeCallback = true;
    const onChangeSpy = vi.fn();

    render(<ControlledHarness onChangeSpy={onChangeSpy} />);

    expect(onChangeSpy).not.toHaveBeenCalled();
    expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
  });

  it('requests raw mode from the parent when the rich editor fails to initialize', async () => {
    mdxEditorState.shouldThrow = true;
    const onSurfaceChange = vi.fn();

    render(<ControlledHarness onSurfaceChange={onSurfaceChange} />);

    expect(await screen.findByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(onSurfaceChange).toHaveBeenCalledWith('raw');
  });

  it('switches into raw mode only through the controlled surface callback', async () => {
    const user = userEvent.setup();
    const onSurfaceChange = vi.fn();

    render(<ControlledHarness onSurfaceChange={onSurfaceChange} />);

    await user.click(screen.getByRole('button', { name: /open raw/i }));

    expect(onSurfaceChange).toHaveBeenCalledWith('raw');
    expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
    expect(screen.queryByRole('textbox', { name: 'Rich markdown editor' })).not.toBeInTheDocument();
  });

  it('emits save and discard callbacks without resetting controlled draft content internally', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onDiscard = vi.fn();

    render(<ControlledHarness onSave={onSave} onDiscard={onDiscard} />);

    const richEditor = screen.getByRole('textbox', { name: 'Rich markdown editor' });
    await user.clear(richEditor);
    await user.type(richEditor, '# Updated');

    await user.click(screen.getByRole('button', { name: /^save$/i }));
    expect(onSave).toHaveBeenCalledWith('# Updated');

    await user.type(screen.getByRole('textbox', { name: 'Rich markdown editor' }), ' draft');
    await user.click(screen.getByRole('button', { name: /^discard$/i }));

    expect(onDiscard).toHaveBeenCalledTimes(1);
    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('# Updated draft');
  });

  it('renders dirty UI from the controlled isDirty prop after a draft change', async () => {
    const user = userEvent.setup();

    render(<ControlledHarness />);

    const richEditor = screen.getByRole('textbox', { name: 'Rich markdown editor' });
    await user.type(richEditor, 'extra');

    expect(screen.getByText(/unsaved changes/i)).toBeInTheDocument();
  });

  it('remounts the rich editor for external whitespace-only updates that setMarkdown would ignore', async () => {
    const user = userEvent.setup();

    render(<ExternalResetHarness />);

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('Hello');

    await user.click(screen.getByRole('button', { name: /apply trailing newline/i }));

    expect(screen.getByRole('textbox', { name: 'Rich markdown editor' })).toHaveValue('Hello\n');
  });

  it('stretches the editor shell to full height in split mode', () => {
    render(<ControlledHarness mode="split" />);

    const editorSection = screen.getByRole('textbox', { name: 'Rich markdown editor' }).closest('section');

    expect(editorSection).toHaveClass('h-full');
    expect(editorSection).toHaveClass('min-h-0');
  });

  it('stretches the raw textarea shell to full height in split mode', () => {
    render(<ControlledHarness mode="split" initialSurface="raw" />);

    const rawEditor = screen.getByRole('textbox', { name: 'Raw markdown editor' });

    expect(rawEditor.parentElement).toHaveClass('h-full');
    expect(rawEditor.parentElement).toHaveClass('flex-1');
  });
});
