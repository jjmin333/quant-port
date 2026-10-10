"use client";

import { Fragment, type ReactNode } from "react";
import { AssetMentionCard, AssetMentionInline, assetCardsIn } from "@/components/mentions/asset-mention";
import { useSymbolQuickLook } from "@/components/stocks/symbol-quick-look-modal";
import type { LiveQuote } from "@/hooks/use-live-quotes";
import { formatPercent, formatPrice } from "@/lib/format";
import {
  formatMentionValue,
  mentionValueChanged,
  splitMentions,
  type ContentMention,
  type MentionSet,
} from "@/lib/mentions";
import { SCORE_TONE_BG, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 본문 속 태그 하나 (2026-09-28 사용자 요청 2).
 *
 *  - `@종목`       → 종목 칩(이름 + 지금 가격·등락). 누르면 종목 화면.
 *  - `@종목:지표`  → 「삼성전자 점수 70점 → 지금 72」. 값은 **쓴 순간** 것이고, 달라졌을 때만 지금 값을 붙인다.
 *  - `#종목`       → 칩(카드는 그 줄 아래 — `MentionCards`).
 *
 * 누르면 **글 위에 종목 퀵룩 모달**(2026-10-01 요청 4, 결정 가) — 글을 떠나지 않는다. 종목 화면은 모달의 「종목 분석 보기」.
 * 점수 태그(`:점수`·`:추세` …)는 값 옆에 0~100 게이지를 붙여 미리보기처럼 읽힌다.
 */
export function MentionInline({
  text,
  token,
  set,
  quotes,
}: {
  text: string;
  token: MentionSet["tokens"][number];
  set: MentionSet;
  quotes?: Map<string, LiveQuote>;
}) {
  if (token.kind) return <AssetMentionInline text={text} token={token} set={set} />;
  const m = set.symbols.find((s) => s.symbol === token.symbol);
  if (!m) return <>{text}</>;
  return <MentionChip token={token} set={set} m={m} quotes={quotes} />;
}

function MentionChip({
  token,
  set,
  m,
  quotes,
}: {
  token: MentionSet["tokens"][number];
  set: MentionSet;
  m: ContentMention;
  quotes?: Map<string, LiveQuote>;
}) {
  const quickLook = useSymbolQuickLook();
  const name = m.name || m.symbol;
  const open = () => quickLook.open(m.symbol);
  // 모달은 열렸을 때만 그린다 — 글에 태그가 스무 개여도 닫힌 모달 스무 벌을 두지 않는다.
  const modal = quickLook.symbol ? quickLook.modal : null;

  if (token.field) {
    const field = set.fields?.[token.field];
    const snap = m.snap?.[token.field];
    const now = m.now?.[token.field];
    const changed = mentionValueChanged(snap, now, field);
    const when = m.snapAt ? m.snapAt.slice(0, 10) : "";
    const isScore = isScoreField(token.field) && typeof snap === "number";
    return (
      <>
        <button
          type="button"
          onClick={open}
          title={`${name} ${field?.label ?? ""} — ${when ? `${when}에 쓴 값` : "쓴 순간 값"}${changed ? `, 지금 ${formatMentionValue(now, field)}` : ""} · 눌러서 종목 보기`}
          className={cn(
            "inline-flex items-baseline gap-1 whitespace-nowrap px-0.5 align-baseline hover:bg-[var(--brand)]/10",
            isScore
              ? "rounded-md border border-[var(--border)] bg-[var(--card)] px-1.5"
              : "rounded-md border-b-[1.5px] border-dotted border-[var(--brand)]",
          )}
        >
          <span className="text-[0.85em] text-[var(--text-muted)]">
            {name} {isScore ? `AI ${field?.label ?? "점수"}` : (field?.label ?? token.field)}
          </span>
          <span
            className={cn("font-bold tabular-nums", isScore ? SCORE_TONE_TEXT[scoreTone(snap)] : "text-[var(--text-primary)]")}
          >
            {formatMentionValue(snap, field)}
          </span>
          {isScore && <ScoreGauge value={snap} />}
          {changed && (
            <span className="text-[0.8em] text-[var(--text-muted)] tabular-nums">→ 지금 {formatMentionValue(now, field)}</span>
          )}
        </button>
        {modal}
      </>
    );
  }

  const q = quotes?.get(m.symbol.toUpperCase());
  return (
    <>
      <button
        type="button"
        onClick={open}
        title={`${name} (${m.symbol}) · 눌러서 종목 보기`}
        className="inline-flex items-baseline gap-1 whitespace-nowrap rounded-md bg-[var(--brand)]/10 px-1.5 align-baseline font-semibold text-[var(--brand)] hover:bg-[var(--brand)]/20"
      >
        {name}
        {q && q.livePrice > 0 && (
          <span className={cn("text-[0.8em] font-medium tabular-nums", trendTextClass(q.liveChangeRate))}>
            {formatPrice(q.livePrice, q.currency as "KRW" | "USD")} {formatPercent(q.liveChangeRate)}
          </span>
        )}
      </button>
      {modal}
    </>
  );
}

/** 점수 필드 — 종합 점수와 축 다섯(0~100). */
export function isScoreField(field: string): boolean {
  return field === "score" || field.startsWith("axis.");
}

function ScoreGauge({ value }: { value: number }) {
  const pct = Math.max(0, Math.min(100, value));
  return (
    <span aria-hidden className="relative inline-block h-1.5 w-8 self-center overflow-hidden rounded-full bg-[var(--surface-elevated)]">
      <span className={cn("absolute inset-y-0 left-0 rounded-full", SCORE_TONE_BG[scoreTone(pct)])} style={{ width: `${pct}%` }} />
    </span>
  );
}

/** 한 덩어리 글을 글자·태그로 그린다. */
export function MentionText({
  text,
  set,
  quotes,
}: {
  text: string;
  set?: MentionSet | null;
  quotes?: Map<string, LiveQuote>;
}): ReactNode {
  if (!set || set.tokens.length === 0) return text;
  return splitMentions(text, set.tokens).map((seg, i) =>
    seg.kind === "text" ? (
      <Fragment key={i}>{seg.text}</Fragment>
    ) : (
      <MentionInline key={i} text={seg.text} token={seg.token} set={set} quotes={quotes} />
    ),
  );
}

/** 이 글자 덩어리의 `#` 태그가 부른 종목들(순서 유지, 중복 제거). */
export function cardMentionsIn(text: string, set?: MentionSet | null): ContentMention[] {
  if (!set) return [];
  const seen = new Set<string>();
  const out: ContentMention[] = [];
  for (const seg of splitMentions(text, set.tokens)) {
    if (seg.kind !== "mention" || !seg.token.card || seen.has(seg.token.symbol)) continue;
    const m = set.symbols.find((s) => s.symbol === seg.token.symbol);
    if (m) {
      seen.add(m.symbol);
      out.push(m);
    }
  }
  return out;
}

/** 리포트 본문(Markdown)용 — `<Markdown renderer={...}>`에 넣는다. */
export function mentionMarkdownRenderer(
  set: MentionSet | null | undefined,
  quotes: Map<string, LiveQuote> | undefined,
  renderCard: (m: ContentMention) => ReactNode,
) {
  if (!set || set.tokens.length === 0) return undefined;
  return {
    text: (value: string) => <MentionText text={value} set={set} quotes={quotes} />,
    after: (paragraph: string) => {
      const cards = cardMentionsIn(paragraph, set);
      const assets = assetCardsIn(paragraph, set);
      if (cards.length === 0 && assets.length === 0) return null;
      return (
        <div className="space-y-2">
          {cards.map((m) => (
            <Fragment key={m.symbol}>{renderCard(m)}</Fragment>
          ))}
          {assets.map((a) => (
            <AssetMentionCard key={`${a.kind}|${a.key}`} a={a} set={set} />
          ))}
        </div>
      );
    },
  };
}
