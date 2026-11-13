import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { api, ApiError } from '../api/client';
import type { Overview } from '../api/client';
import { ToastProvider } from '../components/Toast';
import { I18nProvider } from '../i18n';
import { queryKeys } from '../lib/queryKeys';
import ConfigPage from './ConfigPage';

vi.mock('../context/AppContext', () => ({ useAppContext: () => ({ isProjectMode: false }) }));
// CodeMirror does not edit under jsdom; a textarea stands in for the editor, and a stub view records
// where the page puts the cursor.
const view = { text: '', state: { doc: { get length() { return view.text.length; }, toString: () => view.text } }, dispatch: vi.fn(), focus: vi.fn() };
vi.mock('@uiw/react-codemirror', () => ({
  default: ({ value, onChange, onCreateEditor }: { value: string; onChange: (v: string) => void; onCreateEditor?: (v: unknown) => void }) => {
    view.text = value;
    onCreateEditor?.(view);
    return <textarea aria-label="editor" value={value} onChange={(e) => onChange(e.target.value)} />;
  },
}));
vi.mock('../api/client', async (load) => {
  const actual = await load<typeof import('../api/client')>();
  return {
    ...actual,
    api: { ...actual.api, getOverview: vi.fn(), getConfig: vi.fn(), getSkillignore: vi.fn(), getAgentignore: vi.fn(), putConfig: vi.fn() },
  };
});

function renderPage(query: string) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = render(
    <MemoryRouter initialEntries={[`/config?${query}`]}>
      <QueryClientProvider client={client}><I18nProvider><ToastProvider><ConfigPage /></ToastProvider></I18nProvider></QueryClientProvider>
    </MemoryRouter>,
  );
  return { ...result, client };
}

describe('ConfigPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('skillshare:locale', 'en');
    vi.mocked(api.getOverview).mockResolvedValue({} as Overview);
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw: 'source: ~/skills\n' });
    vi.mocked(api.getSkillignore).mockResolvedValue({ exists: true, path: '', raw: 'draft-*\n' });
    vi.mocked(api.getAgentignore).mockResolvedValue({ exists: true, path: '', raw: 'old-*\n' });
  });

  it('reverts the open ignore file, not config.yaml', async () => {
    const user = userEvent.setup();
    renderPage('tab=skillignore');

    const editor = await screen.findByDisplayValue('draft-*');
    fireEvent.change(editor, { target: { value: 'draft-*\ntmp-*\n' } });
    await user.click(screen.getAllByRole('button', { name: 'Revert' })[0]);
    await user.click(within(await screen.findByRole('dialog')).getByRole('button', { name: 'Revert' }));

    expect((screen.getByLabelText('editor') as HTMLTextAreaElement).value).toBe('draft-*\n');
  });

});

describe('shared settings repair', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    localStorage.setItem('skillshare:locale', 'en');
    vi.mocked(api.getOverview).mockResolvedValue({} as Overview);
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw: 'source: ~/skills\n' });
    vi.mocked(api.getSkillignore).mockResolvedValue({ exists: true, path: '', raw: '' });
    vi.mocked(api.getAgentignore).mockResolvedValue({ exists: true, path: '', raw: '' });
  });
  it('keeps a refused config draft and translates the shared refusal', async () => {
    vi.mocked(api.putConfig).mockRejectedValue(new ApiError(409, 'server fallback', {
      code: 'shared_owned', params: { name: 'rules', path: '/home/test/team.yaml' },
    }));
    renderPage('');
    const editor = await screen.findByDisplayValue('source: ~/skills');
    fireEvent.change(editor, { target: { value: 'source: ~/skills\nmode: copy\n' } });
    await userEvent.click(screen.getAllByRole('button', { name: /^Save/ })[0]);
    expect(await screen.findByText(/rules comes from/)).toBeTruthy();
    expect((screen.getByLabelText('editor') as HTMLTextAreaElement).value).toContain('mode: copy');
    expect((screen.getAllByRole('button', { name: /^Save/ })[0] as HTMLButtonElement).disabled).toBe(false);
  });

  it('shows shared availability and clears it after refetch without disabling the editor', async () => {
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw: 'source: ~/skills\n',
      shared: { enabled: true, usable: false, path: '/team.yaml', reason: 'version' } });
    const { client } = renderPage('');
    expect(await screen.findByRole('alert')).toHaveTextContent('newer shared settings version');
    expect((screen.getByLabelText('editor') as HTMLTextAreaElement).disabled).toBe(false);
    vi.mocked(api.getConfig).mockResolvedValue({ config: {}, raw: 'source: ~/skills\n',
      shared: { enabled: true, usable: true, path: '/team.yaml' } });
    await client.refetchQueries({ queryKey: queryKeys.config });
    await waitFor(() => expect(screen.queryByRole('alert')).toBeNull());
  });

});
