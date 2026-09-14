import { Briefcase, Crosshair, Heart, NotepadText, Stamp, Target } from "lucide-react";
import type { MenuKey } from "@/lib/menu-copy";

/**
 * 「내 투자」 메뉴의 아이콘 — 빈 화면(로그인 전·후 양쪽)이 같은 그림을 쓴다.
 *
 * 화면마다 각자 고르게 두면 로그인 게이트에서 본 아이콘과 로그인 뒤 빈 화면의
 * 아이콘이 갈라진다. 같은 메뉴를 두 번 처음 만나는 셈이 되므로 한곳에 모은다.
 */
export function menuIcon(menu: MenuKey, className = "size-5") {
  switch (menu) {
    case "holdings":
      return <Briefcase className={className} />;
    case "watchlist":
      return <Heart className={className} />;
    case "stamps":
      return <Stamp className={className} />;
    case "journal":
      return <NotepadText className={className} />;
    case "targets":
      return <Target className={className} />;
    // 통합 메뉴. 사이드바와 같은 아이콘을 쓴다 — 로그인 게이트에서 본 그림과
    // 로그인 뒤 빈 화면의 그림이 갈라지면 같은 메뉴를 두 번 처음 만나는 셈이 된다.
    case "predictions":
      return <Crosshair className={className} />;
  }
}
