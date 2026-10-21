"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, Plus, RefreshCw, Trash2 } from "lucide-react";
import Link from "next/link";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import { formatDateTimeKST } from "@/lib/format";
import {
  createAdminNewsKeyword,
  deleteAdminNewsKeyword,
  fetchAdminNewsKeywords,
  fetchAdminNewsPollSettings,
  fetchAdminNewsStats,
  fetchAdminMarketDataSettings,
  resetAdminNewsCursors,
  updateAdminNewsKeyword,
  updateAdminNewsPollSettings,
  updateAdminMarketDataSettings,
  type AdminMarketDataSettings,
  type AdminNewsPollSettings,
  type AdminNewsStats,
  type NewsPollKeyword,
} from "@/lib/admin";
import { cn } from "@/lib/utils";
import { useConfirm } from "@/components/providers/confirm-provider";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

type MarketTab = "domestic" | "us";

function formatDate(value: string | null | undefined) {
  if (!value) return "-";
  return formatDateTimeKST(value);
}

export function AdminPanel() {
  const confirm = useConfirm();
  const router = useRouter();
  const { session, loading } = useAuth();
  const [market, setMarket] = useState<MarketTab>("domestic");
  const [keywords, setKeywords] = useState<NewsPollKeyword[]>([]);
  const [stats, setStats] = useState<AdminNewsStats | null>(null);
  const [pollSettings, setPollSettings] = useState<AdminNewsPollSettings | null>(null);
  const [pollDraft, setPollDraft] = useState<AdminNewsPollSettings | null>(null);
  const [marketDataSettings, setMarketDataSettings] = useState<AdminMarketDataSettings | null>(null);
  const [marketDataDraft, setMarketDataDraft] = useState<AdminMarketDataSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [newKeyword, setNewKeyword] = useState("");
  const [newTopic, setNewTopic] = useState<NewsPollKeyword["topic"]>("");
  const [error, setError] = useState<string | null>(null);

  const loadAll = useCallback(async () => {
    setBusy(true);
    setError(null);
    try {
      const [kw, st, ps, mds] = await Promise.all([
        fetchAdminNewsKeywords(market),
        fetchAdminNewsStats(),
        fetchAdminNewsPollSettings(),
        fetchAdminMarketDataSettings(),
      ]);
      setKeywords(kw);
      setStats(st);
      setPollSettings(ps);
      setPollDraft(ps);
      setMarketDataSettings(mds);
      setMarketDataDraft(mds);
    } catch (e) {
      setError(toUserMessage(e, "불러오기 실패"));
    } finally {
      setBusy(false);
    }
  }, [market]);

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    void loadAll();
  }, [loading, session, router, loadAll]);

  const handleCreate = async () => {
    const keyword = newKeyword.trim();
    if (!keyword) return;
    setBusy(true);
    setError(null);
    try {
      await createAdminNewsKeyword({ market, keyword, enabled: true, topic: market === "domestic" ? newTopic : "" });
      setNewKeyword("");
      await loadAll();
    } catch (e) {
      setError(toUserMessage(e, "추가 실패"));
      setBusy(false);
    }
  };

  const handleToggle = async (item: NewsPollKeyword) => {
    setBusy(true);
    setError(null);
    try {
      await updateAdminNewsKeyword(item.id, {
        keyword: item.keyword,
        enabled: !item.enabled,
        sortOrder: item.sortOrder,
      });
      await loadAll();
    } catch (e) {
      setError(toUserMessage(e, "수정 실패"));
      setBusy(false);
    }
  };

  const handleTopic = async (item: NewsPollKeyword, topic: NewsPollKeyword["topic"]) => {
    setBusy(true);
    setError(null);
    try {
      await updateAdminNewsKeyword(item.id, {
        keyword: item.keyword,
        enabled: item.enabled,
        sortOrder: item.sortOrder,
        topic,
      });
      await loadAll();
    } catch (e) {
      setError(toUserMessage(e, "수정 실패"));
      setBusy(false);
    }
  };

  const handleDelete = async (id: number) => {
    if (!(await confirm({ title: "이 키워드를 삭제할까요?", confirmLabel: "삭제", tone: "danger" }))) return;
    setBusy(true);
    setError(null);
    try {
      await deleteAdminNewsKeyword(id);
      await loadAll();
    } catch (e) {
      setError(toUserMessage(e, "삭제 실패"));
      setBusy(false);
    }
  };

  const handleResetCursors = async () => {
    setBusy(true);
    setError(null);
    try {
      await resetAdminNewsCursors();
      await loadAll();
    } catch (e) {
      setError(toUserMessage(e, "커서 초기화 실패"));
      setBusy(false);
    }
  };

  const handleSavePollSettings = async () => {
    if (!pollDraft) return;
    setBusy(true);
    setError(null);
    try {
      const saved = await updateAdminNewsPollSettings({
        domesticNaverKeywordsPerTick: pollDraft.domesticNaverKeywordsPerTick,
        domesticKakaoKeywordsPerTick: pollDraft.domesticKakaoKeywordsPerTick,
        domesticKakaoPollMinutes: pollDraft.domesticKakaoPollMinutes,
        usGoogleKeywordsPerTick: pollDraft.usGoogleKeywordsPerTick,
      });
      setPollSettings(saved);
      setPollDraft(saved);
      setBusy(false);
    } catch (e) {
      setError(toUserMessage(e, "수집 설정 저장 실패"));
      setBusy(false);
    }
  };

  const handleSaveMarketDataSettings = async () => {
    if (!marketDataDraft) return;
    setBusy(true);
    setError(null);
    try {
      const saved = await updateAdminMarketDataSettings({
        rankingsPollRealtimeMinutes: marketDataDraft.rankingsPollRealtimeMinutes,
        rankingsPollShortMinutes: marketDataDraft.rankingsPollShortMinutes,
        rankingsPollLongMinutes: marketDataDraft.rankingsPollLongMinutes,
        quoteOverlayPollSeconds: marketDataDraft.quoteOverlayPollSeconds,
        rankingsClientPollSeconds: marketDataDraft.rankingsClientPollSeconds,
        sharesPollHours: marketDataDraft.sharesPollHours,
        fundamentalsPollHours: marketDataDraft.fundamentalsPollHours,
      });
      setMarketDataSettings(saved);
      setMarketDataDraft(saved);
      setBusy(false);
    } catch (e) {
      setError(toUserMessage(e, "마켓 데이터 설정 저장 실패"));
      setBusy(false);
    }
  };

  const usKeywordCount = market === "us" ? keywords.filter((k) => k.enabled).length : 0;
  const usCycleMinutes =
    pollDraft && usKeywordCount > 0
      ? Math.ceil(usKeywordCount / pollDraft.usGoogleKeywordsPerTick) * 2
      : null;

  if (loading || !isAdminSession(session)) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  return (
    <>
      <PageHeader title="뉴스 관리" subtitle="뉴스 수집 키워드 · 통계 · 마켓 레이더 폴링 설정" />
      <PageScrollShell bare>
        <div className="page-shell-wide min-w-0 space-y-6 py-4">
          <div className="flex flex-wrap items-center gap-2 text-cell">
            <Link
              href="/admin/api"
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
            >
              API 관리 →
            </Link>
            <Link
              href="/admin/cache"
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
            >
              캐시 관리 →
            </Link>
            <Link
              href="/admin/database"
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)]")}
            >
              데이터베이스 →
            </Link>
          </div>
          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
              {error}
            </div>
          )}

          <section className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <StatCard label="국내 기사" value={stats?.domesticCount ?? "-"} sub={formatDate(stats?.domesticUpdated)} />
            <StatCard label="해외 기사" value={stats?.usCount ?? "-"} sub={formatDate(stats?.usUpdated)} />
            <StatCard label="현재 탭 키워드" value={keywords.length} sub={market === "domestic" ? "국내" : "해외"} />
            <StatCard label="수집 커서" value="관리" sub="변경 후 초기화 권장" />
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-section font-semibold text-[var(--text-primary)]">수집 배치 설정</h2>
              <button
                type="button"
                onClick={() => void handleSavePollSettings()}
                disabled={busy || !pollDraft}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-50")}
              >
                저장
              </button>
            </div>
            <p className="mb-4 text-cell text-[var(--text-muted)]">
              각 주기마다 큐에서 가져와 검색하는 키워드 개수입니다. 저장 시 수집 커서가 자동 초기화됩니다.
            </p>
            {pollDraft && (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <PollSettingField
                  label="국내 · 네이버 (2분)"
                  hint="1~30, 키워드당 API 1회"
                  max={30}
                  value={pollDraft.domesticNaverKeywordsPerTick}
                  onChange={(v) => setPollDraft({ ...pollDraft, domesticNaverKeywordsPerTick: v })}
                />
                <PollSettingField
                  label="국내 · 카카오 (키워드/회)"
                  hint="1~10"
                  max={10}
                  value={pollDraft.domesticKakaoKeywordsPerTick}
                  onChange={(v) => setPollDraft({ ...pollDraft, domesticKakaoKeywordsPerTick: v })}
                />
                <PollSettingField
                  label="국내 · 카카오 (주기, 분)"
                  hint="1~60"
                  max={60}
                  value={pollDraft.domesticKakaoPollMinutes}
                  onChange={(v) => setPollDraft({ ...pollDraft, domesticKakaoPollMinutes: v })}
                />
                <PollSettingField
                  label="해외 · Google News (2분)"
                  hint="1~30"
                  max={30}
                  value={pollDraft.usGoogleKeywordsPerTick}
                  onChange={(v) => setPollDraft({ ...pollDraft, usGoogleKeywordsPerTick: v })}
                  extra={
                    market === "us" && usCycleMinutes != null
                      ? `활성 키워드 ${usKeywordCount}개 → 약 ${usCycleMinutes}분에 한 바퀴`
                      : undefined
                  }
                />
              </div>
            )}
            {pollSettings?.updatedAt && (
              <p className="mt-3 text-cell text-[var(--text-muted)]">
                마지막 저장: {formatDate(pollSettings.updatedAt)}
              </p>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
              <h2 className="text-section font-semibold text-[var(--text-primary)]">마켓 레이더 · 밸류에이션</h2>
              <button
                type="button"
                onClick={() => void handleSaveMarketDataSettings()}
                disabled={busy || !marketDataDraft}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-50")}
              >
                저장
              </button>
            </div>
            <p className="mb-4 text-cell text-[var(--text-muted)]">
              랭킹·시세 오버레이·재무/발행주식수 갱신 주기입니다. 배포 없이 조정할 수 있습니다.
            </p>
            {marketDataDraft && (
              <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
                <PollSettingField
                  label="랭킹 (실시간, 분)"
                  hint="BE poller · 1~60"
                  max={60}
                  value={marketDataDraft.rankingsPollRealtimeMinutes}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, rankingsPollRealtimeMinutes: v })}
                />
                <PollSettingField
                  label="랭킹 (1일·1주, 분)"
                  hint="5~120"
                  max={120}
                  value={marketDataDraft.rankingsPollShortMinutes}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, rankingsPollShortMinutes: v })}
                />
                <PollSettingField
                  label="랭킹 (장기, 분)"
                  hint="15~240"
                  max={240}
                  value={marketDataDraft.rankingsPollLongMinutes}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, rankingsPollLongMinutes: v })}
                />
                <PollSettingField
                  label="FE 랭킹 새로고침 (초)"
                  hint="10~600"
                  max={600}
                  value={marketDataDraft.rankingsClientPollSeconds}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, rankingsClientPollSeconds: v })}
                />
                <PollSettingField
                  label="목록 시세 오버레이 (초)"
                  hint="1~60"
                  max={60}
                  value={marketDataDraft.quoteOverlayPollSeconds}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, quoteOverlayPollSeconds: v })}
                />
                <PollSettingField
                  label="발행주식수 갱신 (시간)"
                  hint="1~168 · Toss catalog"
                  max={168}
                  value={marketDataDraft.sharesPollHours}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, sharesPollHours: v })}
                />
                <PollSettingField
                  label="재무제표 갱신 (시간)"
                  hint="1~168 · DART/EDGAR"
                  max={168}
                  value={marketDataDraft.fundamentalsPollHours}
                  onChange={(v) => setMarketDataDraft({ ...marketDataDraft, fundamentalsPollHours: v })}
                />
              </div>
            )}
            {marketDataSettings?.updatedAt && (
              <p className="mt-3 text-cell text-[var(--text-muted)]">
                마지막 저장: {formatDate(marketDataSettings.updatedAt)}
              </p>
            )}
          </section>

          <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
            <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
              <div className="flex gap-1">
                {([
                  ["domestic", "국내 뉴스"],
                  ["us", "해외 뉴스"],
                ] as const).map(([id, label]) => (
                  <button
                    key={id}
                    type="button"
                    onClick={() => setMarket(id)}
                    className={cn(
                      "rounded-lg px-3 py-1.5 text-sm font-medium transition-colors",
                      market === id
                        ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                        : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                    )}
                  >
                    {label}
                  </button>
                ))}
              </div>
              <div className="flex gap-2">
                <button
                  type="button"
                  onClick={() => void loadAll()}
                  disabled={busy}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:opacity-50")}
                >
                  <RefreshCw className={cn("h-4 w-4", busy && "animate-spin")} />
                  새로고침
                </button>
                <button
                  type="button"
                  onClick={() => void handleResetCursors()}
                  disabled={busy}
                  className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] text-[var(--text-secondary)] hover:bg-[var(--surface-elevated)] disabled:opacity-50")}
                >
                  수집 커서 초기화
                </button>
              </div>
            </div>

            <p className="mb-3 text-cell text-[var(--text-muted)]">
              {market === "domestic"
                ? "국내: 「종목 뉴스」는 네이버·카카오 종목 뉴스 큐에, 부동산·금리·예·적금·대출은 2분마다 2개씩 도는 주제 수집에 쓰입니다(제목에 주제어가 있는 기사만 남김)."
                : "해외: Google News RSS 토픽 검색에 사용됩니다. (Finnhub 종목 검색과 별도)"}
            </p>

            <div className="mb-4 flex gap-2">
              {market === "domestic" && (
                <select
                  value={newTopic}
                  onChange={(e) => setNewTopic(e.target.value as NewsPollKeyword["topic"])}
                  aria-label="키워드 종류"
                  className="rounded-lg border border-[var(--border)] bg-[var(--background)] px-2 py-2 text-sm"
                >
                  {KEYWORD_TOPICS.map((t) => (
                    <option key={t.id} value={t.id}>
                      {t.label}
                    </option>
                  ))}
                </select>
              )}
              <input
                value={newKeyword}
                onChange={(e) => setNewKeyword(e.target.value)}
                onKeyDown={(e) => e.key === "Enter" && void handleCreate()}
                placeholder="새 키워드"
                className="min-w-0 flex-1 rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm"
              />
              <button
                type="button"
                onClick={() => void handleCreate()}
                disabled={busy || !newKeyword.trim()}
                className={cn(buttonVariants({ variant: "default", size: "default" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-50")}
              >
                <Plus className="h-4 w-4" />
                추가
              </button>
            </div>

            <div className="overflow-x-auto">
              <table className="w-full min-w-[480px] text-left text-sm">
                <thead>
                  <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
                    <th className="px-2 py-2 font-medium">키워드</th>
                    {market === "domestic" && <th className="px-2 py-2 font-medium">종류</th>}
                    <th className="px-2 py-2 font-medium">순서</th>
                    <th className="px-2 py-2 font-medium">상태</th>
                    <th className="px-2 py-2 font-medium">작업</th>
                  </tr>
                </thead>
                <tbody>
                  {keywords.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="px-2 py-6 text-center text-[var(--text-muted)]">
                        등록된 키워드가 없습니다.
                      </td>
                    </tr>
                  ) : (
                    keywords.map((item) => (
                      <tr key={item.id} className="border-b border-[var(--border)]/60">
                        <td className="px-2 py-2 text-[var(--text-primary)]">{item.keyword}</td>
                        {market === "domestic" && (
                          <td className="px-2 py-2">
                            <select
                              value={item.topic ?? ""}
                              disabled={busy}
                              onChange={(e) => void handleTopic(item, e.target.value as NewsPollKeyword["topic"])}
                              aria-label={`${item.keyword} 종류`}
                              className="rounded-md border border-[var(--border)] bg-[var(--background)] px-1.5 py-0.5 text-cell"
                            >
                              {KEYWORD_TOPICS.map((t) => (
                                <option key={t.id} value={t.id}>
                                  {t.label}
                                </option>
                              ))}
                            </select>
                          </td>
                        )}
                        <td className="px-2 py-2 text-[var(--text-secondary)]">{item.sortOrder}</td>
                        <td className="px-2 py-2">
                          <button
                            type="button"
                            onClick={() => void handleToggle(item)}
                            disabled={busy}
                            className={cn(
                              "rounded-full px-2 py-0.5 text-cell font-medium",
                              item.enabled
                                ? "bg-emerald-500/15 text-emerald-400"
                                : "bg-[var(--surface-elevated)] text-[var(--text-muted)]",
                            )}
                          >
                            {item.enabled ? "사용" : "중지"}
                          </button>
                        </td>
                        <td className="px-2 py-2">
                          <button
                            type="button"
                            onClick={() => void handleDelete(item.id)}
                            disabled={busy}
                            className="inline-flex items-center gap-1 text-cell text-red-400 hover:text-red-300 disabled:opacity-50"
                          >
                            <Trash2 className="h-3.5 w-3.5" />
                            삭제
                          </button>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </section>

        </div>
      </PageScrollShell>
    </>
  );
}

function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
      <p className="text-cell text-[var(--text-muted)]">{label}</p>
      <p className="mt-1 text-2xl font-semibold text-[var(--text-primary)]">{value}</p>
      {sub && <p className="mt-1 text-cell text-[var(--text-secondary)]">{sub}</p>}
    </div>
  );
}

function PollSettingField({
  label,
  hint,
  max,
  value,
  onChange,
  extra,
}: {
  label: string;
  hint: string;
  max: number;
  value: number;
  onChange: (value: number) => void;
  extra?: string;
}) {
  return (
    <label className="block space-y-1.5">
      <span className="text-sm font-medium text-[var(--text-primary)]">{label}</span>
      <input
        type="number"
        min={1}
        max={max}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 py-2 text-sm"
      />
      <span className="block text-cell text-[var(--text-muted)]">{hint}</span>
      {extra && <span className="block text-cell text-[var(--brand)]">{extra}</span>}
    </label>
  );
}

// 국내 키워드 종류 — "" 는 종목 뉴스 큐, 나머지는 뉴스 카테고리 주제 차선(be news_topics.go).
const KEYWORD_TOPICS: { id: NewsPollKeyword["topic"]; label: string }[] = [
  { id: "", label: "종목 뉴스" },
  { id: "real_estate", label: "부동산" },
  { id: "rates", label: "금리" },
  { id: "deposit", label: "예·적금" },
  { id: "loan", label: "대출" },
];
