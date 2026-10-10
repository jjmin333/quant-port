"use client";

import { useState } from "react";
import { Popover } from "radix-ui";
import { Sparkles } from "lucide-react";
import { Card } from "@/components/ui/card";
import { SCORE_TONE_BG, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { GRADE_CLASS, type AINote, type ScoreView } from "@/lib/wealth-api";
import { cn } from "@/lib/utils";

/*
 * 자산 점수·AI 해설 표시 조각 (2026-10-01). 점수는 서버 규칙(asset_scores), 해설은 자산 전용 LLM(asset_ai) 또는 규칙.
 * 점수는 추천이 아니라 시장 지표라는 말을 늘 곁에 둔다.
 *
 * 2026-10-02 사용자: 「점수도 종목 종합 점수 카드처럼 정확한 근거가 있어야 신뢰」 — 점수가 보이는 곳이면
 * 어디서든(목록 배지 포함) 축별 백분위 막대 + **그 백분위를 만든 원래 숫자**를 볼 수 있게 했다.
 */

/** 축 하나 — 막대(같은 묶음 안 백분위)와 그 백분위를 만든 원값(근거). */
export type ScoreAxis = { key: string; label: string; weight: number; raw?: React.ReactNode };

export type ScoreKind = "region" | "sido" | "complex" | "fin" | "onbid";

const fnum = (f: Record<string, unknown>, k: string) => (typeof f[k] === "number" ? (f[k] as number) : null);
const pp = (v: number | null, d = 2) => (v == null ? "-" : `${v.toFixed(d)}%`);

/** 시군구·시도 축 — 주간 지수 4주 40 · 12주 20 · 거래량 추세 20 · 전세지수 4주 20. */
function regionAxes(score: ScoreView): ScoreAxis[] {
  const f = score.facts;
  const recent = fnum(f, "recentVol");
  const prior = fnum(f, "priorVol");
  return [
    { key: "w4", label: "4주 지수", weight: 40, raw: <>매매지수 4주 <Pct v={fnum(f, "w4Pct")} digits={2} /> · 1주 <Pct v={fnum(f, "w1Pct")} digits={2} /></> },
    { key: "w12", label: "12주 지수", weight: 20, raw: <>매매지수 12주 <Pct v={fnum(f, "w12Pct")} digits={2} /></> },
    { key: "volume", label: "거래량 추세", weight: 20,
      raw: prior ? <>최근 3개월 {recent?.toLocaleString()}건 vs 그 전 {prior.toLocaleString()}건 (<Pct v={fnum(f, "volTrendPct")} digits={0} />)</> : "거래 집계 부족" },
    { key: "jeonse", label: "전세 흐름", weight: 20, raw: <>전세지수 4주 <Pct v={fnum(f, "jeonse4Pct")} digits={2} /></> },
  ];
}

/** 단지 축 — 모멘텀 35 · 거래 25 · 전세 받침 20 · 고점 위치 20. */
function complexAxes(score: ScoreView): ScoreAxis[] {
  const f = score.facts;
  const jr = fnum(f, "jeonseRatio");
  const pk = fnum(f, "peakPct");
  return [
    { key: "momentum", label: "가격 모멘텀", weight: 35, raw: <>최근 3개월 ㎡당 중앙가, 그 전 9개월 대비 <Pct v={fnum(f, "momentumPct")} /></> },
    { key: "liquidity", label: "거래 활발", weight: 25, raw: <>최근 6개월 매매 {fnum(f, "trades6m") ?? "-"}건</> },
    { key: "jeonse", label: "전세 받침", weight: 20, raw: jr ? <>전세가율 {jr.toFixed(0)}%{jr > 80 ? " — 80% 넘어 깎음(역전세 위험)" : ""}</> : "전세 거래 부족" },
    { key: "peak", label: "고점 위치", weight: 20, raw: pk ? <>최근 3개월이 12개월 월별 최고의 {pk.toFixed(0)}%</> : undefined },
  ];
}

const FIN_GROUP: Record<string, string> = { "020000": "은행 100", "030300": "저축은행 60" };

/** 예·적금(기본 50 · 최고 15 · 우대 의존 15 · 가입 제한 10 · 금융권 10) / 대출(금리 60 · 금리 폭 20 · 금융권 20). */
function finAxes(score: ScoreView): ScoreAxis[] {
  const f = score.facts;
  const base = fnum(f, "base");
  const max = fnum(f, "max");
  const min = fnum(f, "min");
  const group = <>{FIN_GROUP[String(f.finGroup)] ?? "그 밖 70"} (은행 100 · 기타 70 · 저축은행 60)</>;
  if ("rate" in score.parts) {
    return [
      { key: "rate", label: "평균금리", weight: 60, raw: <>평균 {pp(base)} — 낮을수록 높은 백분위</> },
      { key: "range", label: "금리 폭", weight: 20, raw: <>최저 {pp(min)} ~ 최고 {pp(max)} — 좁을수록 예측 가능</> },
      { key: "group", label: "금융권", weight: 20, raw: group },
    ];
  }
  const term = fnum(f, "term");
  return [
    { key: "base", label: "기본금리", weight: 50, raw: <>{term ? `${term}개월 ` : ""}기본 {pp(base)} (우대 없이 받는 금리)</> },
    { key: "max", label: "최고금리", weight: 15, raw: <>우대 모두 충족 시 {pp(max)}</> },
    { key: "conditions", label: "우대 의존", weight: 15, raw: <>최고 − 기본 = {base != null && max != null ? `${(max - base).toFixed(2)}%p` : "-"} — 작을수록 높은 백분위</> },
    { key: "open", label: "가입 제한", weight: 10, raw: fnum(f, "joinDeny") === 1 ? "제한 없음(100)" : "가입 대상 제한 있음(0)" },
    { key: "group", label: "금융권", weight: 10, raw: group },
  ];
}

/** 공매 — 할인 60 · 유찰 이력 20 · 주의 표지 20. */
function onbidAxes(score: ScoreView): ScoreAxis[] {
  const f = score.facts;
  const ratio = fnum(f, "ratio");
  const fails = fnum(f, "fails") ?? 0;
  const flags = Array.isArray(f.flags) ? (f.flags as string[]) : [];
  return [
    { key: "discount", label: "할인", weight: 60, raw: <>최저가 = 감정가의 {ratio == null ? "-" : `${ratio}%`} — 같은 시도·용도 물건끼리 백분위</> },
    { key: "fails", label: "유찰 이력", weight: 20, raw: <>유찰 {fails}회 (1~3회 100 · 0·4~6회 40 · 7회 이상 10)</> },
    { key: "clean", label: "주의 표지", weight: 20, raw: flags.length ? <>{flags.join(" · ")} (표지 하나에 −40)</> : "주의 표지 없음" },
  ];
}

const KINDS: Record<ScoreKind, { axes: (s: ScoreView) => ScoreAxis[]; group: string; peer?: string }> = {
  region: { axes: regionAxes, group: "전국 시군구", peer: "전국 시군구" },
  sido: { axes: regionAxes, group: "시도", peer: "시도" },
  complex: { axes: complexAxes, group: "같은 시도 단지" },
  fin: { axes: finAxes, group: "같은 종류 상품" },
  onbid: { axes: onbidAxes, group: "같은 시도·용도 물건" },
};

/** 「전국 시군구 254곳 중 12위 · 상위 5%」 — facts.rank/of가 있을 때만. */
function peerText(score: ScoreView, unit?: string) {
  const rank = fnum(score.facts, "rank");
  const of = fnum(score.facts, "of");
  if (!unit || !rank || !of) return undefined;
  return `${unit} ${of}곳 중 ${rank}위 · 상위 ${Math.max(1, Math.round((rank / of) * 100))}%`;
}

/**
 * 점수 배지 — 「B 68」. 누르면 근거 카드(축별 막대 + 원값)가 뜬다. 목록 행이 링크·클릭 행이어도 행으로 새지 않게
 * 기본 동작과 전파를 막고 열림 상태를 직접 쥔다(Radix 트리거는 defaultPrevented면 토글하지 않는다).
 */
export function ScoreBadge({ score, kind, className }: { score?: ScoreView | null; kind: ScoreKind; className?: string }) {
  const [open, setOpen] = useState(false);
  if (!score) return null;
  const stop = (e: React.SyntheticEvent) => e.stopPropagation();
  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={`점수 ${score.score}점 ${score.grade}등급 — 근거 보기`}
          onClick={(e) => { e.preventDefault(); e.stopPropagation(); setOpen((v) => !v); }}
          className={cn("inline-flex shrink-0 cursor-pointer items-baseline gap-1 rounded-md px-1.5 py-0.5 font-num text-micro font-semibold tabular-nums hover:ring-1 hover:ring-current/40", GRADE_CLASS[score.grade], className)}
        >
          {score.grade}
          <span className="font-normal opacity-80">{score.score}</span>
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content sideOffset={4} align="end" collisionPadding={12} onClick={stop} onPointerDown={stop}
          className="z-50 w-[min(24rem,calc(100vw-1.5rem))] rounded-lg shadow-lg outline-none">
          <ScoreCard score={score} kind={kind} />
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  );
}

/**
 * 점수 카드 — 종목 「종합 점수」 카드와 같은 모양. 축마다 백분위 막대 옆에 **그 백분위를 만든 원래 숫자**를 붙이고,
 * 아래에 가중치와 비교 묶음을 밝힌다.
 */
export function ScoreCard({ title = "점수", score, kind, className }: { title?: string; score: ScoreView; kind: ScoreKind; className?: string }) {
  const k = KINDS[kind];
  const axes = k.axes(score);
  const peer = peerText(score, k.peer);
  const tone = scoreTone(score.score);
  return (
    <Card className={cn("p-0", className)}>
      <div className="flex flex-wrap items-center gap-x-2 gap-y-1 border-b px-3 py-2">
        <h3 className="shrink-0 text-cell font-semibold">{title}</h3>
        <div className="flex shrink-0 items-baseline gap-1.5">
          <span className={cn("text-2xl font-bold leading-none tabular-nums", SCORE_TONE_TEXT[tone])}>{score.score}</span>
          <span className="text-micro text-[var(--text-muted)]">/ 100</span>
          <span className={cn("rounded-md border border-current px-1.5 py-0.5 text-micro font-semibold leading-none", SCORE_TONE_TEXT[tone])}>{score.grade}</span>
        </div>
        {peer && <p className="ml-auto min-w-0 shrink truncate text-right text-micro text-[var(--text-muted)]">{peer}</p>}
      </div>
      <ul className="space-y-1.5 px-3 py-2">
        {axes.map((a) => {
          const v = score.parts[a.key];
          const t = v == null ? "mid" : scoreTone(v);
          return (
            <li key={a.key} className="grid grid-cols-[5.5rem_minmax(0,1fr)_1.75rem] items-center gap-x-2 gap-y-0.5">
              <span className="truncate text-cell leading-none text-[var(--text-muted)]">
                {a.label} <span className="text-micro opacity-70">{a.weight}</span>
              </span>
              <div className="h-1.5 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
                {v != null && <div className={cn("h-full rounded-full", SCORE_TONE_BG[t])} style={{ width: `${Math.max(2, Math.min(100, v))}%` }} />}
              </div>
              <span className={cn("text-right text-cell leading-none tabular-nums", v == null ? "text-[var(--text-muted)]" : SCORE_TONE_TEXT[t])}>
                {v == null ? "-" : Math.round(v)}
              </span>
              {a.raw != null && <span className="col-start-2 col-end-4 text-micro leading-tight text-[var(--text-muted)]">{a.raw}</span>}
            </li>
          );
        })}
      </ul>
      {kind === "region" && typeof score.facts.news30 === "number" && score.facts.news30 > 0 && (
        // 2026-10-09 결정 A — 기사는 근거로만 보인다(점수식에는 넣지 않음: 지역 크기·언론 관심에 치우쳐 검증 전).
        <p className="border-t px-3 py-1.5 text-micro leading-relaxed text-[var(--text-muted)]">
          최근 30일 이 지역 기사 <b className="text-[var(--text-primary)]">{score.facts.news30}건</b>
          {typeof score.facts.newsTitle === "string" && score.facts.newsTitle ? <> · 「{score.facts.newsTitle}」</> : null}
          <span className="opacity-70"> · 점수에는 넣지 않음</span>
        </p>
      )}
      <p className="border-t px-3 py-1.5 text-micro leading-relaxed text-[var(--text-muted)]">
        막대 = {k.group} 중 백분위(0~100, 높을수록 좋음), 점수 = 이름 옆 작은 숫자를 비중으로 한 가중 평균. 추천이 아니라 시장 지표입니다 · {score.asOf}
      </p>
    </Card>
  );
}

/** AI 한두 줄 — 출처(AI/규칙)를 함께. */
export function AINoteLine({ note, className }: { note?: AINote | null; className?: string }) {
  const text = note?.body?.note;
  if (!text) return null;
  const ai = note?.source === "llm";
  return (
    <p className={cn("flex gap-1.5 rounded-md bg-[var(--surface-elevated)] px-2 py-1.5 text-cell leading-relaxed", className)}>
      <Sparkles className={cn("mt-0.5 size-3.5 shrink-0", ai ? "text-[var(--brand)]" : "text-[var(--text-muted)]")} />
      <span>
        {text}
        <span className="ml-1 text-micro text-[var(--text-muted)]">{ai ? "· AI 요약" : "· 지표 요약"}</span>
        {note?.body?.tags && note.body.tags.length > 0 && (
          <span className="ml-1 inline-flex flex-wrap gap-1 align-middle">
            {note.body.tags.map((t) => (
              <span key={t} className="rounded-full border px-1.5 text-micro text-[var(--text-secondary)]">{t}</span>
            ))}
          </span>
        )}
      </span>
    </p>
  );
}

/** 부호 있는 % — 상승 빨강/하락 파랑(앱 규칙). */
export function Pct({ v, digits = 1, className }: { v?: number | null; digits?: number; className?: string }) {
  if (v == null || !Number.isFinite(v)) return <span className={cn("text-[var(--text-muted)]", className)}>-</span>;
  const tone = v > 0 ? "text-[var(--profit)]" : v < 0 ? "text-[var(--loss)]" : "text-[var(--text-secondary)]";
  return <span className={cn("font-num tabular-nums", tone, className)}>{`${v > 0 ? "+" : ""}${v.toFixed(digits)}%`}</span>;
}
