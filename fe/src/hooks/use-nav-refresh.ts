"use client";

import { useEffect } from "react";
import { usePathname, useRouter } from "next/navigation";

export const NAV_REFRESH_EVENT = "quant-port:nav-refresh";

/** 활성 사이드바 메뉴를 다시 클릭했을 때 페이지 데이터를 새로고침한다. */
export function useNavRefresh(onRefresh: () => void) {
  const pathname = usePathname();
  const router = useRouter();

  useEffect(() => {
    const handler = (event: Event) => {
      const href = (event as CustomEvent<{ href?: string }>).detail?.href;
      if (!href) return;
      if (pathname !== href && !pathname.startsWith(`${href}/`)) return;
      router.refresh();
      onRefresh();
    };
    window.addEventListener(NAV_REFRESH_EVENT, handler);
    return () => window.removeEventListener(NAV_REFRESH_EVENT, handler);
  }, [pathname, router, onRefresh]);
}

export function dispatchNavRefresh(href: string) {
  window.dispatchEvent(new CustomEvent(NAV_REFRESH_EVENT, { detail: { href } }));
}
