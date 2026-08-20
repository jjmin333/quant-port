import { Suspense } from "react";
import { EarningsClient } from "@/components/earnings/earnings-client";

export default function UsEarningsPage() {
  return (
    <Suspense>
      <EarningsClient market="us" />
    </Suspense>
  );
}
