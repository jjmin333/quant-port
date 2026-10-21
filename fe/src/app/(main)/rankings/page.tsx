import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { fetchDailyTopServer } from "@/lib/daily-top.server";
import DailyTopPage, { generateMetadata as dateMetadata } from "./[date]/page";

/*
`/rankings` = 최신 보관일 화면. 처음엔 `redirect()`로 `/rankings/YYYY-MM-DD`에 보냈는데, 스트리밍 중
리다이렉트가 Suspense 경계를 못 끝내 React #419(클라이언트 렌더로 전환)가 났다(10-21 배포 직후
`client_errors` 집계). 같은 화면을 그대로 그리고 canonical은 날짜 주소를 가리킨다.
*/
export async function generateMetadata(): Promise<Metadata> {
  const latest = await fetchDailyTopServer();
  if (!latest) return { title: "AI 점수 상위 종목" };
  return dateMetadata({ params: Promise.resolve({ date: latest.date }) });
}

export default async function DailyTopLatestPage() {
  const latest = await fetchDailyTopServer();
  if (!latest) notFound();
  return <DailyTopPage params={Promise.resolve({ date: latest.date })} />;
}
