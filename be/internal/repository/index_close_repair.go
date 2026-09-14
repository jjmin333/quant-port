package repository

import "context"

/*
DeletePollerDuplicateCloses — 폴러가 하루 밀려 적어 둔 유령 종가를 지운다.

지수 종가에는 두 종류의 행이 섞인다. 하나는 **날짜를 들고 오는 소스**(`fred`,
`toss`, `etf_proxy`)가 백필한 행이고, 다른 하나는 세션이 끝날 때 그때의 시세를
그 세션 날짜로 찍는 **폴러 행**(`index_poller`)이다.

FRED 계열은 관측치를 하루 늦게 공표한다. 그래서 금요일 장 뒤에 폴러가 받아 적은
값은 실은 목요일 종가이고, 목요일 행(`fred`)과 **같은 값이 이틀 연속** 남는다.
실측 08-31: 나스닥·나스닥100·S&P 500이 08-27/08-28에 동일 종가, 다우도 마찬가지,
VIX는 08-26/08-27. 그 행의 전일 종가는 자기 자신이라 등락률이 0%로 저장되고,
화면에서는 "그날 보합이었다"로 읽힌다.

`daily_close.go`의 `indexLiveDate`가 앞으로는 그런 행을 만들지 않는다. 여기서는
**이미 쌓인 것**을 치운다 — 백필이 돌 때마다 함께 도는 자기 치유 단계다.

지우는 조건은 좁게 잡았다:

  - `index_poller` 행만. 날짜를 들고 온 소스의 행은 건드리지 않는다.
  - **직전 행과 종가가 완전히 같을 때만.** 진짜로 보합인 날도 걸릴 수 있지만,
    소수점까지 정확히 같은 날은 지수에서 사실상 없고(us10y처럼 소수 둘째 자리까지만
    쓰는 계열은 폴러 행 자체가 없다), 지워져도 다음 백필이 날짜 있는 소스로 다시
    채운다. 반대로 남겨 두면 등락률 0%가 계속 화면에 남는다.
*/
func (r *IndexDailyCloseRepo) DeletePollerDuplicateCloses(ctx context.Context) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	if !r.HasTable(ctx) {
		return 0, nil
	}
	/*
		지울 행을 파생 테이블로 **먼저 확정한 뒤** 그것과 조인해서 지운다.

		DELETE의 대상 표를 서브쿼리에서 다시 읽으면 MySQL이 거절한다
		(1093: You can't specify target table 'dup' for update in FROM clause).
		파생 테이블은 실체화되므로 같은 표를 봐도 통과한다.
	*/
	res, err := r.db.ExecContext(ctx, `
		DELETE dup FROM index_daily_closes dup
		  JOIN (
		        SELECT a.index_id, a.trade_date, a.close_value,
		               (SELECT MAX(p.trade_date) FROM index_daily_closes p
		                 WHERE p.index_id = a.index_id AND p.trade_date < a.trade_date) AS prev_date
		          FROM index_daily_closes a
		         WHERE a.source = 'index_poller'
		       ) x
		    ON x.index_id = dup.index_id AND x.trade_date = dup.trade_date
		  JOIN index_daily_closes prev
		    ON prev.index_id = x.index_id
		   AND prev.trade_date = x.prev_date
		   AND prev.close_value = x.close_value
		 WHERE dup.source = 'index_poller'`)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}
