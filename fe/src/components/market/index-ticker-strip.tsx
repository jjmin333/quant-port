"use client";

import { useEffect, useRef, useState } from "react";
import { IndexOverviewModal } from "@/components/market/index-overview-modal";
import { IndexQuickLookModal } from "@/components/market/index-quick-look-modal";
import { IndexTagChips } from "@/components/market/index-tag-chips";
import { useIndexScores, type IndexScoreSummary } from "@/hooks/use-index-scores";
import {
  INDEX_SERVER_LOOP_MS,
  retryMarketIndices,
  seedMarketIndices,
  useMarketIndices,
} from "@/hooks/use-market-indices";
import { useFreshness } from "@/hooks/use-freshness";
import { LoadErrorState } from "@/components/common/load-error-state";
import { StaleBadge } from "@/components/common/stale-badge";
import { updatedAtLabel } from "@/lib/data-freshness";
import type { ServerIndicesSeed } from "@/lib/home-api.server";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { formatPercent } from "@/lib/format";
import { formatRateChange, RATE_INDEX_IDS } from "@/lib/market-indices-catalog";
import type { MarketIndex } from "@/lib/quotes-types";
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
 * **PC에서는 잡아서 끈다.** 모바일은 손가락으로 밀 수 있는데 PC에는 미는 방법이
 * 없었다 — 스크롤바를 감췄고(`hide-scrollbar`), 마우스 휠은 세로라 이 띠에 닿지
 * 않는다. 그래서 자동으로 흘러가기를 기다리는 것이 유일한 길이었다. 지금은 커서를
 * 올리면 손 모양이 되고, 누른 채 끌면 띠가 따라온다(`useStripDrag`).
 *
 * 멈추는 조건은 셋이다: **커서가 위에 있을 때**(PC), **손가락이 닿았을 때**(모바일),
 * **키보드 포커스가 안에 있을 때**. 이건 장식이 아니라 기능이다 — 셀이 누르면 모달을
 * 여는 버튼이라, 움직이는 동안에는 조준할 수가 없다.
 *
 * 실측으로 걸린 두 가지를 그대로 반영한다:
 *  - **스파크라인이 없을 수 있다.** 한때 원/달러·나스닥100 선물·필라델피아 반도체가
 *    그랬는데, 지금은 BE가 종가 표로 메워 열세 칸 모두 그림을 들고 온다
 *    (`be/internal/service/market_index_history.go`). 그래도 새 지수를 붙이면 이력이
 *    쌓이기 전까지 빈 칸이 생기므로, 없는 칸은 그림 자리를 아예 빼고 **값을 한 단계
 *    크게** 적는다 — 빈 상자를 두면 그 칸이 구멍처럼 보인다.
 *  - **키워드가 하나도 안 걸리는 칸이 있다**. 조용한 지수는 조용하다고 말할 것이
 *    없으므로 칩 자리를 비운다.
 *
 * 예전에는 이름 옆에 「AI 56」 점수 배지가 붙었다. 지수 점수는 횡단면 랭킹이 아니라
 * 절대 지표로 만든 값이라 코스피 56과 나스닥 62를 비교할 수 없는데, 나란히 놓이면
 * 반드시 비교하게 된다. 지금 그 자리에는 **셀 안의 스파크라인으로 그 자리에서 확인되는
 * 사실**이 온다(`lib/index-tags.ts`). 점수 자체는 지수 모달에 남아 있다 — 거기에는
 * 무엇으로 만든 값인지 적을 자리가 있다.
 */

/** 초당 이동 픽셀. 셀 폭 224px 기준으로 한 칸이 지나가는 데 ~8초. */
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

/**
 * 클릭과 드래그를 가르는 이동 거리(px).
 *
 * 셀은 누르면 모달을 여는 버튼이라, 끌고 놓는 동작이 그대로 클릭이 되면 띠를 민
 * 사람 앞에 지수 모달이 뜬다. 반대로 문턱이 너무 높으면 손이 떨린 클릭이 삼켜진다.
 * 4px은 「누르려다 살짝 밀린 것」과 「밀려던 것」이 갈리는 실측 경계다.
 */
const DRAG_THRESHOLD_PX = 4;

/**
 * 마우스로 띠를 잡아 끄는 동작.
 *
 * `setPointerCapture`를 쓰는 이유는 **커서가 띠 밖으로 나가도 끌던 것이 끊기지
 * 않게** 하기 위해서다. 224px 셀 몇 칸을 넘기려면 커서가 화면 위아래로 쉽게 벗어난다.
 *
 * # 잡는 시점은 **문턱을 넘은 뒤**다 (2026-09-05 사용자 지적 3)
 *
 * 「홈의 지수 카드를 눌러도 모달이 안 뜬다」의 범인이 이 한 줄이었다. 예전에는
 * `pointerdown`에서 곧바로 잡았는데, 포인터가 잡혀 있으면 브라우저는 뒤이어 오는
 * `click`을 **누른 요소가 아니라 잡은 요소**(이 뷰포트 div)에 보낸다. 셀 버튼의
 * `onClick`은 그래서 한 번도 불리지 않았다.
 *
 * 원인이 오래 안 잡힌 이유가 여기 있다 — 스크립트로 `cell.click()`을 부르면 모달이
 * 정상적으로 열린다(캡처는 실제 포인터 입력에만 걸린다). 컴포넌트도 모달도 멀쩡한데
 * 손으로만 안 되는 모양이라, 눈으로는 「가끔 안 눌린다」로 보였다.
 *
 * 지금은 **4px을 넘긴 순간에** 잡는다. 그 전까지 잡는 것이 없으므로 평범한 클릭은
 * 평범하게 셀에 도착하고, 끌기 시작한 뒤에야 캡처가 붙어 커서가 띠 밖으로 나가도
 * 안 끊긴다. 얻으려던 것은 그대로 두고 클릭만 되살린다.
 *
 * 터치는 여기서 다루지 않는다(`pointerType === "mouse"`만 받는다). 브라우저의 기본
 * 팬이 이미 훨씬 매끄럽고, 둘을 겹치면 관성이 두 번 걸린다.
 *
 * 끌고 난 직후의 클릭은 캡처 단계에서 삼킨다 — `onClick`이 아니라 `onClickCapture`인
 * 것이 중요하다. 셀 버튼의 `onClick`이 먼저 돌면 막을 기회가 없다.
 */
function useStripDrag(viewportRef: React.RefObject<HTMLDivElement | null>) {
  const stateRef = useRef({ active: false, startX: 0, startScroll: 0, moved: false });
  const [dragging, setDragging] = useState(false);

  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.pointerType !== "mouse" || e.button !== 0) return;
    const viewport = viewportRef.current;
    if (!viewport) return;
    stateRef.current = {
      active: true,
      startX: e.clientX,
      startScroll: viewport.scrollLeft,
      moved: false,
    };
    // 여기서 잡지 않는다 — 잡으면 click이 셀이 아니라 이 div로 간다(머리글 참조).
  };

  const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
    const st = stateRef.current;
    if (!st.active) return;
    const viewport = viewportRef.current;
    if (!viewport) return;
    const dx = e.clientX - st.startX;
    if (!st.moved && Math.abs(dx) < DRAG_THRESHOLD_PX) return;
    if (!st.moved) {
      st.moved = true;
      setDragging(true);
      /*
        **이제** 잡는다. 여기까지 왔으면 클릭이 아니라 끌기다.

        try로 감싸는 이유: 이미 끝난 포인터 id로 부르면 `NotFoundError`가 난다
        (합성 이벤트나, 브라우저가 우리보다 먼저 포인터를 회수한 경우). 캡처는
        「커서가 밖으로 나가도 안 끊긴다」는 **덤**이지 끌기의 조건이 아니므로,
        여기서 던지게 두면 덤 하나 때문에 끌기 자체가 죽는다.
      */
      try {
        viewport.setPointerCapture(e.pointerId);
      } catch {
        // 못 잡아도 아래 scrollLeft 이동은 그대로 돈다.
      }
    }
    /*
      스크롤을 직접 옮긴다. 자동 흐름도 `scrollLeft`를 쓰므로 둘이 같은 속성을 다투는
      일이 없고, 위 `onScroll`이 「우리가 지정한 값과 다르다」로 이 이동을 사용자 조작으로
      알아채 유예를 건다 — 손을 떼자마자 띠가 튀어나가지 않는다.
    */
    viewport.scrollLeft = st.startScroll - dx;
    e.preventDefault();
  };

  const endDrag = (e: React.PointerEvent<HTMLDivElement>) => {
    const st = stateRef.current;
    if (!st.active) return;
    st.active = false;
    setDragging(false);
    const viewport = viewportRef.current;
    if (viewport?.hasPointerCapture(e.pointerId)) {
      viewport.releasePointerCapture(e.pointerId);
    }
  };

  const onClickCapture = (e: React.MouseEvent<HTMLDivElement>) => {
    if (!stateRef.current.moved) return;
    // 끌어서 놓은 뒤 딸려 오는 클릭 하나만 삼킨다.
    stateRef.current.moved = false;
    e.preventDefault();
    e.stopPropagation();
  };

  return { dragging, onPointerDown, onPointerMove, endDrag, onClickCapture };
}

export function IndexTickerStrip({ seed }: { seed?: ServerIndicesSeed }) {
  /*
    **서버가 받아 온 지수를 렌더 전에 넣는다** (2026-09-20 점검 요청 5·9).

    `useSyncExternalStore`의 `getServerSnapshot`이 이 값을 읽으므로, 서버 렌더와
    hydration이 **같은 값을 본다** — 넣는 시점이 양쪽 다 이 줄이다. 이펙트로
    옮기면 서버 HTML은 다시 비고, 첫 그림이 스켈레톤으로 돌아간다.

    모듈 전역을 렌더 중에 건드리지만 `seedMarketIndices`는 알림을 보내지 않고
    이미 값이 있으면 물러난다 — 그 함수 주석에 왜 그래도 되는지 적어 두었다.
  */
  if (seed) seedMarketIndices(seed);

  const { indices, updatedAt, checkedAt, loaded, error } = useMarketIndices();
  /*
    값은 `useMarketIndices`가 30초마다 밀어 주지만 **점수 배지는 하루 한 번 도는
    배치**가 만든다 — 홈을 열어 둔 채 날짜가 바뀌면 어제 등급이 오늘 값 옆에 붙어
    있었다. 이 스트립은 홈에만 있다(2026-09-16 사용자 요청).
  */
  const scoresById = useIndexScores(SCREEN_SLOW_REFRESH_MS);
  const [picked, setPicked] = useState<MarketIndex | null>(null);
  const [overview, setOverview] = useState(false);

  const rows = indices.filter((i) => i.value > 0);
  /*
    **네 상태를 가른다** (2026-09-20 점검 요청 6).

    예전에는 `rows.length === 0` 하나로 「로딩」을 판정했고, 그래서 적재 실패도
    빈 응답도 전부 「갱신 중…」 + 스켈레톤이었다 — 실패가 로딩과 같은 모습이면
    사용자는 영원히 기다린다.

      loading  아직 안 받아 봤다            → 스켈레톤
      error    받으려다 실패했다            → LoadErrorState + 다시 시도
      empty    받았는데 값이 없다           → 한 줄 안내
      ready    값이 있다                    → 띠
  */
  const loading = !loaded && rows.length === 0;
  const showError = error != null && rows.length === 0;
  const empty = loaded && rows.length === 0 && error == null;

  /*
    「낡은 값」은 **서버가 마지막으로 확인한 시각**(`checkedAt`)으로 판정한다 (2026-09-26).

    예전에는 `updatedAt`을 30초 × 3으로 쟀는데, `updatedAt`은 「값이 바뀐 시각」이라
    휴장에는 **멈춘다** — 주말에는 비트코인·금만 5분마다 움직여, 5분 중 3분 반 동안
    멀쩡한 값에 「낡은 값」이 붙었다(09-26 토요일 실측). 서버의 갱신 회차는 1분이므로
    그 세 배(3분)를 넘기면 정말로 파이프라인이 멈춘 것이다. 옛 서버 응답에 `checkedAt`이
    없으면 예전처럼 `updatedAt`으로 잰다.
  */
  const freshness = useFreshness(checkedAt ?? updatedAt, INDEX_SERVER_LOOP_MS);

  const viewportRef = useRef<HTMLDivElement>(null);
  const copyRef = useRef<HTMLDivElement>(null);
  /*
   * 멈춤 사유를 **개수로** 센다. 커서가 올라간 채 포커스가 들어왔다가 포커스만 빠지면,
   * 불리언 하나로는 커서가 아직 위에 있는데도 다시 흐르기 시작한다.
   */
  const holdsRef = useRef(0);
  const drag = useStripDrag(viewportRef);
  // 모달이 열려 있는 동안은 뒤에서 계속 흐를 이유가 없다 — 「한 눈에 보기」도 마찬가지다.
  const modalOpen = picked != null || overview;

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

  /*
    **손을 대고 있는 동안에는 멈춘다** (2026-09-05 사용자 지적 3).

    위의 멈춤 조건 셋은 전부 PC·키보드의 것이다(hover·포커스·모달). 터치에서는
    아무것도 멈추지 않아, 손가락을 얹은 채로도 셀이 초당 28px씩 흘러갔다.

    셀은 누르면 모달을 여는 버튼이다. 누른 순간의 셀과 뗀 순간의 셀이 다르면
    브라우저는 `click`을 두 요소의 **공통 조상** — 즉 이 컨테이너 — 에 보내고,
    그러면 아무 일도 일어나지 않는다. 셀 경계 근처를 누를 때만 생기므로 재현이
    들쭉날쭉해 「가끔 안 눌린다」로 보인다.

    포인터 id를 세는 이유는 멀티터치다. 손가락 둘이 얹히면 hold도 둘이어야
    하나를 떼도 안 흐른다. `delete`가 false를 주면 이미 푼 것이라 그냥 넘어간다 —
    up·cancel·leave 셋이 같은 포인터에 대해 겹쳐 올 수 있다.
  */
  const pressedRef = useRef<Set<number>>(new Set());
  const pressHold = (e: React.PointerEvent<HTMLDivElement>) => {
    if (pressedRef.current.has(e.pointerId)) return;
    pressedRef.current.add(e.pointerId);
    hold();
  };
  const pressRelease = (e: React.PointerEvent<HTMLDivElement>) => {
    if (!pressedRef.current.delete(e.pointerId)) return;
    release();
  };

  return (
    <section>
      <div className="mb-2 flex items-baseline justify-between gap-3">
        <h2 className="text-section font-semibold">오늘의 시장</h2>
        <span className="flex items-baseline gap-2">
          {/*
            **「갱신 중…」은 진짜로 받고 있을 때만 적는다** (2026-09-20 점검 요청 3·6).

            예전에는 `updatedAt ? 시각 : "갱신 중…"` 한 줄이었다. 그래서 적재가
            실패해 `updatedAt`이 영영 null로 남으면 화면은 **끝나지 않는 「갱신
            중…」**을 그렸다 — 실패했다는 말도, 다시 시도할 길도 없었다.
          */}
          <span className="text-cell text-muted-foreground">
            {freshness.at
              ? `${updatedAtLabel(freshness.at)} · 지수 ${rows.length}종`
              : showError
                ? "갱신 실패"
                : empty
                  ? "표시할 지수 없음"
                  : "갱신 중…"}
          </span>
          {/*
            값은 그대로 두고 늙었다는 것만 알린다. 마운트 전에는 뜨지 않는다 —
            `useFreshness`가 hydration 불일치를 피하는 방식이다.
          */}
          <StaleBadge ageMs={freshness.stale ? freshness.ageMs : null} at={freshness.at} />
          {/*
            흐르는 띠에서 나가는 문. 모바일에서는 한 번에 한두 칸밖에 안 보여, 두 지수를
            견주려면 띠가 한 바퀴 돌기를 기다려야 했다 — `IndexOverviewModal` 참고.
          */}
          {rows.length > 0 && (
            <button
              type="button"
              onClick={() => setOverview(true)}
              className="shrink-0 text-cell font-medium text-[var(--brand)] hover:underline"
            >
              한 눈에 보기
            </button>
          )}
        </span>
      </div>

      {showError ? (
        /*
          **실패에는 되돌아갈 길을 함께 준다** — 뉴스 화면이 이미 쓰는 규칙이고
          (§BUG-11), 홈에만 빠져 있었다. 값이 하나도 없을 때만 이 자리를 쓴다:
          보고 있던 지수가 있으면 위의 「갱신 실패」 한 줄로 충분하고, 띠를
          지워 버리면 갱신 한 번의 실패가 화면을 없애는 일이 된다.
        */
        <LoadErrorState
          compact
          message={error ?? "지수를 불러오지 못했습니다."}
          onRetry={retryMarketIndices}
        />
      ) : empty ? (
        /*
          받았는데 비어 있는 경우. 실패와 **다른 말**을 해야 한다 — 여기서
          「다시 시도」를 권하면 사용자는 고칠 수 없는 것을 고치려 든다.
        */
        <p className="px-3 py-6 text-center text-cell text-[var(--text-muted)]">
          표시할 지수가 없습니다. 잠시 후 다시 확인해 주세요.
        </p>
      ) : loading ? (
        <div className="flex gap-2 overflow-hidden">
          {Array.from({ length: 7 }).map((_, i) => (
            <div
              key={i}
              className="h-[52px] w-[224px] shrink-0 animate-pulse rounded-xl border bg-card"
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
            className={cn(
              "hide-scrollbar overflow-x-auto overscroll-x-contain touch-pan-x",
              // 커서가 「끌 수 있다」고 먼저 말한다 — 눌러 보기 전에 알 수 있어야 한다.
              drag.dragging ? "cursor-grabbing select-none" : "cursor-grab",
            )}
            /*
             * 포인터로 잡는 것은 **PC hover와 키보드 포커스**뿐이다. 터치는 여기서
             * 다루지 않는다 — 팬이 시작되면 브라우저가 `pointercancel`·`pointerleave`를
             * 쏘아 손가락이 아직 있는데 멈춤이 풀린다. 손가락은 위 `onScroll`이 잡는다.
             */
            onPointerEnter={(e) => {
              if (e.pointerType === "mouse") hold();
            }}
            onPointerLeave={(e) => {
              pressRelease(e);
              if (e.pointerType === "mouse") release();
            }}
            onFocusCapture={hold}
            onBlurCapture={release}
            /*
              멈춤은 **캡처 단계**에서 건다. 셀 버튼이 이 이벤트를 삼키지는 않지만,
              누르는 순간과 멈추는 순간 사이에 프레임 하나라도 끼면 그 프레임에서
              셀이 이미 움직인다.
            */
            onPointerDownCapture={pressHold}
            onPointerUpCapture={pressRelease}
            onPointerCancelCapture={pressRelease}
            onPointerDown={drag.onPointerDown}
            onPointerMove={drag.onPointerMove}
            onPointerUp={drag.endDrag}
            onPointerCancel={drag.endDrag}
            onClickCapture={drag.onClickCapture}
            /*
              끌 때 브라우저의 기본 이미지/텍스트 드래그가 끼어들면 셀이 통째로 유령처럼
              따라다닌다. 스파크라인 SVG와 지수 이름이 그 대상이다.
            */
            onDragStart={(e) => e.preventDefault()}
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

      {/* 출처·저작권은 지수 모달 안에 있다(`INDEX_ATTRIBUTION`, 2026-10-07 요청 3 — 홈 공간 절약). */}

      <IndexOverviewModal
        open={overview}
        indices={rows}
        scoresById={scoresById}
        updatedAt={updatedAt}
        onClose={() => setOverview(false)}
        onPick={setPicked}
      />

      {/*
        퀵룩은 「한 눈에 보기」 **위에** 열린다 — 닫으면 목록으로 돌아오므로, 지수를
        하나씩 짚어 보려고 목록을 매번 다시 열 필요가 없다. 뒤로가기도 맨 위부터
        닫는다(`ui/dialog.tsx`의 히스토리 처리).
      */}
      <IndexQuickLookModal
        index={picked}
        score={picked ? scoresById[picked.id] : undefined}
        allIndices={indices}
        open={picked != null}
        onClose={() => setPicked(null)}
      />
    </section>
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
      className="flex w-[224px] shrink-0 items-center gap-2.5 rounded-xl border bg-card px-3 py-2 text-left transition-colors hover:border-[var(--brand)]"
    >
      {hasSpark && <MiniSpark points={spark} changeRate={index.changeRate} />}

      <span className="min-w-0 flex-1">
        <span className="flex min-w-0 items-center gap-1">
          <span className="min-w-0 flex-1 truncate text-cell font-medium">{index.name}</span>
          {/*
            **화면 폭과 상관없이 하나**다 (2026-09-05 사용자 지적 3).

            한동안은 「데스크톱 둘, 모바일 하나」였다. 좁은 화면만 문제라고 봤기
            때문인데, 실제로는 **데스크톱에서도 둘째 칩이 지수 이름을 덮었다** —
            셀 폭이 224px 고정이고 그 안에서 이름만 `flex-1 truncate`라, 칩이 하나
            더 들어오면 줄어드는 것은 언제나 이름 쪽이다. 「필라델피아 반도체」가
            「필라델…」이 되면 그 칸은 어느 지수인지부터 못 말한다.

            칩 하나는 남긴다. BE가 드문 사건을 앞에 놓고 보내므로(`index_tags.go`)
            남는 하나가 그 지수에서 가장 할 말이 있는 것이다. **나머지는 눌러서
            여는 지수 모달이 전부 그대로 받는다** — 이 카드에서 덜어낸 것이지
            어디에서도 못 보게 된 것이 아니다.
          */}
          <IndexTagChips tags={score?.tags} changeRate={index.changeRate} max={1} />
        </span>
        <span className="mt-0.5 flex items-baseline gap-1.5">
          <span
            className={cn(
              "font-num font-bold tabular-nums leading-none",
              // 그림이 없는 칸은 남는 자리를 값이 가져간다.
              hasSpark ? "text-note" : "text-section",
            )}
          >
            {index.value.toLocaleString("ko-KR", { maximumFractionDigits: 2 })}
            {RATE_INDEX_IDS.has(index.id) ? "%" : ""}
          </span>
          <span
            className={cn(
              "font-num text-micro tabular-nums",
              index.changeRate === 0 ? "text-muted-foreground" : trendTextClass(index.changeRate),
            )}
          >
            {RATE_INDEX_IDS.has(index.id) ? formatRateChange(index.change) : formatPercent(index.changeRate)}
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
