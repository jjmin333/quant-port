"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/providers/auth-provider";
import { fetchMyChangeSummary, type MyChangeSummary } from "@/lib/changes-api";
import { SCREEN_SLOW_REFRESH_MS } from "@/lib/screen-refresh";
import { cachedGet } from "@/lib/fetch-cache";

/*
  사이드바는 **두 벌**이 동시에 그려진다(데스크톱 레일 + 폭 0인 모바일 셸). 둘이 이 훅을
  각자 부르면 화면마다 같은 요청이 두 번, 타이머도 둘이었다(2026-09-26 네트워크 실측 —
  페이지를 열 때마다 `/user/changes/summary` 2회). 요청을 사용자별 키로 묶어 **하나를
  나눠 쓴다.** 30초는 두 타이머가 몇 초 어긋나도 뒤쪽이 캐시를 받게 하는 여유다.
*/
const SHARED_TTL_MS = 30_000;

/**
 * 「내 종목 변화」 — 사이드바 배지와 홈 한 줄 띠가 같은 값을 쓴다.
 *
 * 변화는 하루 한 번(00:25 점수 배치 끝) 만들어지므로 느린 주기로 충분하다. 로그인하지
 * 않았으면 부르지 않는다(401을 만들 이유가 없다). 실패는 삼킨다 — 배지 하나 때문에
 * 화면이 시끄러울 이유가 없고, 값이 없으면 그리지 않는다.
 */
export function useMyChangeSummary(): MyChangeSummary | null {
  const { session } = useAuth();
  const [data, setData] = useState<MyChangeSummary | null>(null);
  const loggedIn = Boolean(session);
  const userId = session?.user.id ?? 0;

  useEffect(() => {
    if (!loggedIn) return;
    let cancelled = false;
    const load = async () => {
      try {
        const res = await cachedGet(
          `my-change-summary:${userId}`,
          fetchMyChangeSummary,
          SHARED_TTL_MS,
        );
        if (!cancelled) setData(res);
      } catch {
        if (!cancelled) setData(null);
      }
    };
    void load();
    const timer = setInterval(() => void load(), SCREEN_SLOW_REFRESH_MS);
    return () => {
      cancelled = true;
      clearInterval(timer);
    };
  }, [loggedIn, userId]);

  return loggedIn ? data : null;
}
