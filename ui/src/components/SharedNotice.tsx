import { AlertTriangle } from 'lucide-react';
import type { SharedStatusPayload } from '../api/types/config';
import { useT } from '../i18n';
import { sharedReasonMessage } from '../lib/sharedSettings';

export default function SharedNotice({ shared }: { shared?: SharedStatusPayload }) {
  const t = useT();
  if (!shared?.enabled) return null;
  return <>
    {!shared.usable && <div className="ss-note bad" role="alert">
      <AlertTriangle size={16} />
      <span className="min-w-0 flex-1 break-words">{sharedReasonMessage(shared, t)}</span>
    </div>}
    {shared.warnings?.map((warning) => <div key={warning} className="ss-note warn" role="status">
      <AlertTriangle size={16} /><span className="min-w-0 flex-1 break-words">{warning}</span>
    </div>)}
  </>;
}
