import { useLayoutEffect, useRef, useState } from 'react';

/**
 * Height that makes the element reach the bottom of the viewport, leaving
 * bottom px (the page's own bottom padding) and never less than min. It is
 * remeasured as the page above it or the window changes.
 */
export function useFillHeight<T extends HTMLElement>(bottom: number, min: number) {
  const ref = useRef<T>(null);
  const [height, setHeight] = useState<number>();
  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const measure = () => {
      const top = el.getBoundingClientRect().top + window.scrollY;
      setHeight(Math.max(min, Math.floor(window.innerHeight - top - bottom)));
    };
    measure();
    window.addEventListener('resize', measure);
    // jsdom has no ResizeObserver; the page then keeps its first measure.
    const ro = typeof ResizeObserver === 'undefined' ? null : new ResizeObserver(measure);
    ro?.observe(document.body);
    return () => {
      window.removeEventListener('resize', measure);
      ro?.disconnect();
    };
  }, [bottom, min]);
  return [ref, height] as const;
}
