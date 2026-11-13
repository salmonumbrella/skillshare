import { Check, ChevronDown } from 'lucide-react';
import AgentIcon from '../AgentIcon';
import { targetLabel } from './mcpView';

const STACK = 6;

/** The selected Agents as a stack of logos. `text` is the count, or a word such as "inherited". */
export function TargetPill({ selected, text, expanded, label, onClick }: { selected: string[]; text: string; expanded: boolean; label: string; onClick: () => void }) {
  return (
    <button type="button" className={`ss-btn ${selected.length > 0 ? '!pl-1.5' : ''}`} aria-expanded={expanded} aria-label={label} onClick={onClick}>
      {selected.length > 0 && (
        <span className="ss-stack" aria-hidden="true">
          {selected.slice(0, STACK).map((target) => (
            <span key={target} className="ss-at"><AgentIcon target={target} size={13} /></span>
          ))}
        </span>
      )}
      {text}
      <ChevronDown size={14} className={expanded ? 'rotate-180' : ''} />
    </button>
  );
}

/** One tickable logo per Agent. Claude Desktop only takes stdio servers, so `http` locks it. */
export function TargetToggles({ offered, selected, onToggle, http = false, disabled = false }: { offered: readonly string[]; selected: string[]; onToggle: (target: string, on: boolean) => void; http?: boolean; disabled?: boolean }) {
  return <>
    {offered.map((target) => {
      const on = selected.includes(target);
      return (
        <button key={target} type="button" role="checkbox" aria-checked={on} className={`ss-tgl ${on ? 'on' : ''}`} onClick={() => onToggle(target, !on)} disabled={disabled || (target === 'claude-desktop' && http && !on)}>
          <span className="ic"><AgentIcon target={target} size={20} /><i><Check size={9} strokeWidth={3.5} /></i></span>
          {targetLabel(target)}{target === 'claude-desktop' && <span className="ss-tag">stdio</span>}
        </button>
      );
    })}
  </>;
}
