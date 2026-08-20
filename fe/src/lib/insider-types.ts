export type InsiderTransaction = {
  date: string;
  reporterName: string;
  relation?: string;
  transactionType?: string;
  sharesChange?: number;
  changeRate?: number;
  sharesOwnedAfter?: number;
  ownershipRate?: number;
  pricePerShare?: number;
  source: "dart" | "edgar";
};

export type InsiderTabData = {
  symbol: string;
  assetType: string;
  transactions: InsiderTransaction[];
  note?: string;
};
