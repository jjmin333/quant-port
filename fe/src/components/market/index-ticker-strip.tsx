"use client";

import { useEffect, useRef, useState } from "react";
import { IndexQuickLookModal } from "@/components/market/index-quick-look-modal";
import { useIndexScores, type IndexScoreSummary } from "@/hooks/use-index-scores";
import { useMarketIndices } from "@/hooks/use-market-indices";
import { formatPercent } from "@/lib/format";
import type { MarketIndex } from "@/lib/quotes-types";
import { SCORE_TONE_BORDER, SCORE_TONE_TEXT, scoreTone } from "@/lib/stock-score-api";
import { trendTextClass } from "@/lib/trend-colors";
import { cn } from "@/lib/utils";

/**
 * 홈 지수 스트립 — **한 줄로 흐르고, 손을 대면 멈추고, 멈춘 채로 밀 수 있다**.
 *
 * 예전엔 지수 다섯 개를 카드 다섯 장으로 깔고 나머지 일곱은 화면 맨 아래 「전체 지수」
 * 접힘 안에 넣어 뒀다. 접힌 것은 사실상 아무도 펼치지 않으므로 홈이 실제로 보여주는
 * 지수는 다섯이었다. 여기서는 **카탈로그의 전 종목**이 같은 줄에 있다.
 *
 * 흐름을 CSS `transform`이 아니라 **`scrollLeft`로 돌린다**. 그래야 멈춘 뒤 사용자가
 * 직접 미는 동작(휠·트랙패드·손가락)과 저절로 흐르는 동작이 **같은 속성**을 다투지
 * 않는다. transform으로 밀면 컨테이너를 `overflow:hidden`으로 잠가야 해서, 멈춰 놓고
 * 볼 수 있는 것이 화면에 걸린 일곱 칸뿐이 된다 — 멈춘 보람이 없다.
 *
 * 멈추는 조건은 셋이다: **커서가 위에 있을 때**(PC), **손가락이 닿았을 때**(모바일),
 * **키보드 포커스가 안에 있을 때**. 이건 장식이 아니라 기능이다 — 셀이 누르면 모달을
 * 여는 버튼이라, 움직이는 동안에는 조준할 수가 없다.
 *
 * 실측으로 걸린 두 가지를 그대로 반영한다:
 *  - **스파크라인이 없는 칸이 있다**(원/달러·나스닥100 선물·필라델피아 반도체 등).
 *    빈 배열). 없는 칸에 빈 상자를 두면 세 칸이 구멍처럼 보이므로, 그림 자리를 아예 빼고
 *    **값을 한 단계 크게** 적는다.
 *  - **점수도 아홉 종만 있다**(금·비트코인·원/달러 없음). 배지를 그리지 않을 뿐이다.
 */

/** 초당 이동 픽셀. 셀 폭 200px 기준으로 한 칸이 지나가는 데 ~7초. */
const SPEED_PX_PER_SEC = 28;

/**
 * 사용자가 직접 민 뒤 다시 흐르기까지의 유예.
 *
 * 커서 hover만으로 멈추게 두면 **모바일에서 손가락과 싸운다** — 터치로 밀기 시작하면
 * 브라우저가 `pointercancel`을 쏘고 곧이어 `pointerleave`가 따라와, 손가락이 아직 화면에
 * 있는데 멈춤이 풀린다. 그래서 포인터 이벤트가 아니라 **스크롤이 실제로 우리 것이
 * 아니었다는 사실**로 판정한다. 관성 스크롤이 잦아드는 시간까지 덮는다.
 */
const RESUME_AFTER_USER_MS = 1200;

export function IndexTickerStrip() {
  const { indices, updatedAt } = useMarketIndices();
  const scoresById = useIndexScores();
  const [picked, setPicked] = useState<MarketIndex | null>(null);

  const rows = indices.filter((i) => i.value > 0);
  const loading = rows.length === 0;

  const viewportRef = useRef<HTMLDivElement>(null);
  const copyRef = useRef<HTMLDivElement>(null);
  /*
   * 멈춤 사유를 **개수로** 센다. 커서가 올라간 채 포커스가 들어왔다가 포커스만 빠지면,
   * 불리언 하나로는 커서가 아직 위에 있는데도 다시 흐르기 시작한다.
   */
  const holdsRef = useRef(0);
  // 모달이 열려 있는 동안은 뒤에서 계속 흐를 이유가 없다.
  const modalOpen = picked != null;

  useEffect(() => {
    const viewport = viewportRef.current;
    const copy = copyRef.current;
    if (!viewport || !copy || loading) return;

    const reduceMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;

    /**
     * 한 벌 폭. 세 벌을 이어 붙였으므로 **가운데 벌에서 시작**해 좌우 어느 쪽으로 밀어도
     * 끝이 보이지 않는다. 두 벌이면 뒤로 밀 때 곧장 시작점에 부딪힌다.
     */
    const copyWidth = () => copy.getBoundingClientRect().width;

    let width = copyWidth();
    if (width <= 0) return;

    /*
     * 우리가 마지막으로 지정한 위치. 스크롤 이벤트가 왔을 때 이 값과 어긋나 있으면
     * **사용자가 민 것**이다 — 휠·트랙패드·손가락·관성이 전부 같은 방식으로 잡힌다.
     * 읽어서 되담는 이유는 브라우저가 정수로 반올림하고 양 끝에서 잘라 내기 때문이다.
     */
    let expected = 0;
    const setScroll = (x: number) => {
      viewport.scrollLeft = x;
      expected = viewport.scrollLeft;
    };

    setScroll(width);

    /** 가운데 벌 밖으로 나가면 한 벌만큼 되돌린다 — 내용이 같아 눈에는 이어져 보인다. */
    const normalize = () => {
      if (width <= 0) return;
      const x = viewport.scrollLeft;
      if (x >= width * 2) setScroll(x - width);
      else if (x <= 1) setScroll(x + width);
    };

    let idleUntil = 0;
    const onScroll = () => {
      if (Math.abs(viewport.scrollLeft - expected) > 2) {
        idleUntil = performance.now() + RESUME_AFTER_USER_MS;
      }
      normalize();
    };
    viewport.addEventListener("scroll", onScroll, { passive: true });

    const onResize = () => {
      width = copyWidth();
    };
    window.addEventListener("resize", onResize);

    let raf = 0;
    let last = 0;
    /* scrollLeft는 정수로 떨어지므로 1px이 안 되는 이동분은 따로 모았다가 넘긴다. */
    let carry = 0;

    const tick = (now: number) => {
      raf = window.requestAnimationFrame(tick);
      const dt = last ? Math.min(now - last, 100) : 0;
      last = now;
      if (dt === 0) return;
      if (holdsRef.current > 0 || now < idleUntil) {
        // 멈춰 있는 동안 모은 이동분을 버린다 — 안 그러면 놓는 순간 훌쩍 건너뛴다.
        carry = 0;
        return;
      }

      carry += (SPEED_PX_PER_SEC * dt) / 1000;
      const step = Math.floor(carry);
      if (step < 1) return;
      carry -= step;
      setScroll(viewport.scrollLeft + step);
      normalize();
    };

    if (!reduceMotion) raf = window.requestAnimationFrame(tick);

    return () => {
      if (raf) window.cancelAnimationFrame(raf);
      viewport.removeEventListener("scroll", onScroll);
      window.removeEventListener("resize", onResize);
    };
  }, [loading, rows.length]);

  useEffect(() => {
    if (!modalOpen) return;
    holdsRef.current += 1;
    return () => {
      holdsRef.current = Math.max(0, holdsRef.current - 1);
    };
  }, [modalOpen]);

  const hold = () => {
    holdsRef.current += 1;
  };
  const release = () => {
    holdsRef.current = Math.max(0, holdsRef.current - 1);
  };

  return (
    <section>
      <div className="mb-2 flex items-baseline justify-between gap-3">
        <h2 className="text-sm font-semibold">오늘의 시장</h2>
        <span className="text-[11px] text-muted-foreground">
          {updatedAt
            ? `${new Date(updatedAt).toLocaleTimeString("ko-KR", {
                timeZone: "Asia/Seoul",
                hour: "2-digit",
                minute: "2-digit",
              })} 기준 · 지수 ${rows.length}종`
            : "갱신 중…"}
        </span>
      </div>

      {loading ? (
        <div className="flex gap-2 overflow-hidden">
          {Array.from({ length: 7 }).map((_, i) => (
            <div
              key={i}
              className="h-[52px] w-[200px] shrink-0 animate-pulse rounded-xl border bg-card"
            />
          ))}
        </div>
      ) : (
        <div className="relative">
          <div
            ref={viewportRef}
            /*
             * 스크롤바는 감추되 스크롤은 살린다(`hide-scrollbar`). `touch-pan-x`는 세로
             * 페이지 스크롤을 이 띠가 삼키지 않게 한다 — 모바일에서 띠 위에 손가락을
             * 얹고 아래로 쓸면 페이지가 내려가야 한다.
             */
            className="hide-scrollbar overflow-x-auto overscroll-x-contain touch-pan-x"
            /*
             * 포인터로 잡는 것은 **PC hover와 키보드 포커스**뿐이다. 터치는 여기서
             * 다루지 않는다 — 팬이 시작되면 브라우저가 `pointercancel`·`pointerleave`를
             * 쏘아 손가락이 아직 있는데 멈춤이 풀린다. 손가락은 위 `onScroll`이 잡는다.
             */
            onPointerEnter={(e) => {
              if (e.pointerType === "mouse") hold();
            }}
            onPointerLeave={(e) => {
              if (e.pointerType === "mouse") release();
            }}
            onFocusCapture={hold}
            onBlurCapture={release}
          >
            {/*
              바깥에는 gap을 두지 않는다. 각 벌이 `pr-2`로 자기 오른쪽 여백을 들고 있어,
              여기에 `gap-2`를 더하면 **벌과 벌 사이만 16px**이 되어 이음매에서 간격이
              한 번 벌어진다 — 무한히 이어져 보여야 할 띠에서 그 자리가 눈에 띈다.
            */}
            <div className="flex w-max">
              {/*
                같은 목록 세 벌. 2·3번째 벌은 같은 내용의 사본이므로 스크린 리더와 탭
                순서에서 제외한다.
              */}
              {[0, 1, 2].map((copy) => (
                <div
                  key={copy}
                  ref={copy === 0 ? copyRef : undefined}
                  className="flex gap-2 pr-2"
                  aria-hidden={copy > 0}
                >
                  {rows.map((index) => (
                    <TickerCell
                      key={`${copy}-${index.id}`}
                      index={index}
                      score={scoresById[index.id]}
                      duplicate={copy > 0}
                      onOpen={() => setPicked(index)}
                    />
                  ))}
                </div>
              ))}
            </div>
          </div>

          {/* 양옆을 배경색으로 흐린다 — 잘린 셀이 경계에서 뚝 끊기면 거기서 끝난 것처럼 보인다. */}
          <span className="pointer-events-none absolute inset-y-0 left-0 w-8 bg-gradient-to-r from-[var(--background)] to-transparent" />
          <span className="pointer-events-none absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-[var(--background)] to-transparent" />
        </div>
      )}

      <IndexQuickLookModal
        index={picked}
        score={picked ? scoresById[picked.id] : undefined}
        allIndices={indices}
        open={modalOpen}
        onClose={() => setPicked(null)}
      />
    </section>
  );
}

/**
 * 점수 배지.
 *
 * 「AI」를 함께 적는 이유는 **숫자만으로는 이게 무엇인지 알 수 없기 때문**이다. 처음 온
 * 사람에게 코스피 옆의 `60`은 등락률도 아니고 순위도 아닌 정체불명의 수다.
 *
 * 다만 툴팁은 정확해야 한다 — 이 점수는 **LLM이 매긴 것이 아니라** 20일선·RSI·최근
 * 등락으로 계산한 규칙 기반 추세 점수다(`be/internal/service/index_score.go`).
 * 신호등 구간은 `lib/stock-score-api`의 단일 정의를 따른다.
 */
export function IndexScoreBadge({
  score,
  className,
}: {
  score?: IndexScoreSummary;
  className?: string;
}) {
  if (score?.score == null) return null;
  const tone = scoreTone(score.score);
  return (
    <span
      title={`AI 추세 점수 ${score.score}${score.grade ? ` (${score.grade})` : ""} · ${
        score.tradeDate
      } 기준 — 20일선·RSI 등 기술 지표로 산출합니다. 매매 신호가 아닙니다.`}
      className={cn(
        "inline-flex shrink-0 items-center gap-0.5 rounded-full border px-1 text-[9px] font-bold tabular-nums",
        SCORE_TONE_TEXT[tone],
        SCORE_TONE_BORDER[tone],
        className,
      )}
    >
      <span className="font-extrabold tracking-tight opacity-80">AI</span>
      {score.score}
    </span>
  );
}

function TickerCell({
  index,
  score,
  duplicate,
  onOpen,
}: {
  index: MarketIndex;
  score?: IndexScoreSummary;
  duplicate: boolean;
  onOpen: () => void;
}) {
  const spark = (index.sparkline ?? []).filter((v) => Number.isFinite(v) && v > 0);
  const hasSpark = spark.length >= 2;

  return (
    <button
      type="button"
      onClick={onOpen}
      tabIndex={duplicate ? -1 : undefined}
      className="flex w-[200px] shrink-0 items-center gap-2.5 rounded-xl border bg-card px-3 py-2 text-left transition-colors hover:border-[var(--brand)]"
    >
      {hasSpark && <MiniSpark points={spark} changeRate={index.changeRate} />}

      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1">
          <span className="min-w-0 flex-1 truncate text-[11px] font-medium">{index.name}</span>
          <IndexScoreBadge score={score} />
        </span>
        <span className="mt-0.5 flex items-baseline gap-1.5">
          <span
            className={cn(
              "font-mono font-bold tabular-nums leading-none",
              // 그림이 없는 칸은 남는 자리를 값이 가져간다.
              hasSpark ? "text-[13px]" : "text-[15px]",
            )}
          >
            {index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
          </span>
          <span
            className={cn(
              "font-mono text-[10.5px] tabular-nums",
              index.changeRate === 0 ? "text-muted-foreground" : trendTextClass(index.changeRate),
            )}
          >
            {formatPercent(index.changeRate)}
          </span>
        </span>
      </span>
    </button>
  );
}

/**
 * 셀 안의 작은 흐름선.
 *
 * `IndexSparkline`을 쓰지 않는 이유는 크기뿐이다 — 저쪽은 카드용이라 여기서는 셀 높이를
 * 넘긴다. 색 규칙(상승 빨강·하락 파랑)은 같은 토큰을 쓴다.
 */
function MiniSpark({ points, changeRate }: { points: number[]; changeRate: number }) {
  const min = Math.min(...points);
  const max = Math.max(...points);
  const span = max - min || Math.abs(max) || 1;
  const d = points
    .map((v, i) => {
      const x = (i / (points.length - 1)) * 44;
      const y = 26 - ((v - min) / span) * 22;
      return `${i === 0 ? "M" : "L"}${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg
      viewBox="0 0 44 30"
      preserveAspectRatio="none"
      className="h-[30px] w-[44px] shrink-0"
      aria-hidden
    >
      <path
        d={d}
        fill="none"
        stroke={changeRate >= 0 ? "var(--profit)" : "var(--loss)"}
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  );
}
