package service

import "testing"

func TestKRCommonShareCode(t *testing.T) {
	cases := map[string]string{
		"001275": "001270", // 부국증권우
		"00279K": "002790", // 아모레퍼시픽홀딩스3우C
		"005935": "005930", // 삼성전자우
		"005930": "",       // 보통주 자신
		"AAPL":   "",
	}
	for in, want := range cases {
		if got := krCommonShareCode(in); got != want {
			t.Errorf("%s: got %q want %q", in, got, want)
		}
	}
}
