import { apiRequest } from "@/lib/api-client";
import type { PageMetaInput, PageMetaOverride } from "@/lib/page-meta-types";

export type NewsPollKeyword = {
  id: number;
  market: "domestic" | "us";
  keyword: string;
  /** "" = 종목 뉴스 큐의 일반 키워드 · 그 밖은 국내 뉴스 주제 차선 검색어(2026-10-09). */
  topic: "" | "real_estate" | "rates" | "deposit" | "loan";
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

export type ApiQuotaCategory = "market" | "corp" | "news" | "translation" | "llm" | "";

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
  llm: "AI 요약",
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
  topic?: NewsPollKeyword["topic"];
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
  input: { keyword: string; enabled: boolean; sortOrder: number; topic?: NewsPollKeyword["topic"] },
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
  /** Distinct GEMINI_KR / GEMINI_US keys (separate RPD buckets). */
  splitKeys?: boolean;
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
  callsRemaining: number;
  tokensRemaining: number;
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

/** Mirrors be/internal/service/score_weight_suggestion.go. */
export type WeightSuggestionAxis = {
  axis: keyof ScoreWeights;
  days: number;
  meanIC?: number;
  tStat?: number;
  significant: boolean;
  current: number;
  suggested: number;
};

export type WeightSuggestion = {
  market: "domestic" | "us";
  ready: boolean;
  reason?: string;
  minDays: number;
  axes: WeightSuggestionAxis[];
};

/** 성적표 기반 가중치 제안 — 저장하지 않는다. 적용은 편집기에 채운 뒤 사람이 저장한다. */
export function fetchAdminWeightSuggestion(market: "domestic" | "us"): Promise<WeightSuggestion> {
  return adminFetch<WeightSuggestion>(`/api/v1/admin/score/weight-suggestion?market=${market}`);
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

/**
 * 잡의 판정 상태. BE의 model.BatchHealth* 와 같은 값.
 *
 * 원장은 실행된 것만 알기 때문에, 실행되지 **않은** 것은 기대 시각을 따로 계산해야
 * 보인다 — 휴장이라 안 돈 것과 죽어서 안 돈 것을 가르는 것이 이 값의 전부다.
 */
export type BatchHealth =
  | "ok"
  | "pending"
  | "late"
  | "failed"
  | "stuck"
  | "unmanaged";

export const BATCH_HEALTH_LABELS: Record<BatchHealth, string> = {
  ok: "정상",
  pending: "대기",
  late: "지연",
  failed: "실패",
  stuck: "멈춤",
  unmanaged: "예약 없음",
};

/** 사람이 봐야 하는 상태인가. BE의 BatchHealthNeedsAttention과 같은 기준. */
export function batchHealthNeedsAttention(health?: string): boolean {
  return health === "late" || health === "failed" || health === "stuck";
}

export type BatchJobStatus = {
  spec: BatchJobSpec;
  lastRun?: BatchJobRun;
  running: boolean;
  coverage?: Record<string, unknown>;
  health?: BatchHealth;
  /** 이 잡이 가장 최근에 돌았어야 하는 시각. 예약이 없으면 없다. */
  expectedAt?: string;
};

export type BatchJobHealth = {
  jobId: string;
  label: string;
  health: BatchHealth;
  expectedAt?: string;
  lastSuccessAt?: string;
  lastRunAt?: string;
  lastStatus?: string;
  message?: string;
  lateBySeconds?: number;
};

export type BatchHealthReport = {
  computedAt: string;
  jobs: BatchJobHealth[];
  unhealthy: number;
  staleAfterSeconds: number;
};

/**
 * report는 **읽는 순간 계산한** 판정이고, watchdog* 은 저장된 마지막 판정의 나이다.
 * 워치독이 죽어도 판정은 옳게 나오되, 감시가 멈췄다는 사실을 함께 띄울 수 있어야 한다.
 */
export type BatchHealthView = {
  report: BatchHealthReport;
  watchdogAgeSeconds?: number;
  watchdogStale: boolean;
};

export type BatchJobsResponse = {
  jobs: BatchJobStatus[];
  history: BatchJobRun[];
  health?: BatchHealthView;
  /** 커버리지를 서버가 아직 한 번도 세지 못했다 — 잡 행의 `coverage`가 비어 있다. */
  coveragePending?: boolean;
};

export function fetchAdminBatchJobs(): Promise<BatchJobsResponse> {
  return adminFetch<BatchJobsResponse>("/api/v1/admin/batch/jobs");
}

/**
 * 판정만 가져오는 가벼운 입구 — 사이드바 뱃지가 모든 화면에서 주기적으로 부른다.
 * 목록·커버리지·이력이 딸려 오는 fetchAdminBatchJobs와 용도가 다르다.
 */
export function fetchAdminBatchHealth(): Promise<BatchHealthView> {
  return adminFetch<BatchHealthView>("/api/v1/admin/batch/health");
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

export type BatchOverdueItem = { jobId: string; label: string; reason?: string; llm?: boolean };
export type BatchOverduePlan = { run: BatchOverdueItem[]; skip: BatchOverdueItem[] };

/** 제때 돌지 않은 배치 일괄 실행. dryRun이면 계획만 받는다(제외·순서 보류는 서버가 정한다). */
export function runAdminOverdueBatches(dryRun: boolean): Promise<BatchOverduePlan> {
  return adminFetch<BatchOverduePlan>(
    `/api/v1/admin/batch/run-overdue${dryRun ? "?dryRun=1" : ""}`,
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

/* ---------------------------------------------------------------------------
   종목 별칭 (`/admin/aliases`) — 2026-09-14 사용자 요청 1

   무엇을 담는 표인지는 BE `migrations/070_stock_aliases.sql` 머리글에 있다.
   화면이 알아야 하는 것은 갈래가 둘이고 대상의 뜻이 갈래마다 다르다는 것 하나다.
--------------------------------------------------------------------------- */

/** 갈래. BE `repository.AliasKinds`와 같은 값이어야 한다. */
export type StockAliasKind = "brand" | "symbol";

export type StockAlias = {
  id: number;
  alias: string;
  kind: StockAliasKind;
  /** brand면 영문 표기(소문자), symbol이면 종목 심볼(대문자). */
  target: string;
  /** seed(마이그레이션) · toss(배치) · manual(사람). 화면은 지우기 경고에 쓴다. */
  source: string;
  note: string;
  enabled: boolean;
  createdAt: string;
  updatedAt: string;
};

export type StockAliasList = {
  items: StockAlias[];
  /** 켜진 행의 수. 기동 로그가 말하는 수와 같은 것을 센다. */
  summary: { brand: number; symbol: number; disabled: number };
};

export type StockAliasInput = {
  alias: string;
  kind: StockAliasKind;
  target: string;
  note?: string;
  enabled?: boolean;
};

export function fetchAdminStockAliases(params: {
  kind?: StockAliasKind | "";
  q?: string;
}): Promise<StockAliasList> {
  const search = new URLSearchParams();
  if (params.kind) search.set("kind", params.kind);
  if (params.q?.trim()) search.set("q", params.q.trim());
  const qs = search.toString();
  return adminFetch<StockAliasList>(`/api/v1/admin/aliases${qs ? `?${qs}` : ""}`);
}

export function createAdminStockAlias(input: StockAliasInput): Promise<StockAlias> {
  return adminFetch<StockAlias>("/api/v1/admin/aliases", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateAdminStockAlias(id: number, input: StockAliasInput): Promise<StockAlias> {
  return adminFetch<StockAlias>(`/api/v1/admin/aliases/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export async function deleteAdminStockAlias(id: number): Promise<void> {
  await adminFetch<{ id: number }>(`/api/v1/admin/aliases/${id}`, { method: "DELETE" });
}

/* ---------------------------------------------------------------------------
   화면 메타 (`/admin/seo`) — 2026-09-15 사용자 요청 2

   브라우저 탭 이름과 검색 결과에 나가는 글을 배포 없이 고친다. 표에 무엇이 들어
   있고 왜 「덮어쓰기」인지는 BE `migrations/074_page_meta.sql` 머리글에 있다.

   화면이 알아야 하는 것은 하나다 — **빈 칸은 코드 기본값**이고, 전부 비우면 그 행이
   지워진다. 그래서 저장과 되돌리기가 같은 입구다.
--------------------------------------------------------------------------- */

export function fetchAdminPageMeta(): Promise<{ items: PageMetaOverride[] }> {
  return adminFetch<{ items: PageMetaOverride[] }>("/api/v1/admin/page-meta");
}

export function saveAdminPageMeta(input: PageMetaInput): Promise<{ items: PageMetaOverride[] }> {
  return adminFetch<{ items: PageMetaOverride[] }>("/api/v1/admin/page-meta", {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function resetAdminPageMeta(path: string): Promise<{ items: PageMetaOverride[] }> {
  return adminFetch<{ items: PageMetaOverride[] }>(
    `/api/v1/admin/page-meta?path=${encodeURIComponent(path)}`,
    { method: "DELETE" },
  );
}

/**
 * 가중치를 **저장하기 전에** 결과를 본다 (2026-09-17 사용자 결정 D).
 *
 * 눌러 저장한 뒤 배치로 확인하면 17,906종목이 한 번 더 계산되고, 되돌리려면 또 한 번이다.
 * 이 입구는 저장된 한 행을 다른 가중치로 다시 합산할 뿐이라 쓰기도 배치도 없다.
 *
 * 축 값과 달리 가중치는 분모에도 들어가 선형이 아니므로, 서버가 축을 정해진 순서로
 * 하나씩 바꾸며 증분을 적는다(`order`). 합은 정확하고 각 항은 그 순서에서의 몫이다.
 */
export type ScoreWeightChange = {
  symbol: string;
  tradeDate: string;
  fromWeights: ScoreWeights;
  toWeights: ScoreWeights;
  fromScore: number;
  toScore: number;
  delta: number;
  exactDelta: number;
  order: string[];
  drivers: {
    kind: string;
    axis?: string;
    label: string;
    detail: string;
    effect: number;
    from?: number;
    to?: number;
  }[];
  roundingResidual: number;
};

export function fetchAdminScoreWeightPreview(
  symbol: string,
  weights: ScoreWeights,
): Promise<ScoreWeightChange | null> {
  const params = new URLSearchParams({ symbol });
  for (const [axis, value] of Object.entries(weights)) {
    params.set(axis, String(value));
  }
  return adminFetch<ScoreWeightChange | null>(
    `/api/v1/admin/score/explain-weights?${params.toString()}`,
  );
}

/* ---------------------------------------------------------------------------
   검색 유입 보고서 (`/admin/growth`) — 2026-09-25

   보고서는 worker-growth가 매일 07:30에 만든다(docs/GROWTH.md). 이 화면은 읽기만
   한다 — 다시 만들기는 배치 콘솔의 「검색 유입 감시」가 한 곳이다.
--------------------------------------------------------------------------- */

export type GrowthReportKind = "daily" | "weekly";

export type GrowthReportSummary = {
  id: number;
  kind: GrowthReportKind;
  periodStart: string;
  periodEnd: string;
  createdAt: string;
};

export type GrowthReport = GrowthReportSummary & { markdown: string };

export function fetchAdminGrowthReports(kind: GrowthReportKind, limit = 60): Promise<GrowthReportSummary[]> {
  return adminFetch<GrowthReportSummary[]>(`/api/v1/admin/growth/reports?kind=${kind}&limit=${limit}`);
}

export function fetchAdminGrowthReport(id: number): Promise<GrowthReport> {
  return adminFetch<GrowthReport>(`/api/v1/admin/growth/reports/${id}`);
}
