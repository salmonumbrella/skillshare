import { useOnline, useSlow } from '../../hooks/useSlow';
import { useT } from '../../i18n';

/** Why reading a source may be taking long: the machine is offline, or the source is being downloaded. */
export default function SourceHint({ active }: { active: boolean }) {
  const t = useT();
  const online = useOnline();
  const slow = useSlow(active, 5000);
  if (!online) return <span className="text-xs text-warn">{t('plugins.offline')}</span>;
  return slow ? <span className="text-xs text-ink-3">{t('plugins.sourceSlow')}</span> : null;
}
