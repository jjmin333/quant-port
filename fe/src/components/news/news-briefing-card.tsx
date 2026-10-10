"use client";

import { useState } from "react";
import { Sparkles, X } from "lucide-react";
import type { NewsMarketBriefing } from "@/lib/news-briefing-api";
import { formatBriefingTimestamp, formatCarryClock } from "@/lib/news-market-briefing";
import { formatPercent } from "@/lib/format";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * A3 — 「AI 시장 브리핑」.
 *
 * 카드에는 **숫자 4칸과 문단 앞부분**만 둔다. 문장은 서버에서 길게 받아 두고(6~10문장)
 * 여기서 잘라 보여준 뒤, 전체는 모달에서 읽는다 — 화면 위쪽을 문단 하나로 채우면
 * 정작 목록이 밀린다.
 *
 * 숫자 4칸은 LLM이 쓴 것이 아니라 서버가 **센** 값이다. 그래서 AI가 꺼져 있어도(그때는
 * `source === "rules"`) 그대로 맞는다. 시황 배지(강세/약세)는 모델의 판단이라 실제 지수
 * 등락률을 **항상 옆에 같이** 둔다 — 둘이 어긋나는 날 배지만 보고 오해하지 않도록.
 */
export function NewsBriefingCard({ briefing }: { briefing: NewsMarketBriefing | null }) {
  const [open, setOpen] = useState(false);
  if (!briefing) return null;

  const facts = briefing.facts;
  const headIndex = facts.indices?.[0];
  const fxIndex = facts.indices?.find((i) => i.id === "usd-krw");

  return (
    <>
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-3">
        <div className="grid gap-3 @3xl:grid-cols-[minmax(0,1fr)_320px] @3xl:items-center">
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-1.5">
              <span className="inline-flex items-center gap-1 text-cell font-bold text-[var(--text-primary)]">
                <Sparkles className="size-3.5 text-[var(--brand)]" />
                AI 시장 브리핑
              </span>
              {briefing.mood && <MoodBadge mood={briefing.mood} />}
              <span className="text-micro text-[var(--text-muted)]">
                {formatBriefingTimestamp(briefing.generatedAt)} · {facts.session}
              </span>
              <BriefingSourceBadge briefing={briefing} />
            </div>

            <p className="mt-1.5 line-clamp-3 text-read text-[var(--text-secondary)]">
              {briefing.summary}
            </p>

            <button
              type="button"
              onClick={() => setOpen(true)}
              className="mt-1 text-cell font-medium text-[var(--brand)] hover:underline"
            >
              전체 분석 읽기
            </button>
          </div>

          <div className="grid grid-cols-4 gap-2 text-center">
            <Tile label="기사" value={facts.articleCount.toLocaleString("ko-KR")} sub="최근 200건 기준" />
            <Tile
              label="속보"
              value={String(facts.breakingCount)}
              sub="24시간"
              valueClassName={facts.breakingCount > 0 ? "text-[var(--loss)]" : undefined}
            />
            {headIndex ? (
              <Tile
                label={headIndex.name}
                value={formatPercent(headIndex.changeRate)}
                sub={headIndex.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
                valueClassName={trendTextClass(headIndex.changeRate)}
              />
            ) : (
              <Tile label="지수" value="—" sub="집계 없음" />
            )}
            {fxIndex ? (
              <Tile
                label="환율"
                value={formatPercent(fxIndex.changeRate)}
                sub={fxIndex.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
                valueClassName={trendTextClass(fxIndex.changeRate)}
              />
            ) : (
              <Tile label="환율" value="—" sub="집계 없음" />
            )}
          </div>
        </div>
      </div>

      {open && <NewsBriefingModal briefing={briefing} onClose={() => setOpen(false)} />}
    </>
  );
}

function Tile({
  label,
  value,
  sub,
  valueClassName,
}: {
  label: string;
  value: string;
  sub: string;
  valueClassName?: string;
}) {
  return (
    <div className="min-w-0">
      <div className="truncate text-micro text-[var(--text-muted)]">{label}</div>
      <div className={cn("font-num text-section font-bold tabular-nums", valueClassName)}>
        {value}
      </div>
      <div className="truncate text-micro text-[var(--text-muted)]">{sub}</div>
    </div>
  );
}

/**
 * 문장의 출처 배지 — 홈 브리핑 카드도 같은 것을 쓴다.
 *
 * `llm`이면 아무것도 그리지 않는다. 정상이 기본값이고, 매번 「AI」라고 붙여 두면
 * 예외인 두 경우가 눈에 띄지 않는다.
 *
 * 승계(`llm_carry`)에 **시각을 박아 두는 이유**: 카드 위쪽 타임스탬프는 이 슬롯
 * 것으로 갱신된다(멈춘 화면처럼 보이지 않도록). 그러면 문장만 한 시간 전 것이라는
 * 사실을 말해 줄 자리가 여기밖에 없다.
 */
export function BriefingSourceBadge({ briefing }: { briefing: NewsMarketBriefing }) {
  if (briefing.source === "llm_carry") {
    // 자정을 넘긴 승계면 「어제 21:00」 — 시각만 적으면 아직 오지 않은 오늘 21시로 읽힌다.
    const clock = briefing.carriedFrom
      ? formatCarryClock(briefing.carriedFrom, briefing.generatedAt)
      : "";
    return (
      <span
        className="rounded-md bg-[var(--brand)]/12 px-1.5 py-px text-micro font-semibold text-[var(--brand)]"
        title={
          clock
            ? `이번 생성이 실패해 ${clock} 분석을 그대로 씁니다. 숫자와 시각은 지금 것입니다.`
            : "이번 생성이 실패해 직전 분석을 그대로 씁니다. 숫자와 시각은 지금 것입니다."
        }
      >
        {clock ? `AI ${clock}` : "직전 AI"}
      </span>
    );
  }
  if (briefing.source === "rules") {
    return (
      <span
        className="rounded-md bg-[var(--surface-elevated)] px-1.5 py-px text-micro font-semibold text-[var(--text-muted)]"
        title="AI 생성에 실패했거나 꺼져 있어, 집계만으로 만든 문장입니다"
      >
        규칙
      </span>
    );
  }
  return null;
}

/** 시황 배지 — 홈 브리핑 카드도 같은 것을 쓴다(강세=빨강은 이 앱의 상승색이다). */
export function MoodBadge({ mood }: { mood: string }) {
  // 상승·하락은 홈 「부동산·금리 브리핑」(2026-10-03)의 분위기 — 같은 배지를 쓴다.
  const up = mood === "강세" || mood === "상승";
  const down = mood === "약세" || mood === "하락";
  const tone = up
    ? "bg-[var(--profit)]/12 text-[var(--profit)]"
    : down
      ? "bg-[var(--loss)]/12 text-[var(--loss)]"
      : "bg-[var(--surface-elevated)] text-[var(--text-muted)]";
  const arrow = up ? "▲" : down ? "▼" : "―";
  return (
    <span className={cn("rounded-full px-2 py-px text-cell font-bold", tone)}>
      {arrow} {mood}
    </span>
  );
}

/** 전체 분석 — 카드에서 잘린 문단과 하이라이트·지켜볼 것을 모두 보여준다. */
function NewsBriefingModal({
  briefing,
  onClose,
}: {
  briefing: NewsMarketBriefing;
  onClose: () => void;
}) {
  const facts = briefing.facts;

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center p-0 sm:items-center sm:p-4">
      <button
        type="button"
        aria-label="닫기"
        className="absolute inset-0 bg-black/60 backdrop-blur-[1px]"
        onClick={onClose}
      />
      <div className="relative flex max-h-[88dvh] w-full flex-col overflow-hidden rounded-t-2xl border border-[var(--border)] bg-[var(--surface)] shadow-2xl sm:max-w-2xl sm:rounded-2xl">
        <div className="flex items-center gap-2 border-b border-[var(--border)] px-4 py-3">
          <Sparkles className="size-4 shrink-0 text-[var(--brand)]" />
          <div className="min-w-0 flex-1">
            <p className="text-sm font-bold text-[var(--text-primary)]">AI 시장 브리핑</p>
            <p className="text-cell text-[var(--text-muted)]">
              {formatBriefingTimestamp(briefing.generatedAt)} · {facts.session} · 기사{" "}
              {facts.articleCount.toLocaleString("ko-KR")}건 분석
            </p>
          </div>
          {briefing.mood && <MoodBadge mood={briefing.mood} />}
          <button
            type="button"
            aria-label="닫기"
            onClick={onClose}
            className="rounded-lg p-1 text-[var(--text-muted)] hover:bg-[var(--surface-elevated)]"
          >
            <X className="size-4" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto px-4 py-4">
          {briefing.moodReason && (
            <p className="mb-3 rounded-lg bg-[var(--surface-elevated)] px-3 py-2 text-cell text-[var(--text-secondary)]">
              {briefing.moodReason}
            </p>
          )}

          <p className="whitespace-pre-line text-read text-[var(--text-primary)]">
            {briefing.summary}
          </p>

          {(briefing.highlights?.length ?? 0) > 0 && (
            <Section title="핵심 사건">
              <ul className="list-disc space-y-1 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
                {briefing.highlights!.map((item, i) => (
                  <li key={i}>{item}</li>
                ))}
              </ul>
            </Section>
          )}

          {(briefing.watchPoints?.length ?? 0) > 0 && (
            <Section title="지켜볼 것">
              <ul className="list-disc space-y-1 pl-4 text-cell leading-relaxed text-[var(--text-secondary)]">
                {briefing.watchPoints!.map((item, i) => (
                  <li key={i}>{item}</li>
                ))}
              </ul>
            </Section>
          )}

          {(facts.indices?.length ?? 0) > 0 && (
            <Section title="분석에 쓰인 지수">
              <ul className="min-w-0">
                {facts.indices!.map((idx) => (
                  <li
                    key={idx.id}
                    className="flex items-center gap-2 border-b border-[var(--border)]/60 py-1 text-cell last:border-b-0"
                  >
                    <span className="min-w-0 flex-1 truncate">{idx.name}</span>
                    <span className="shrink-0 font-num tabular-nums">
                      {idx.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
                    </span>
                    <span
                      className={cn(
                        "w-16 shrink-0 text-right font-num tabular-nums",
                        trendTextClass(idx.changeRate),
                      )}
                    >
                      {formatPercent(idx.changeRate)}
                    </span>
                  </li>
                ))}
              </ul>
            </Section>
          )}

          {(facts.topMentioned?.length ?? 0) > 0 && (
            <Section title="가장 많이 언급된 종목">
              <ul className="min-w-0">
                {facts.topMentioned!.map((m) => (
                  <li
                    key={m.symbol}
                    className="flex items-center gap-2 border-b border-[var(--border)]/60 py-1 text-cell last:border-b-0"
                  >
                    <span className="min-w-0 flex-1 truncate">{m.name}</span>
                    <span className="shrink-0 text-[var(--text-muted)]">{m.mentions}건</span>
                    <span
                      className={cn(
                        "w-16 shrink-0 text-right font-num tabular-nums",
                        m.hasQuote ? trendTextClass(m.changeRate) : "text-[var(--text-muted)]",
                      )}
                    >
                      {m.hasQuote ? formatPercent(m.changeRate) : "—"}
                    </span>
                  </li>
                ))}
              </ul>
            </Section>
          )}
        </div>
      </div>
    </div>
  );
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="mt-4">
      <p className="mb-1.5 text-cell font-semibold text-[var(--text-muted)]">{title}</p>
      {children}
    </div>
  );
}
