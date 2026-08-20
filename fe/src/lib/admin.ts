import { apiRequest } from "@/lib/api-client";

export type NewsPollKeyword = {
  id: number;
  market: "domestic" | "us";
  keyword: string;
  enabled: boolean;
  sortOrder: number;
  createdAt: string;
  updatedAt: string;
};

export type AdminNewsStats = {
  domesticCount: number;
  usCount: number;
  domesticUpdated: string | null;
  usUpdated: string | null;
};

export type ApiQuotaCategory = "market" | "corp" | "news" | "translation" | "";

export type ApiQuotaUsage = {
  provider: string;
  label: string;
  purpose: string;
  category: ApiQuotaCategory;
  monthCount: number;
  dayCount: number;
  minuteCount: number;
  minuteTokens?: number;
  dayTokens?: number;
  lastCallAt?: string;
  blockedUntil?: string;
  limits: {
    monthlyQuota?: number;
    dailyQuota?: number;
    perMinute?: number;
    minIntervalMs?: number;
    tokensPerMinute?: number;
    tokensPerDay?: number;
  };
};

/** Mirrors be/internal/marketdata/apibudget/catalog.go CategoryLabels. */
export const API_QUOTA_CATEGORY_LABELS: Record<string, string> = {
  market: "시장 데이터",
  corp: "공시 · 재무",
  news: "뉴스",
  translation: "번역",
};

function adminFetch<T>(path: string, init: RequestInit = {}): Promise<T> {
  return apiRequest<T>(
    path,
    {
      ...init,
      headers: {
        "Content-Type": "application/json",
        ...(init.headers ?? {}),
      },
    },
    "Request failed",
  );
}

export async function fetchAdminNewsKeywords(market: "domestic" | "us"): Promise<NewsPollKeyword[]> {
  return adminFetch<NewsPollKeyword[]>(`/api/v1/admin/news-keywords?market=${market}`);
}

export async function createAdminNewsKeyword(input: {
  market: "domestic" | "us";
  keyword: string;
  enabled?: boolean;
  sortOrder?: number;
}): Promise<NewsPollKeyword> {
  return adminFetch<NewsPollKeyword>("/api/v1/admin/news-keywords", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function updateAdminNewsKeyword(
  id: number,
  input: { keyword: string; enabled: boolean; sortOrder: number },
): Promise<NewsPollKeyword> {
  return adminFetch<NewsPollKeyword>(`/api/v1/admin/news-keywords/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteAdminNewsKeyword(id: number): Promise<void> {
  await adminFetch<{ ok: boolean }>(`/api/v1/admin/news-keywords/${id}`, {
    method: "DELETE",
  });
}

export async function fetchAdminNewsStats(): Promise<AdminNewsStats> {
  return adminFetch<AdminNewsStats>("/api/v1/admin/news/stats");
}

export async function resetAdminNewsCursors(): Promise<void> {
  await adminFetch<{ ok: boolean }>("/api/v1/admin/news/reset-cursors", {
    method: "POST",
  });
}

export type AdminNewsPollSettings = {
  domesticNaverKeywordsPerTick: number;
  domesticKakaoKeywordsPerTick: number;
  domesticKakaoPollMinutes: number;
  usGoogleKeywordsPerTick: number;
  updatedAt?: string;
};

export async function fetchAdminNewsPollSettings(): Promise<AdminNewsPollSettings> {
  return adminFetch<AdminNewsPollSettings>("/api/v1/admin/news/poll-settings");
}

export async function updateAdminNewsPollSettings(
  input: Omit<AdminNewsPollSettings, "updatedAt">,
): Promise<AdminNewsPollSettings> {
  return adminFetch<AdminNewsPollSettings>("/api/v1/admin/news/poll-settings", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function fetchAdminApiQuotas(): Promise<ApiQuotaUsage[]> {
  return adminFetch<ApiQuotaUsage[]>("/api/v1/admin/api-quotas");
}

export type StockSummaryPilotSymbolStatus = {
  symbol: string;
  cached: boolean;
  source?: "llm" | "rule";
  generatedAt?: string;
};

export type StockSummaryStatus = {
  provider: string;
  enabled: boolean;
  symbols: StockSummaryPilotSymbolStatus[];
};

export async function fetchAdminStockSummaryStatus(): Promise<StockSummaryStatus> {
  return adminFetch<StockSummaryStatus>("/api/v1/admin/stock-summary");
}

/** Mirrors be/internal/model/llm_settings.go. */
export type LLMTargetMode = "radar" | "manual";

export type LLMSettings = {
  /** Always-include symbols, run ahead of the ranking-derived targets. */
  pilotSymbols: string[];
  targetMode: LLMTargetMode;
  dailyCap: number;
  quotaReserve: number;
  tokenReserve: number;
  updatedAt?: string;
};

export async function fetchAdminLLMSettings(): Promise<LLMSettings> {
  return adminFetch<LLMSettings>("/api/v1/admin/llm-settings");
}

export async function updateAdminLLMSettings(
  input: Partial<Omit<LLMSettings, "updatedAt">> & { pilotSymbols: string[] },
): Promise<LLMSettings> {
  return adminFetch<LLMSettings>("/api/v1/admin/llm-settings", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

/** Mirrors be/internal/model/llm_batch.go. */
export type LLMBatchSymbolResult = {
  symbol: string;
  name?: string;
  status: "done" | "error" | "skipped";
  source?: "llm" | "rule";
  tokens?: number;
  error?: string;
};

export type LLMBatchRun = {
  market: string;
  tradeDate: string;
  startedAt: string;
  finishedAt?: string;
  deadlineAt: string;
  running: boolean;
  considered: number;
  excluded?: Record<string, number>;
  planned: number;
  succeeded: number;
  failed: number;
  skipped: number;
  tokensUsed: number;
  callsRemaining: number;
  tokensRemaining: number;
  stopReason?: string;
  carryover?: string[];
  symbols: LLMBatchSymbolResult[] | null;
};

export type LLMBatchMarketStatus = {
  market: string;
  nextRunAt?: string;
  nextDeadlineAt?: string;
  lastRun?: LLMBatchRun;
  history?: LLMBatchRun[];
};

export type LLMBatchStatus = {
  targetMode: LLMTargetMode;
  dailyCap: number;
  /** -1 means the provider ceiling is unset, not that nothing is left. */
  callsRemaining: number;
  tokensRemaining: number;
  avgTokensPerCall: number;
  markets: LLMBatchMarketStatus[];
};

export type LLMBatchTarget = {
  symbol: string;
  name: string;
  rank: number;
  pinned?: boolean;
};

export type LLMBatchSelection = {
  market: string;
  considered: number;
  excluded?: Record<string, number>;
  targets: LLMBatchTarget[] | null;
  rankedAt?: string;
};

/** Why a ranked symbol never reached the LLM — mirrors LLMBatchExclude*. */
export const LLM_BATCH_EXCLUDE_LABELS: Record<string, string> = {
  derivative: "ETF·ETN·ETP",
  unlisted: "종목 정보 없음",
  suspended: "거래정지·상폐",
  over_budget: "한도 초과로 제외",
  deadline: "시간 초과로 제외",
};

export const LLM_BATCH_STOP_LABELS: Record<string, string> = {
  completed: "정상 완료",
  deadline: "개장 전 마감 시각 도달",
  quota: "일일 요청 한도",
  tokens: "일일 토큰 한도",
  shutdown: "서버 종료",
  select_failed: "대상 선정 실패",
};

export async function fetchAdminLLMBatchStatus(): Promise<LLMBatchStatus | null> {
  const data = await adminFetch<LLMBatchStatus | null>("/api/v1/admin/llm/batch");
  return data ?? null;
}

export async function fetchAdminLLMBatchTargets(market: "KR" | "US"): Promise<LLMBatchSelection | null> {
  const data = await adminFetch<LLMBatchSelection | null>(
    `/api/v1/admin/llm/batch/targets?market=${market}`,
  );
  return data ?? null;
}

/** Mirrors be/internal/model/llm_manual_refresh.go. */
export type LLMRefreshSymbolState = {
  symbol: string;
  status: "pending" | "running" | "done" | "error";
  source?: "llm" | "rule";
  generatedAt?: string;
  error?: string;
};

export type LLMManualRefreshJob = {
  id: string;
  running: boolean;
  startedAt: string;
  finishedAt?: string;
  symbols: LLMRefreshSymbolState[];
};

/** Max symbols per manual run — mirrors service.maxManualRefreshSymbols. */
export const LLM_MANUAL_REFRESH_MAX_SYMBOLS = 10;

export async function triggerAdminLLMRefresh(symbols: string[]): Promise<LLMManualRefreshJob> {
  return adminFetch<LLMManualRefreshJob>("/api/v1/admin/llm/refresh", {
    method: "POST",
    body: JSON.stringify({ symbols }),
  });
}

export async function fetchAdminLLMRefreshStatus(): Promise<LLMManualRefreshJob | null> {
  const data = await adminFetch<LLMManualRefreshJob | null>("/api/v1/admin/llm/refresh");
  return data ?? null;
}

export type LLMRequestLogEntry = {
  symbol: string;
  name: string;
  timestamp: string;
  success: boolean;
  error?: string;
  factsJson: string;
  manual?: boolean;
};

export type LLMRequestLogPage = {
  entries: LLMRequestLogEntry[];
  total: number;
  page: number;
  pageSize: number;
};

export async function fetchAdminLLMRequestLog(opts?: {
  status?: "all" | "failed";
  page?: number;
  pageSize?: number;
}): Promise<LLMRequestLogPage> {
  const params = new URLSearchParams();
  if (opts?.status && opts.status !== "all") params.set("status", opts.status);
  if (opts?.page) params.set("page", String(opts.page));
  if (opts?.pageSize) params.set("pageSize", String(opts.pageSize));
  const qs = params.toString();
  const data = await adminFetch<LLMRequestLogPage | null>(
    `/api/v1/admin/llm-requests${qs ? `?${qs}` : ""}`,
  );
  return data ?? { entries: [], total: 0, page: 1, pageSize: 30 };
}

export type AdminMarketDataSettings = {
  rankingsPollRealtimeMinutes: number;
  rankingsPollShortMinutes: number;
  rankingsPollLongMinutes: number;
  quoteOverlayPollSeconds: number;
  rankingsClientPollSeconds: number;
  sharesPollHours: number;
  fundamentalsPollHours: number;
  updatedAt?: string;
};

export async function fetchAdminMarketDataSettings(): Promise<AdminMarketDataSettings> {
  return adminFetch<AdminMarketDataSettings>("/api/v1/admin/market-data/settings");
}

export async function updateAdminMarketDataSettings(
  input: Omit<AdminMarketDataSettings, "updatedAt">,
): Promise<AdminMarketDataSettings> {
  return adminFetch<AdminMarketDataSettings>("/api/v1/admin/market-data/settings", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export type CacheNamespace = {
  id: string;
  label: string;
  pattern: string;
  description: string;
};

export type CacheEntrySummary = {
  key: string;
  type: string;
  ttlSeconds: number;
  sizeBytes: number;
};

export type CacheListResult = {
  pattern: string;
  cursor: number;
  entries: CacheEntrySummary[];
};

export type CacheEntryDetail = {
  key: string;
  type: string;
  ttlSeconds: number;
  value: string;
  valueFormat: "json" | "text";
  readOnly: boolean;
};

/** Plain keywords become *keyword*; scoped to namespace prefix when possible. */
export function buildCacheScanPattern(namespacePattern: string, keyword: string): string {
  const ns = namespacePattern.trim() || "quotes:*";
  const term = keyword.trim();
  if (!term) return ns;
  if (/[*?\[]/.test(term)) return term;
  const prefix = ns.replace(/\*+$/, "");
  if (prefix && prefix !== "*" && !term.includes(":")) {
    return `${prefix}*${term}*`;
  }
  return `*${term}*`;
}

export async function fetchAdminCacheNamespaces(): Promise<CacheNamespace[]> {
  const list = await adminFetch<CacheNamespace[]>("/api/v1/admin/cache/namespaces");
  return list ?? [];
}

export async function fetchAdminCacheEntries(params: {
  pattern: string;
  cursor?: number;
  limit?: number;
}): Promise<CacheListResult> {
  const q = new URLSearchParams();
  q.set("pattern", params.pattern);
  if (params.cursor != null) q.set("cursor", String(params.cursor));
  if (params.limit != null) q.set("limit", String(params.limit));
  const data = await adminFetch<CacheListResult>(`/api/v1/admin/cache/entries?${q.toString()}`);
  return { ...data, entries: data.entries ?? [] };
}

export async function fetchAdminCacheEntry(key: string): Promise<CacheEntryDetail> {
  const q = new URLSearchParams({ key });
  return adminFetch<CacheEntryDetail>(`/api/v1/admin/cache/entry?${q.toString()}`);
}

export async function updateAdminCacheEntry(input: {
  key: string;
  value: string;
  ttlSeconds?: number | null;
}): Promise<CacheEntryDetail> {
  return adminFetch<CacheEntryDetail>("/api/v1/admin/cache/entry", {
    method: "PUT",
    body: JSON.stringify({
      key: input.key,
      value: input.value,
      ttlSeconds: input.ttlSeconds ?? undefined,
    }),
  });
}

export async function deleteAdminCacheEntry(key: string): Promise<void> {
  const q = new URLSearchParams({ key });
  await adminFetch<{ ok: boolean }>(`/api/v1/admin/cache/entry?${q.toString()}`, {
    method: "DELETE",
  });
}

// ---------------------------------------------------------------- database

export type DBTableAccess = "full" | "read-only" | "redacted";

export type DBTableSummary = {
  name: string;
  group: string;
  groupLabel: string;
  label: string;
  purpose: string;
  access: DBTableAccess;
  primaryKey: string[];
  defaultSort: string;
  redactColumns?: string[];
  note?: string;
  estimatedRows: number;
  dataBytes: number;
  indexBytes: number;
  exists: boolean;
};

export type DBColumnInfo = {
  name: string;
  dataType: string;
  columnType: string;
  nullable: boolean;
  default?: string;
  extra?: string;
  isPrimary: boolean;
  redacted: boolean;
  readOnly: boolean;
};

export type DBTableDetail = DBTableSummary & { columns: DBColumnInfo[] };

export type DBRow = Record<string, string | null>;

export type DBRowPage = {
  table: string;
  columns: DBColumnInfo[];
  rows: DBRow[];
  total: number;
  limit: number;
  offset: number;
};

export async function fetchAdminDBTables(): Promise<DBTableSummary[]> {
  const data = await adminFetch<DBTableSummary[] | null>("/api/v1/admin/db/tables");
  return data ?? [];
}

export function fetchAdminDBTable(table: string): Promise<DBTableDetail> {
  return adminFetch<DBTableDetail>(`/api/v1/admin/db/tables/${encodeURIComponent(table)}`);
}

export async function fetchAdminDBRows(params: {
  table: string;
  limit?: number;
  offset?: number;
  q?: string;
  sortBy?: string;
  sortDir?: "asc" | "desc";
}): Promise<DBRowPage> {
  const search = new URLSearchParams();
  if (params.limit != null) search.set("limit", String(params.limit));
  if (params.offset != null) search.set("offset", String(params.offset));
  if (params.q) search.set("q", params.q);
  if (params.sortBy) search.set("sortBy", params.sortBy);
  if (params.sortDir) search.set("sortDir", params.sortDir);
  const page = await adminFetch<DBRowPage>(
    `/api/v1/admin/db/tables/${encodeURIComponent(params.table)}/rows?${search.toString()}`,
  );
  return { ...page, rows: page.rows ?? [], columns: page.columns ?? [] };
}

export function createAdminDBRow(
  table: string,
  values: Record<string, unknown>,
): Promise<{ affected: number }> {
  return adminFetch<{ affected: number }>(
    `/api/v1/admin/db/tables/${encodeURIComponent(table)}/rows`,
    { method: "POST", body: JSON.stringify({ values }) },
  );
}

export function updateAdminDBRow(
  table: string,
  key: Record<string, unknown>,
  values: Record<string, unknown>,
): Promise<{ affected: number }> {
  return adminFetch<{ affected: number }>(
    `/api/v1/admin/db/tables/${encodeURIComponent(table)}/rows`,
    { method: "PUT", body: JSON.stringify({ key, values }) },
  );
}

export function deleteAdminDBRow(
  table: string,
  key: Record<string, unknown>,
): Promise<{ affected: number }> {
  return adminFetch<{ affected: number }>(
    `/api/v1/admin/db/tables/${encodeURIComponent(table)}/rows`,
    { method: "DELETE", body: JSON.stringify({ key }) },
  );
}

// ---- 점수 관리 (/admin/score) ---------------------------------------------

/** Mirrors be/internal/model/stock_score.go ScoreWeights. */
export type ScoreWeights = {
  technical: number;
  quality: number;
  growth: number;
  valuation: number;
  sentiment: number;
};

export type ScoreAxisId = keyof ScoreWeights;

export type ScoreAxisMeta = { id: ScoreAxisId; label: string };

export type ScoreSettings = {
  weights: ScoreWeights;
  coverageFloor: number;
  shrinkFull: number;
  updatedAt: string;
};

export type SymbolScoreWeights = {
  symbol: string;
  name?: string;
  weights: ScoreWeights;
  note?: string;
  updatedAt: string;
};

export type ScoreSettingsResponse = {
  settings: ScoreSettings;
  symbols: SymbolScoreWeights[];
  axes: ScoreAxisMeta[];
};

export type ScoreDistribution = {
  count: number;
  mean: number;
  stdDev: number;
  min: number;
  max: number;
  atFloor: number;
  atCeil: number;
  withheld: number;
  healthy: boolean;
  warnings?: string[];
};

export type ScoreStatus = {
  tradeDate: string;
  total?: number;
  scored?: number;
};

export function fetchAdminScoreSettings(): Promise<ScoreSettingsResponse> {
  return adminFetch<ScoreSettingsResponse>("/api/v1/admin/score/settings");
}

export function updateAdminScoreSettings(input: {
  weights: ScoreWeights;
  coverageFloor: number;
  shrinkFull: number;
}): Promise<ScoreSettings> {
  return adminFetch<ScoreSettings>("/api/v1/admin/score/settings", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function upsertAdminScoreSymbolWeights(input: {
  symbol: string;
  weights: ScoreWeights;
  note?: string;
}): Promise<{ symbol: string }> {
  return adminFetch<{ symbol: string }>("/api/v1/admin/score/symbol-weights", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteAdminScoreSymbolWeights(symbol: string): Promise<{ symbol: string }> {
  return adminFetch<{ symbol: string }>(
    `/api/v1/admin/score/symbol-weights?symbol=${encodeURIComponent(symbol)}`,
    { method: "DELETE" },
  );
}

export function recomputeAdminScores(
  tradeDate?: string,
): Promise<{ tradeDate: string; distribution: ScoreDistribution }> {
  const q = tradeDate ? `?tradeDate=${encodeURIComponent(tradeDate)}` : "";
  return adminFetch<{ tradeDate: string; distribution: ScoreDistribution }>(
    `/api/v1/admin/score/recompute${q}`,
    { method: "POST" },
  );
}

export function fetchAdminScoreStatus(): Promise<ScoreStatus> {
  return adminFetch<ScoreStatus>("/api/v1/admin/score/status");
}

// ---- 배치 관리 (/admin/batch) ----------------------------------------------

/** Mirrors be/internal/model/batch_job.go. */
export type BatchJobCost = "free" | "rate_limited";

/** 배치 관리 화면의 묶음. BE의 model.BatchJobCategory와 같은 값. */
export type BatchJobCategory = "ingest" | "derive" | "llm";

export type BatchJobSpec = {
  id: string;
  label: string;
  purpose: string;
  category: BatchJobCategory;
  cost: BatchJobCost;
  schedule: string;
  acceptsSince: boolean;
  acceptsLimit: boolean;
  note?: string;
};

export type BatchJobRun = {
  id: number;
  jobId: string;
  status: "running" | "done" | "error";
  triggerKind: string;
  startedAt: string;
  finishedAt?: string;
  message?: string;
  result?: Record<string, unknown>;
};

export type BatchJobStatus = {
  spec: BatchJobSpec;
  lastRun?: BatchJobRun;
  running: boolean;
  coverage?: Record<string, unknown>;
};

export type BatchJobsResponse = {
  jobs: BatchJobStatus[];
  history: BatchJobRun[];
};

export function fetchAdminBatchJobs(): Promise<BatchJobsResponse> {
  return adminFetch<BatchJobsResponse>("/api/v1/admin/batch/jobs");
}

export function runAdminBatchJob(input: {
  job: string;
  since?: string;
  limit?: number;
}): Promise<{ job: string; started: boolean }> {
  const params = new URLSearchParams({ job: input.job });
  if (input.since) params.set("since", input.since);
  if (input.limit) params.set("limit", String(input.limit));
  return adminFetch<{ job: string; started: boolean }>(
    `/api/v1/admin/batch/run?${params.toString()}`,
    { method: "POST" },
  );
}

export type BatchScheduleDoc = {
  markdown: string;
  path: string;
  source: "file" | "embed";
  writable: boolean;
};

/**
 * 배치 시간표·수동 실행 안내 문서.
 *
 * 화면이 규칙을 따로 설명하지 않고 리포지터리의 문서를 그대로 렌더한다 —
 * 두 벌로 두면 배치를 고칠 때 화면 설명이 남아 거짓말이 된다.
 * 저장은 같은 파일을 덮어쓴다(`PUT`).
 */
export function fetchAdminBatchSchedule(): Promise<BatchScheduleDoc> {
  return adminFetch<BatchScheduleDoc>("/api/v1/admin/batch/schedule");
}

export function saveAdminBatchSchedule(markdown: string): Promise<BatchScheduleDoc> {
  return adminFetch<BatchScheduleDoc>("/api/v1/admin/batch/schedule", {
    method: "PUT",
    body: JSON.stringify({ markdown }),
  });
}
