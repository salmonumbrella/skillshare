import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it } from 'vitest';
import { I18nProvider, LOCALE_STORAGE_KEY } from '../i18n';
import LanguagePopover from './LanguagePopover';

it('lets a keyboard user browse languages, cancel, and choose a different locale', async () => {
  const user = userEvent.setup();
  localStorage.setItem(LOCALE_STORAGE_KEY, 'en');
  render(
    <I18nProvider>
      <LanguagePopover />
    </I18nProvider>,
  );
  const trigger = screen.getByRole('button');
  await user.tab();
  await user.keyboard('{Enter}');
  expect(screen.getByText('English').closest('button')).toHaveFocus();
  await user.keyboard('{ArrowUp}');
  expect(screen.getByText('Bahasa Indonesia').closest('button')).toHaveFocus();
  await user.keyboard('{Escape}');
  expect(trigger).toHaveFocus();
  expect(localStorage.getItem(LOCALE_STORAGE_KEY)).toBe('en');

  await user.keyboard('{Enter}{ArrowDown}');
  expect(screen.getByText('繁體中文').closest('button')).toHaveFocus();
  await user.keyboard('{Enter}');
  expect(document.documentElement.lang).toBe('zh-TW');
  expect(trigger).toHaveAttribute('aria-expanded', 'false');
  expect(trigger).toHaveFocus();
});
