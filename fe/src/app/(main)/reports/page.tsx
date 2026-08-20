import { Suspense } from "react";
import { ResearchClient } from "@/components/research/research-client";

// 사용자에게 보이는 이름은 "리포트 / 투자 리포트"지만, 코드·API·테이블은 BE의
// `research_*` 명칭을 그대로 쓴다 (엔드포인트는 `/api/v1/research/*`).
export default function ReportsPage() {
  return (
    <Suspense>
      <ResearchClient />
    </Suspense>
  );
}
