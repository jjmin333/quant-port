export type BrokerAccount = {
  accountNo: string;
  accountSeq: number;
  accountType: string;
};

export type OrderSide = "BUY" | "SELL";
export type OrderType = "LIMIT" | "MARKET";
export type OrderStatus =
  | "PENDING"
  | "PARTIAL_FILLED"
  | "FILLED"
  | "CANCELED"
  | "REJECTED"
  | string;

export type Order = {
  orderId: string;
  symbol: string;
  side: OrderSide;
  orderType: OrderType;
  timeInForce: string;
  status: OrderStatus;
  price?: string | null;
  quantity: string;
  orderAmount?: string | null;
  currency: string;
  orderedAt: string;
  canceledAt?: string | null;
  execution: {
    filledQuantity: string;
    averageFilledPrice?: string | null;
    filledAmount?: string | null;
    commission?: string | null;
    tax?: string | null;
    filledAt?: string | null;
  };
};

export type OrdersListResponse = {
  orders: Order[];
  nextCursor?: string;
  hasNext: boolean;
};
