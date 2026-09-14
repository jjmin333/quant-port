package service

import (
	"errors"
	"fmt"
)

/*
클라이언트 입력이 잘못됐다는 것을 핸들러가 알아볼 수 있게 하는 표식.

없을 때 무슨 일이 벌어졌나: `handler.writeServiceError`가 못 알아본 에러는 전부
`writeTossError`로 떨어지고 그 기본 분기가 **502 Bad Gateway**다. 그래서 사용자가
음수 잔고를 보내거나 심볼을 비워 보내면 "상류 게이트웨이 장애"로 보고됐다. 결과가
두 가지로 나쁘다 —

  - 클라이언트가 **재시도해야 할 서버 문제**와 **고쳐서 다시 보내야 할 입력 문제**를
    구분할 수 없다. 502는 보통 재시도 대상이다.
  - 사용자 오타가 5xx 알람을 울린다. 실측으로 검증 실패 6경로 중 4곳이 502였다.

`errors.New` 대신 이 표식을 쓰면 메시지는 그대로 두고 상태코드만 400이 된다.
새 검증을 추가할 때도 `invalidInput(...)`을 쓰면 자동으로 400이 된다.
*/

// ErrInvalidInput marks a rejection caused by what the caller sent.
var ErrInvalidInput = errors.New("invalid input")

// InvalidInputError carries the user-facing reason unchanged.
//
// 메시지를 감싸지 않고 그대로 들고 다니는 이유: 이미 화면에 그대로 노출되는
// 문자열이라("cash amount must be >= 0") 접두사를 붙이면 UI 문구가 바뀐다.
type InvalidInputError struct {
	Reason string
}

func (e *InvalidInputError) Error() string { return e.Reason }

// Unwrap lets `errors.Is(err, ErrInvalidInput)` work alongside `errors.As`.
func (e *InvalidInputError) Unwrap() error { return ErrInvalidInput }

// invalidInput builds a rejection the handler will report as 400.
func invalidInput(format string, args ...any) error {
	return &InvalidInputError{Reason: fmt.Sprintf(format, args...)}
}
