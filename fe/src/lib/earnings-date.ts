/**
 * 실적·공시 일정의 날짜 하나를 읽는 법.
 *
 * **출처마다 형식이 다르다** (실측 08-16):
 *  - DART(국내): `2026-08-14` — ISO, 이미 **제출된** 공시(`kind: "filed"`)
 *  - Finnhub(미국): `8/26` — **연도가 없는** M/D, 앞으로의 일정(`kind: "upcoming"`)
 *
 * 그래서 지금까지 화면들이 `date.slice(5)`로 잘라 쓰고 있었는데, 그러면 미국 일정은
 * `8/26` → **`6`** 이라는 글자가 뜬다. 자를 게 아니라 읽어야 한다.
 *
 * 연도가 없는 값은 **올해로 읽되**, 그 결과가 반년 이상 과거면 내년으로 넘긴다 —
 * 실적 캘린더는 늘 가까운 미래를 말하므로, 12월에 받은 `1/28`은 내년 1월이다.
 */

const ISO = /^(\d{4})-(\d{2})-(\d{2})$/;
const MONTH_DAY = /^(\d{1,2})\/(\d{1,2})$/;

const DAY_MS = 86_400_000;

/** 오늘 자정(KST)의 타임스탬프. */
function todayKstMs(): number {
  const ymd = new Date().toLocaleDateString("en-CA", { timeZone: "Asia/Seoul" });
  return new Date(`${ymd}T00:00:00+09:00`).getTime();
}

/** 날짜 문자열 → KST 자정 타임스탬프. 읽을 수 없으면 null. */
export function parseEarningsDateMs(raw: string): number | null {
  const text = raw.trim();

  const iso = ISO.exec(text);
  if (iso) {
    const at = new Date(`${text}T00:00:00+09:00`).getTime();
    return Number.isFinite(at) ? at : null;
  }

  const md = MONTH_DAY.exec(text);
  if (md) {
    const month = Number(md[1]);
    const day = Number(md[2]);
    if (month < 1 || month > 12 || day < 1 || day > 31) return null;
    const year = new Date().getFullYear();
    const pad = (n: number) => String(n).padStart(2, "0");
    let at = new Date(`${year}-${pad(month)}-${pad(day)}T00:00:00+09:00`).getTime();
    if (!Number.isFinite(at)) return null;
    // 반년 넘게 지난 값이면 해가 바뀐 것으로 본다.
    if (at < todayKstMs() - 183 * DAY_MS) {
      at = new Date(`${year + 1}-${pad(month)}-${pad(day)}T00:00:00+09:00`).getTime();
    }
    return Number.isFinite(at) ? at : null;
  }

  return null;
}

/** 오늘로부터 며칠 뒤(음수면 과거). 읽을 수 없으면 null. */
export function earningsDaysFromToday(raw: string): number | null {
  const at = parseEarningsDateMs(raw);
  if (at == null) return null;
  return Math.round((at - todayKstMs()) / DAY_MS);
}

/**
 * 목록 왼쪽 칸에 들어가는 짧은 표기.
 *
 * 가까운 날은 `D-3`·`오늘`처럼 **거리**로, 먼 날은 `8/26`처럼 **날짜**로 적는다 —
 * `D-214`는 세어 보게 만들지만 날짜는 바로 읽힌다. 지난 일은 `3일 전`이다(국내 공시는
 * 전부 이미 제출된 것이라 이 표기를 쓴다).
 */
export function formatEarningsDateLabel(raw: string): string {
  const days = earningsDaysFromToday(raw);
  if (days == null) return raw.trim();
  if (days === 0) return "오늘";
  if (days > 0) return days <= 60 ? `D-${days}` : shortDate(raw);
  if (days >= -30) return `${-days}일 전`;
  return shortDate(raw);
}

function shortDate(raw: string): string {
  const at = parseEarningsDateMs(raw);
  if (at == null) return raw.trim();
  const d = new Date(at);
  return `${d.getMonth() + 1}/${d.getDate()}`;
}
