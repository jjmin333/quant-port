package service

import (
	"context"
	"strings"
)

/*
FetchSIC — 한 종목의 SEC 산업 코드(SIC)와 그 이름.

내부자 적재와 같은 `submissions/CIK….json`을 읽는다 — 그 응답 머리에 `sic`이 있다.
호출 하나로 끝나고, 배치 버킷(`UseBatchBudget`)의 간격을 스스로 기다린다.

SIC가 빈 발행사(외국 기업 일부 · 펀드)는 `("", "", nil)`이다 — 「없다」는 실패가 아니다.
*/
func (e *EdgarClient) FetchSIC(ctx context.Context, ticker string) (code, name string, err error) {
	cik, err := e.LookupCIK(ctx, ticker)
	if err != nil {
		return "", "", err
	}
	e.pace(ctx)
	subs, err := e.fetchSubmissions(ctx, cik)
	if err != nil {
		return "", "", err
	}
	return strings.TrimSpace(subs.Sic), strings.TrimSpace(subs.SicDescription), nil
}
