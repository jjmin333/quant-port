import type { Metadata } from "next";
import { NOINDEX } from "@/lib/seo";

/**
 * 관리자 화면 전체를 색인에서 뺀다.
 *
 * 레이아웃에 한 번 두면 `/admin` 아래 새 화면이 생겨도 자동으로 따라온다 —
 * 화면마다 적게 두면 다음에 추가되는 관리 화면이 조용히 색인된다.
 * `robots.txt`의 Disallow와 **둘 다** 건다: robots는 크롤링만 막고, 다른 데서
 * 링크된 주소는 크롤링 없이도 색인될 수 있다.
 */
export const metadata: Metadata = { title: "관리", robots: NOINDEX };

export default function AdminLayout({ children }: { children: React.ReactNode }) {
  return children;
}
