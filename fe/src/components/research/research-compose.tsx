"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { Loader2, Sparkles } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { Card } from "@/components/ui/card";
import { MarkdownEditor } from "@/components/ui/markdown-editor";
import { StockCatalogSearchField } from "@/components/market/stock-catalog-search-field";
import { StockAvatar } from "@/components/stocks/stock-avatar";
import {
  ContrarianBadge,
  EvidenceCard,
  TargetOddsNote,
  toneClass,
} from "@/components/research/research-shared";
import { ResearchThumbnailPicker } from "@/components/research/research-thumbnail-picker";
import { previewResearchClient, publishResearchClient } from "@/lib/research-api";
import {
  DIRECTION_LABELS,
  RESEARCH_HORIZON_OPTIONS,
  VISIBILITY_LABELS,
  type ResearchDirection,
  type ResearchPreview,
  type ResearchVisibility,
} from "@/lib/research-types";
import { formatPercent, formatPrice } from "@/lib/format";
import { cn } from "@/lib/utils";

const PREVIEW_DEBOUNCE_MS = 350;

/**
 * 리포트 작성 화면 (docs/RESEARCH_CALL_DESIGN.md §7).
 *
 * 한 컬럼 중앙 정렬 4단계: 종목 → 투자 의견 → 본문 → 발행. 이전에는 "콜 값 패널 +
 * 에디터" 2단이었는데, 그 배치는 넓은 화면에서만 성립했고 모바일에서는 어차피
 * 세로로 쌓이면서 순서가 뒤엉켰다. 글쓰기 화면의 주인공은 본문이므로 에디터가
 * 중앙 컬럼 전체 폭을 쓰고, 서버가 잰 값들은 그 위 단계에 접혀 들어간다.
 *
 * 마법사(wizard)는 아니다 — 모든 단계가 한 화면에 있고 위아래로만 움직인다.
 * 마찰이 이 기능의 사용률을 좌우한다.
 */
export function ResearchCompose() {
  const router = useRouter();
  const params = useSearchParams();

  const [symbol, setSymbol] = useState(() => (params.get("symbol") ?? "").toUpperCase());
  const [symbolName, setSymbolName] = useState("");
  const [direction, setDirection] = useState<ResearchDirection>("buy");
  const [targetInput, setTargetInput] = useState("");
  const [stopInput, setStopInput] = useState("");
  const [horizonDays, setHorizonDays] = useState<number>(90);
  const [conviction, setConviction] = useState(60);
  const [title, setTitle] = useState("");
  const [body, setBody] = useState("");
  const [thumbnailUrl, setThumbnailUrl] = useState("");
  const [visibility, setVisibility] = useState<ResearchVisibility>("public");

  const [preview, setPreview] = useState<ResearchPreview | null>(null);
  const [previewing, setPreviewing] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The draft is injected once per symbol. Re-injecting on every preview would
  // wipe whatever the author has already written.
  const draftedFor = useRef<string | null>(null);

  const targetPrice = parsePrice(targetInput);
  const stopPrice = parsePrice(stopInput);

  const runPreview = useCallback(async () => {
    if (!symbol) {
      setPreview(null);
      return;
    }
    setPreviewing(true);
    setError(null);
    try {
      const next = await previewResearchClient({
        symbol,
        direction,
        targetPrice,
        stopPrice,
        horizonDays,
        conviction,
      });
      setPreview(next);
      if (draftedFor.current !== next.symbol) {
        draftedFor.current = next.symbol;
        // The draft is a two-line prompt, not content: it goes in only while
        // the editor is untouched, and the editor drops it on the first click.
        setBody((current) => (current.trim() === "" ? next.draftMd : current));
        setSymbolName(next.symbolName ?? "");
      }
    } catch {
      setPreview(null);
      setError("이 종목의 발행 시점 데이터를 불러오지 못했습니다.");
    } finally {
      setPreviewing(false);
    }
  }, [symbol, direction, targetPrice, stopPrice, horizonDays, conviction]);

  useEffect(() => {
    const timer = window.setTimeout(runPreview, PREVIEW_DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [runPreview]);

  /**
   * Appends the server-measured data section. It is a button rather than a
   * pre-fill because the author should decide whether their post carries the
   * numbers — the same figures are already frozen on the published page.
   */
  function insertDataBlock(dataMd: string) {
    setBody((current) => {
      const kept = current === preview?.draftMd ? "" : current;
      if (kept.includes(dataMd.trim())) return kept;
      return kept.trimEnd() === "" ? dataMd : `${kept.trimEnd()}\n\n${dataMd}`;
    });
  }

  const basePrice = preview?.basePrice ?? 0;
  const currency = preview?.currency ?? "KRW";
  const boundsError = validateBounds(direction, basePrice, targetPrice, stopPrice);
  const canPublish = Boolean(symbol) && Boolean(preview) && !boundsError && !publishing;

  async function handlePublish() {
    if (!canPublish) return;
    setPublishing(true);
    setError(null);
    try {
      const created = await publishResearchClient({
        symbol,
        title,
        direction,
        targetPrice,
        stopPrice,
        horizonDays,
        conviction,
        // An untouched prompt is guidance, not a body — publishing it would put
        // our instructions on someone's public page.
        bodyMd: body === preview?.draftMd ? "" : body,
        thumbnailUrl,
        visibility,
      });
      router.push(`/reports/${created.id}`);
    } catch (err) {
      setError(publishErrorMessage(err));
      setPublishing(false);
    }
  }

  return (
    <>
      <PageHeader
        width="reading"
        title="리포트 쓰기"
        subtitle="근거는 서버가 박제합니다. 작성자는 왜 그렇게 보는지만 쓰면 됩니다"
      />
      <PageScrollShell width="reading" contentClassName="space-y-4">
        {/* ---- 1. 종목 ---- */}
        <Card className="space-y-3 p-4">
          <SectionLabel step={1} title="종목" />
          {symbol ? (
            <div className="flex flex-wrap items-center gap-2">
              <StockAvatar symbol={symbol} name={symbolName} size="xs" />
              <span className="text-sm font-medium text-[var(--text-primary)]">
                {symbolName || symbol}
              </span>
              <span className="text-xs text-[var(--text-muted)]">{symbol}</span>
              {basePrice > 0 && (
                <span className="text-xs text-[var(--text-secondary)]">
                  현재가 {formatPrice(basePrice, currency)}
                </span>
              )}
              {preview?.contrarian && <ContrarianBadge />}
              <button
                type="button"
                onClick={() => {
                  setSymbol("");
                  setSymbolName("");
                  setPreview(null);
                  draftedFor.current = null;
                }}
                className="ml-auto text-xs text-[var(--text-muted)] hover:text-[var(--text-secondary)]"
              >
                변경
              </button>
            </div>
          ) : (
            <StockCatalogSearchField
              onSelect={(stock) => {
                setSymbol(stock.symbol.toUpperCase());
                setSymbolName(stock.name);
              }}
              suggestionsId="research-symbol-suggestions"
            />
          )}
          {preview && !preview.eligible && (
            <p className="rounded-lg bg-amber-500/10 px-2.5 py-1.5 text-[11px] text-amber-400">
              {preview.ineligibleReason}
            </p>
          )}
        </Card>

        {symbol && (
          <>
            {/* ---- 2. 투자 의견 ---- */}
            <Card className="space-y-3.5 p-4">
              <SectionLabel step={2} title="투자 의견" />

              <div className="flex gap-1 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                {(["buy", "sell", "hold"] as ResearchDirection[]).map((d) => (
                  <button
                    key={d}
                    type="button"
                    onClick={() => setDirection(d)}
                    className={cn(
                      "flex-1 rounded-md px-2 py-2 text-xs font-medium transition-colors",
                      direction === d
                        ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                        : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                    )}
                  >
                    {DIRECTION_LABELS[d]}
                  </button>
                ))}
              </div>

              {direction === "hold" ? (
                <p className="text-[11px] text-[var(--text-muted)]">
                  관망 의견은 목표가·손절가를 두지 않습니다. 기간이 끝났을 때 주가가 ±3% 밴드
                  안에 있으면 적중으로 판정합니다.
                </p>
              ) : (
                <>
                  <div className="grid gap-3 sm:grid-cols-2">
                    <PriceField
                      label="목표가"
                      value={targetInput}
                      onChange={setTargetInput}
                      basePrice={basePrice}
                      currency={currency}
                    />
                    <PriceField
                      label="손절가"
                      value={stopInput}
                      onChange={setStopInput}
                      basePrice={basePrice}
                      currency={currency}
                    />
                  </div>
                  {preview && preview.levels.length > 0 && (
                    <div className="space-y-1">
                      <p className="text-[10px] text-[var(--text-muted)]">
                        실제 지표에서 유래한 가격대 — 눌러서 채우기
                      </p>
                      <div className="flex flex-wrap gap-1">
                        {preview.levels.map((level) => (
                          <button
                            key={`${level.label}-${level.price}`}
                            type="button"
                            onClick={() =>
                              level.distancePct >= 0
                                ? setTargetInput(String(level.price))
                                : setStopInput(String(level.price))
                            }
                            className="rounded-full bg-[var(--surface-elevated)] px-2.5 py-1 text-[10px] text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                            title={`${level.label} ${formatPrice(level.price, currency)}`}
                          >
                            {level.label} {formatPercent(level.distancePct)}
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                  {boundsError && <p className="text-[11px] text-[var(--loss)]">{boundsError}</p>}
                </>
              )}

              <div className="grid gap-3.5 sm:grid-cols-2">
                <div className="space-y-1">
                  <p className="text-[11px] text-[var(--text-muted)]">투자 기간</p>
                  <div className="flex gap-1 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                    {RESEARCH_HORIZON_OPTIONS.map((opt) => (
                      <button
                        key={opt.days}
                        type="button"
                        onClick={() => setHorizonDays(opt.days)}
                        className={cn(
                          "flex-1 rounded-md px-2 py-1.5 text-xs font-medium transition-colors",
                          horizonDays === opt.days
                            ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                            : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                        )}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-1">
                  <div className="flex items-baseline justify-between">
                    <p className="text-[11px] text-[var(--text-muted)]">확신도</p>
                    <p className="text-xs font-medium tabular-nums text-[var(--text-primary)]">
                      {conviction}%
                    </p>
                  </div>
                  <input
                    type="range"
                    min={10}
                    max={100}
                    step={5}
                    value={conviction}
                    onChange={(e) => setConviction(Number(e.target.value))}
                    className="w-full accent-[var(--brand)]"
                    aria-label="확신도"
                  />
                </div>
              </div>
              <p className="text-[10px] text-[var(--text-muted)]">
                확신도는 장식이 아닙니다 — 실제 적중률과의 차이가 캘리브레이션 점수로 남습니다.
              </p>
            </Card>

            {/* ---- 서버가 재는 값들 ---- */}
            {preview && (
              <div className="grid gap-3 sm:grid-cols-2">
                <EvidenceCard
                  evidence={preview.evidence}
                  score={preview.score}
                  grade={preview.grade}
                  currency={currency}
                />
                <div className="space-y-3">
                  {preview.odds && (
                    <Card className="space-y-1.5 p-3">
                      <p className="text-[11px] font-medium text-[var(--text-muted)]">
                        목표 도달 난이도 (실측)
                      </p>
                      <TargetOddsNote odds={preview.odds} />
                    </Card>
                  )}
                  {preview.expectedValue && (
                    <Card className="space-y-1.5 p-3">
                      <p className="text-[11px] font-medium text-[var(--text-muted)]">기대값</p>
                      <div className="grid grid-cols-2 gap-2">
                        <EVTile
                          label="내 확신도 기준"
                          value={preview.expectedValue.statedEV}
                          hint={`${conviction}% 가정`}
                        />
                        <EVTile
                          label="내 실적 기준"
                          value={preview.expectedValue.historicalEV}
                          hint={
                            preview.expectedValue.hitRateUsed === undefined
                              ? "판정된 리포트가 쌓이면 표시됩니다"
                              : `실제 적중률 ${Math.round(preview.expectedValue.hitRateUsed)}%`
                          }
                        />
                      </div>
                      {preview.expectedValue.riskReward !== undefined && (
                        <p className="text-[10px] text-[var(--text-muted)]">
                          손익비 {preview.expectedValue.riskReward.toFixed(2)}:1
                        </p>
                      )}
                    </Card>
                  )}
                </div>
              </div>
            )}

            {/* ---- 3. 본문 ---- */}
            <Card className="space-y-3 p-4 sm:p-5">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <SectionLabel step={3} title="리포트 본문" />
                {preview?.dataMd && (
                  <button
                    type="button"
                    onClick={() => insertDataBlock(preview.dataMd)}
                    className="inline-flex items-center gap-1 rounded-lg border border-[var(--border)] px-2.5 py-1 text-[10px] text-[var(--text-secondary)] transition-colors hover:border-[var(--brand)] hover:text-[var(--text-primary)]"
                    title="종합 점수·투자 포인트·주요 가격대·뉴스를 마크다운으로 넣습니다 (LLM 미사용)"
                  >
                    <Sparkles className="h-3 w-3" />
                    발행 시점 데이터 넣기
                  </button>
                )}
              </div>

              <input
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                maxLength={200}
                placeholder={`제목 (비워두면 "${symbolName || symbol} ${DIRECTION_LABELS[direction]} 리포트")`}
                className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3.5 py-2.5 text-base font-semibold text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
              />

              <MarkdownEditor
                value={body}
                onChange={setBody}
                clearOnFirstEdit={preview?.draftMd}
                rows={18}
                placeholder="왜 그렇게 보는지, 무엇이 확인되면 틀린 것인지를 적어주세요."
              />

              {/* 표지는 본문을 다 쓴 다음에 고르는 게 자연스럽다 — 본문 첫 이미지를
                  그대로 가져오는 선택지가 여기 있어야 의미가 있다. */}
              <div className="space-y-1.5 border-t border-[var(--border)] pt-3">
                <p className="text-[11px] font-medium text-[var(--text-muted)]">표지 이미지 (선택)</p>
                <ResearchThumbnailPicker
                  value={thumbnailUrl}
                  onChange={setThumbnailUrl}
                  bodyMd={body}
                  symbol={symbol}
                  symbolName={symbolName}
                  currency={currency}
                />
              </div>
            </Card>

            {/* ---- 4. 발행 ---- */}
            <Card className="space-y-3 p-4">
              <SectionLabel step={4} title="발행" />

              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex gap-0.5 rounded-lg bg-[var(--surface-elevated)] p-0.5">
                  {(["public", "unlisted", "private"] as ResearchVisibility[]).map((v) => (
                    <button
                      key={v}
                      type="button"
                      onClick={() => setVisibility(v)}
                      className={cn(
                        "rounded-md px-2.5 py-1 text-xs font-medium transition-colors",
                        visibility === v
                          ? "bg-[var(--surface)] text-[var(--text-primary)] shadow-sm"
                          : "text-[var(--text-muted)] hover:text-[var(--text-secondary)]",
                      )}
                    >
                      {VISIBILITY_LABELS[v]}
                    </button>
                  ))}
                </div>
                <button
                  type="button"
                  onClick={handlePublish}
                  disabled={!canPublish}
                  className="inline-flex w-full items-center justify-center gap-1.5 rounded-lg bg-[var(--brand)] px-4 py-2.5 text-sm font-medium text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40 sm:w-auto"
                >
                  {publishing && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
                  발행하기
                </button>
              </div>

              {previewing && (
                <p className="text-[10px] text-[var(--text-muted)]">발행 시점 데이터 확인 중…</p>
              )}
              {error && <p className="text-xs text-[var(--loss)]">{error}</p>}

              <p className="rounded-lg bg-[var(--surface-elevated)] px-3 py-2.5 text-[10px] leading-relaxed text-[var(--text-muted)]">
                발행하면 <strong>방향·기준가·목표가·손절가·기간·확신도와 근거 스냅샷이
                잠깁니다.</strong> 이후 수정할 수 있는 것은 제목·본문·태그·공개 범위뿐이며, 본문
                수정은 수정 이력으로 공개됩니다. 철회는 가능하지만 통계에서 빠지지 않습니다.
              </p>
            </Card>
          </>
        )}
      </PageScrollShell>
    </>
  );
}

function SectionLabel({ step, title }: { step: number; title: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="flex h-5 w-5 items-center justify-center rounded-full bg-[var(--brand-muted)] text-[10px] font-semibold text-[var(--brand)]">
        {step}
      </span>
      <span className="text-sm font-medium text-[var(--text-primary)]">{title}</span>
    </div>
  );
}

function PriceField({
  label,
  value,
  onChange,
  basePrice,
  currency,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
  basePrice: number;
  currency: "KRW" | "USD";
}) {
  const parsed = parsePrice(value);
  const pct = parsed && basePrice > 0 ? ((parsed - basePrice) / basePrice) * 100 : undefined;
  return (
    <div className="space-y-1">
      <div className="flex items-baseline justify-between">
        <label className="text-[11px] text-[var(--text-muted)]">{label}</label>
        {pct !== undefined && (
          <span className={cn("text-[11px] tabular-nums", toneClass(pct))}>
            {formatPercent(pct)}
          </span>
        )}
      </div>
      <input
        inputMode="decimal"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={basePrice > 0 ? formatPrice(basePrice, currency) : "가격"}
        className="w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2 text-sm tabular-nums text-[var(--text-primary)] outline-none focus:border-[var(--brand)]"
      />
    </div>
  );
}

function EVTile({
  label,
  value,
  hint,
}: {
  label: string;
  value?: number;
  hint: string;
}) {
  return (
    <div className="rounded-lg bg-[var(--surface-elevated)] px-2.5 py-2">
      <p className="text-[10px] text-[var(--text-muted)]">{label}</p>
      <p className={cn("text-sm font-semibold tabular-nums", toneClass(value))}>
        {value === undefined ? "—" : formatPercent(value)}
      </p>
      <p className="text-[9px] text-[var(--text-muted)]">{hint}</p>
    </div>
  );
}

function parsePrice(input: string): number | null {
  const cleaned = input.replace(/[,\s]/g, "");
  if (cleaned === "") return null;
  const n = Number(cleaned);
  return Number.isFinite(n) && n > 0 ? n : null;
}

/**
 * Mirrors the server's normalizeBounds so the author sees the problem next to
 * the field instead of after pressing 발행. The server still decides.
 */
function validateBounds(
  direction: ResearchDirection,
  base: number,
  target: number | null,
  stop: number | null,
): string | null {
  if (direction === "hold" || base <= 0) return null;
  if (direction === "buy") {
    if (target !== null && target <= base) return "매수 의견의 목표가는 현재가보다 높아야 합니다";
    if (stop !== null && stop >= base) return "매수 의견의 손절가는 현재가보다 낮아야 합니다";
  } else {
    if (target !== null && target >= base) return "매도 의견의 목표가는 현재가보다 낮아야 합니다";
    if (stop !== null && stop <= base) return "매도 의견의 손절가는 현재가보다 높아야 합니다";
  }
  return null;
}

function publishErrorMessage(err: unknown): string {
  const message = err instanceof Error ? err.message : "";
  if (message.includes("daysRemaining") || message.includes("too recently")) {
    return "같은 종목으로 최근 30일 안에 이미 리포트를 발행했습니다.";
  }
  if (message.includes("한도")) return message;
  return message || "발행에 실패했습니다.";
}
