import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { I18nProvider } from '../../i18n';
import { ProjectTargets } from './MCPProjectSettings';

const renderTargets = (onChange = vi.fn()) => {
  render(<I18nProvider><ProjectTargets value={['opencode', 'pi']} defaults={['claude']} offered={['claude', 'opencode', 'pi']} onChange={onChange} /></I18nProvider>);
  return onChange;
};

describe('project targets', () => {
  it("asks before going back to the global targets, which drops the project's own list", async () => {
    const user = userEvent.setup();
    const onChange = renderTargets();
    await user.click(screen.getByText('Same as global'));
    expect(onChange).not.toHaveBeenCalled();
  });

  it('follows the global targets once that is confirmed', async () => {
    const user = userEvent.setup();
    const onChange = renderTargets();
    await user.click(screen.getByText('Same as global'));
    await user.click(screen.getByRole('button', { name: 'Use global targets' }));
    expect(onChange).toHaveBeenCalledWith(undefined);
  });
});
