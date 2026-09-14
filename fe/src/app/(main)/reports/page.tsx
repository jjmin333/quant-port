import { Suspense } from "react";
import { ResearchClient } from "@/components/research/research-client";
import { fetchResearchCallsServer } from "@/lib/research-api.server";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";

// 사용자에게 보이는 이름은 "리포트 / 투자 리포트"지만, 코드·API·테이블은 BE의
// `research_*` 명칭을 그대로 쓴다 (엔드포인트는 `/api/v1/research/*`).

export const metadata = pageMetadata({
  title: "투자 리포트",
  description:
    "다른 투자자가 공개한 종목 리포트를 읽습니다. 근거와 목표가를 적어 발행한 글이라, 결론만이 아니라 어떤 판단으로 그 결론에 왔는지 볼 수 있습니다.",
  path: "/reports",
});

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
      <Suspense>
        <ResearchClient initialCalls={initialCalls} />
      </Suspense>
    </>
  );
}
