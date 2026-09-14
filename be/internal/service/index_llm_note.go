package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/llm"
)

/*
지수 LLM 해설 — 열세 칸이 각자 **무슨 국면인지**를 한 문단으로.

규칙 문장(`indexSummaryText`)은 지표를 나열한다. 사실은 다 맞지만 읽어도 상황은
모른다 — RSI 56.8이 좋은 건지, 20일선 위인데 52주 최고 대비 −2%인 건 회복인지
정체인지를 읽는 사람이 조립해야 했다. 여기서는 그 조립만 맡긴다.

**숫자는 여전히 규칙이 뽑는다.** 모델에는 계산이 끝난 지표 JSON만 넘어가고,
프롬프트는 JSON 밖의 수치를 금지한다. 그래서 이 단계가 실패해도 잃는 것은 문장뿐이고,
규칙 문장은 그 자리에 그대로 남는다.

**호출 규칙**(docs/BATCH_SCHEDULE.md의 LLM 지출 원칙):

  - 조회 경로에서는 절대 부르지 않는다. 화면은 저장된 것만 읽는다.
  - 하루 한 번, 지수 점수 배치와 같은 자리에서 돈다. 관리자 수동 실행도 여기로 온다.
  - **같은 거래일에 이미 만든 지수는 건너뛴다.** 배치가 두 번 돌거나 재기동돼도
    같은 날 같은 지수에 두 번 쓰지 않는다.
  - 지표가 얕은 지수(30봉 미만)는 부르지 않는다 — 할 말이 없는 것에 문단을 붙이면
    모델이 채워 넣는다.
*/

// indexNoteMinBars — 이만큼은 있어야 20일선·RSI가 의미를 가진다(`indexTagMinBars`와 같은 기준).
const indexNoteMinBars = 30

// indexNoteFacts is what the model receives. Field names match the prompt's
// glossary — renaming one here without the prompt would silently drop it.
type indexNoteFacts struct {
	Name           string   `json:"name"`
	TradeDate      string   `json:"tradeDate"`
	Close          float64  `json:"close"`
	Return5D       *float64 `json:"return5d,omitempty"`
	Return20D      *float64 `json:"return20d,omitempty"`
	SMA20Position  string   `json:"sma20Position,omitempty"`
	RSI14          *float64 `json:"rsi14,omitempty"`
	PctFromHigh52W *float64 `json:"pctFromHigh52w,omitempty"`
	/*
		Directional=false면 오르는 것이 좋은 소식이라고 단정할 수 없는 지표다
		(원/달러·VIX·미 국채 금리). FE가 이 셋의 칩에서 방향색을 빼는 것과 같은
		이유이고, 프롬프트도 이 값을 보고 좋다·나쁘다를 말하지 않는다.
	*/
	Directional bool `json:"direction"`
}

// nonDirectionalIndexIDs mirrors fe/src/lib/index-tags.ts NON_DIRECTIONAL_INDEX_IDS.
var nonDirectionalIndexIDs = map[string]bool{
	"usd-krw": true,
	"vix":     true,
	"us10y":   true,
}

// IndexNoteResult reports one run.
type IndexNoteResult struct {
	Generated int
	Skipped   int
	Failed    int
	Tokens    int64
}

/*
GenerateNotes writes an LLM paragraph for every index whose newest close does
not have one yet.

`force`는 관리자 수동 실행용이다 — 프롬프트를 고친 뒤 같은 날 다시 돌려 보려면
「오늘 것은 이미 있다」를 넘어설 수 있어야 한다.
*/
func (s *IndexScoreService) GenerateNotes(ctx context.Context, force bool) (IndexNoteResult, error) {
	var res IndexNoteResult
	if s == nil || s.db == nil {
		return res, fmt.Errorf("index score service not configured")
	}
	if s.llm == nil || !s.llm.Enabled() {
		// 꺼져 있는 것은 실패가 아니다 — 규칙 문장이 그대로 화면을 채운다.
		return res, nil
	}
	// 지수는 한 시장에 속하지 않는다(코스피와 나스닥이 같은 목록에 있다). 공용 키를 쓴다.
	client := s.llm.For("")
	if client == nil || !client.Enabled() {
		return res, nil
	}

	series, err := s.loadIndexSeries(ctx)
	if err != nil {
		return res, err
	}

	for _, entry := range marketIndexCatalog {
		sr, ok := series[entry.id]
		if !ok || len(sr.closes) < indexNoteMinBars {
			res.Skipped++
			continue
		}
		latest := s.noteTargetDate(ctx, entry.id, sr.dates[len(sr.dates)-1])

		if !force {
			has, err := s.hasNote(ctx, entry.id, latest)
			if err != nil {
				return res, err
			}
			if has {
				res.Skipped++
				continue
			}
		}

		facts := s.buildNoteFacts(entry.id, entry.name, latest, sr)
		if facts == nil {
			res.Skipped++
			continue
		}
		payload, err := json.Marshal(facts)
		if err != nil {
			res.Failed++
			continue
		}

		out, err := client.GenerateIndexNote(ctx, string(payload))
		if err != nil {
			log.Printf("index note %s: %v", entry.id, err)
			res.Failed++
			continue
		}
		if out.Note == "" {
			res.Failed++
			continue
		}
		if err := s.storeNote(ctx, entry.id, latest, out); err != nil {
			log.Printf("index note %s: store: %v", entry.id, err)
			res.Failed++
			continue
		}
		res.Generated++
		res.Tokens += out.TotalTokens

		// 제공자가 요구하는 최소 간격을 지킨다. 열세 번을 연달아 쏘면 429가 난다.
		if !waitInterval(ctx, client.Limits().MinInterval) {
			return res, ctx.Err()
		}
	}
	return res, nil
}

func (s *IndexScoreService) buildNoteFacts(
	id, name, tradeDate string, sr *indexCloseSeries,
) *indexNoteFacts {
	ti := indexTechnicals(sr.closes)
	if ti == nil || ti.LastClose == nil {
		return nil
	}
	facts := &indexNoteFacts{
		Name:           name,
		TradeDate:      tradeDate,
		Close:          *ti.LastClose,
		Return5D:       ti.Return5D,
		Return20D:      ti.Return20D,
		RSI14:          ti.RSI14,
		PctFromHigh52W: ti.PctFromHigh52Week,
		Directional:    !nonDirectionalIndexIDs[id],
	}
	if ti.SMA20 != nil {
		facts.SMA20Position = "위"
		if *ti.LastClose < *ti.SMA20 {
			facts.SMA20Position = "아래"
		}
	}
	return facts
}

/*
noteTargetDate는 해설을 **점수가 앉아 있는 행**에 쓰도록 날짜를 고른다.

해설은 독립된 값이 아니라 그 행의 점수·키워드에 붙는 주석이다. 그런데 둘을 각자
「가장 최근 종가 행」에 쓰면 그 사이에 종가가 하나 들어온 순간 갈라진다 — 실측 08-31:
VIX의 점수·키워드는 08-27 행에 있는데 해설만 08-28 행에 쓰였고, 화면이 읽는
`LatestAll`은 점수가 있는 행을 고르므로 **새로 만든 해설이 통째로 묻혔다**(LLM 호출
하나를 버린 셈이다).

그래서 점수나 규칙 문장이 이미 있는 가장 최근 행을 먼저 찾는다. 하나도 없으면(첫
실행이거나 점수 배치가 아직 안 돈 경우) 최신 종가 행으로 떨어진다 — 그때는 갈라질
상대가 없다.
*/
func (s *IndexScoreService) noteTargetDate(ctx context.Context, indexID, fallback string) string {
	var scored sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT DATE_FORMAT(MAX(trade_date), '%Y-%m-%d')
		  FROM index_daily_closes
		 WHERE index_id = ? AND (score IS NOT NULL OR ai_summary <> '')`, indexID).Scan(&scored)
	if err != nil || !scored.Valid || scored.String == "" {
		return fallback
	}
	return scored.String
}

func (s *IndexScoreService) hasNote(ctx context.Context, indexID, tradeDate string) (bool, error) {
	var note string
	err := s.db.QueryRowContext(ctx, `
		SELECT llm_note FROM index_daily_closes
		 WHERE index_id = ? AND trade_date = ?`, indexID, tradeDate).Scan(&note)
	if err != nil {
		// 행이 없으면 아직 없는 것이다 — 그 자체는 오류가 아니다.
		return false, nil
	}
	return note != "", nil
}

func (s *IndexScoreService) storeNote(
	ctx context.Context, indexID, tradeDate string, out *llm.IndexNoteLLMOutput,
) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE index_daily_closes
		   SET llm_note = ?, llm_stance = ?, llm_at = ?
		 WHERE index_id = ? AND trade_date = ?`,
		out.Note, out.Stance, time.Now().UTC(), indexID, tradeDate)
	return err
}

// waitInterval sleeps out a provider's minimum spacing, reporting false when the
// context was cancelled while waiting.
func waitInterval(ctx context.Context, gap time.Duration) bool {
	if gap <= 0 {
		return true
	}
	timer := time.NewTimer(gap)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
