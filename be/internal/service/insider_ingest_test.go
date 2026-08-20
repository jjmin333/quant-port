package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
)

// 같은 날 같은 보고자의 서로 다른 보고가 다른 키를 받아야 한다 — 같은 키가
// 나오면 한 건이 다른 건을 덮어써 이력이 조용히 사라진다.
func TestDartInsiderFilingKeyDistinguishesSameDayFilings(t *testing.T) {
	a := dartInsiderFilingKey("20260210", "홍길동", "대표이사", "1000", "100")
	b := dartInsiderFilingKey("20260210", "홍길동", "대표이사", "2000", "1000")
	if a == b {
		t.Fatalf("같은 날 다른 수량의 보고가 같은 키를 받았다: %s", a)
	}

	// 반대로 같은 내용은 재실행에도 같은 키여야 upsert가 행을 복제하지 않는다.
	if again := dartInsiderFilingKey("20260210", "홍길동", "대표이사", "1000", "100"); again != a {
		t.Fatalf("같은 내용이 다른 키를 받았다: %s vs %s", a, again)
	}

	// 공백만 다른 값도 같은 키여야 한다 — DART 응답은 항목마다 공백이 섞인다.
	if padded := dartInsiderFilingKey(" 20260210", "홍길동 ", "대표이사", "1000", "100"); padded != a {
		t.Fatalf("공백 차이로 키가 갈렸다: %s vs %s", a, padded)
	}
}

// 배치 한 번의 결과 요약. 개별 종목 실패는 잡 실패가 아니지만, 전부 실패하면
// 종목이 아니라 잡의 문제이므로 에러여야 한다.
func TestInsiderRunStatsFinish(t *testing.T) {
	partial := &ingestStats{ok: 3, empty: 1, errored: 1, rows: 12, firstErr: "AAPL: boom"}
	result, _, err := partial.finish("insider_edgar", 5)
	if err != nil {
		t.Fatalf("일부 실패는 잡 실패가 아니어야 한다: %v", err)
	}
	if result["rows"] != 12 || result["errors"] != 1 {
		t.Fatalf("결과 집계가 맞지 않다: %v", result)
	}
	if result["firstError"] != "AAPL: boom" {
		t.Fatalf("첫 실패 사유가 결과에 없다: %v", result)
	}

	total := &ingestStats{errored: 4, firstErr: "AAPL: rate limited"}
	if _, _, err := total.finish("insider_edgar", 4); err == nil {
		t.Fatal("전 종목 실패는 잡 실패여야 한다")
	}

	// 대상이 0건인 실행(전부 최근에 받은 상태)은 성공이다.
	if _, _, err := newIngestStats().finish("insider_dart", 0); err != nil {
		t.Fatalf("대상 0건은 성공이어야 한다: %v", err)
	}
}

// 상태 분류가 화면 문구를 가른다: 'empty'는 "신고 내역 없음", 미기록은
// "아직 수집 전". 둘을 섞으면 배치가 안 온 종목이 "내역 없음"으로 보인다.
func TestInsiderRunStatsCountsByStatus(t *testing.T) {
	st := newIngestStats()
	for _, status := range []string{
		model.SyncOK, model.SyncOK,
		model.SyncEmpty,
		model.SyncNotFound,
		model.SyncError,
	} {
		switch status {
		case model.SyncOK:
			st.ok++
		case model.SyncEmpty:
			st.empty++
		case model.SyncNotFound:
			st.notFound++
		default:
			st.errored++
		}
	}
	result, _, _ := st.finish("insider_edgar", 5)
	if result["withData"] != 2 || result["empty"] != 1 || result["notFound"] != 1 || result["errors"] != 1 {
		t.Fatalf("상태별 집계가 맞지 않다: %v", result)
	}
}
