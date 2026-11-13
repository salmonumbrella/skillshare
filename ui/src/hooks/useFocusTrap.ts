import { useEffect, useRef } from 'react';

export function useFocusTrap(active: boolean) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!active || !ref.current) return;

    const el = ref.current;
    const selector = 'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])';

    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return;
      const focusable = el.querySelectorAll<HTMLElement>(selector);
      if (focusable.length === 0) {
        e.preventDefault();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault();
          last?.focus();
        }
      } else {
        if (document.activeElement === last) {
          e.preventDefault();
          first?.focus();
        }
      }
    };

    el.addEventListener('keydown', handler);
    // A field inside that took focus itself (autoFocus) keeps it.
    if (!el.contains(document.activeElement)) el.querySelector<HTMLElement>(selector)?.focus();
    return () => el.removeEventListener('keydown', handler);
  }, [active]);

  return ref;
}
