import { createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import { useOverviewQuery } from '../hooks/useSharedQueries';

interface AppContextValue {
  isProjectMode: boolean;
  projectRoot?: string;
  version?: string;
}

const AppContext = createContext<AppContextValue>({ isProjectMode: false });

export function useAppContext() {
  return useContext(AppContext);
}

export function AppProvider({ children }: { children: ReactNode }) {
  const { data } = useOverviewQuery();

  const value: AppContextValue = {
    isProjectMode: data?.isProjectMode ?? false,
    projectRoot: data?.projectRoot,
    version: data?.version,
  };

  return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}
