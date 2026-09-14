"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import { useQuoteStreamByMarket, type StreamableRow } from "@/hooks/use-quote-stream";
import { changeRateFrom, streamSymbolsKey, symbolsFromKey } from "@/lib/quote-stream-state";
import type { QuoteSummary } from "@/lib/quotes-types";

/*
**종목이 들어간 카드는 전부 이 훅으로 가격을 받는다** (2026-09-04 사용자 요청 8·9).

# 무엇을 고치나

09-04 이전에 실시간으로 움직이는 가격은 두 화면(실시간 차트·AI 랭킹)뿐이었다.
나머지 — 홈의 실적 일정 카드, 뉴스 레일의 다가오는 일정·내 종목 뉴스·많이 언급된
종목, 브리핑 하단 키워드 — 는 종목을 나열하면서 가격을 아예 안 적거나(「예정 ·
2027 1Q」), 적더라도 화면을 처음 열 때의 값에서 멈춰 있었다.

사용자에게는 그것이 **같은 앱 안에서 어떤 종목은 살아 있고 어떤 종목은 죽어 있는
것**으로 보인다.

# 왜 카드마다 따로 만들지 않는가

카드마다 자기 `useEffect`로 배치 시세를 부르면 여섯 개 카드가 여섯 번 부른다.
그것이 09-04에 사용자가 지적한 「요청한 횟수가 너무 잦습니다」의 한 갈래다(요청 11).
훅 하나로 모으면 적어도 **규칙이 한 곳**에 있어, 다음에 캐시나 합치기를 넣을 때
고칠 자리가 하나다.

# 첫 값은 REST, 그 뒤는 푸시

배치 시세(`/quotes/batch`)를 **한 번** 불러 이름·통화·기준가를 받고, 그 뒤 가격은
다심볼 SSE가 민다. 스트림이 가격만 보내는 이유와 등락률을 화면이 계산하는 이유는
`quote-stream-state.ts`의 `changeRateFrom` 머리글에 있다 — 서버가 틱마다 기준가를
풀면 종목당 4~5 SELECT가 붙는다.

# 마감이면 붙지 않는다

`useQuoteStreamByMarket`이 이미 그렇게 한다 — 서버가 열자마자 `session` 이벤트
하나를 주고 끊고, 화면은 그 안의 다음 개장 시각에 타이머 하나로 다시 붙는다.
그래서 이 훅을 카드마다 달아도 **마감 시간(하루의 대부분)에는 연결이 0개**다
(사용자 요청 9의 「불필요한 SSE 연결을 하지 않게」).

목록이 두 시장에 걸쳐 있어도 된다 — 뉴스 레일처럼 국내·해외가 섞인 목록은
시장별로 연결을 나눠 연다(그 함수 머리글 참조).
*/

/** 배치로 받은 종목에 **지금 가격**을 얹은 것. */
export type LiveQuote = QuoteSummary & {
  /** 스트림이 준 가격. 아직 안 왔으면 배치가 준 값 그대로다. */
  livePrice: number;
  /** 그 가격으로 다시 낸 등락률. 기준가를 모르면 배치가 준 값 그대로다. */
  liveChangeRate: number;
};

const EMPTY: Map<string, LiveQuote> = new Map();

/**
 * 아직 이 목록의 시세가 없을 때 쓰는 **하나뿐인 빈 배열**.
 *
 * 매번 `[]`를 새로 만들면 참조가 렌더마다 달라져 아래 `useMemo`들이 전부 헛돌고,
 * 그중 하나가 SSE 구독 목록이라 **렌더마다 연결을 끊고 다시 맺는다** — 연결 하나로
 * 아끼려던 것을 렌더 수만큼 되돌린다.
 */
const NO_ROWS: QuoteSummary[] = [];

/**
 * 심볼 목록의 이름·가격·등락률. 심볼이 없으면 빈 맵이고 아무것도 부르지 않는다.
 *
 * 반환 맵의 키는 **대문자 심볼**이다 — 호출부가 `q.symbol`을 그대로 쓰면 국내
 * 6자리 코드는 문제없지만 미국 티커는 대소문자가 섞여 들어올 수 있다.
 */
export function useLiveQuotes(symbols: readonly string[]): Map<string, LiveQuote> {
  /*
    목록은 렌더마다 새 배열이다. 키로 바꾸지 않으면 이펙트가 **렌더마다** 배치
    시세를 다시 부른다 — 요청을 줄이려고 만든 훅이 요청을 만드는 자리가 된다.
  */
  const key = useMemo(() => streamSymbolsKey(symbols), [symbols]);
  const [base, setBase] = useState<{ key: string; rows: QuoteSummary[] }>({ key: "", rows: [] });

  useEffect(() => {
    if (!key) return;
    let alive = true;
    fetchQuotesBatchClient(symbolsFromKey(key))
      .then((rows) => {
        if (alive) setBase({ key, rows });
      })
      // 한 카드의 시세 실패가 그 카드의 나머지(이름·일정)를 지우면 안 된다.
      .catch(() => {
        if (alive) setBase({ key, rows: NO_ROWS });
      });
    return () => {
      alive = false;
    };
  }, [key]);

  /*
    **이 목록의 시세가 맞는지 확인한다.** 심볼이 바뀌면 새 응답이 오기 전까지
    앞 목록의 시세가 남아 있는데, 그대로 쓰면 다른 종목 자리에 앞 종목 가격이
    한 프레임 얹힌다(v1.9.0에 시장 전환에서 같은 종류의 버그가 있었다).
  */
  const rows = base.key === key ? base.rows : NO_ROWS;

  const streamRows = useMemo<StreamableRow[]>(
    () => rows.map((r) => ({ symbol: r.symbol, assetType: r.assetType })),
    [rows],
  );
  const { prices } = useQuoteStreamByMarket(streamRows);

  return useMemo(() => {
    if (rows.length === 0) return EMPTY;
    const out = new Map<string, LiveQuote>();
    for (const row of rows) {
      const sym = row.symbol.toUpperCase();
      const tick = prices.get(sym);
      const livePrice = tick && tick.price > 0 ? tick.price : row.currentPrice;
      /*
        기준가는 배치가 준 값에서 되돌린다(현재가 − 전일대비). 세션 안에서 변하지
        않는 값이라 한 번 받아 두면 되고, 서버가 틱마다 풀면 그 비용이 그대로
        돌아온다. 되돌린 값이 0 이하면 등락률을 새로 내지 않는다 — 0.00%는
        「보합」으로 읽히는데 실제 뜻은 「기준가를 모른다」이다.
      */
      const rate = changeRateFrom(livePrice, row.currentPrice - row.change);
      out.set(sym, {
        ...row,
        livePrice,
        liveChangeRate: rate ?? row.changeRate,
      });
    }
    return out;
  }, [rows, prices]);
}
