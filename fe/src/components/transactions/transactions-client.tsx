"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { PageHeader } from "@/components/layout/header";
import { useHeaderSearch } from "@/components/providers/header-search-provider";
import { Card, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { BrokerAccount, Order } from "@/lib/orders-types";
import {
  fetchAccountsClient,
  fetchOrdersClient,
  PortfolioFetchError,
} from "@/lib/market-api";
import { PriceDisplay } from "@/components/ui/price-display";
import { formatDateKST } from "@/lib/format";
import { useExchangeRate } from "@/hooks/use-exchange-rate";

const POLL_MS = 30_000;

function sideLabel(side: Order["side"]) {
  return side === "BUY" ? "매수" : "매도";
}

function statusVariant(status: Order["status"]) {
  if (status === "FILLED") return "profit" as const;
  if (status === "CANCELED" || status === "REJECTED") return "loss" as const;
  return "default" as const;
}

function statusLabel(status: Order["status"]) {
  switch (status) {
    case "FILLED":
      return "체결";
    case "CANCELED":
      return "취소";
    case "REJECTED":
      return "거부";
    case "PARTIAL_FILLED":
      return "부분체결";
    default:
      return status;
  }
}

export function TransactionsClient() {
  const { activeQuery, isSearchActive, mode } = useHeaderSearch();
  const txSearchActive = isSearchActive && mode === "transactions";
  const [accounts, setAccounts] = useState<BrokerAccount[]>([]);
  const [accountSeq, setAccountSeq] = useState<number | null>(null);
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const exchangeRate = useExchangeRate(true);

  const loadOrders = useCallback(async (seq: number) => {
    try {
      const res = await fetchOrdersClient(seq, "CLOSED");
      setOrders(res.orders);
      setError(null);
    } catch (err) {
      setError(err instanceof PortfolioFetchError ? err.message : "거래내역 조회 실패");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchAccountsClient()
      .then((list) => {
        setAccounts(list);
        if (list.length > 0) {
          setAccountSeq(list[0].accountSeq);
        } else {
          setLoading(false);
        }
      })
      .catch((err) => {
        setError(err instanceof PortfolioFetchError ? err.message : "계좌 조회 실패");
        setLoading(false);
      });
  }, []);

  useEffect(() => {
    if (!accountSeq) return;
    void loadOrders(accountSeq);
    const timer = setInterval(() => void loadOrders(accountSeq), POLL_MS);
    return () => clearInterval(timer);
  }, [accountSeq, loadOrders]);

  const visibleOrders = useMemo(() => {
    if (!txSearchActive) return orders;
    const q = activeQuery.toLowerCase();
    return orders.filter((order) => {
      if (order.symbol.toLowerCase().includes(q)) return true;
      if (order.orderId.toLowerCase().includes(q)) return true;
      if (sideLabel(order.side).includes(q)) return true;
      if (statusLabel(order.status).includes(q)) return true;
      return false;
    });
  }, [orders, activeQuery, txSearchActive]);

  return (
    <>
      <PageHeader title="거래내역 분석" subtitle="체결 · 취소 주문 내역 (30초 갱신)" />
      <main className="flex-1 overflow-y-auto p-6">
        <div className="mx-auto min-w-[var(--dashboard-content-min)] max-w-5xl space-y-6">
          {error && (
            <Card className="border-rose-500/40 bg-rose-500/5 p-4 text-sm text-rose-300">
              {error}
            </Card>
          )}

          {accounts.length > 1 && (
            <div className="flex flex-wrap gap-2">
              {accounts.map((acc) => (
                <button
                  key={acc.accountSeq}
                  type="button"
                  onClick={() => {
                    setLoading(true);
                    setAccountSeq(acc.accountSeq);
                  }}
                  className={`rounded-lg border px-3 py-1.5 text-sm ${
                    accountSeq === acc.accountSeq
                      ? "border-[var(--accent)] bg-[var(--accent-muted)] text-[var(--accent)]"
                      : "border-[var(--border)] text-[var(--text-secondary)] hover:text-[var(--text-primary)]"
                  }`}
                >
                  {acc.accountNo}
                </button>
              ))}
            </div>
          )}

          <Card className="overflow-x-auto p-0">
            <CardHeader className="px-5 pt-5">
              <CardTitle>주문 내역</CardTitle>
              <Badge variant="info">
                {visibleOrders.length}건{txSearchActive ? " · 검색" : ""}
              </Badge>
            </CardHeader>

            {loading ? (
              <div className="px-5 pb-5">
                <div className="h-32 animate-pulse rounded-lg bg-[var(--surface-elevated)]" />
              </div>
            ) : visibleOrders.length === 0 ? (
              <p className="px-5 pb-5 text-sm text-[var(--text-muted)]">
                {txSearchActive ? "검색 결과가 없습니다." : "체결·취소된 주문이 없습니다."}
              </p>
            ) : (
              <table className="w-full min-w-[720px] text-sm">
                <thead>
                  <tr className="border-t border-[var(--border)] text-left text-xs text-[var(--text-muted)]">
                    <th className="px-5 py-3 font-medium">일시</th>
                    <th className="px-3 py-3 font-medium">종목</th>
                    <th className="px-3 py-3 font-medium">구분</th>
                    <th className="px-3 py-3 font-medium">유형</th>
                    <th className="px-3 py-3 font-medium">수량</th>
                    <th className="px-3 py-3 font-medium">체결가</th>
                    <th className="px-5 py-3 font-medium">상태</th>
                  </tr>
                </thead>
                <tbody>
                  {visibleOrders.map((order) => {
                    const filledPrice = order.execution.averageFilledPrice
                      ? Number(order.execution.averageFilledPrice)
                      : order.price
                        ? Number(order.price)
                        : null;
                    const currency = order.currency === "USD" ? "USD" : "KRW";

                    return (
                      <tr
                        key={order.orderId}
                        className="border-t border-[var(--border)] hover:bg-[var(--surface-elevated)]/50"
                      >
                        <td className="px-5 py-3 tabular-nums text-[var(--text-secondary)]">
                          {formatDateKST(order.orderedAt)}
                        </td>
                        <td className="px-3 py-3 font-medium text-[var(--text-primary)]">
                          {order.symbol}
                        </td>
                        <td className="px-3 py-3">
                          <Badge variant={order.side === "BUY" ? "profit" : "loss"}>
                            {sideLabel(order.side)}
                          </Badge>
                        </td>
                        <td className="px-3 py-3 text-[var(--text-secondary)]">
                          {order.orderType === "MARKET" ? "시장가" : "지정가"}
                        </td>
                        <td className="px-3 py-3 font-mono tabular-nums text-[var(--text-secondary)]">
                          {order.execution.filledQuantity || order.quantity}
                        </td>
                        <td className="px-3 py-3 tabular-nums text-[var(--text-secondary)]">
                          {filledPrice != null && !Number.isNaN(filledPrice) ? (
                            <PriceDisplay
                              value={filledPrice}
                              currency={currency}
                              size="sm"
                              className="font-normal text-[var(--text-secondary)]"
                            />
                          ) : (
                            "—"
                          )}
                        </td>
                        <td className="px-5 py-3">
                          <Badge variant={statusVariant(order.status)}>
                            {statusLabel(order.status)}
                          </Badge>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            )}
          </Card>
        </div>
      </main>
    </>
  );
}
