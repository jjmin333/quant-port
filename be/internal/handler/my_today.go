package handler

import (
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

// MyTodayHandler serves the home 「내 종목」 card — stored rows only, no LLM.
type MyTodayHandler struct {
	today   *repository.MyTodayRepo
	changes *repository.ChangeEventRepo
}

func NewMyTodayHandler(today *repository.MyTodayRepo, changes *repository.ChangeEventRepo) *MyTodayHandler {
	return &MyTodayHandler{today: today, changes: changes}
}

type myTodayChange struct {
	Title       string `json:"title"`
	Direction   int    `json:"direction"`
	Unexplained bool   `json:"unexplained,omitempty"`
}

type myTodayItem struct {
	repository.MyTodaySymbol
	Change *myTodayChange `json:"change,omitempty"`
}

/*
Get — `GET /user/today`.

관심·보유가 없으면 빈 목록을 준다(화면은 그때 「종목 담기」 안내를 그린다). 점수·뉴스·변화
중 하나를 못 읽어도 나머지로 답한다 — 카드 한 칸 때문에 홈의 첫 카드가 통째로 비면 안 된다.
*/
func (h *MyTodayHandler) Get(w http.ResponseWriter, r *http.Request) {
	userID, err := authctx.RequireUserID(r.Context())
	if err != nil {
		response.Error(w, http.StatusUnauthorized, "login required")
		return
	}
	ctx := r.Context()
	tracked, err := h.today.Tracked(ctx, userID)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := map[string]any{"scoreDate": "", "changeDate": "", "items": []myTodayItem{}}
	if len(tracked) == 0 {
		response.JSON(w, http.StatusOK, map[string]any{"data": out})
		return
	}

	scoreDate, _ := h.today.AttachScores(ctx, tracked)
	_ = h.today.AttachNewsCounts(ctx, tracked, time.Now())
	_ = h.today.AttachAIOverviews(ctx, tracked)

	// 변화 — 종목마다 그날 가장 중요한 한 줄(List는 중요도 내림차순).
	changes := map[string]*myTodayChange{}
	changeDate := ""
	if dates, err := h.changes.Dates(ctx, 1); err == nil && len(dates) > 0 {
		changeDate = dates[0]
		symbols := make([]string, 0, len(tracked))
		for _, t := range tracked {
			symbols = append(symbols, t.Symbol)
		}
		events, err := h.changes.List(ctx, repository.ChangeListQuery{Date: changeDate, Symbols: symbols, Limit: 500})
		if err == nil {
			for _, e := range events {
				unexplained := e.Kind == "price_move" && e.Explained != nil && !*e.Explained
				if c, ok := changes[e.Symbol]; ok {
					if unexplained {
						c.Unexplained = true
					}
					continue
				}
				changes[e.Symbol] = &myTodayChange{Title: e.Title, Direction: e.Direction, Unexplained: unexplained}
			}
		}
	}

	items := make([]myTodayItem, 0, len(tracked))
	for _, t := range tracked {
		items = append(items, myTodayItem{MyTodaySymbol: t, Change: changes[t.Symbol]})
	}
	out["scoreDate"] = scoreDate
	out["changeDate"] = changeDate
	out["items"] = items
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}
