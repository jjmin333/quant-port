"use client";

import { useEffect, useState } from "react";
import { Loader2, UserRound } from "lucide-react";
import { Card } from "@/components/ui/card";
import { ProvenanceStamp } from "@/components/ui/provenance-stamp";
import { sourceLabel } from "@/lib/data-reliability";
import { fetchInsiderClient } from "@/lib/insider-api";
import type { InsiderTabData, InsiderTransaction } from "@/lib/insider-types";
import { formatPercent, formatPrice } from "@/lib/format";
import type { QuoteCurrency } from "@/lib/quotes-types";
import { toUserMessage } from "@/lib/error-message";

type InsiderTabProps = {
  symbol: string;
  currency: QuoteCurrency;
};

export function InsiderTab({ symbol, currency }: InsiderTabProps) {
  const [data, setData] = useState<InsiderTabData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    setError(null);
    fetchInsiderClient(symbol)
      .then((res) => {
        if (!cancelled) setData(res);
      })
      .catch((e) => {
        if (!cancelled) setError(toUserMessage(e, "내부자 거래 정보 조회 실패"));
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [symbol]);

  if (loading) {
    return (
      <Card className="flex min-h-[160px] items-center justify-center p-6 text-cell text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        내부자 거래 정보 불러오는 중…
      </Card>
    );
  }
  if (error) {
    return <Card className="p-4 text-cell text-[var(--loss)]">{error}</Card>;
  }

  const isDomestic = data?.assetType === "domestic";
  const transactions = data?.transactions ?? [];

  return (
    <div className="space-y-2.5">
      <Card className="p-3">
        <h3 className="mb-2 flex flex-wrap items-center gap-x-1.5 gap-y-1 text-cell font-semibold text-[var(--text-primary)]">
          <UserRound className="h-3.5 w-3.5 text-[var(--text-muted)]" />
          {isDomestic ? "임원ㆍ주요주주 소유 변동" : "내부자 거래 (Form 4)"}
          {/*
            출처는 이미 응답에 실려 왔는데 화면이 안 쓰고 있었다 (요구 8). 국내는
            DART, 해외는 EDGAR라 제목으로 짐작은 되지만, **짐작과 표기는 다르다.**
          */}
          <ProvenanceStamp className="ml-auto" parts={{ source: sourceLabel(transactions[0]?.source) }} />
        </h3>
        {transactions.length === 0 ? (
          <p className="text-cell text-[var(--text-muted)]">최근 내부자 거래 내역이 없습니다.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[560px] text-left text-cell">
              <thead>
                <tr className="border-b border-[var(--border)] text-[var(--text-muted)]">
                  <th className="py-1.5 pr-2 font-medium">날짜</th>
                  <th className="py-1.5 pr-2 font-medium">보고자</th>
                  <th className="py-1.5 pr-2 font-medium">구분</th>
                  <th className="py-1.5 pr-2 font-medium">변동수량</th>
                  <th className="py-1.5 pr-2 font-medium">변동 후 보유</th>
                  <th className="py-1.5 font-medium">{isDomestic ? "지분율" : "단가"}</th>
                </tr>
              </thead>
              <tbody>
                {transactions.map((t, i) => (
                  <InsiderRow key={`${t.date}-${t.reporterName}-${i}`} t={t} currency={currency} isDomestic={isDomestic} />
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Card>

      {data?.note && (
        <Card className="border-dashed p-3">
          <p className="text-micro leading-relaxed text-[var(--text-muted)]">{data.note}</p>
        </Card>
      )}
    </div>
  );
}

function InsiderRow({
  t,
  currency,
  isDomestic,
}: {
  t: InsiderTransaction;
  currency: QuoteCurrency;
  isDomestic: boolean;
}) {
  const changeColor =
    t.sharesChange == null
      ? "text-[var(--text-secondary)]"
      : t.sharesChange > 0
        ? "text-[var(--profit)]"
        : t.sharesChange < 0
          ? "text-[var(--loss)]"
          : "text-[var(--text-secondary)]";

  return (
    <tr className="border-b border-[var(--border)]/60 last:border-0 align-top">
      <td className="whitespace-nowrap py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">{t.date}</td>
      <td className="py-1.5 pr-2">
        <p className="text-[var(--text-primary)]">{t.reporterName || "—"}</p>
        {t.relation && <p className="text-micro text-[var(--text-muted)]">{t.relation}</p>}
      </td>
      <td className={`py-1.5 pr-2 whitespace-nowrap font-medium ${changeColor}`}>{t.transactionType || "—"}</td>
      <td className={`py-1.5 pr-2 tabular-nums ${changeColor}`}>
        {t.sharesChange != null
          ? `${t.sharesChange > 0 ? "+" : ""}${t.sharesChange.toLocaleString("ko-KR")}주`
          : "—"}
      </td>
      <td className="py-1.5 pr-2 tabular-nums text-[var(--text-secondary)]">
        {t.sharesOwnedAfter != null ? `${t.sharesOwnedAfter.toLocaleString("ko-KR")}주` : "—"}
      </td>
      <td className="py-1.5 tabular-nums text-[var(--text-secondary)]">
        {isDomestic
          ? t.ownershipRate != null
            ? formatPercent(t.ownershipRate, false)
            : "—"
          : t.pricePerShare != null && t.pricePerShare > 0
            ? formatPrice(t.pricePerShare, currency)
            : "—"}
      </td>
    </tr>
  );
}
