package service

import (
	"context"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
「이 종목에 대한 내 기록」 (V2 요구 9) — 근거는 `model/symbol_context.go` 머리글에.

# 판정은 **빌려 쓴다**

보유 이유 점검의 판정을 여기서 다시 계산하지 않고 `HoldingsDiagnosisService`를
부른다. 두 벌로 만들면 언젠가 갈라지고, 그때 사용자는 보유 화면에서 「약화」를
보고 종목 화면에서 「유지」를 본다 — 어느 쪽을 믿어야 할지 알 수 없는 상태가
가장 나쁘다.

# 보유 중일 때만 부른다

`Diagnose`는 보유 목록 전체를 전제로 한 배치 조회 다섯이다. 안 가진 종목에는
「보유 이유」가 없으므로 부를 이유도 없고, 종목 화면 대부분이 그 경우다.
*/

// SymbolContextService assembles one user's footprint on one symbol.
type SymbolContextService struct {
	repo      *repository.SymbolContextRepo
	diagnosis *HoldingsDiagnosisService
}

func NewSymbolContextService(
	repo *repository.SymbolContextRepo,
	diagnosis *HoldingsDiagnosisService,
) *SymbolContextService {
	return &SymbolContextService{repo: repo, diagnosis: diagnosis}
}

func (s *SymbolContextService) Get(
	ctx context.Context, userID int64, symbol string,
) (*model.SymbolContext, error) {
	if s == nil || s.repo == nil {
		return nil, invalidInput("내 기록을 사용할 수 없습니다")
	}
	sym := strings.ToUpper(strings.TrimSpace(symbol))
	if sym == "" {
		return nil, invalidInput("종목을 지정해 주세요")
	}

	out, err := s.repo.Get(ctx, userID, sym)
	if err != nil {
		return nil, err
	}

	if out.Held && s.diagnosis != nil {
		/*
			판정을 못 구해도 **나머지는 내보낸다.** 이 한 칩 때문에 보유·관심·스탬프가
			전부 사라지면, 사용자는 자기 기록이 없어진 줄 안다.
		*/
		if diag, derr := s.diagnosis.Diagnose(ctx, userID); derr == nil && diag != nil {
			for _, item := range diag.Items {
				if strings.EqualFold(item.Symbol, sym) {
					out.DiagnosisVerdict = string(item.Verdict)
					out.DiagnosisSignals = item.SignalCount
					break
				}
			}
		}
	}
	return &out, nil
}
