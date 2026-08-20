import { Suspense } from "react";
import { EarningsClient } from "@/components/earnings/earnings-client";

export default function DomesticEarningsPage() {
  return (
    <Suspense>
      <EarningsClient market="domestic" />
    </Suspense>
  );
}
