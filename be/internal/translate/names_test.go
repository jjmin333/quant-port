package translate

import (
	"context"
	"strings"
	"testing"
	"time"
)

/*
이름 잠그기의 **두 실패**를 고정한다 — 둘 다 2026-09-15에 LMT-60-4B를 실측하다
나온 것이고, 둘 다 조용히 틀린다(사람이 목록을 읽어야만 보인다).
*/

// 카탈로그에 없는 이름은 모델이 **낱말로 읽는다.** 실측: Perplexity → 「당혹감」,
// Anthropic → 「인간학」, Yahoo Finance → 「야후 재정」.
func TestMaskNamesLocksBuiltinNames(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		text string
		want string // 잠긴 뒤 원문에 남아 있으면 안 되는 말
		ko   string // 되돌린 뒤 나와야 하는 말
	}{
		{"비상장 AI 기업", "Perplexity local agent platform now available", "Perplexity", "퍼플렉시티"},
		{"소유격도 잠긴다", "Anthropic's Amodei spoke with a senator", "Anthropic", "앤스로픽"},
		{"출처 표기", "Nasdaq rises on chip earnings - Yahoo Finance", "Yahoo Finance", "야후 파이낸스"},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			masked, restore := MaskNames(tc.text, nil)
			if strings.Contains(masked, tc.want) {
				t.Fatalf("이름이 안 잠겼다: %q 가 %q 에 그대로 있다", tc.want, masked)
			}
			// 모델이 자리표시자를 그대로 돌려줬다고 치고 되돌린다.
			out, ok := RestoreNames(masked, restore)
			if !ok {
				t.Fatalf("되돌리기 실패: %q", masked)
			}
			if !strings.Contains(out, tc.ko) {
				t.Fatalf("한국어 이름이 안 들어갔다: %q 에 %q 가 없다", out, tc.ko)
			}
		})
	}
}

// 호출자가 준 이름이 없어도 내장 목록만으로 잠근다 — 예전에는 pairs 가 비면
// 함수가 곧바로 돌아왔다.
func TestMaskNamesWithoutCallerPairs(t *testing.T) {
	t.Parallel()

	masked, restore := MaskNames("Reuters: Fed holds rates steady", nil)
	if strings.Contains(masked, "Reuters") {
		t.Fatalf("내장 목록이 적용되지 않았다: %q", masked)
	}
	if len(restore) != 1 {
		t.Fatalf("되돌릴 이름표가 %d개다 (1개여야 한다)", len(restore))
	}
}

// 호출자가 준 짧은 이름(카탈로그의 "Yahoo")보다 내장 목록의 긴 이름("Yahoo
// Finance")이 먼저 잠겨야 한다 — 둘이 한 목록에 섞여 길이순으로 정렬된다.
func TestMaskNamesPrefersLongerBuiltinName(t *testing.T) {
	t.Parallel()

	pairs := []NamePair{{English: "Yahoo", Korean: "야후"}}
	masked, restore := MaskNames("Story - Yahoo Finance", pairs)
	out, ok := RestoreNames(masked, restore)
	if !ok {
		t.Fatalf("되돌리기 실패: %q", masked)
	}
	if !strings.Contains(out, "야후 파이낸스") {
		t.Fatalf("긴 이름이 먼저 잠기지 않았다: %q", out)
	}
}

/*
모델이 **달러 기호만 떨어뜨린** 경우에도 되돌린다.

실측(2026-09-15, LMT-60-4B 20건 중 1건): "Fantastic News for Broadcom Stock
Investors" → 「QPZAA 주식 투자자에게 환상적인 소식」. 이 한 건이 그대로 실패로
남으면 그 제목은 세 번 시도 끝에 영어로 굳는다.
*/
func TestRestoreNamesAcceptsDroppedDollar(t *testing.T) {
	t.Parallel()

	out, ok := RestoreNames("QPZAA 주식 투자자에게 환상적인 소식", []string{"브로드컴"})
	if !ok {
		t.Fatal("달러 기호가 빠진 자리표시자를 못 되돌렸다")
	}
	if !strings.Contains(out, "브로드컴") {
		t.Fatalf("이름이 안 들어갔다: %q", out)
	}
	if strings.Contains(out, "QPZAA") {
		t.Fatalf("자리표시자가 남았다: %q", out)
	}
}

// 느슨함의 한계 — 글자가 바뀐 것은 여전히 실패여야 한다. 그 자리에 어느 회사가
// 있었는지 더는 알 수 없기 때문이다.
func TestRestoreNamesStillFailsOnAlteredPlaceholder(t *testing.T) {
	t.Parallel()

	if _, ok := RestoreNames("QPZ년 주식이 올랐다", []string{"브로드컴"}); ok {
		t.Fatal("글자가 바뀐 자리표시자를 받아들였다")
	}
}

/*
**서버가 죽은 것을 「제목이 나쁘다」로 세면 안 된다** (2026-09-16 실측).

드레인 도중 번역기를 재시작했더니 그 몇 초 동안 요청이 전부 실패했는데,
Local이 오류를 삼키고 빈 결과만 돌려줘서 부르는 쪽이 `translate_attempts`를
올렸다 — **615건이 세 번에 닿아 큐에서 영구히 빠졌다.**

닿을 수 없는 주소를 주면 그 상황이 그대로 재현된다.
*/
func TestLocalReportsErrorWhenEveryRequestFails(t *testing.T) {
	t.Parallel()

	// 127.0.0.1:1 — 아무도 안 듣는 포트. 연결이 즉시 거절된다.
	dead := NewLocal("http://127.0.0.1:1", 2, 2*time.Second)
	out, err := dead.TranslateENtoKO(context.Background(), []string{"Nasdaq rises", "Fed holds"})
	if err == nil {
		t.Fatal("서버가 통째로 안 되는데 오류를 안 올렸다 — 부르는 쪽이 제목 탓으로 센다")
	}
	for i, got := range out {
		if got != "" {
			t.Fatalf("out[%d] = %q, 빈 문자열이어야 한다", i, got)
		}
	}
}

// 빈 입력만 있으면 보낸 것이 없다 — 오류가 아니다.
func TestLocalNoErrorWhenNothingToSend(t *testing.T) {
	t.Parallel()

	dead := NewLocal("http://127.0.0.1:1", 2, 2*time.Second)
	if _, err := dead.TranslateENtoKO(context.Background(), []string{"", "  "}); err != nil {
		t.Fatalf("보낸 것이 없는데 오류가 났다: %v", err)
	}
}
