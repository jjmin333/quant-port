"use client";

import { useEffect, useMemo, useState } from "react";
import { fetchQuotesBatchClient } from "@/lib/market-api";
import type { JournalNote } from "@/lib/journal-types";
import type { QuoteSummary } from "@/lib/quotes-types";

/**
 * 메모장 재설계 시안 전용 픽스처 (2026-09-07).
 *
 * # 왜 실서비스 예시(`/api/v1/sample/journal`)를 그대로 쓰지 않나
 *
 * 그쪽은 **5건**이다. 지금 화면을 보기에는 충분하지만, 여기서 재려는 것은 「메모가
 * 쌓였을 때 이 배치가 견디는가」다 — 월별 묶음도, 종목별 묶음도, 성과 분포도 5건
 * 에서는 아무 말을 하지 않는다. 그렇다고 서버 픽스처를 늘리면 `sample.Journal`이
 * 요청마다 종목 시세와 그날 종가를 다시 재므로(캐시 없이) 실서비스 예시 모드의
 * 외부 호출이 종목 수만큼 늘어난다. 시안 하나 보자고 낼 값이 아니다.
 *
 * 그래서 **행만** 이쪽에서 만든다. 그리는 컴포넌트는 전부 실서비스의 것이고
 * (`JournalPriceStrip`·`JournalTagChips`·`StockAvatar`·`JournalNoteModal`…),
 * 심볼도 실제 심볼이라 로고·차트·퀵룩은 진짜 데이터를 받아 온다.
 *
 * # 「작성 당시 가격」을 박아 두지 않고 성과율을 박아 둔다
 *
 * 값을 박아 두면 며칠 뒤 실제 주가가 움직였을 때 **작성 당시 가격이 최근 캔들 범위
 * 밖으로 밀려난다** — 상세 모달의 「작성 시점」 기준선이 차트 밖으로 나가 선이 없는
 * 게 아니라 잘못 그려진 것처럼 보인다. 그래서 각 메모는 성과율(`outcome`)만 갖고,
 * 작성 당시 가격은 **현재가 ÷ (1 + 성과율)**로 만든다. 두 값이 늘 같은 세계에 있고,
 * 시안을 언제 열어도 「어느 판단이 맞았나」의 답은 같다.
 *
 * # 날짜는 고정 기준일에서 센다
 *
 * `Date.now()`로 만들면 서버 렌더와 클라이언트 렌더의 문자열이 갈려 하이드레이션이
 * 어긋난다. 기준일 하나를 박아 두고 거기서 뺀다.
 */

/** 시안을 만든 날. 모든 메모 날짜가 여기서 역산된다. */
const ANCHOR = new Date("2026-09-07T09:00:00+09:00").getTime();

function ago(days: number): string {
  return new Date(ANCHOR - days * 24 * 60 * 60 * 1000).toISOString();
}

function dateOnly(days: number): string {
  return new Date(ANCHOR - days * 24 * 60 * 60 * 1000).toISOString().slice(0, 10);
}

type Seed = {
  title: string;
  body: string;
  symbol?: string;
  name?: string;
  currency?: "KRW" | "USD";
  /** 작성 이후 성과(%). 작성 당시 가격은 현재가에서 이 값으로 역산한다. */
  outcome?: number;
  tags: string[];
  pinned?: boolean;
  daysAgo: number;
};

/**
 * 열여섯 건. 고른 기준은 「이 화면이 감당해야 하는 경우」가 한 번씩은 나오도록 —
 * 종목 연결 있음/없음, 국내/해외, 크게 맞음/크게 틀림/제자리, 태그 0~3개,
 * 본문 한 줄/여러 문단, 고정 3건, 오늘부터 넉 달 반 전까지.
 */
const SEEDS: Seed[] = [
  {
    title: "삼성전자 — 왜 담았나",
    body: "HBM 공급 계약 확대가 실적에 반영되기 시작하는 시점이라고 봤다.\n\n확인할 것\n· 다음 분기 반도체 부문 영업이익률\n· 외국인 순매수가 이어지는가\n\n손절은 평단 -12%. 이유가 깨지는 지점은 「HBM 물량이 경쟁사로 넘어갔다」는 소식 하나다.",
    symbol: "005930",
    name: "삼성전자",
    currency: "KRW",
    outcome: 26.2,
    tags: ["매수근거"],
    pinned: true,
    daysAgo: 132,
  },
  {
    title: "SK하이닉스 — 비중 얼마까지",
    body: "반도체 하나에 포트의 40%가 몰렸다. 같은 이유로 오르내리는 종목을 두 개 들고 있으면 그건 분산이 아니다. 하나는 줄인다.",
    symbol: "000660",
    name: "SK하이닉스",
    currency: "KRW",
    outcome: 27.5,
    tags: ["리스크", "복기"],
    daysAgo: 96,
  },
  {
    title: "엔비디아 관찰",
    body: "데이터센터 매출 비중이 계속 오르는 중. 다만 밸류에이션 축이 5축 중 유일하게 30점대라 조정 시 변동성이 크다는 점을 기억할 것.",
    symbol: "NVDA",
    name: "엔비디아",
    currency: "USD",
    outcome: 33.2,
    tags: ["관찰"],
    daysAgo: 74,
  },
  {
    title: "카카오 — 손절 근거 정리",
    body: "광고 매출 둔화가 두 분기째다. 반등 근거로 삼았던 신규 서비스 지표가 아직 안 나온다. 다음 분기까지 확인하고 정리 판단.",
    symbol: "035720",
    name: "카카오",
    currency: "KRW",
    outcome: -17.2,
    tags: ["리스크", "복기"],
    daysAgo: 61,
  },
  {
    title: "이번 달 반성",
    body: "실적 발표를 앞둔 종목을 이유 없이 들고 있었다. 발표 일정을 먼저 캘린더에 넣고 나서 담기로.",
    tags: ["복기"],
    pinned: true,
    daysAgo: 54,
  },
  {
    title: "테슬라 — 진입 보류",
    body: "인도량은 좋았지만 마진이 계속 눌린다. 가격 인하로 산 물량은 다음 분기 마진을 또 깎는다. 마진이 두 분기 연속 올라오는 걸 보고 들어간다.",
    symbol: "TSLA",
    name: "테슬라",
    currency: "USD",
    outcome: -12.2,
    tags: ["관찰", "매수근거"],
    daysAgo: 47,
  },
  {
    title: "한미반도체 실적 발표 전 정리",
    body: "장비 수주 잔고가 관건. 컨센을 상회해도 잔고가 안 늘면 다음 분기 이야기가 없다.",
    symbol: "042700",
    name: "한미반도체",
    currency: "KRW",
    outcome: 21.7,
    tags: ["관찰"],
    daysAgo: 38,
  },
  {
    title: "환율 메모",
    body: "달러 예수금 비중이 40%를 넘었다. 원/달러가 1,400원 아래로 내려오면 일부 환전 검토.",
    tags: ["관찰"],
    daysAgo: 31,
  },
  {
    title: "NAVER — 다시 본다",
    body: "커머스보다 광고 회복이 먼저 온다고 봤는데 순서가 반대였다. 가정이 틀린 게 아니라 순서를 못 맞춘 것 — 방향은 유지하되 진입 시점을 늦춘다.",
    symbol: "035420",
    name: "NAVER",
    currency: "KRW",
    outcome: -7.6,
    tags: ["복기", "매수근거"],
    daysAgo: 27,
  },
  {
    title: "애플 — 서비스 부문만 본다",
    body: "하드웨어 사이클은 이제 예측이 안 된다. 서비스 매출 성장률 두 자리만 유지되면 들고 간다.",
    symbol: "AAPL",
    name: "애플",
    currency: "USD",
    outcome: 17.9,
    tags: ["매수근거"],
    daysAgo: 22,
  },
  {
    title: "손절 규칙을 안 지켰다",
    body: "-12%에서 끊기로 적어놓고 -19%까지 들고 있었다. 「조금만 더 보면 오를 것 같아서」가 이유였는데, 그건 이유가 아니라 기분이다.\n\n다음부터는 메모에 적은 숫자를 목표가로도 등록해 둔다.",
    tags: ["복기", "리스크"],
    pinned: true,
    daysAgo: 18,
  },
  {
    title: "LG에너지솔루션 — 물량 확인",
    body: "북미 증설분이 언제 매출로 잡히는지가 전부다. 지금 주가는 그걸 이미 반영했다고 보는 쪽에 가깝다.",
    symbol: "373220",
    name: "LG에너지솔루션",
    currency: "KRW",
    outcome: -3.6,
    tags: ["리스크"],
    daysAgo: 14,
  },
  {
    title: "마이크로소프트 — 클라우드 성장률",
    body: "Azure 성장률이 재가속했다. AI 관련 매출이 별도 항목으로 잡히기 시작한 게 크다.",
    symbol: "MSFT",
    name: "마이크로소프트",
    currency: "USD",
    outcome: -2.4,
    tags: ["매수근거", "관찰"],
    daysAgo: 9,
  },
  {
    title: "시장 전체 — 지금 현금 비중",
    body: "현금 12%. 조정이 오면 살 수 있는 수준은 아니다. 20%까지 올려두고 기다린다.",
    tags: [],
    daysAgo: 6,
  },
  {
    title: "SK하이닉스 재진입",
    body: "앞서 줄인 판단은 맞았다. 다만 줄인 뒤 더 오른 구간이 있었고, 그건 「비중을 줄인 것」과 「전량 판 것」을 구분 못 한 결과다. 이번엔 절반만.",
    symbol: "000660",
    name: "SK하이닉스",
    currency: "KRW",
    outcome: 4.4,
    tags: ["매수근거", "복기"],
    daysAgo: 3,
  },
  {
    title: "삼성전자 — 목표가 다시 계산",
    body: "처음 적은 근거는 아직 유효하다. 다만 목표가를 올릴 근거는 없다 — 실적이 예상대로 나온 것이지 예상을 넘은 게 아니다.",
    symbol: "005930",
    name: "삼성전자",
    currency: "KRW",
    outcome: 2.1,
    tags: ["관찰"],
    daysAgo: 1,
  },
];

export const LAB_SYMBOLS = Array.from(
  new Set(SEEDS.map((s) => s.symbol).filter((s): s is string => Boolean(s))),
);

/**
 * 현재가를 못 받았을 때 쓰는 값. 시안이 시세 하나 때문에 텅 비면 배치를 볼 수 없다 —
 * 실서비스 화면이 「시세는 덧붙는 정보이지 메모의 전제가 아니다」로 만들어져 있는 것과
 * 같은 태도다.
 */
const FALLBACK_PRICE: Record<string, number> = {
  "005930": 269500,
  "000660": 1787000,
  "035720": 36550,
  "035420": 218000,
  "373220": 364500,
  "042700": 241500,
  NVDA: 229.47,
  AAPL: 320.05,
  TSLA: 352.9,
  MSFT: 499.49,
};

function round(value: number, currency: "KRW" | "USD"): number {
  return currency === "KRW" ? Math.round(value / 50) * 50 : Math.round(value * 100) / 100;
}

function buildNotes(price: (symbol: string) => number): JournalNote[] {
  return SEEDS.map((s, i) => {
    const currency = s.currency ?? "KRW";
    const current = s.symbol ? price(s.symbol) : 0;
    const base =
      s.symbol && s.outcome !== undefined && current > 0
        ? round(current / (1 + s.outcome / 100), currency)
        : undefined;
    return {
      id: i + 1,
      title: s.title,
      body: s.body,
      symbol: s.symbol,
      symbolName: s.name,
      priceAtWrite: base,
      priceCurrency: base ? currency : undefined,
      priceDate: base ? dateOnly(s.daysAgo) : undefined,
      tags: s.tags,
      pinned: Boolean(s.pinned),
      createdAt: ago(s.daysAgo),
      updatedAt: ago(s.daysAgo),
    } satisfies JournalNote;
  });
}

/**
 * 메모 열여섯 건 + 그 종목의 현재 시세.
 *
 * 시세는 실서비스와 **같은 경로**로 한 번에 받는다(`fetchQuotesBatchClient`) — 시안이
 * 따로 만든 통로로 숫자를 가져오면, 화면이 실제로 감당해야 하는 지연·실패가 시안에서
 * 빠진다. 실패하면 위의 대체 값으로 그대로 그린다.
 */
export function useLabJournalData() {
  const [quotes, setQuotes] = useState<Record<string, QuoteSummary>>({});

  useEffect(() => {
    let alive = true;
    fetchQuotesBatchClient(LAB_SYMBOLS)
      .then((rows) => {
        if (alive) setQuotes(Object.fromEntries(rows.map((q) => [q.symbol, q])));
      })
      .catch(() => {
        // 시세 없이도 메모는 유효하다 — 대체 값으로 계속 간다.
      });
    return () => {
      alive = false;
    };
  }, []);

  const notes = useMemo(
    () => buildNotes((symbol) => quotes[symbol]?.currentPrice || FALLBACK_PRICE[symbol] || 0),
    [quotes],
  );

  return { notes, quotes };
}
