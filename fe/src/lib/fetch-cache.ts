/**
 * 브라우저 안에서만 사는 짧은 GET 캐시.
 *
 * 이 앱의 화면은 카드 여러 장이고, 카드마다 자기 `useEffect`에서 자기 fetch를 한다.
 * 그 구조 자체는 옳다 — 한 카드가 죽어도 나머지가 뜬다. 다만 두 가지가 딸려 왔다.
 *
 * 1. **같은 값을 동시에 두 번 부른다.** 지수 스트립과 뉴스 카드의 지수 칩이 각자
 *    `/quotes/indices`를 부르는 식이다. 화면에는 한 벌만 필요한데 회선에는 두 벌이 간다.
 * 2. **화면을 떠났다 돌아오면 전부 다시 부른다.** 뒤로가기 한 번에 열몇 개가 다시
 *    나가고, 그동안 화면은 스켈레톤이다 — 3초 전에 받은 값이 그대로 맞는데도.
 *
 * 그래서 두 가지만 한다: **진행 중인 같은 요청은 하나로 합치고**, 방금 받은 응답은
 * 짧은 시간 다시 쓴다. 그 이상(백그라운드 갱신·포커스 재검증·무효화 그래프)은 이
 * 파일이 할 일이 아니다 — 필요해지면 그때 라이브러리를 들이는 편이 낫다.
 *
 * ⚠️ **읽기 전용 GET에만 쓴다.** 쓰기 요청이나 사용자마다 답이 달라야 하는 값에
 * 쓰면 남의 화면을 보게 된다. 캐시 키에 사용자 정보가 없기 때문이다 — 로그아웃
 * 시에는 `clearFetchCache()`로 통째로 버린다.
 */

type Entry<T> = {
  at: number;
  value: T;
};

const cache = new Map<string, Entry<unknown>>();
const inflight = new Map<string, Promise<unknown>>();

/** 이보다 오래된 응답은 없는 셈 친다. */
export const DEFAULT_CACHE_TTL_MS = 30_000;

/**
 * 캐시가 살아 있으면 그 값을, 아니면 `loader()`를 부른다. 같은 키로 동시에 들어온
 * 호출은 **하나의 요청을 함께 기다린다**.
 *
 * 실패는 캐시하지 않는다 — 한 번 실패한 화면이 TTL 동안 실패한 채로 굳으면,
 * 사용자가 할 수 있는 일이 기다리는 것뿐이 된다.
 */
export function cachedGet<T>(
  key: string,
  loader: () => Promise<T>,
  ttlMs: number = DEFAULT_CACHE_TTL_MS,
): Promise<T> {
  const hit = cache.get(key) as Entry<T> | undefined;
  if (hit && Date.now() - hit.at < ttlMs) {
    return Promise.resolve(hit.value);
  }

  const pending = inflight.get(key) as Promise<T> | undefined;
  if (pending) return pending;

  const request = loader()
    .then((value) => {
      cache.set(key, { at: Date.now(), value });
      return value;
    })
    .finally(() => {
      inflight.delete(key);
    });

  inflight.set(key, request);
  return request;
}

/**
 * 특정 키(또는 접두사로 시작하는 키 전부)를 버린다. 쓰기 직후 그 목록을 다시
 * 읽어야 할 때 쓴다 — 관심종목을 담고 목록으로 돌아왔는데 30초 전 목록이 뜨면
 * 방금 한 일이 사라진 것처럼 보인다.
 */
export function invalidateFetchCache(prefix: string): void {
  for (const key of cache.keys()) {
    if (key.startsWith(prefix)) {
      cache.delete(key);
    }
  }
}

/** 로그인·로그아웃처럼 "누구인지"가 바뀌었을 때. */
export function clearFetchCache(): void {
  cache.clear();
}
