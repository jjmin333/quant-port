import { redirect } from "next/navigation";

/**
 * 실적 메뉴는 국내·해외로 갈렸다(뉴스와 같은 결). 이 경로는 예전 링크와
 * 북마크를 위해 남겨 두고 국내로 보낸다.
 */
export default function EarningsIndexPage() {
  redirect("/earnings/domestic");
}
