import { randomUUID } from "node:crypto";

import { TEST_LOGIN_PREFIX } from "./env";

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

/** 비밀번호는 8~15자 (2026-09-27 가입 규칙). */
export const PASSWORD = "e2e-qp-2026!";

/**
 * 계정 아이디. 가입이 이메일 → 아이디로 바뀌었다(2026-09-27 요청 3): 영문 소문자로
 * 시작하는 소문자·숫자 4~20자. 예약 접두사 `qae2e` + UUID 16진수 15자 = 20자로
 * **동시 실행되는 워커끼리도 겹치지 않게** 한다(타임스탬프만 쓰면 같은 초에 시작한
 * 두 워커가 같은 아이디를 만들어 한쪽이 409로 죽는다). 형식이 좁아 태그는 싣지 못한다.
 */
export function uniqueLoginId(_tag?: string): string {
  return `${TEST_LOGIN_PREFIX}${randomUUID().replace(/-/g, "").slice(0, 20 - TEST_LOGIN_PREFIX.length)}`;
}

/** 한글 음절 n개 — 닉네임 글자 수 경계를 한글로 시험할 때(중복 불가라 매번 달라야 한다). */
export function randomHangul(n: number): string {
  let out = "";
  for (let i = 0; i < n; i++) out += String.fromCharCode(0xac00 + Math.floor(Math.random() * 11172));
  return out;
}

/**
 * 닉네임은 2~10자이고 **다른 회원과 겹칠 수 없다**(2026-09-27). `QA` + UUID 8자 = 10자.
 * 태그는 싣지 못한다 — 10자 안에 겹치지 않을 만큼의 난수를 넣는 편이 중요하다.
 */
export function uniqueDisplayName(_tag?: string): string {
  return `QA${randomUUID().replace(/-/g, "").slice(0, 8)}`;
}
