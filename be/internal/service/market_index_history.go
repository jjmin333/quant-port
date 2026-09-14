package service

import (
	"context"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/calendar"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
지수 이력 보정 — 실시간 시세가 못 채운 칸을 `index_daily_closes`로 메운다.

홈 스트립에서 세 지수가 반쪽으로 나왔다:

	usd-krw   차트 없음 · 등락률 0.00%   FX 스냅샷이 주는 건 중간환율 하나뿐이다
	sox       차트 없음                  Finnhub `/quote` 응답에 이력이 없다
	nq100     차트 없음                  같은 이유

셋 다 **없는 데이터가 아니라 읽지 않던 데이터**다. `index_daily_closes`에는 이미
매일 종가가 쌓이고 있고(`daily_close.go`의 `dailyCloseIndexSchedules`가 셋 다
포함한다), 지수 점수 배치도 그 표를 읽어 260거래일을 돌린다. 그래서 새 API를 부르지
않고 DB에서 메운다 — 쿼터도 지출도 늘지 않는다.

보정은 **비어 있을 때만** 한다. 코스피는 토스 캔들로, 나스닥은 FRED로 이미 20포인트를
채워 오므로 손대지 않는다. 여기서 DB가 앞지르면 실시간 값과 차트가 어긋난다.
*/
const (
	// 20포인트를 채우려면 주말·휴장을 감안해 넉넉히 읽어야 한다.
	// 60일이면 최악(연휴가 낀 구간)에도 20거래일이 남는다.
	indexHistoryLookbackDays = 60
	// 종가는 하루 한 번 바뀐다. 1분짜리 폴링마다 DB를 때릴 이유가 없다.
	indexHistoryCacheTTL = 30 * time.Minute
)

// SetDailyCloses는 이력 보정에 쓸 종가 표를 붙인다. 생성자에 넣지 않은 이유는
// 워커 부트스트랩이 읽기 전용 스토어를 nil 의존성으로 만들기 때문이다 — 거기서는
// 보정이 필요 없고, 필요해지면 이 setter를 부르면 된다.
func (st *MarketIndexStore) SetDailyCloses(repo *repository.IndexDailyCloseRepo) {
	st.closes = repo
}

/*
indexLiveSessionDate는 **화면에 뜬 값이 속한 거래일**을, 그 지수의 자기 달력에서 본
날짜로 돌려준다.

기준을 벽시계로 잡으면 안 된다. 한국 시각 오전에 원/달러의 오늘은 이미 그날이지만
뉴욕은 아직 전날 장이 끝난 직후다. 잘못 잡으면 어제 종가를 오늘 자리에 세어 등락률이
한 칸씩 밀린다.

`updatedAt`이 있으면 그쪽을 쓴다 — 실시간 지수의 이 값은 시세의 `AsOf`, 즉 **마지막
체결 시각**이다(`indexDisplayTime`). SOXX를 뉴욕 장 마감 뒤에 받으면 벽시계는 이미
다음 날이어도 이 값은 직전 세션을 가리키므로, 그 세션의 종가 행을 실시간 값과 나란히
두 번 그리는 일이 없다.
*/
func indexLiveSessionDate(id, updatedAt string, now time.Time) string {
	schedule, ok := dailyCloseIndexSchedules[id]
	if !ok {
		schedule = calendar.ScheduleUS
	}
	ref := now
	if at, err := time.Parse(time.RFC3339, updatedAt); err == nil && !at.IsZero() {
		ref = at
	}
	return ref.In(calendar.LocationFor(schedule)).Format("2006-01-02")
}

/*
indexLiveDate는 같은 판단을 **날짜 값으로** 돌려준다 — 종가 적재가 행의 날짜를 정할 때 쓴다.

`session`은 그 세션이 끝난 날이다. 시세가 스스로 말하는 거래일이 그보다 **앞서면** 그쪽을
쓴다: FRED 계열은 관측치를 하루 늦게 공표해서, 금요일 장 뒤에 받은 값이 목요일 종가다.
그걸 금요일 자리에 넣으면 같은 값이 이틀 연속으로 저장되고 등락률이 0%가 된다.

반대 방향으로는 움직이지 않는다. 자정 근처에 도는 24/7 자산(비트코인)은 시세의 시각이
세션 날짜보다 뒤일 수 있는데, 그 자리는 여전히 그 세션의 몫이다.
*/
func indexLiveDate(id, updatedAt string, session time.Time) time.Time {
	at, err := time.Parse(time.RFC3339, updatedAt)
	if err != nil || at.IsZero() {
		return session
	}
	schedule, ok := dailyCloseIndexSchedules[id]
	if !ok {
		schedule = calendar.ScheduleUS
	}
	loc := calendar.LocationFor(schedule)
	local := at.In(loc)
	asOf := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	day := session.In(loc)
	sessionDay := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, loc)
	if asOf.Before(sessionDay) {
		return asOf
	}
	return session
}

// storedCloses는 전 지수의 최근 종가를 **한 번의 쿼리**로 받아 캐시한다.
//
// 지수마다 따로 물으면 13왕복이고, 폴링은 1분마다 돈다. 운영 DB가 1 ECPU라
// (docs/OCI_MIGRATION_PLAN.md) 이런 종류의 반복이 제일 먼저 눈에 띈다.
func (st *MarketIndexStore) storedCloses(ctx context.Context) map[string][]repository.DailyClosePoint {
	if st.closes == nil {
		return nil
	}
	st.histMu.Lock()
	defer st.histMu.Unlock()

	if st.hist != nil && time.Since(st.histAt) < indexHistoryCacheTTL {
		return st.hist
	}
	if !st.closes.HasTable(ctx) {
		return nil
	}

	since := time.Now().AddDate(0, 0, -indexHistoryLookbackDays).Format("2006-01-02")
	reqs := make([]repository.SeriesRequest, 0, len(marketIndexCatalog))
	for _, entry := range marketIndexCatalog {
		reqs = append(reqs, repository.SeriesRequest{Key: entry.id, Since: since})
	}
	byID, err := st.closes.ClosesSinceMulti(ctx, reqs)
	if err != nil {
		log.Printf("market indices history: %v", err)
		// 직전 캐시라도 그대로 쓴다 — 하루 지난 종가가 빈 차트보다 낫다.
		return st.hist
	}
	st.hist = byID
	st.histAt = time.Now()
	return byID
}

// fillGapsFromHistory는 스냅샷을 훑어 빈 스파크라인과 0.00% 등락률을 메운다.
func (st *MarketIndexStore) fillGapsFromHistory(ctx context.Context, out []model.MarketIndex) {
	hist := st.storedCloses(ctx)
	if len(hist) == 0 {
		return
	}
	now := time.Now()

	for i := range out {
		idx := &out[i]
		if idx.Value <= 0 {
			continue
		}
		needSpark := len(idx.Sparkline) < 2
		// 등락과 등락률이 **둘 다** 0일 때만 손댄다. 진짜 보합인 날에도 걸리지만,
		// 그날은 DB로 계산해도 같은 0이 나오므로 화면이 달라지지 않는다.
		needChange := idx.Change == 0 && idx.ChangeRate == 0
		if !needSpark && !needChange {
			continue
		}

		points := hist[idx.ID]
		/*
			화면 값이 속한 거래일의 행은 잘라 낸다 — 그 자리는 지금 뜬 실시간 값의
			몫이다. 남겨 두면 같은 세션을 두 번 그리고(차트의 마지막 두 점), 등락률은
			자기 자신과 비교해 0%가 된다.
		*/
		session := indexLiveSessionDate(idx.ID, idx.UpdatedAt, now)
		cut := len(points)
		for cut > 0 && points[cut-1].Date >= session {
			cut--
		}
		points = points[:cut]
		if len(points) == 0 {
			continue
		}

		if needChange {
			if prev := points[len(points)-1].Close; prev > 0 {
				idx.Change = idx.Value - prev
				idx.ChangeRate = (idx.Value - prev) / prev * 100
			}
		}
		if needSpark {
			tail := points
			if n := indexSparklinePoints - 1; len(tail) > n {
				tail = tail[len(tail)-n:]
			}
			spark := make([]float64, 0, len(tail)+1)
			for _, p := range tail {
				if p.Close > 0 {
					spark = append(spark, p.Close)
				}
			}
			// 마지막 점은 실시간 값이다. 옆에 적힌 숫자와 선의 끝이 어긋나면
			// 둘 중 무엇을 믿어야 할지 알 수 없다.
			spark = append(spark, idx.Value)
			if len(spark) >= 2 {
				idx.Sparkline = spark
			}
		}
	}
}
