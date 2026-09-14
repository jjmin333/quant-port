import { Suspense } from "react";
import { EarningsClient } from "@/components/earnings/earnings-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "해외 실적 발표 일정",
  description:
    "미국 상장사의 실적 발표 일정과 EPS·매출 추정치를 봅니다. 장전·장후 발표 시각까지 함께 표시되어 어느 종목이 언제 발표하는지 한눈에 들어옵니다.",
  path: "/earnings/us",
});

export default function UsEarningsPage() {
  return (
    <Suspense>
      <EarningsClient market="us" />
    </Suspense>
  );
}
