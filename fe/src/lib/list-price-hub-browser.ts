"use client";

import { installStreamPort, setVisible, type StreamHandle, type StreamPort } from "@/lib/list-price-hub";
import { listPriceStreamUrl } from "@/lib/market-api";
import { STREAM_BACKGROUND_GRACE_MS } from "@/lib/stream-awake";
import type { ListPriceTick } from "@/lib/quote-stream-state";

/*
허브(`lib/list-price-hub`)에 **브라우저 쪽 이음매**를 꽂는다.

허브가 EventSource를 직접 알지 않는 이유는 시험이다 — 이 저장소의 vitest는 node
환경이라 EventSource도 document도 없다. 합집합·재연결·유예처럼 틀리기 쉬운 규칙은
저쪽에서 가짜 이음매로 확인하고, 여기는 **진짜 EventSource를 만드는 일**만 한다.

`visibilitychange`도 여기서 **앱 전체에 하나만** 듣는다. 예전에는 스트림을 여는 훅마다
자기 리스너를 달았는데, 홈 한 화면에 그 훅이 네 개였다.
*/

let installed = false;

/** 브라우저에서 처음 스트림이 필요해질 때 한 번 부른다. 두 번째부터는 아무 일도 없다. */
export function ensureBrowserStreamPort(): void {
  if (installed || typeof window === "undefined") return;
  installed = true;

  const port: StreamPort = {
    now: () => Date.now(),
    setTimer: (fn, ms) => window.setTimeout(fn, ms),
    clearTimer: (id) => window.clearTimeout(id),
    open: (symbols, assetType, handlers): StreamHandle => {
      const es = new EventSource(listPriceStreamUrl(symbols, assetType));

      /*
        붙자마자 오는 현재 값 묶음. 한 프레임에 목록 전체가 들어 있어서, 스무 줄이면
        SSE 프레임 스무 개가 아니라 하나다.
      */
      es.addEventListener("snapshot", (msg) => {
        try {
          handlers.onSnapshot(JSON.parse((msg as MessageEvent).data) as ListPriceTick[]);
        } catch {
          // 깨진 프레임 하나로 연결을 버리지 않는다 — 다음 틱이 곧 온다.
        }
      });

      es.addEventListener("price", (msg) => {
        try {
          handlers.onPrice(JSON.parse((msg as MessageEvent).data) as ListPriceTick);
        } catch {
          // 같은 이유로 무시.
        }
      });

      /* 마감 통보. 서버는 이 이벤트를 주고 끊는다 — 허브가 다음 개장 시각에 다시 연다. */
      es.addEventListener("session", (msg) => {
        let nextSessionAt: string | undefined;
        try {
          nextSessionAt = (
            JSON.parse((msg as MessageEvent).data) as { nextSessionAt?: string }
          ).nextSessionAt;
        } catch {
          nextSessionAt = undefined;
        }
        handlers.onSession(nextSessionAt);
      });

      es.onerror = () => handlers.onError();

      return {
        close: () => es.close(),
        isClosed: () => es.readyState === EventSource.CLOSED,
      };
    },
  };

  installStreamPort(port, STREAM_BACKGROUND_GRACE_MS);

  const sync = () => setVisible(document.visibilityState === "visible");
  document.addEventListener("visibilitychange", sync);
  sync();
}
