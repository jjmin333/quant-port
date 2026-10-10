"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { FlaskConical, Gauge, Loader2, Play, Plus, RefreshCw, Save, Trash2 } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { isAdminSession } from "@/lib/admin-features";
import {
  deleteAdminScoreSymbolWeights,
  fetchAdminScoreSettings,
  fetchAdminScoreStatus,
  fetchAdminScoreWeightPreview,
  type ScoreWeightChange,
  recomputeAdminScores,
  updateAdminScoreSettings,
  upsertAdminScoreSymbolWeights,
  type ScoreAxisMeta,
  type ScoreDistribution,
  type ScoreSettings,
  type ScoreStatus,
  type ScoreWeights,
  type SymbolScoreWeights,
} from "@/lib/admin";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";
import { AdminWeightSuggestion } from "@/components/admin/admin-weight-suggestion";

const AXIS_ORDER: (keyof ScoreWeights)[] = [
  "technical",
  "quality",
  "growth",
  "valuation",
  "sentiment",
];

const FALLBACK_AXES: ScoreAxisMeta[] = [
  { id: "technical", label: "추세" },
  { id: "quality", label: "수익성" },
  { id: "growth", label: "성장" },
  { id: "valuation", label: "가치" },
  { id: "sentiment", label: "심리·수급" },
];

const AXIS_HINTS: Record<keyof ScoreWeights, string> = {
  technical: "RSI·MACD·이동평균·52주 위치 (일봉)",
  quality: "ROE·영업이익률·순이익률·부채비율·유동비율",
  growth: "매출 YoY·EPS YoY",
  valuation: "PER·PBR·PSR (낮을수록 높은 점수)",
  sentiment: "뉴스 감성(로컬 모델)·내부자 거래·국내 수급(외국인·기관 순매수)",
};

const DEFAULT_WEIGHTS: ScoreWeights = {
  technical: 30,
  quality: 25,
  growth: 20,
  valuation: 15,
  sentiment: 10,
};

/**
 * Weights are relative: the backend renormalizes over whichever axes actually
 * have data. The editor still shows each axis's share of the current total,
 * because that is how people reason about "뉴스가 5% 정도" — but it never forces
 * the inputs to sum to 100.
 */
function sharePercent(weights: ScoreWeights, axis: keyof ScoreWeights): number {
  const total = AXIS_ORDER.reduce((sum, key) => sum + (weights[key] || 0), 0);
  if (total <= 0) return 0;
  return ((weights[axis] || 0) / total) * 100;
}

function WeightEditor({
  weights,
  axes,
  onChange,
  disabled,
  compact = false,
}: {
  weights: ScoreWeights;
  axes: ScoreAxisMeta[];
  onChange: (next: ScoreWeights) => void;
  disabled?: boolean;
  compact?: boolean;
}) {
  const total = AXIS_ORDER.reduce((sum, key) => sum + (weights[key] || 0), 0);

  return (
    <div className="space-y-2">
      {axes.map((axis) => {
        const value = weights[axis.id] ?? 0;
        const share = sharePercent(weights, axis.id);
        return (
          <div key={axis.id} className="flex items-center gap-3">
            <div className={cn("shrink-0", compact ? "w-20" : "w-24")}>
              <div className="text-cell font-medium text-[var(--text-primary)]">{axis.label}</div>
              {!compact && (
                <div className="text-micro leading-tight text-[var(--text-muted)]">
                  {AXIS_HINTS[axis.id]}
                </div>
              )}
            </div>
            <input
              type="range"
              min={0}
              max={100}
              step={1}
              value={value}
              disabled={disabled}
              onChange={(e) => onChange({ ...weights, [axis.id]: Number(e.target.value) })}
              className="h-1.5 min-w-0 flex-1 accent-[var(--brand)]"
              aria-label={`${axis.label} 가중치`}
            />
            <input
              type="number"
              min={0}
              max={100}
              value={value}
              disabled={disabled}
              onChange={(e) => onChange({ ...weights, [axis.id]: Number(e.target.value) })}
              className="w-14 shrink-0 rounded border border-[var(--border)] bg-[var(--surface)] px-1.5 py-1 text-right text-cell"
            />
            <span className="w-14 shrink-0 text-right text-cell tabular-nums text-[var(--text-secondary)]">
              {share.toFixed(1)}%
            </span>
          </div>
        );
      })}
      <div className="flex justify-between border-t border-[var(--border)] pt-2 text-cell text-[var(--text-muted)]">
        <span>합계 {total}</span>
        <span>가중치는 상대값 — 데이터가 있는 축끼리 자동 재정규화됩니다</span>
      </div>
    </div>
  );
}

function DistributionCard({ dist }: { dist: ScoreDistribution }) {
  return (
    <div
      className={cn(
        "rounded-lg border px-3 py-2.5 text-cell",
        dist.healthy
          ? "border-[var(--border)] bg-[var(--surface-elevated)]/50"
          : "border-[var(--danger)]/40 bg-[var(--danger)]/5",
      )}
    >
      <div className="mb-1.5 flex items-center gap-2 font-medium">
        <span>분포 검증</span>
        <span className={dist.healthy ? "text-[var(--success)]" : "text-[var(--danger)]"}>
          {dist.healthy ? "정상" : "경고"}
        </span>
      </div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1 tabular-nums sm:grid-cols-4">
        <span className="text-[var(--text-muted)]">점수 산출 {dist.count.toLocaleString()}</span>
        <span className="text-[var(--text-muted)]">평균 {dist.mean.toFixed(1)}</span>
        <span className="text-[var(--text-muted)]">표준편차 {dist.stdDev.toFixed(1)}</span>
        <span className="text-[var(--text-muted)]">
          범위 {dist.min}~{dist.max}
        </span>
        <span className="text-[var(--text-muted)]">보류 {dist.withheld.toLocaleString()}</span>
        <span className="text-[var(--text-muted)]">0점 부근 {dist.atFloor}</span>
        <span className="text-[var(--text-muted)]">100점 부근 {dist.atCeil}</span>
      </div>
      {dist.warnings?.length ? (
        <ul className="mt-2 list-inside list-disc space-y-0.5 text-cell text-[var(--danger)]">
          {dist.warnings.map((wrn) => (
            <li key={wrn}>{wrn}</li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

export function AdminScorePanel() {
  const router = useRouter();
  const { session, loading: authLoading } = useAuth();
  const isAdmin = isAdminSession(session);

  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [axes, setAxes] = useState<ScoreAxisMeta[]>(FALLBACK_AXES);
  const [settings, setSettings] = useState<ScoreSettings | null>(null);
  const [weights, setWeights] = useState<ScoreWeights>(DEFAULT_WEIGHTS);
  const [coverageFloor, setCoverageFloor] = useState(0.2);
  const [shrinkFull, setShrinkFull] = useState(0.6);
  const [symbols, setSymbols] = useState<SymbolScoreWeights[]>([]);
  const [status, setStatus] = useState<ScoreStatus | null>(null);

  const [saving, setSaving] = useState(false);
  const [recomputing, setRecomputing] = useState(false);
  const [dist, setDist] = useState<ScoreDistribution | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  /*
    가중치 바꿔 보기 (2026-09-17 사용자 결정 D). **저장하지 않는다** — 저장된 한 행을
    지금 편집 중인 가중치로 다시 합산해 볼 뿐이라 배치도 쓰기도 없다.
  */
  const [previewSymbol, setPreviewSymbol] = useState("005930");
  const [preview, setPreview] = useState<ScoreWeightChange | null>(null);
  const [previewBusy, setPreviewBusy] = useState(false);
  const [previewError, setPreviewError] = useState<string | null>(null);

  const [newSymbol, setNewSymbol] = useState("");
  const [newWeights, setNewWeights] = useState<ScoreWeights>(DEFAULT_WEIGHTS);
  const [newNote, setNewNote] = useState("");
  const [addingBusy, setAddingBusy] = useState(false);

  async function handlePreview() {
    const symbol = previewSymbol.trim().toUpperCase();
    if (!symbol) return;
    setPreviewBusy(true);
    setPreviewError(null);
    try {
      const res = await fetchAdminScoreWeightPreview(symbol, weights);
      setPreview(res);
      if (!res) setPreviewError("이 종목은 아직 점수가 산출되지 않았습니다.");
    } catch (e) {
      setPreview(null);
      setPreviewError(toUserMessage(e, "미리보기를 불러오지 못했습니다"));
    } finally {
      setPreviewBusy(false);
    }
  }

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [res, st] = await Promise.all([
        fetchAdminScoreSettings(),
        fetchAdminScoreStatus().catch(() => null),
      ]);
      setAxes(res.axes?.length ? res.axes : FALLBACK_AXES);
      setSettings(res.settings);
      setWeights(res.settings.weights);
      setCoverageFloor(res.settings.coverageFloor);
      setShrinkFull(res.settings.shrinkFull);
      setSymbols(res.symbols ?? []);
      if (st) setStatus(st);
    } catch (e) {
      setError(toUserMessage(e, "점수 설정을 불러오지 못했습니다"));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (authLoading) return;
    if (!isAdmin) {
      router.replace("/dashboard");
      return;
    }
    void load();
  }, [authLoading, isAdmin, router, load]);

  const dirty = useMemo(() => {
    if (!settings) return false;
    return (
      AXIS_ORDER.some((k) => settings.weights[k] !== weights[k]) ||
      settings.coverageFloor !== coverageFloor ||
      settings.shrinkFull !== shrinkFull
    );
  }, [settings, weights, coverageFloor, shrinkFull]);

  const totalWeight = AXIS_ORDER.reduce((sum, k) => sum + (weights[k] || 0), 0);

  async function handleSave() {
    if (totalWeight <= 0) {
      setError("가중치 합이 0보다 커야 합니다");
      return;
    }
    setSaving(true);
    setError(null);
    setNotice(null);
    try {
      const updated = await updateAdminScoreSettings({ weights, coverageFloor, shrinkFull });
      setSettings(updated);
      setWeights(updated.weights);
      setCoverageFloor(updated.coverageFloor);
      setShrinkFull(updated.shrinkFull);
      setNotice("저장했습니다. 반영하려면 재계산을 실행하세요.");
    } catch (e) {
      setError(toUserMessage(e, "저장 실패"));
    } finally {
      setSaving(false);
    }
  }

  async function handleRecompute() {
    setRecomputing(true);
    setError(null);
    setNotice(null);
    try {
      const res = await recomputeAdminScores();
      setDist(res.distribution);
      setNotice(`${res.tradeDate} 기준 재계산 완료`);
      const st = await fetchAdminScoreStatus().catch(() => null);
      if (st) setStatus(st);
    } catch (e) {
      setError(toUserMessage(e, "재계산 실패"));
    } finally {
      setRecomputing(false);
    }
  }

  async function handleAddSymbol() {
    const symbol = newSymbol.trim().toUpperCase();
    if (!symbol) return;
    setAddingBusy(true);
    setError(null);
    try {
      await upsertAdminScoreSymbolWeights({ symbol, weights: newWeights, note: newNote.trim() });
      setNewSymbol("");
      setNewNote("");
      setNewWeights(DEFAULT_WEIGHTS);
      await load();
      setNotice(`${symbol} 가중치를 저장했습니다. 반영하려면 재계산을 실행하세요.`);
    } catch (e) {
      setError(toUserMessage(e, "종목 가중치 저장 실패"));
    } finally {
      setAddingBusy(false);
    }
  }

  async function handleUpdateSymbol(item: SymbolScoreWeights, next: ScoreWeights) {
    setSymbols((prev) =>
      prev.map((s) => (s.symbol === item.symbol ? { ...s, weights: next } : s)),
    );
  }

  async function handleSaveSymbol(item: SymbolScoreWeights) {
    setError(null);
    try {
      await upsertAdminScoreSymbolWeights({
        symbol: item.symbol,
        weights: item.weights,
        note: item.note ?? "",
      });
      setNotice(`${item.symbol} 저장 완료. 반영하려면 재계산을 실행하세요.`);
    } catch (e) {
      setError(toUserMessage(e, "저장 실패"));
    }
  }

  async function handleDeleteSymbol(symbol: string) {
    setError(null);
    try {
      await deleteAdminScoreSymbolWeights(symbol);
      setSymbols((prev) => prev.filter((s) => s.symbol !== symbol));
      setNotice(`${symbol} 오버라이드를 제거했습니다 (전역 가중치 적용)`);
    } catch (e) {
      setError(toUserMessage(e, "삭제 실패"));
    }
  }

  if (authLoading || (!isAdmin && !error)) {
    return (
      <>
        <PageHeader title="점수 관리" />
        <PageScrollShell>
          <div className="p-6 text-sm text-[var(--text-muted)]">확인 중…</div>
        </PageScrollShell>
      </>
    );
  }

  return (
    <>
      <PageHeader
        title="점수 관리"
        subtitle="종합 점수의 축별 가중치 — 전역 기본값과 종목별 오버라이드"
      />
      <PageScrollShell>
      <div className="space-y-4 p-3 sm:p-4">
        {error && (
          <div className="rounded-lg border border-[var(--danger)]/40 bg-[var(--danger)]/5 px-3 py-2 text-cell text-[var(--danger)]">
            {error}
          </div>
        )}
        {notice && (
          <div className="rounded-lg border border-[var(--brand)]/40 bg-[var(--brand)]/5 px-3 py-2 text-cell text-[var(--brand)]">
            {notice}
          </div>
        )}

        <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h2 className="flex items-center gap-2 text-section font-semibold">
              <Gauge className="h-4 w-4 text-[var(--brand)]" />
              전역 기본 가중치
            </h2>
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() => void load()}
                disabled={loading}
                className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--border)] hover:bg-[var(--surface-elevated)]")}
              >
                <RefreshCw className={cn("h-3.5 w-3.5", loading && "animate-spin")} />
                새로고침
              </button>
              <button
                type="button"
                onClick={() => void handleSave()}
                disabled={saving || !dirty}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white disabled:opacity-50")}
              >
                {saving ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Save className="h-3.5 w-3.5" />}
                저장
              </button>
            </div>
          </div>

          {loading ? (
            <div className="py-6 text-center text-cell text-[var(--text-muted)]">불러오는 중…</div>
          ) : (
            <>
              <WeightEditor weights={weights} axes={axes} onChange={setWeights} disabled={saving} />

              <div className="mt-4 grid gap-3 sm:grid-cols-2">
                <label className="text-cell">
                  <span className="mb-1 block font-medium">커버리지 하한 ({coverageFloor.toFixed(2)})</span>
                  <input
                    type="range"
                    min={0}
                    max={0.9}
                    step={0.05}
                    value={coverageFloor}
                    onChange={(e) => setCoverageFloor(Number(e.target.value))}
                    className="w-full accent-[var(--brand)]"
                  />
                  <span className="text-micro text-[var(--text-muted)]">
                    데이터가 이 비율 미만이면 점수를 내지 않고 &apos;-&apos; 로 둡니다
                  </span>
                </label>
                <label className="text-cell">
                  <span className="mb-1 block font-medium">수축 해제 지점 ({shrinkFull.toFixed(2)})</span>
                  <input
                    type="range"
                    min={0.1}
                    max={1}
                    step={0.05}
                    value={shrinkFull}
                    onChange={(e) => setShrinkFull(Number(e.target.value))}
                    className="w-full accent-[var(--brand)]"
                  />
                  <span className="text-micro text-[var(--text-muted)]">
                    커버리지가 이 값 미만이면 점수를 50 쪽으로 수축시켜 극단값을 막습니다
                  </span>
                </label>
              </div>
            </>
          )}
        </section>

        <AdminWeightSuggestion
          onApply={(next) => {
            setWeights(next);
            setNotice("제안을 편집기에 채웠습니다 — 저장해야 반영됩니다.");
          }}
        />

        <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4">
          <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
            <h2 className="flex items-center gap-2 text-section font-semibold">
              <FlaskConical className="h-4 w-4 text-[var(--brand)]" />
              가중치 바꿔 보기
            </h2>
            <div className="flex items-center gap-2">
              <input
                value={previewSymbol}
                onChange={(e) => setPreviewSymbol(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") void handlePreview();
                }}
                placeholder="종목 코드"
                className="h-7 w-28 rounded-md border border-[var(--border)] bg-[var(--surface)] px-2 text-cell"
              />
              <button
                type="button"
                onClick={() => void handlePreview()}
                disabled={previewBusy}
                className={cn(
                  buttonVariants({ variant: "outline", size: "sm" }),
                  "border-[var(--border)] hover:bg-[var(--surface-elevated)] disabled:opacity-50",
                )}
              >
                {previewBusy ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <FlaskConical className="h-3.5 w-3.5" />
                )}
                미리보기
              </button>
            </div>
          </div>
          <p className="mb-2 text-cell text-[var(--text-muted)]">
            위에서 편집 중인 가중치를 저장하지 않은 채, 저장된 최신 점수 행에만 적용해
            결과를 봅니다. 배치를 돌리지 않으므로 다른 종목과 화면에는 영향이 없습니다.
          </p>

          {previewError && (
            <p className="text-cell text-[var(--text-muted)]">{previewError}</p>
          )}

          {preview && (
            <div className="rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)]/40 p-2.5">
              <div className="mb-2 flex flex-wrap items-baseline gap-x-2 gap-y-1 text-cell">
                <b>{preview.symbol}</b>
                <span className="tabular-nums text-[var(--text-muted)]">
                  {preview.tradeDate} 회차
                </span>
                <span className="ml-auto font-num text-base font-bold tabular-nums">
                  {preview.fromScore} → {preview.toScore}
                  <span
                    className={cn(
                      "ml-1.5",
                      preview.delta > 0
                        ? "text-[var(--profit)]"
                        : preview.delta < 0
                          ? "text-[var(--loss)]"
                          : "text-[var(--text-muted)]",
                    )}
                  >
                    ({preview.delta > 0 ? "+" : ""}
                    {preview.delta})
                  </span>
                </span>
              </div>

              {preview.drivers.length === 0 ? (
                <p className="text-cell text-[var(--text-muted)]">
                  이 종목의 점수를 바꾸는 가중치 변경이 없습니다. 데이터가 없는 축은
                  가중치를 올려도 점수에 들어가지 않습니다.
                </p>
              ) : (
                <ul className="space-y-0.5">
                  {preview.drivers.map((d) => (
                    <li key={d.axis ?? d.label} className="flex items-center gap-2 text-cell">
                      <span className="min-w-0 flex-1 truncate text-[var(--text-secondary)]">
                        {d.detail}
                      </span>
                      <span
                        className={cn(
                          "w-14 shrink-0 text-right font-num font-semibold tabular-nums",
                          d.effect > 0 ? "text-[var(--profit)]" : "text-[var(--loss)]",
                        )}
                      >
                        {d.effect > 0 ? "+" : ""}
                        {d.effect.toFixed(2)}
                      </span>
                    </li>
                  ))}
                </ul>
              )}

              {/*
                가중치는 분모에도 들어가 선형이 아니다 — 각 항은 「이 순서에서의 몫」이고
                합만 정확하다. 그 사실을 숨기면 항을 따로 떼어 인용하게 된다.
              */}
              <p className="mt-2 text-micro text-[var(--text-muted)]">
                합 {preview.exactDelta > 0 ? "+" : ""}
                {preview.exactDelta.toFixed(2)}점
                {Math.abs(preview.roundingResidual) >= 0.01 &&
                  ` · 표시 반올림 ${preview.roundingResidual > 0 ? "+" : ""}${preview.roundingResidual.toFixed(2)}`}
                {" · 축을 "}
                {preview.order.join(" → ")}
                {" 순으로 하나씩 바꾸며 잰 값입니다"}
              </p>
            </div>
          )}
        </section>

        <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4">
          <div className="mb-3 flex flex-wrap items-center justify-between gap-2">
            <h2 className="text-section font-semibold">배치 재계산</h2>
            <button
              type="button"
              onClick={() => void handleRecompute()}
              disabled={recomputing}
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "border-[var(--brand)] text-[var(--brand)] disabled:opacity-50")}
            >
              {recomputing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
              지금 재계산
            </button>
          </div>
          <p className="mb-2 text-cell text-[var(--text-muted)]">
            외부 API를 호출하지 않고 DB 데이터만 사용합니다. 전 종목 기준 수 초 소요.
            {status?.tradeDate && (
              <>
                {" "}최근 적재: <b>{status.tradeDate}</b>
                {typeof status.scored === "number" && typeof status.total === "number" && (
                  <> · {status.scored.toLocaleString()}/{status.total.toLocaleString()} 종목 점수 산출</>
                )}
              </>
            )}
          </p>
          {dist && <DistributionCard dist={dist} />}
        </section>

        <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3 sm:p-4">
          <h2 className="mb-1 text-section font-semibold">종목별 가중치</h2>
          <p className="mb-3 text-cell text-[var(--text-muted)]">
            등록된 종목은 전역 기본값 대신 아래 가중치로 계산됩니다. 삭제하면 다시 전역값을 따릅니다.
          </p>

          <div className="mb-4 rounded-lg border border-dashed border-[var(--border)] p-3">
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <input
                value={newSymbol}
                onChange={(e) => setNewSymbol(e.target.value)}
                placeholder="종목코드 (예: 005930, AAPL)"
                className="w-48 rounded border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 text-cell"
              />
              <input
                value={newNote}
                onChange={(e) => setNewNote(e.target.value)}
                placeholder="메모 (선택)"
                className="w-56 rounded border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 text-cell"
              />
              <button
                type="button"
                onClick={() => void handleAddSymbol()}
                disabled={addingBusy || !newSymbol.trim()}
                className={cn(buttonVariants({ variant: "default", size: "sm" }), "bg-[var(--brand)] text-white disabled:opacity-50")}
              >
                {addingBusy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Plus className="h-3.5 w-3.5" />}
                추가
              </button>
            </div>
            <WeightEditor weights={newWeights} axes={axes} onChange={setNewWeights} compact />
          </div>

          {symbols.length === 0 ? (
            <div className="py-4 text-center text-cell text-[var(--text-muted)]">
              등록된 종목별 가중치가 없습니다 — 전 종목이 전역 기본값을 사용 중입니다
            </div>
          ) : (
            <div className="space-y-3">
              {symbols.map((item) => (
                <div key={item.symbol} className="rounded-lg border border-[var(--border)] p-3">
                  <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
                    <div className="min-w-0">
                      <span className="text-sm font-medium">{item.name || item.symbol}</span>
                      {item.name && (
                        <span className="ml-1.5 text-cell text-[var(--text-muted)]">{item.symbol}</span>
                      )}
                      {item.note && (
                        <span className="ml-2 text-cell text-[var(--text-muted)]">· {item.note}</span>
                      )}
                    </div>
                    <div className="flex items-center gap-1.5">
                      <button
                        type="button"
                        onClick={() => void handleSaveSymbol(item)}
                        className="flex items-center gap-1 rounded border border-[var(--border)] px-2 py-1 text-cell hover:bg-[var(--surface-elevated)]"
                      >
                        <Save className="h-3 w-3" />
                        저장
                      </button>
                      <button
                        type="button"
                        onClick={() => void handleDeleteSymbol(item.symbol)}
                        className="flex items-center gap-1 rounded border border-[var(--border)] px-2 py-1 text-cell text-[var(--danger)] hover:bg-[var(--danger)]/10"
                      >
                        <Trash2 className="h-3 w-3" />
                        삭제
                      </button>
                    </div>
                  </div>
                  <WeightEditor
                    weights={item.weights}
                    axes={axes}
                    compact
                    onChange={(next) => void handleUpdateSymbol(item, next)}
                  />
                </div>
              ))}
            </div>
          )}
        </section>

        <p className="pb-4 text-cell leading-relaxed text-[var(--text-muted)]">
          점수는 서버가 계산합니다(LLM이 만든 숫자가 아님). 각 지표는 피어 그룹 내 순위 백분위로
          정규화되며, 데이터가 없는 축은 0점이 아니라 가중치에서 제외됩니다. 설계 상세는
          <code className="mx-1">docs/STOCK_SCORE_DESIGN.md</code> 참조.
        </p>
      </div>
      </PageScrollShell>
    </>
  );
}
