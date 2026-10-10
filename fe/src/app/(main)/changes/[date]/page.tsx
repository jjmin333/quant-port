import { Suspense } from "react";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { ChangesClient } from "@/components/changes/changes-client";
import { CHANGE_DATE_RE, ChangesSeoList, fetchChangesServer } from "@/components/changes/changes-seo-list";
import { OG_BASE, TWITTER_BASE } from "@/lib/seo";

/*
 * 날짜별 「변화」 보관 주소 (2026-10-10, BUSINESS_REVIEW §4.2). 화면은 `/changes`와 같고, 그날로 열린다.
 * 서버가 그 날의 목록을 그리므로 크롤러가 읽을 본문과 종목 링크가 있다. 보관 목록에 없는 날짜는 404.
 */
type Props = { params: Promise<{ date: string }> };

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { date } = await params;
  // 보관 목록에 없는 날짜는 본문이 404 화면이다 — 상태코드는 셸이 먼저 나가 200이라(next.config 주석) 색인만 막는다.
  const known = CHANGE_DATE_RE.test(date) ? await fetchChangesServer("domestic", date) : null;
  if (!known?.dates?.includes(date)) return { title: "찾을 수 없는 날짜", robots: { index: false, follow: true } };
  const title = `${date} 주식 변화 — 급등락·공시·실적·점수`;
  const description = `${date} 국내·미국 종목의 급등락, 공시, 실적 발표, AI 점수 급변을 시가총액 순으로 모았습니다.`;
  const path = `/changes/${date}`;
  return {
    title,
    description,
    alternates: { canonical: path },
    openGraph: { ...OG_BASE, title, description, url: path },
    twitter: { ...TWITTER_BASE, title, description },
  };
}

export default async function ChangesDatePage({ params }: Props) {
  const { date } = await params;
  if (!CHANGE_DATE_RE.test(date)) notFound();
  const known = await fetchChangesServer("domestic", date);
  if (!known?.dates?.includes(date)) notFound();
  return (
    <>
      <ChangesSeoList date={date} />
      <Suspense
        fallback={
          <main className="flex flex-1 items-center justify-center p-8 text-sm text-[var(--text-muted)]">
            변화를 불러오는 중…
          </main>
        }
      >
        <ChangesClient initialDate={date} />
      </Suspense>
    </>
  );
}
