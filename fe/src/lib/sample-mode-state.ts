/**
 * 예시 데이터 모드가 **켜졌는지 정하는 규칙** — 화면도 브라우저도 모르는 순수 함수.
 *
 * # 왜 컴포넌트에서 떼어 냈나 (2026-09-14 사용자 요청 4)
 *
 * 이 규칙은 `components/common/sample-mode.tsx` 안에 모듈 전역 변수와 뒤엉켜 있었다.
 * 그래서 시험이 닿지 않았고, 실제로 **끌 때만 고장나는** 버그가 두 번 그 안에서
 * 나왔다(2026-09-02의 「OR로 합치기」, 그리고 자동 켜기를 넣으면서 생길 뻔한
 * 「꺼도 도로 켜짐」). 켜는 경로와 끄는 경로는 대칭이 아니라서 눈으로는 안 갈린다.
 *
 * 상태를 들고 있는 것은 여전히 그 파일이다. 여기 있는 것은 **무엇이 무엇을 이기는가**
 * 뿐이고, 그 층이 셋이다.
 *
 *   1. `override` — 사용자가 직접 누른 값. 언제나 이긴다.
 *   2. `fromUrl`  — 주소창 `?sample=1`. 링크로 온 사람.
 *   3. `auto`     — 화면이 「보여 줄 내 것이 아직 없다」고 알려 온 경우.
 *
 * 1이 3보다 위인 것이 요점이다. 예시를 끈 사람에게 「아직 데이터가 없으니까」라며
 * 다시 켜 주면 그 버튼은 눌러도 안 되는 버튼이 된다.
 *
 * 경로(`pathname`)를 함께 보는 이유: 2·3은 모듈 전역이라 기록해 두지 않으면
 * `/holdings`에서 켠 예시가 링크로 이동한 `/watchlist`까지 따라간다.
 */

/** 경로에 매인 참·거짓. 다른 화면으로 떠나면 무효가 된다. */
export type PathScopedFlag = { pathname: string; value: boolean } | null;

export type SampleInputs = {
  /** 지금 보고 있는 화면. */
  pathname: string;
  /** 주소창에 `?sample=1`이 있나. */
  fromUrl: boolean;
  /** 사용자가 직접 누른 값. */
  override: PathScopedFlag;
  /** 화면이 알려 온 「내 것이 아직 없다」. */
  auto: PathScopedFlag;
};

function matches(flag: PathScopedFlag, pathname: string): boolean {
  return flag !== null && flag.pathname === pathname && flag.value;
}

/**
 * 「내 것이 아직 없다」에 답하는 **기여자가 여럿**일 때 하나로 합친다 — 하나라도
 * 그렇다고 하면 그렇다.
 *
 * # 왜 마지막 사람이 이기면 안 되나 (2026-09-14, 실측으로 잡은 버그)
 *
 * 한 화면에서 이 질문에 답하는 곳이 둘이다.
 *
 *   · `AuthRequiredShell` — 「로그인을 안 했다」. 로그인 안 한 사람에게 내 것이
 *     있을 리 없으므로 **곧바로** 참이다.
 *   · 본문 클라이언트 — 「목록을 받아 보니 0건이다」. 받아 보기 **전에는 거짓**이다
 *     (아직 모르므로).
 *
 * 값을 한 칸에 덮어쓰면 이렇게 된다: 껍데기가 참을 넣어 예시가 켜지고 → 그 덕에
 * 본문 클라이언트가 처음 마운트되어 → 아직 목록을 못 받았으니 거짓을 넣고 →
 * 예시가 꺼지고 → 본문이 사라진다. 실측 로그가 정확히 그 순서였다(참 → 거짓 →
 * 거짓). 화면에는 「예시로 둘러보기」를 묻는 카드가 그대로 남았다.
 *
 * 기여자마다 한 표씩 들고 OR로 합치면 그 경합이 없어진다 — 「모른다」(거짓)가
 * 「없는 걸 안다」(참)를 덮지 못한다.
 */
export function anyAutoVote(votes: Iterable<PathScopedFlag>, pathname: string): boolean {
  for (const vote of votes) {
    if (matches(vote, pathname)) return true;
  }
  return false;
}

/**
 * 사용자가 고른 값을 **뺀** 나머지 — 주소창과 자동 켜기.
 *
 * 이 값이 따로 필요한 이유는 `shouldDropOverride`에 있다. 「사용자가 고른 값이 이제
 * 필요 없는가」는 **밑바닥이 이미 같은 말을 하는가**로 판정되는데, 밑바닥이 무엇인지
 * 한 군데서만 정해져야 켜기와 끄기가 같은 기준을 쓴다.
 */
export function sampleBase({ pathname, fromUrl, auto }: Omit<SampleInputs, "override">): boolean {
  return fromUrl || matches(auto, pathname);
}

/** 지금 예시 모드인가. */
export function resolveSample({ pathname, fromUrl, override, auto }: SampleInputs): boolean {
  if (override !== null && override.pathname === pathname) return override.value;
  return sampleBase({ pathname, fromUrl, auto });
}

/**
 * 사용자가 고른 값을 버려도 되나.
 *
 * override는 **주소창이 따라올 때까지만** 살아 있어야 한다. 남겨 두면 나중에 같은
 * 경로로 push 이동해 되돌아왔을 때(`/holdings?sample=1` 링크 등) 죽은 값이 주소창을
 * 이긴다 — push는 popstate를 일으키지 않아 그쪽 정리에 안 걸린다.
 *
 * 다만 비교 대상은 주소창**만**이 아니다. 자동 켜기가 생기면서 그 차이가 버그가
 * 된다 — 예시를 끄면 override는 `false`, 주소창에는 파라미터가 없으니 「같다」로
 * 판정되어 override가 버려지고, 그 순간 자동 켜기가 다시 이겨 **예시가 도로 켜진다.**
 */
export function shouldDropOverride({
  pathname,
  fromUrl,
  override,
  auto,
}: SampleInputs): boolean {
  if (override === null) return false;
  // 다른 화면으로 떠났다 — 이 값은 그 화면의 것이었다.
  if (override.pathname !== pathname) return true;
  return override.value === sampleBase({ pathname, fromUrl, auto });
}

/**
 * 「내 것이 아직 없다」 표를 **언제 고쳐 쓰는가** (2026-09-27 사용자 제보 1).
 *
 * 예전에는 화면이 매 렌더 `!loading && count === 0`을 그대로 표로 냈다. 그런데
 * 예시가 켜지면 화면이 **예시 목록**을 세고(0건 아님) 로딩도 처음부터 다시 돈다 —
 * 표가 거짓으로 뒤집혀 예시가 꺼지고, 꺼지면 다시 내 목록 0건을 받아 켜진다.
 * 목표가 예측·메모장이 그렇게 무한히 재적재하다 429를 맞았다(실측).
 *
 * 그래서 표는 **내 데이터를 실제로 본 순간에만** 고친다.
 *  - `observed === null` — 아직 받는 중이라 모른다 → 직전 표 유지.
 *  - `showingSample` — 지금 화면의 숫자는 예시다, 내 것에 대해 아무것도 말하지 않는다 → 직전 표 유지.
 */
export function nextAutoVote(
  prev: boolean,
  observed: boolean | null,
  showingSample: boolean,
): boolean {
  if (showingSample || observed === null) return prev;
  return observed;
}
