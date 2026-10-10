import { Suspense } from "react";
import { ResearchClient } from "@/components/research/research-client";
import { fetchResearchCallsServer } from "@/lib/research-api.server";
import { resolvePageMetadata } from "@/lib/page-meta.server";
import { breadcrumbJsonLd } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";
import { PageHeading } from "@/components/layout/page-heading";

// 사용자에게 보이는 이름은 "리포트 / 투자 리포트"지만, 코드·API·테이블은 BE의
// `research_*` 명칭을 그대로 쓴다 (엔드포인트는 `/api/v1/research/*`).

/*
 * 제목·설명은 `lib/page-meta-routes`의 기본값이고, 관리자가 `/admin/seo`에서
 * 덮어쓸 수 있다 (2026-09-15 사용자 요청 2). `const metadata`가 아니라 함수인
 * 이유는 그 덮어쓰기를 **렌더 시점에** 읽어야 하기 때문이다.
 */
export function generateMetadata() {
  return resolvePageMetadata("/reports");
}

export default async function ReportsPage() {
  /*
   * **크롤러의 유일한 발견 경로다.** 목록을 클라이언트에서만 그리던 동안에는
   * 첫 HTML에 리포트 링크가 하나도 없어서, 사이트맵에 없는 `/reports/<id>`를
   * 크롤러가 찾아갈 방법이 아예 없었다. 이제 서버가 제목과 링크를 먼저 찍는다.
   */
  const initialCalls = await fetchResearchCallsServer();

  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "투자 리포트", path: "/reports" }])} />
      <PageHeading>투자 리포트</PageHeading>
      <Suspense>
        <ResearchClient initialCalls={initialCalls} />
      </Suspense>
    </>
  );
}
