import { createContext } from 'react';

/**
 * The built-in Agent behind a target that is another config folder of it (claude-work
 * is claude), so such a target shows that Agent's logo. Empty outside the app shell.
 */
export const TargetAgents = createContext<Record<string, string>>({});
