import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import FileViewerModal from './FileViewerModal';
import { api } from '../api/client';
import { ToastProvider } from './Toast';

let shouldThrowEditor = false;

vi.mock('@uiw/react-codemirror', () => ({
  default: function MockCodeMirror(props: { value: string; readOnly?: boolean; editable?: boolean }) {
    return (
      <textarea
        aria-label="Code viewer"
        readOnly={props.readOnly}
        data-editable={String(props.editable)}
        value={props.value}
        onChange={() => {}}
      />
    );
  },
}));

vi.mock('@codemirror/lang-json', () => ({
  json: () => ({ name: 'json' }),
}));

vi.mock('@codemirror/lang-yaml', () => ({
  yaml: () => ({ name: 'yaml' }),
}));

vi.mock('@codemirror/lang-python', () => ({
  python: () => ({ name: 'python' }),
}));

vi.mock('@codemirror/lang-javascript', () => ({
  javascript: () => ({ name: 'javascript' }),
}));

vi.mock('@codemirror/view', () => ({
  EditorView: {
    lineWrapping: { name: 'lineWrapping' },
    editable: {
      of: () => ({ name: 'editable' }),
    },
  },
}));

vi.mock('../lib/codemirror-theme', () => ({
  handTheme: [],
}));

vi.mock('./SkillMarkdownEditor', () => ({
  default: function MockSkillMarkdownEditor(props: {
    value: string;
    surface: 'rich' | 'raw';
    mode: 'edit' | 'split';
    isDirty: boolean;
    onChange: (value: string) => void;
    onSave: (value: string) => void;
    onDiscard: () => void;
    onSurfaceChange: (surface: 'rich' | 'raw') => void;
  }) {
    if (shouldThrowEditor) {
      throw new Error('Mock lazy editor failure');
    }

    return (
      <section>
        <div>Mock editor mode: {props.mode}</div>
        <div>Mock editor surface: {props.surface}</div>
        {props.surface === 'rich' ? (
          <button type="button" onClick={() => props.onSurfaceChange('raw')}>
            Open Raw
          </button>
        ) : null}
        <label htmlFor="file-viewer-markdown-editor">Raw markdown editor</label>
        <textarea
          id="file-viewer-markdown-editor"
          aria-label="Raw markdown editor"
          value={props.value}
          onChange={(event) => props.onChange(event.currentTarget.value)}
        />
        <button type="button" onClick={() => props.onSave(props.value)} disabled={!props.isDirty}>
          Save
        </button>
        <button type="button" onClick={props.onDiscard} disabled={!props.isDirty}>
          Discard
        </button>
      </section>
    );
  },
}));

vi.mock('../api/client', async () => {
  const actual = await vi.importActual<typeof import('../api/client')>('../api/client');
  return {
    ...actual,
    api: {
      ...actual.api,
      getSkillFile: vi.fn(),
      saveSkillFile: vi.fn(),
    },
  };
});

describe('FileViewerModal', () => {
  const getSkillFile = vi.mocked(api.getSkillFile);
  const saveSkillFile = vi.mocked(api.saveSkillFile);

  beforeEach(() => {
    shouldThrowEditor = false;
    getSkillFile.mockReset();
    saveSkillFile.mockReset();
  });

  function renderModal(filepath = 'docs/guide.md', onClose = vi.fn()) {
    render(
      <ToastProvider>
        <FileViewerModal
          skillName="sample-skill"
          filepath={filepath}
          sourcePath="/tmp/sample-skill"
          onClose={onClose}
        />
      </ToastProvider>,
    );

    return { onClose };
  }

  it('shows Read, Edit, and Split controls for markdown files', async () => {
    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    expect(await screen.findByRole('button', { name: 'Read' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Edit' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Split' })).toBeInTheDocument();
  });

  it('renders substitution-shaped markdown tokens as dedicated pills in the file modal', async () => {
    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: [
        '# Guide',
        '',
        'Use $ARGUMENTS and `${CLAUDE_SKILL_DIR}` here.',
      ].join('\n'),
    });

    render(
      <ToastProvider>
        <FileViewerModal
          skillName="sample-skill"
          filepath="docs/guide.md"
          sourcePath="/tmp/sample-skill"
          onClose={vi.fn()}
        />
      </ToastProvider>,
    );

    expect(await screen.findByRole('heading', { name: 'Guide' })).toBeInTheDocument();

    const tokens = (await screen.findAllByTestId('skill-substitution-token'))
      .map((element) => element.textContent);

    expect(tokens).toEqual(expect.arrayContaining([
      '$ARGUMENTS',
      '${CLAUDE_SKILL_DIR}',
    ]));
  });

  it('saves markdown drafts via saveSkillFile, uses the API response as authoritative content, and ignores duplicate saves', async () => {
    const user = userEvent.setup();
    let resolveSave: ((value: { filename: string; content: string }) => void) | undefined;

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockImplementationOnce(() => new Promise((resolve) => {
      resolveSave = resolve;
    }));

    renderModal();

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(await screen.findByRole('textbox', { name: 'Raw markdown editor' }), '\nDraft change');

    await user.click(screen.getByRole('button', { name: 'Save' }));
    await user.click(screen.getByRole('button', { name: 'Save' }));

    await waitFor(() => {
      expect(saveSkillFile).toHaveBeenCalledTimes(1);
    });
    expect(saveSkillFile).toHaveBeenCalledWith('sample-skill', 'docs/guide.md', '# Guide\n\nBody\nDraft change');

    expect(resolveSave).toBeDefined();
    resolveSave?.({
      filename: 'docs/guide.md',
      content: '# Canonical\n\nSaved by API',
    });

    await waitFor(() => {
      expect(screen.queryByText(/unsaved changes/i)).not.toBeInTheDocument();
    });

    await user.click(screen.getByRole('button', { name: 'Read' }));

    expect(await screen.findByRole('heading', { name: 'Canonical' })).toBeInTheDocument();
    expect(screen.getByText('Saved by API')).toBeInTheDocument();
  });

  it('prevents closing the markdown modal while a save is in flight', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    let resolveSave: ((value: { filename: string; content: string }) => void) | undefined;

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });
    saveSkillFile.mockImplementationOnce(() => new Promise((resolve) => {
      resolveSave = resolve;
    }));

    renderModal('docs/guide.md', onClose);

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nDraft change');
    await user.click(screen.getByRole('button', { name: 'Save' }));

    expect(await screen.findByText(/saving/i)).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Close' }));
    await user.keyboard('{Escape}');
    fireEvent.mouseDown(screen.getByRole('dialog'));

    expect(onClose).not.toHaveBeenCalled();
    expect(screen.queryByRole('heading', { name: /discard changes/i })).not.toBeInTheDocument();

    expect(resolveSave).toBeDefined();
    resolveSave?.({
      filename: 'docs/guide.md',
      content: '# Guide\n\nSaved',
    });

    await waitFor(() => {
      expect(screen.queryByText(/saving/i)).not.toBeInTheDocument();
    });
  });

  it('supports Open Raw for markdown files in the modal editor', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    expect(screen.getByText('Mock editor surface: rich')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Open Raw' }));

    expect(await screen.findByText('Mock editor surface: raw')).toBeInTheDocument();
  });

  it('updates the split preview from the markdown draft', async () => {
    const user = userEvent.setup();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    await user.click(await screen.findByRole('button', { name: 'Split' }));
    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nSplit preview');

    expect(await screen.findByText(/Body\s+Split preview/, { selector: 'p' })).toBeInTheDocument();
  });

  it('falls back to a local raw editor if the lazy markdown editor fails', async () => {
    const user = userEvent.setup();
    shouldThrowEditor = true;

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal();

    await user.click(await screen.findByRole('button', { name: 'Edit' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Mock lazy editor failure');
    expect(screen.getByRole('textbox', { name: 'Raw markdown editor' })).toBeInTheDocument();
  });

  it('keeps non-markdown files read-only', async () => {
    getSkillFile.mockResolvedValueOnce({
      filename: 'schema.json',
      contentType: 'application/json',
      content: '{\n  "ok": true\n}',
    });

    renderModal('schema.json');

    expect(await screen.findByRole('textbox', { name: 'Code viewer' })).toHaveValue('{\n  "ok": true\n}');
    expect(screen.queryByRole('button', { name: 'Edit' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Split' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Save' })).not.toBeInTheDocument();
  });

  it('preserves Escape-to-close when the markdown modal is not dirty', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal('docs/guide.md', onClose);
    await screen.findByRole('button', { name: 'Read' });

    await user.keyboard('{Escape}');

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts for discard before closing a dirty markdown modal', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal('docs/guide.md', onClose);

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nUnsaved');

    await user.click(screen.getByRole('button', { name: 'Close' }));

    expect(onClose).not.toHaveBeenCalled();
    expect(await screen.findByRole('heading', { name: /discard changes/i })).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'Discard Changes' }));

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it('prompts for discard before closing a dirty markdown modal with Escape', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal('docs/guide.md', onClose);

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nUnsaved');

    await user.keyboard('{Escape}');

    expect(onClose).not.toHaveBeenCalled();
    expect(await screen.findByRole('heading', { name: /discard changes/i })).toBeInTheDocument();
  });

  it('prompts for discard before closing a dirty markdown modal with a backdrop click', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();

    getSkillFile.mockResolvedValueOnce({
      filename: 'docs/guide.md',
      contentType: 'text/markdown',
      content: '# Guide\n\nBody',
    });

    renderModal('docs/guide.md', onClose);

    await user.click(await screen.findByRole('button', { name: 'Edit' }));
    await user.type(screen.getByRole('textbox', { name: 'Raw markdown editor' }), '\nUnsaved');

    fireEvent.mouseDown(screen.getByRole('dialog'));

    expect(onClose).not.toHaveBeenCalled();
    expect(await screen.findByRole('heading', { name: /discard changes/i })).toBeInTheDocument();
  });
});
