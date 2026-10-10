"use client";

import { useEffect, useMemo, useRef, useState, useSyncExternalStore } from "react";
import { useQuoteStreamByMarket } from "@/hooks/use-quote-stream";
import { useVisibleInterval } from "@/hooks/use-visible-interval";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { hydrateLastViewed, lastViewedSymbol, subscribePinned } from "@/lib/pinned-symbols";
import { formatQuoteTabTitle } from "@/lib/quote-tab-title";
import type { QuoteCurrency } from "@/lib/quotes-types";

/*
브라우저 **탭 제목**이 마지막으로 연 종목의 시세를 말한다 (2026-09-15 사용자 요청 4).

	$141.91 -1.98% | 오라클

# 왜 화면이 아니라 탭 제목인가

사용자가 보낸 사진이 떠 있는 위젯이 아니라 브라우저 탭이었다. 그리고 그것이 요청 3과
4의 앞뒤를 맞춘다 — **탭 제목은 그 탭이 안 보이는 동안에도 탭 스트립에 그대로 보인다.**
안 보이는 탭의 구독을 전부 끊으면서도 「보던 종목 하나」만 남긴 이유가 이것이다.

# 이 컴포넌트가 곧 그 구독자다

핀(`lib/pinned-symbols`)은 「무엇을 살려 둘까」만 답하고 **구독을 만들지는 않는다.**
허브는 구독자들의 합집합에서 좁히기만 하므로(`narrowToPinned`), 종목 화면을 떠나
아무도 그 종목을 구독하지 않게 되면 남길 것도 없어진다.

그 자리를 여기서 메운다. 이 컴포넌트는 껍데기에 **하나만** 떠 있고 마지막으로 연
종목을 스스로 구독한다 — 그래서 어느 메뉴에 가 있든 그 종목은 언제나 합집합 안에
있고, 탭이 숨으면 정확히 그것만 남는다.

# 등락률은 스냅샷 한 벌이 있어야 나온다

스트림은 **가격만** 실어 준다. 전일 종가는 `/quotes/batch` 한 번으로 받아 두고, 새
가격이 올 때마다 화면 쪽에서 다시 센다(`quoteChangeRate`). 스냅샷은 종목이 바뀔 때와
**보이는 동안 5분마다** 다시 받는다 — 날이 바뀌면 전일 종가가 달라지기 때문이고,
`useVisibleInterval`을 쓰므로 안 보는 탭은 그 요청을 만들지 않는다(요청 3의 규칙).

# 남이 쓴 제목을 되돌려 놓는다

제목을 쓰는 쪽이 우리만이 아니다. Next는 화면을 옮길 때마다 그 화면의 메타로 제목을
덮어쓴다(`generateMetadata` — 2026-09-15 요청 2). 그래서 `<title>`을 지켜보다가
남이 바꾸면 **그 값을 원래 제목으로 기억해 두고** 우리 줄을 다시 얹는다. 종목을
그만 보면 기억해 둔 그 제목으로 돌아간다.
*/

/** 전일 종가를 다시 받는 주기 — 날이 바뀌면 기준가가 달라진다. */
const SNAPSHOT_REFRESH_MS = 5 * 60_000;

type Snapshot = {
  symbol: string;
  name: string;
  prevClose: number;
  price: number;
  currency: QuoteCurrency;
};

export function QuoteTabTitle() {
  /*
    새로고침을 견디게 한다(`sessionStorage`). 이펙트에서 부르는 이유는 서버 렌더에는
    이 값이 없기 때문이다 — 렌더 중에 읽으면 첫 HTML과 첫 클라이언트 렌더가 어긋난다.
  */
  useEffect(() => {
    hydrateLastViewed();
  }, []);

  const pinned = useSyncExternalStore(
    subscribePinned,
    lastViewedSymbol,
    // 서버에는 「마지막으로 본 종목」이라는 것이 없다.
    () => null,
  );

  const symbol = pinned?.symbol ?? "";
  const assetType = pinned?.assetType ?? "";

  const [snapshot, setSnapshot] = useState<Snapshot | null>(null);

  /*
    종목이 바뀌면 **옛 종목의 값을 먼저 버린다.** 안 그러면 새 종목의 스냅샷이 올
    때까지 한 박자 동안 탭 제목이 「새 종목 이름 + 옛 가격」이 된다.
  */
  const [lastSymbol, setLastSymbol] = useState(symbol);
  if (lastSymbol !== symbol) {
    setLastSymbol(symbol);
    setSnapshot(null);
  }

  const loadSnapshot = useMemo(
    () => () => {
      if (!symbol) return;
      void fetchQuotesBatchClient([symbol])
        .then((rows) => {
          const row = rows.find((r) => r.symbol.toUpperCase() === symbol);
          if (!row) return;
          setSnapshot({
            symbol,
            name: row.name,
            // 전일 종가는 응답에 따로 없다 — 현재가에서 변화폭을 뺀 값이 그것이다.
            prevClose: row.currentPrice - row.change,
            price: row.currentPrice,
            currency: row.currency,
          });
        })
        .catch(() => {
          // 탭 제목 하나 때문에 화면에 오류를 띄우지 않는다. 다음 회차가 다시 받는다.
        });
    },
    [symbol],
  );

  useEffect(() => {
    loadSnapshot();
  }, [loadSnapshot]);

  // 날이 바뀌면 전일 종가가 달라진다. 안 보는 탭은 이 요청을 만들지 않는다.
  useVisibleInterval(loadSnapshot, SNAPSHOT_REFRESH_MS, Boolean(symbol));

  /*
    **이 구독이 요청 4의 알맹이다.** 허브의 합집합에 이 종목을 넣어 두므로, 탭이
    숨어 유예가 지나면 목록이 통째로 빠지고 이 한 줄만 남는다.
  */
  const rows = useMemo(
    () => (symbol ? [{ symbol, assetType }] : []),
    [symbol, assetType],
  );
  const { prices } = useQuoteStreamByMarket(rows);
  const live = symbol ? prices.get(symbol) : undefined;

  const desired = useMemo(() => {
    if (!snapshot || snapshot.symbol !== symbol) return "";
    return formatQuoteTabTitle({
      name: snapshot.name,
      // 스트림 값이 있으면 그것이 최신이다. 없으면 스냅샷의 값(마감·개장 전).
      price: live?.price && live.price > 0 ? live.price : snapshot.price,
      prevClose: snapshot.prevClose,
      currency: snapshot.currency,
    });
  }, [snapshot, symbol, live?.price]);

  useTabTitleOverride(desired);

  return null;
}

/**
 * `desired`가 비어 있지 않은 동안 문서 제목을 그 값으로 붙들어 둔다.
 *
 * 되돌릴 제목은 **남이 마지막으로 쓴 값**이다. 마운트 시점의 제목을 기억해 두면
 * 화면을 몇 번 옮긴 뒤에 되돌릴 때 첫 화면의 제목이 나온다.
 */
function useTabTitleOverride(desired: string): void {
  /** 남이 쓴 마지막 제목. 종목을 그만 볼 때 여기로 돌아간다. */
  const baseRef = useRef<string>("");

  useEffect(() => {
    if (baseRef.current === "") baseRef.current = document.title;

    if (!desired) {
      // 그만 보기 — 남이 쓴 제목으로 돌아간다.
      if (baseRef.current && document.title !== baseRef.current) {
        document.title = baseRef.current;
      }
      return;
    }

    if (document.title !== desired) document.title = desired;

    /*
      Next가 화면을 옮기며 제목을 덮어쓴다. 그 값을 **되돌릴 제목으로 기억해 두고**
      우리 줄을 다시 얹는다. 우리가 쓴 값으로 다시 불려도 `!== desired`가 거짓이라
      무한히 돌지 않는다.

      **`<title>`이 아니라 `<head>`를 지켜본다.** 처음에는 `<title>` 엘리먼트에 직접
      달았는데, 메뉴를 옮기면 제목이 페이지 제목으로 돌아간 채 다시 안 얹혔다
      (실측 2026-09-15, `/holdings` → `/news/domestic`). Next는 라우트가 바뀔 때
      그 엘리먼트를 **갈아끼우므로**, 옵저버가 문서에서 떨어져 나간 옛 노드를 보고
      있었다. `<head>`는 그대로 남아 있어 교체까지 함께 잡힌다.
    */
    const observer = new MutationObserver(() => {
      if (document.title === desired) return;
      baseRef.current = document.title;
      document.title = desired;
    });
    observer.observe(document.head, {
      childList: true,
      characterData: true,
      subtree: true,
    });
    return () => observer.disconnect();
  }, [desired]);
}
