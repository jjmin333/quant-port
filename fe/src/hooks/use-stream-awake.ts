"use client";

import { useEffect, useRef, useState } from "react";
import { STREAM_BACKGROUND_GRACE_MS, shouldKeepStream } from "@/lib/stream-awake";

/*
탭이 뒤로 가도 **SSE는 붙어 있는다** (2026-09-05 사용자 요청 5).

# 무엇이 잘못돼 있었나

가격 스트림 셋(목록 가격·상세 체결·봉)이 전부 `visibilitychange`에서 연결을 닫고
있었다. 탭을 잠깐 옮겼다 돌아오면 그때부터 다시 붙는데, 그 사이가 짧을수록 손해가
크다 — 붙는 데 드는 것(요청·구독 등록·스냅샷 한 벌)이 붙어 있는 데 드는 것보다
훨씬 무겁기 때문이다. 창을 둘 띄우고 오가며 보는 사용자는 그 왕복을 **몇 초마다**
치르고 있었고, 서버 쪽에서는 그것이 goroutine과 nginx upstream이 계속 갈리는
모양으로 보인다(사용자 지적 6의 「SSE 연결이 자주 끊겼다가 다시 붙는다」).

그래서 숨는 것만으로는 끊지 않는다. 돌아왔을 때 값이 이미 최신이다.

# 그래도 영원히는 아니다 — 3시간

끊는 조건을 아예 없애면 열어 둔 채 잊은 탭이 장 마감까지 파이프를 물고 있는다.
사용자가 정한 값이 3시간이다: 점심을 먹거나 회의를 한 번 하는 시간은 덮고,
「어제 열어 둔 탭」은 덮지 않는다.

시간이 다 되면 연결만 놓는다. 다시 보이는 순간 호출부가 새로 연결하므로 사용자가
할 일은 없다 — 3시간 뒤에 돌아온 사람에게는 어차피 스냅샷 한 벌이 필요하다.

# 왜 폴링은 안 살리는가

같은 규칙을 HTTP 폴링에 주면 안 보이는 탭이 계속 서버에 **요청을 만든다**. 스트림은
서버가 이미 만들어 둔 값을 밀어 주는 것이라 뷰어가 늘어도 DB 작업이 0인데
(`use-quote-stream` 머리글), 폴링은 왕복이 뷰어 수에 정비례한다. 그래서 폴링 쪽
`isPageVisible()` 가드는 그대로 둔다.

판정 자체는 `lib/stream-awake`에 있다 — 이 파일은 시계와 이벤트만 다룬다.
*/

export { STREAM_BACKGROUND_GRACE_MS };

/**
 * 지금 스트림을 붙여 둘 때인가.
 *
 * 보이는 동안은 언제나 true. 숨으면 `graceMs` 동안 true를 유지하다가 false로 떨어지고,
 * 다시 보이면 곧바로 true로 돌아온다.
 *
 * 호출부는 이 값을 **이펙트 의존성**에 넣어 쓴다 — false가 되면 이펙트가 정리되면서
 * `EventSource`가 닫히고, true로 돌아오면 다시 열린다. 「연결을 닫아라」를 따로
 * 부르지 않는 이유는, 닫는 길이 둘이 되면(정리 함수 + 콜백) 어느 쪽이 먼저인지에
 * 따라 열려 있는 연결이 남을 수 있기 때문이다.
 */
export function useStreamAwake(graceMs = STREAM_BACKGROUND_GRACE_MS): boolean {
  const [awake, setAwake] = useState(true);
  /** 숨은 시각. 보이는 중이면 null. 시계가 깨어났을 때 **다시 계산하려고** 들고 있다. */
  const hiddenSinceRef = useRef<number | null>(null);

  useEffect(() => {
    let timer = 0;

    const stopTimer = () => {
      if (timer) {
        window.clearTimeout(timer);
        timer = 0;
      }
    };

    const startTimer = () => {
      // 이미 재고 있으면 다시 걸지 않는다 — 숨은 채로 다른 이벤트가 와도 시계는 하나다.
      if (timer) return;
      timer = window.setTimeout(() => {
        timer = 0;
        /*
          시계가 아니라 **시각으로** 판정한다. 노트북 덮개를 닫으면 타이머가 늦게
          깨어나고, 브라우저는 숨은 탭의 타이머를 늦추기까지 한다 — 「울렸으니
          3시간이 지났다」는 믿을 수 없다.
        */
        if (!shouldKeepStream(hiddenSinceRef.current, Date.now(), graceMs)) {
          setAwake(false);
        } else {
          startTimer();
        }
      }, graceMs);
    };

    const sync = () => {
      if (document.visibilityState === "visible") {
        hiddenSinceRef.current = null;
        stopTimer();
        setAwake(true);
      } else {
        if (hiddenSinceRef.current === null) hiddenSinceRef.current = Date.now();
        startTimer();
      }
    };

    /*
      **마운트 시점에는 setState를 하지 않는다.** 초기값이 이미 true라 보이는 탭에서는
      할 일이 없고, 숨은 탭에서 마운트됐다면 시각을 적고 시계만 걸면 된다. 이펙트
      본문에서 상태를 건드리면 렌더가 한 번 더 돈다(`react-hooks/set-state-in-effect`).
    */
    if (document.visibilityState !== "visible") {
      hiddenSinceRef.current = Date.now();
      startTimer();
    }

    document.addEventListener("visibilitychange", sync);
    return () => {
      document.removeEventListener("visibilitychange", sync);
      stopTimer();
    };
  }, [graceMs]);

  return awake;
}
