/**
 * 예·적금 「내가 채울 수 있는 우대조건」(F1) · 가입 방법·대상 칩(F2) — 2026-10-03 사용자 결정.
 * 순수 함수만(시험 `fin-conds.test.ts`). 우대조건 이름은 asset_ai가 원문을 정리한 짧은 이름(8자 안팎)이다.
 */

export type CondCategory = { id: string; label: string; re: RegExp };

/** 사용자가 고르는 칩. 순서 = 화면 순서. 이름 하나가 여러 칩에 걸리면 앞의 것. */
export const COND_CATEGORIES: CondCategory[] = [
  { id: "online", label: "비대면·앱 가입", re: /비대면|모바일|앱|오픈뱅킹|인터넷|스마트|인증 등록/ },
  { id: "marketing", label: "마케팅·알림 동의", re: /마케팅|동의|알림|정보제공|멤버십/ },
  { id: "first", label: "첫 거래·신규", re: /최초|신규|첫|미보유|미발급/ },
  { id: "salary", label: "급여·연금 이체", re: /급여|연금|소득이체/ },
  { id: "card", label: "카드 실적", re: /카드/ },
  { id: "autopay", label: "자동이체", re: /자동이체|출금실적|공과금|관리비/ },
  { id: "holding", label: "다른 상품 보유", re: /보유|평잔|동시|청약/ },
  { id: "rollover", label: "재예치·자동연장", re: /재예치|자동연장|회전/ },
];

/** 조건 없이 붙는 가산(기본 우대·특판) — 칩과 상관없이 늘 더한다. */
const AUTO_RE = /기본\s*우대|특판/;

export function condCategory(label: string): string {
  if (AUTO_RE.test(label)) return "auto";
  return COND_CATEGORIES.find((c) => c.re.test(label))?.id ?? "other";
}

/**
 * 내 예상 금리 = 기본금리 + (고른 칩에 걸리는 조건 · 자동 가산)의 가산폭 합, 최고금리를 넘지 않게.
 * 조건 정리가 없거나 가산폭이 0(원문에 숫자 없음)이면 기본금리 그대로 — 모르는 가산은 더하지 않는다.
 */
export function myRate(base: number, max: number, conds: { l: string; b: number }[] | undefined, picked: ReadonlySet<string>): number {
  let r = base;
  for (const c of conds ?? []) {
    if (!(c.b > 0)) continue;
    const cat = condCategory(c.l);
    if (cat === "auto" || picked.has(cat)) r += c.b;
  }
  return Math.round(Math.min(Math.max(max, base), r) * 1000) / 1000;
}

// ── 가입 방법 · 대상 ────────────────────────────────────────────────────────

export type JoinWay = "online" | "branch";
export type JoinTarget = "youth" | "senior" | "biz";

export const JOIN_WAY_OPTIONS: { id: "all" | JoinWay; label: string }[] = [
  { id: "all", label: "전체" },
  { id: "online", label: "비대면 가입" },
  { id: "branch", label: "영업점 가입" },
];

export const JOIN_TARGET_OPTIONS: { id: "all" | JoinTarget; label: string }[] = [
  { id: "all", label: "누구나" },
  { id: "youth", label: "청년" },
  { id: "senior", label: "시니어" },
  { id: "biz", label: "사업자" },
];

/** 공시 「가입 방법」(인터넷,스마트폰,영업점…)에서. */
export function joinWays(joinWay: string): Set<JoinWay> {
  const out = new Set<JoinWay>();
  if (/인터넷|스마트폰|모바일|전화|비대면|앱/.test(joinWay)) out.add("online");
  if (/영업점|창구|방문/.test(joinWay)) out.add("branch");
  return out;
}

/**
 * 공시 「가입 대상」 원문에서 특정 대상 전용·가능 여부. 「누구나」는 칩이 아니라 기본값(거르지 않음)이다.
 * 청년: 「청년」·「MZ」·만 34/39세 이하 · 시니어: 「시니어」·「고령」·만 50~65세 이상 · 사업자: 「사업자」·「법인」.
 */
export function joinTargets(member: string): Set<JoinTarget> {
  const out = new Set<JoinTarget>();
  const m = member.replace(/\s+/g, "");
  if (/청년|MZ|만(19|1[7-9])세~?(29|3[0-9])세|(29|3[0-9])세이하/.test(m)) out.add("youth");
  if (/시니어|고령|어르신|만(5[0-9]|6[0-9])세이상/.test(m)) out.add("senior");
  if (/사업자|법인/.test(m) && !/사업자제외|법인불가|법인제외/.test(m)) out.add("biz");
  return out;
}
