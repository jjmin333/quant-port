import { Suspense } from "react";
import { ResearchCompose } from "@/components/research/research-compose";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "리포트 작성",
  description: "새 투자 리포트를 작성합니다.",
  noindex: true,
});

export default function ReportComposePage() {
  return (
    <Suspense>
      <ResearchCompose />
    </Suspense>
  );
}
