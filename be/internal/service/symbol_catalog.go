package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/enact/quant-port/be/internal/repository"
)

/*
카탈로그에 없는 심볼을 사용자 데이터에 넣지 않는다 (2026-09-06 사용자 결정).

# 무엇이 문제였나

`POST /watchlist {"symbol":"ZZZZZZZZZ"}` 가 **201** 이었다. 관심 목록에 이름도
시세도 없는 행이 생기고, 화면은 그것을 종목인 것처럼 한 줄 그린다. 보유 매매도
같았다 — 아무 문자열이나 포지션이 됐다.

# 왜 조회 한 번을 더 감수하나

이 검사는 **쓰기 경로에만** 붙는다. 관심 추가·매매 기록은 사람이 누르는 속도로
일어나고 목록 조회처럼 항목마다 반복되지 않으므로, 종목당 인덱스 조회 한 번은
「항목당 I/O 금지」(AGENT_STATE 응답 속도 원칙 1)에 걸리지 않는다. `stocks.symbol`은
기본키라 이 조회는 단일 행 탐색이다.

# 주입되지 않았으면 통과시킨다

`stocks`가 nil이면 예전처럼 받는다. 카탈로그를 못 읽는 상태에서 사용자의 저장을
막는 것보다, 검사를 건너뛰는 편이 안전하다 — 이 검사는 데이터 위생이지 보안
경계가 아니다.
*/

// ErrUnknownSymbol 은 카탈로그에 없는 심볼이다. 핸들러가 404로 옮긴다.
var ErrUnknownSymbol = errors.New("unknown symbol")

func ensureSymbolInCatalog(ctx context.Context, stocks *repository.StockRepo, symbol string) error {
	if stocks == nil {
		return nil
	}
	found, err := stocks.GetBySymbol(ctx, symbol)
	if err != nil {
		return err
	}
	if found == nil {
		return fmt.Errorf("%w: %s", ErrUnknownSymbol, symbol)
	}
	return nil
}
