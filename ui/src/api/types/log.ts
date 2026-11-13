// Log types
export interface LogEntry {
  ts: string;
  cmd: string;
  args?: Record<string, any>;
  status: string;
  msg?: string;
  ms?: number;
}

export interface LogListResponse {
  entries: LogEntry[];
  total: number;
  totalAll: number;
  commands: string[];
}

export interface CommandStats {
  total: number;
  ok: number;
  error: number;
  partial: number;
  blocked: number;
}

export interface LogStatsResponse {
  total: number;
  success_rate: number;
  by_command: Record<string, CommandStats>;
  last_operation?: LogEntry;
}
