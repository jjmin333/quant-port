/**
 * 「지금 이 값이 움직이고 있는가」 판정.
 *
 * # 고친 것 (2026-09-03 사용자 지적)
 *
 * 종목 헤더는 시세 객체만 있으면 **언제나** 초록 점 + 「실시간」을 찍었다. 그래서
 * 새벽 2시에 열어도 SK하이닉스가 실시간으로 보였다(실측 스크린샷: 「실시간 /
 * 오전 02:07:43 기준」). 값이 안 움직이는 시간대에 실시간이라고 적는 것은
 * 화면이 하는 말 중 가장 나쁜 종류의 거짓말이다 — 사용자가 그 숫자로 판단한다.
 *
 * # 왜 시간대를 여기서 계산하지 않는가
 *
 * 세션 창은 종목마다 다르다. 국내만 해도 넥스트레이드 종목은 08:00부터,
 * 나머지는 09:00부터 거래되고, 시간외까지 치면 20:00까지 간다. 미국은 서머타임에
 * 따라 창이 통째로 한 시간 밀린다. 이걸 브라우저에서 다시 계산하면 **서버의
 * 판정과 어긋나는 두 번째 진실**이 생긴다.
 *
 * 서버는 이미 토스 거래소 달력으로 세션을 판정해 `quote.marketSession`에 담아
 * 보낸다(`be/internal/service/market_calendars.go`의 `detectMarketSession`).
 * 화면은 그 값을 **읽기만** 한다.
 */

/** 서버가 보내는 세션 값. 이 밖의 값이 오면 「모름」으로 다룬다. */
export type MarketSessionId = "regular" | "pre" | "after" | "day" | "closed";

const SESSION_LABELS: Record<MarketSessionId, string> = {
  regular: "실시간",
  // 정규장 밖이지만 **체결은 일어난다.** 「실시간」이라고 뭉뚱그리지 않고 어느
  // 세션인지 적는다 — 시간외 가격은 정규장 가격과 호가 두께가 다르다.
  pre: "장 시작 전 거래",
  after: "시간외 거래",
  day: "데이마켓",
  closed: "장 마감",
};

export function parseMarketSession(value: string | undefined): MarketSessionId | null {
  if (!value) return null;
  if (value in SESSION_LABELS) return value as MarketSessionId;
  return null;
}

/**
 * 가격이 지금 움직이는 세션인가.
 *
 * **모르면 false다.** 세션을 못 받은 상태에서 「실시간」을 찍으면, 그게 바로
 * 고치려던 그 거짓말이다 — 값이 늦게 오는 쪽이 틀린 라벨을 붙이는 쪽보다 낫다.
 */
export function isLiveSession(value: string | undefined): boolean {
  const session = parseMarketSession(value);
  return session != null && session !== "closed";
}

/** 헤더 배지에 적을 말. 세션을 모르면 아무것도 적지 않는다(null). */
export function marketSessionLabel(value: string | undefined): string | null {
  const session = parseMarketSession(value);
  return session ? SESSION_LABELS[session] : null;
}

/**
 * 「이 값이 언제 것인가」를 KST로 찍는다.
 *
 * 초까지 적는 이유는 장중에 1초마다 움직이는 값이기 때문이고, `withDate`를 켜는
 * 자리는 **마감 시각**이다 — 자정을 넘겨서 보는 값이라 「오후 08:00:00」만으로는
 * 오늘 저녁인지 어제 저녁인지 알 수 없다.
 *
 * ko-KR 날짜는 「09. 03.」로 나온다. 그대로 두면 좁은 화면에서 각주 한 줄이 밀려
 * 내려가므로 점 뒤 공백과 끝점을 걷어 「09.03」으로 붙인다.
 *
 * 깨진 값은 빈 문자열이다 — 화면에 `Invalid Date`를 내보내지 않는다.
 */
export function formatSessionStamp(at: string | undefined, withDate = false): string {
  if (!at) return "";
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return "";
  const time = date.toLocaleTimeString("ko-KR", {
    timeZone: "Asia/Seoul",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
  if (!withDate) return time;
  const day = date.toLocaleDateString("ko-KR", {
    timeZone: "Asia/Seoul",
    month: "2-digit",
    day: "2-digit",
  });
  return `${day.replace(/\.\s*$/, "").replace(/\.\s+/g, ".")} ${time}`;
}
