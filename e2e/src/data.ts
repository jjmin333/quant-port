import { randomUUID } from "node:crypto";

import { TEST_EMAIL_DOMAIN, TEST_EMAIL_PREFIX } from "./env";

/**
 * 테스트가 쓰는 고정 데이터.
 *
 * 값을 여기 모아 두는 이유는 **결정성** 때문이다. 수량·단가·날짜를 테스트 안에서
 * 즉석으로 만들면 평단·실현손익 기대값을 사람이 매번 다시 계산해야 하고, 계산이
 * 틀려도 테스트가 통과하는 쪽으로 기대값을 맞추게 된다. 여기 있는 조합은 손으로
 * 검산한 값이며 기대값을 주석에 남긴다.
 */

/**
 * 실제 종목 코드에 의존한다 — 카탈로그·시세가 있어야 스탬프/리서치 발행이
 * 캡처를 만들 수 있다. 존재하지 않는 심볼을 쓰면 "가격 없음"으로 실패하므로
 * 대형주만 고른다.
 *
 * 심볼을 용도별로 나눈 것은 **한 테스트가 다른 테스트의 쿨다운을 건드리지 않게**
 * 하려는 것이 아니다(계정이 테스트마다 새로 만들어지므로 쿨다운은 격리된다).
 * 어떤 테스트가 어떤 시장 데이터에 기대는지 한눈에 보이게 하려는 것이다.
 */
export const SYMBOLS = {
  /** 매매·평단 계산용 (USD). */
  trade: "NVDA",
  /** 스탬프용 (USD). */
  stamp: "AAPL",
  /** 관심종목·메모·목표가용 (USD). */
  watch: "MSFT",
  /** 리서치 콜용 (USD). */
  research: "TSLA",
  /** 국내 종목 경로 (KRW). */
  domestic: "005930",
} as const;

/**
 * 하루 발행 한도(5건)를 넘겨 보려면 **서로 다른 심볼 6개**가 필요하다 — 같은
 * 심볼은 30일 쿨다운에 먼저 걸려서 한도를 확인하기 전에 다른 이유로 막힌다.
 */
export const RESEARCH_CAP_SYMBOLS = [
  "NVDA",
  "AAPL",
  "MSFT",
  "TSLA",
  "GOOGL",
  "AMZN",
] as const;

/** 리서치 하루 발행 한도 (`service.researchDailyCap`). */
export const RESEARCH_DAILY_CAP = 5;

/** 같은 심볼 재발행 쿨다운, 일 단위 (`service.researchSymbolCooldownDays`). */
export const RESEARCH_SYMBOL_COOLDOWN_DAYS = 30;

/** 스탬프 심볼당 쿨다운, 초 단위 (`service.stampCooldown`). */
export const STAMP_COOLDOWN_SECONDS = 300;

/**
 * 매매 시나리오 — 평단·실현손익 기대값을 손으로 검산한 조합.
 *
 *   매수 10주 @100  → 수량 10, 평단 100
 *   매수 10주 @200  → 수량 20, 평단 150   ((10*100 + 10*200) / 20)
 *   매도 100주 @300 → 수량 0,  실현 3000  ((300 - 150) * 20, 보유량 20으로 바닥)
 *
 * 마지막 줄이 핵심이다: 보유량보다 많이 팔아도 수량은 음수가 되지 않고 실현손익도
 * 보유분까지만 계산된다.
 */
export const TRADES = {
  firstBuy: { quantity: 10, price: 100, tradeDate: "2026-01-05" },
  secondBuy: { quantity: 10, price: 200, tradeDate: "2026-01-06" },
  oversell: { quantity: 100, price: 300, tradeDate: "2026-01-07" },
  expected: {
    afterFirstBuy: { quantity: 10, avgPrice: 100, tradeCount: 1 },
    afterSecondBuy: { quantity: 20, avgPrice: 150, tradeCount: 2 },
    afterOversell: { quantity: 0, avgPrice: 0, realizedPnl: 3000, tradeCount: 3 },
  },
} as const;

export const PASSWORD = "e2e-quant-port-2026";

/**
 * 계정 이메일. UUID를 붙여 **동시 실행되는 워커끼리도 절대 겹치지 않게** 한다.
 * 타임스탬프만 쓰면 같은 초에 시작한 두 워커가 같은 이메일을 만들어 한쪽이
 * 409(email already registered)로 죽는다.
 */
export function uniqueEmail(tag: string): string {
  const slug = tag.replace(/[^a-z0-9]+/gi, "-").toLowerCase().slice(0, 24);
  return `${TEST_EMAIL_PREFIX}${slug}-${randomUUID()}@${TEST_EMAIL_DOMAIN}`;
}

/**
 * 표시명은 **짧은 ASCII**로 만든다.
 *
 * 테스트 제목(한글)을 그대로 쓰면 가입이 400으로 막힌다. BE의
 * `validateDisplayName`이 하한은 **글자 수**로, 상한은 **바이트 수**로 재기
 * 때문이다(`len([]rune(name)) < 2 || len(name) > 100`). 한글은 글자당 3바이트라
 * 34자만 넘어도 거부되는데, FE 입력칸은 `maxLength={100}`으로 100자를 받는다.
 * 픽스처가 그 구멍을 피해 가는 것이지, 그 동작이 옳다는 뜻은 아니다.
 */
export function uniqueDisplayName(tag: string): string {
  const slug = tag.replace(/[^a-z0-9]+/gi, "").toLowerCase().slice(0, 12);
  return `QA-${slug || "user"}-${randomUUID().slice(0, 8)}`;
}
