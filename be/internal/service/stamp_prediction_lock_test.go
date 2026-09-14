package service

import (
	"testing"
	"time"
)

/*
스탬프 예측 잠금의 **경계**.

이 규칙은 지금까지 어디에서도 검증되지 않았다 — e2e로 확인하려면 5분을 기다려야
하는데 그 스위트에는 임의 대기가 한 군데도 없고, 그래서 「아직 검증하지 못한 것」에
남아 있었다(`e2e/README.md`). 시각 판정을 순수 함수로 떼어 내면 시계를 인자로
넘길 수 있으므로, 기다리지 않고 경계를 정확히 못박을 수 있다.

경계가 중요한 이유: 여기가 헐거우면 사용자가 **주가가 움직인 뒤에** 목표가를 고칠
수 있고, 그러면 적중률이라는 숫자가 통째로 의미를 잃는다. 반대로 너무 빡빡하면
방금 오타를 낸 사람이 고칠 방법이 없다.
*/
func TestPredictionLocked(t *testing.T) {
	stampedAt := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		now        time.Time
		wantLocked bool
	}{
		{
			name:       "찍은 직후에는 고칠 수 있다",
			now:        stampedAt,
			wantLocked: false,
		},
		{
			name:       "유예 시간 1나노초 전에는 아직 열려 있다",
			now:        stampedAt.Add(stampPredictionGrace - time.Nanosecond),
			wantLocked: false,
		},
		{
			// 경계는 `>`다 — 정확히 5분일 때는 아직 열려 있다.
			name:       "정확히 유예 시간이면 아직 열려 있다",
			now:        stampedAt.Add(stampPredictionGrace),
			wantLocked: false,
		},
		{
			name:       "유예 시간을 1나노초라도 넘기면 잠긴다",
			now:        stampedAt.Add(stampPredictionGrace + time.Nanosecond),
			wantLocked: true,
		},
		{
			name:       "한참 지난 스탬프는 잠겨 있다",
			now:        stampedAt.Add(24 * time.Hour),
			wantLocked: true,
		},
		{
			/*
				시계가 뒤로 간 경우(서버 NTP 보정, 혹은 DB가 조금 앞선 시각을 준 경우).
				`now - stampedAt`이 음수가 되므로 잠기지 않는다 — 이쪽이 안전한
				방향이다. 못 고치게 막는 것보다 잠깐 더 열어 두는 편이 낫다.
			*/
			name:       "시계가 뒤로 가도 잠기지 않는다",
			now:        stampedAt.Add(-time.Hour),
			wantLocked: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := predictionLocked(stampedAt, tc.now); got != tc.wantLocked {
				t.Fatalf("predictionLocked = %v, 원한 값 %v (경과 %v, 유예 %v)",
					got, tc.wantLocked, tc.now.Sub(stampedAt), stampPredictionGrace)
			}
		})
	}
}

// 유예 시간과 쿨다운이 같은 값이라는 것은 우연이 아니라 설계다 — 한쪽만 바뀌면
// 「다시 찍을 수는 있는데 고칠 수는 없는」 창이 생긴다.
func TestPredictionGraceMatchesCooldown(t *testing.T) {
	if stampPredictionGrace != stampCooldown {
		t.Fatalf("유예 %v ≠ 쿨다운 %v — 둘이 어긋나면 고칠 수도 다시 찍을 수도 없는 창이 생긴다",
			stampPredictionGrace, stampCooldown)
	}
}
