import type { Metadata } from "next";
import { NOINDEX } from "@/lib/seo";

/** 개발용 시안·타이포 갤러리. 실서비스 화면이 아니므로 색인하지 않는다. */
export const metadata: Metadata = { title: "개발용", robots: NOINDEX };

export default function DevLayout({ children }: { children: React.ReactNode }) {
  return children;
}
