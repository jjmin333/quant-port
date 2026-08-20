import { redirect } from "next/navigation";

/** 예전 `/calendar`는 종목 하나의 실적 탭으로 보냈다. 이제 독립 화면이 있다. */
export default function CalendarRedirectPage() {
  redirect("/earnings/domestic");
}
