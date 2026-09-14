package translate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

func azureStub(t *testing.T, handler func(texts []string) any) (*httptest.Server, *[]int) {
	t.Helper()
	sizes := make([]int, 0, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Ocp-Apim-Subscription-Key") == "" {
			t.Errorf("키 헤더가 없다")
		}
		if got := r.URL.Query().Get("to"); got != "ko" {
			t.Errorf("to=ko 여야 한다: %q", got)
		}
		raw, _ := io.ReadAll(r.Body)
		var in []map[string]string
		if err := json.Unmarshal(raw, &in); err != nil {
			t.Errorf("본문이 배열이 아니다: %v", err)
		}
		texts := make([]string, 0, len(in))
		for _, item := range in {
			texts = append(texts, item["Text"])
		}
		sizes = append(sizes, len(texts))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(handler(texts))
	}))
	t.Cleanup(srv.Close)
	return srv, &sizes
}

func echoKorean(texts []string) any {
	out := make([]azureTranslateResponse, 0, len(texts))
	for _, txt := range texts {
		item := azureTranslateResponse{}
		item.Translations = append(item.Translations, struct {
			Text string `json:"text"`
			To   string `json:"to"`
		}{Text: "한국어 " + txt, To: "ko"})
		out = append(out, item)
	}
	return out
}

func TestAzure_translatesAndKeepsInputOrder(t *testing.T) {
	srv, _ := azureStub(t, echoKorean)
	a := NewAzure(srv.URL, "k", "koreacentral", time.Second, nil)

	got, err := a.TranslateENtoKO(context.Background(), []string{"alpha", "beta", "gamma"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"한국어 alpha", "한국어 beta", "한국어 gamma"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("자리 %d: %q, want %q", i, got[i], want[i])
		}
	}
}

func TestAzure_emptyInputsKeepTheirSlotAndAreNotSent(t *testing.T) {
	// 빈 문자열을 보내면 상대가 400을 준다. 그 자리는 빈 출력으로 남아야 하고,
	// 무엇보다 **출력 길이가 입력과 같아야** 한다 — 다르면 제목이 엉뚱한 기사에 붙는다.
	srv, _ := azureStub(t, func(texts []string) any {
		for _, txt := range texts {
			if strings.TrimSpace(txt) == "" {
				t.Errorf("빈 문자열이 전송됐다")
			}
		}
		return echoKorean(texts)
	})
	a := NewAzure(srv.URL, "k", "", time.Second, nil)

	in := []string{"", "beta", "   "}
	got, err := a.TranslateENtoKO(context.Background(), in)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(in) {
		t.Fatalf("길이가 입력과 같아야 한다: %d vs %d", len(got), len(in))
	}
	if got[0] != "" || got[2] != "" {
		t.Fatalf("빈 자리는 빈 출력이어야 한다: %#v", got)
	}
	if got[1] != "한국어 beta" {
		t.Fatalf("두 번째 자리가 틀렸다: %q", got[1])
	}
}

func TestAzure_splitsIntoRequestSizedBatches(t *testing.T) {
	srv, sizes := azureStub(t, echoKorean)
	a := NewAzure(srv.URL, "k", "", time.Second, nil)

	texts := make([]string, azureMaxBatch+5)
	for i := range texts {
		texts[i] = "headline"
	}
	if _, err := a.TranslateENtoKO(context.Background(), texts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(*sizes) != 2 {
		t.Fatalf("묶음이 둘로 갈려야 한다: %v", *sizes)
	}
	if (*sizes)[0] != azureMaxBatch || (*sizes)[1] != 5 {
		t.Fatalf("묶음 크기가 틀렸다: %v", *sizes)
	}
}

func TestAzure_monthlyCharBudgetBlocksTheBatch(t *testing.T) {
	srv, sizes := azureStub(t, echoKorean)
	budget := apibudget.NewManager(nil)
	budget.SetLimits(apibudget.ProviderAzureTranslate, apibudget.Limits{TokensPerMonth: 10})
	a := NewAzure(srv.URL, "k", "", time.Second, budget)

	// 원문 11자 > 남은 10자 → 보내지 않는다.
	got, err := a.TranslateENtoKO(context.Background(), []string{"12345678901"})
	if err == nil {
		t.Fatal("예산 초과는 오류로 올라와야 한다 (사슬이 다음 엔진으로 내려간다)")
	}
	if got[0] != "" {
		t.Fatalf("보내지 않은 제목은 빈 출력이어야 한다: %q", got[0])
	}
	if len(*sizes) != 0 {
		t.Fatalf("한도를 넘겼는데 요청이 나갔다: %v", *sizes)
	}
}

func TestAzure_recordsCharactersItSent(t *testing.T) {
	srv, _ := azureStub(t, echoKorean)
	budget := apibudget.NewManager(nil)
	budget.SetLimits(apibudget.ProviderAzureTranslate, apibudget.Limits{TokensPerMonth: 2_000_000})
	a := NewAzure(srv.URL, "k", "", time.Second, budget)

	ctx := context.Background()
	before := budget.Headroom(ctx, apibudget.ProviderAzureTranslate).MonthTokens
	if _, err := a.TranslateENtoKO(ctx, []string{"abcde"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := budget.Headroom(ctx, apibudget.ProviderAzureTranslate).MonthTokens
	if before-after != 5 {
		t.Fatalf("보낸 원문 문자 수(5)만큼 줄어야 한다: %d → %d", before, after)
	}
}

func TestAzure_unavailableWithoutAKey(t *testing.T) {
	// 키가 없으면 사슬에 아예 안 붙는다 — 조용히 자체 모델로 떨어지지 않는다.
	a := NewAzure("", "", "", time.Second, nil)
	if a.Available() {
		t.Fatal("키 없는 엔진이 available 이다")
	}
	got, err := a.TranslateENtoKO(context.Background(), []string{"x"})
	if err != nil || got[0] != "" {
		t.Fatalf("키 없으면 조용히 빈 결과여야 한다: %v %#v", err, got)
	}
}

func TestAzure_rejectsLengthMismatch(t *testing.T) {
	// 응답 길이가 어긋나면 제목이 엉뚱한 기사에 붙는다 — Translator 인터페이스의 규약.
	srv, _ := azureStub(t, func(texts []string) any {
		return echoKorean(texts[:1])
	})
	a := NewAzure(srv.URL, "k", "", time.Second, nil)

	got, err := a.TranslateENtoKO(context.Background(), []string{"a", "b"})
	if err == nil {
		t.Fatal("길이 불일치는 오류여야 한다")
	}
	if got[0] != "" || got[1] != "" {
		t.Fatalf("불일치 응답은 버려야 한다: %#v", got)
	}
}

func TestAzure_buildDropsAzureWithoutAKey(t *testing.T) {
	chain := Build(Options{Providers: []string{"azure"}}, nil)
	if chain.Available() {
		t.Fatal("키 없는 azure 하나만으로는 사슬이 살아 있을 수 없다")
	}
}

func TestParseOptions_defaultsToAzure(t *testing.T) {
	opts := ParseOptions(func(string) string { return "" })
	if len(opts.Providers) != 1 || opts.Providers[0] != "azure" {
		t.Fatalf("기본 엔진은 azure 여야 한다: %v", opts.Providers)
	}
}
