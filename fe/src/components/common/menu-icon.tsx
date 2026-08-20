import { Briefcase, Pencil, Stamp, Star, Target } from "lucide-react";
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
      return <Star className={className} />;
    case "stamps":
      return <Stamp className={className} />;
    case "journal":
      return <Pencil className={className} />;
    case "targets":
      return <Target className={className} />;
  }
}
