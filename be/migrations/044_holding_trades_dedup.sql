USE quantport;

-- holding_trades 중복 정리: 동일 사용자·종목·매수·수량·단가·날짜 조합은 id가 작은 행만 유지

DELETE t1 FROM holding_trades t1
INNER JOIN holding_trades t2
  ON t1.user_id = t2.user_id
 AND t1.symbol = t2.symbol
 AND t1.side = t2.side
 AND ABS(t1.quantity - t2.quantity) < 0.00001
 AND ABS(t1.price - t2.price) < 0.00001
 AND t1.trade_date = t2.trade_date
 AND t1.id > t2.id;
