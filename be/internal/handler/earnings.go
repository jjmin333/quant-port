package handler

import (
	"net/http"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type EarningsHandler struct {
	svc *service.EarningsTabService
}

func NewEarningsHandler(svc *service.EarningsTabService) *EarningsHandler {
	return &EarningsHandler{svc: svc}
}

func (h *EarningsHandler) Get(w http.ResponseWriter, r *http.Request) {
	symbol := chi.URLParam(r, "symbol")
	if symbol == "" {
		response.Error(w, http.StatusBadRequest, "symbol is required")
		return
	}

	view, err := h.svc.Get(r.Context(), symbol)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": view})
}

// earningsBatchMaxSymbols — 한 번에 물어볼 수 있는 종목 수 상한.
//
// 이 엔드포인트를 쓰는 화면(홈 「실적·공시」)이 여덟 종목을 본다. 상한을 두지 않으면
// 심볼 목록이 URL로 들어오므로 누구나 수백 종목을 한 요청에 얹어 서버에 그만큼의
// 팬아웃을 시킬 수 있다.
const earningsBatchMaxSymbols = 20

// GetMany serves GET /stocks/earnings/batch?symbols=A,B,C.
//
// 화면 쪽에서 종목당 한 번씩 부르던 것을 한 요청으로 접는다. 홈 대시보드는 거래대금
// 상위 여덟 종목의 일정을 그리는데, 배치 엔드포인트가 없어 브라우저가 여덟 개를
// 동시에 쐈다. 로컬에서는 티가 안 나지만 왕복 지연이 붙는 회선에서는 그 여덟이
// 연결 슬롯을 먹고 같은 화면의 다른 카드를 밀어낸다.
//
// **한 종목이 실패해도 나머지는 내려간다.** 없는 심볼 하나 때문에 카드 전체가
// 비면, 여덟 번 나눠 부르던 예전보다 나빠진다 — 그때는 일곱 개가 떴다.
func (h *EarningsHandler) GetMany(w http.ResponseWriter, r *http.Request) {
	raw := strings.Split(r.URL.Query().Get("symbols"), ",")
	symbols := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, s := range raw {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" {
			continue
		}
		if _, dup := seen[s]; dup {
			continue
		}
		seen[s] = struct{}{}
		symbols = append(symbols, s)
		if len(symbols) >= earningsBatchMaxSymbols {
			break
		}
	}
	if len(symbols) == 0 {
		response.Error(w, http.StatusBadRequest, "symbols is required")
		return
	}

	views := make([]*model.EarningsTabView, len(symbols))
	var wg sync.WaitGroup
	for i, symbol := range symbols {
		wg.Add(1)
		go func(i int, symbol string) {
			defer wg.Done()
			view, err := h.svc.Get(r.Context(), symbol)
			if err != nil {
				return
			}
			views[i] = view
		}(i, symbol)
	}
	wg.Wait()

	data := make(map[string]*model.EarningsTabView, len(symbols))
	for i, symbol := range symbols {
		if views[i] != nil {
			data[symbol] = views[i]
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": data})
}
