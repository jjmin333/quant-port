import { Suspense } from "react";
import { notFound } from "next/navigation";
import type { Metadata } from "next";
import { ResearchDetailClient } from "@/components/research/research-detail-client";
import { fetchResearchCallServer } from "@/lib/research-api.server";
import { articleJsonLd, breadcrumbJsonLd, NOINDEX, pageMetadata } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";
import { markdownExcerpt } from "@/lib/markdown";

type Props = { params: Promise<{ id: string }> };

function parseId(raw: string): number | null {
  const id = Number(raw);
  return Number.isInteger(id) && id > 0 ? id : null;
}

/**
 * 리포트마다 다른 제목·설명을 붙인다.
 *
 * 예전에는 모든 리포트가 루트의 브랜드 제목 하나를 공유했다 — 검색 결과에서
 * 서로 구분되지 않고, 구글이 보기엔 제목이 같은 문서 더미다.
 *
 * **`visibility`가 `public`이 아니거나 철회·숨김이면 색인하지 않는다.**
 * 「링크 공개(unlisted)」는 링크를 아는 사람만 보라는 뜻이라 검색에 올리면
 * 작성자가 고른 설정을 우리가 뒤집는 것이 된다.
 */
export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const id = parseId((await params).id);
  if (id === null) return { robots: NOINDEX };

  const call = await fetchResearchCallServer(id);
  if (!call) return { title: "리포트", robots: NOINDEX };

  const indexable = call.visibility === "public" && !call.retractedAt && !call.hiddenAt;
  const symbol = call.symbolName ?? call.symbol;
  const description =
    markdownExcerpt(call.bodyMd, 155) ||
    `${symbol}에 대한 투자 리포트입니다. 목표가와 근거, 이후 성과가 함께 기록됩니다.`;

  return {
    ...pageMetadata({
      title: call.title,
      description,
      path: `/reports/${id}`,
      noindex: !indexable,
    }),
    authors: call.authorName ? [{ name: call.authorName }] : undefined,
  };
}

export default async function ReportDetailPage({ params }: Props) {
  const id = parseId((await params).id);
  if (id === null) notFound();

  /*
   * 서버가 먼저 받아 첫 HTML에 본문을 싣는다. 실패하면 `null`이고, 그때는
   * 예전처럼 클라이언트가 받아 온다 — SEO용 선반입이 화면을 막지 않는다.
   */
  const call = await fetchResearchCallServer(id);
  const indexable =
    call !== null && call.visibility === "public" && !call.retractedAt && !call.hiddenAt;

  return (
    <>
      {/*
        구조화 데이터는 색인 대상일 때만 낸다 — 비공개 글의 제목·작성자를
        기계가 읽는 형식으로 굳이 내보낼 이유가 없다.
      */}
      {indexable && call && (
        <>
          <JsonLd data={articleJsonLd(call, `/reports/${id}`)} />
          <JsonLd
            data={breadcrumbJsonLd([
              { name: "투자 리포트", path: "/reports" },
              { name: call.title, path: `/reports/${id}` },
            ])}
          />
        </>
      )}
      <Suspense>
        <ResearchDetailClient id={id} initialCall={call} />
      </Suspense>
    </>
  );
}
