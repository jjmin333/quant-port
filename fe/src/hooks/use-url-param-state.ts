"use client";

import { useCallback, useMemo } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

/**
 * 화면의 필터·정렬·탭 상태를 **주소에 싣는다** (§C-2).
 *
 * 왜 필요한가: 이 상태들은 전부 컴포넌트 안에만 있었다. 그래서
 *
 *  - 「내 리포트」로 옮긴 뒤 새로고침하면 「전체」로 돌아갔고,
 *  - 상세를 보고 뒤로 오면 걸어 둔 필터가 풀려 있었고,
 *  - 「매도 의견만 최신순」 같은 화면을 남에게 보내거나 북마크할 수 없었다.
 *
 * 같은 앱의 `/market`은 이미 `?market=&type=&symbol=`로 제대로 하고 있었다 — 규칙이
 * 화면마다 갈리는 것이 문제였다. 이 훅이 그 규칙을 한곳에 담는다.
 *
 * **`replace`를 쓰고 `push`를 쓰지 않는다.** 필터를 누를 때마다 히스토리가 쌓이면
 * 목록을 떠나는 데 뒤로가기를 여섯 번 눌러야 한다. 반면 상세로 가는 이동은 `push`라
 * 뒤로가기 한 번이면 **필터가 걸린 그 주소로** 정확히 돌아온다. 필요한 것은 그것이다.
 *
 * 기본값은 주소에서 지운다 — `?direction=all&sort=recent` 처럼 아무 의미 없는
 * 파라미터가 붙은 링크를 공유하게 되면 주소가 상태를 설명하지 못한다.
 */
export function useUrlParamState<T extends string>(
  key: string,
  fallback: T,
  isValid: (raw: string) => raw is T,
): [T, (next: T) => void] {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const raw = searchParams.get(key);
  const value = raw !== null && isValid(raw) ? raw : fallback;

  const setValue = useCallback(
    (next: T) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next === fallback) params.delete(key);
      else params.set(key, next);
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [key, fallback, pathname, router, searchParams],
  );

  return useMemo(() => [value, setValue], [value, setValue]);
}

/** 켜짐/꺼짐 하나짜리. 꺼짐이 기본이라 켜졌을 때만 주소에 남는다. */
export function useUrlFlagState(key: string): [boolean, (next: boolean) => void] {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  const value = searchParams.get(key) === "1";

  const setValue = useCallback(
    (next: boolean) => {
      const params = new URLSearchParams(searchParams.toString());
      if (next) params.set(key, "1");
      else params.delete(key);
      const qs = params.toString();
      router.replace(qs ? `${pathname}?${qs}` : pathname, { scroll: false });
    },
    [key, pathname, router, searchParams],
  );

  return useMemo(() => [value, setValue], [value, setValue]);
}
