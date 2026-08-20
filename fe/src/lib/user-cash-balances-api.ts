import { apiRequest } from "@/lib/api-client";
import type { UserCashBalances } from "@/lib/user-cash-balances-types";

export async function fetchUserCashBalancesClient(): Promise<UserCashBalances> {
  const data = await apiRequest<UserCashBalances | null>("/api/v1/user/cash-balances");
  return data ?? { krw: 0, usd: 0 };
}

export async function saveUserCashBalancesClient(balances: UserCashBalances): Promise<UserCashBalances> {
  return apiRequest<UserCashBalances>("/api/v1/user/cash-balances", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(balances),
  });
}
