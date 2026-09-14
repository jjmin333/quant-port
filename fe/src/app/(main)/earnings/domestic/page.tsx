import { Suspense } from "react";
import { EarningsClient } from "@/components/earnings/earnings-client";
import { pageMetadata } from "@/lib/seo";

export const metadata = pageMetadata({
  title: "국내 실적 발표 일정",
  description:
    "국내 상장사의 잠정실적 발표 일정과 컨센서스·발표치를 한 화면에서 봅니다. 어느 종목이 언제 발표하고 시장 예상을 넘겼는지 날짜순으로 정리됩니다.",
  path: "/earnings/domestic",
});

export default function DomesticEarningsPage() {
  return (
    <Suspense>
      <EarningsClient market="domestic" />
    </Suspense>
  );
}
