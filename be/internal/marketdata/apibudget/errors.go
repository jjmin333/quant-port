package apibudget

import (
	"errors"
	"fmt"
)

// 예산 거절에는 두 종류가 있고, 부르는 쪽이 해야 할 일이 서로 반대다.
//
//   - ErrExhausted — 허용량을 다 썼다. 일·월·분 호출 한도, 토큰 상한, 그리고
//     429 뒤의 차단 구간이 여기 속한다. 다시 물어도 같은 대답이고, 설령
//     통과한다면 그것은 **쓰지 않기로 정한 예산을 쓰는 것**이다. 재시도 금지.
//   - ErrPacing — 부를 수는 있는데 아직 이르다(MinInterval). 간격만큼 기다렸다
//     한 번 더 거는 것이 정답이다.
//
// 이 구분이 필요해진 계기: 뉴스 브리핑이 실패하면 규칙 문장으로 내려가는데,
// 「한 번 더 걸어 볼 실패」와 「걸어 봐야 소용없는 실패」를 메시지 문자열로
// 가려내고 있으면 문구를 고칠 때마다 조용히 어긋난다.
var (
	ErrExhausted = errors.New("api budget exhausted")
	ErrPacing    = errors.New("api budget pacing")
)

// budgetError는 메시지를 그대로 두면서 종류만 덧붙인다. 로그·관리자 화면에
// 찍히던 문장이 한 글자도 바뀌지 않아야 해서 `%w` 래핑을 쓰지 않았다 —
// 래핑하면 모든 줄 끝에 ": api budget exhausted"가 붙는다.
type budgetError struct {
	msg  string
	kind error
}

func (e *budgetError) Error() string { return e.msg }

func (e *budgetError) Is(target error) bool { return target == e.kind }

// Exhaustedf builds an ErrExhausted-class rejection with the given message.
func Exhaustedf(format string, a ...any) error {
	return &budgetError{msg: fmt.Sprintf(format, a...), kind: ErrExhausted}
}

// Pacingf builds an ErrPacing-class rejection with the given message.
func Pacingf(format string, a ...any) error {
	return &budgetError{msg: fmt.Sprintf(format, a...), kind: ErrPacing}
}
