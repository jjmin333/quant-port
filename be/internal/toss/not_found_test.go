package toss

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

/*
「상대가 모르는 종목」과 「지금 안 되는 것」을 갈라야 한다 (2026-09-11 사용자 요청 7).

갈라지지 않으면 상대에게 없는 종목을 매일 다시 묻는다 — 실측으로 warnings 잡의
Toss 호출 19%(17,918 중 3,388)가 그 질문이었다.
*/

func TestIsStockNotFound(t *testing.T) {
	notFound := &Error{StatusCode: http.StatusNotFound, Code: "stock-not-found", Message: "종목을 찾을 수 없습니다."}
	if !IsStockNotFound(notFound) {
		t.Fatal("404 stock-not-found 를 못 알아본다")
	}
	// 코드만 와도(상태가 다른 경우) 같은 뜻이다.
	if !IsStockNotFound(&Error{StatusCode: 400, Code: "stock-not-found"}) {
		t.Fatal("코드만으로도 판정해야 한다")
	}
	// 감싸도 통해야 한다 — 호출부는 대개 fmt.Errorf 로 감싼다.
	if !IsStockNotFound(fmt.Errorf("warnings %s: %w", "005930", notFound)) {
		t.Fatal("감싼 오류를 못 알아본다")
	}
}

func TestIsStockNotFound_doesNotSwallowTransientFailures(t *testing.T) {
	// 429·5xx·네트워크 오류는 **다시 물어야** 나아진다. not_found 로 접으면
	// 그 종목이 30일간 갱신되지 않는다.
	for _, err := range []error{
		&Error{StatusCode: http.StatusTooManyRequests, Code: errCodeRateLimitExceeded},
		&Error{StatusCode: http.StatusInternalServerError, Code: "internal"},
		&Error{StatusCode: http.StatusBadRequest, Code: "invalid-request"},
		errors.New("http do: context deadline exceeded"),
		nil,
	} {
		if IsStockNotFound(err) {
			t.Fatalf("일시 실패를 not_found 로 접었다: %v", err)
		}
	}
}
