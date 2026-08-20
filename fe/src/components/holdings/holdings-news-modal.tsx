"use client";

import { SymbolNewsModal } from "@/components/stocks/symbol-news-modal";

/**
 * 보유 종목 관련 뉴스 모달.
 *
 * 08-14: 관심종목 화면이 같은 모달을 필요로 하면서 본체를 `SymbolNewsModal`로 옮겼다.
 * 여기 남은 건 보유 화면의 문구뿐이다 — 호출부 이름을 바꾸지 않기 위해 얇게 둔다.
 */
export function HoldingsNewsModal({
  open,
  onOpenChange,
  symbols,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  symbols: Array<{ symbol: string; name: string; market: "domestic" | "us" }>;
}) {
  return (
    <SymbolNewsModal
      open={open}
      onOpenChange={onOpenChange}
      symbols={symbols}
      title="보유 종목 뉴스"
      subjectLabel="비중 상위"
    />
  );
}
