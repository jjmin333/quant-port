package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/translate"
)

func TestIsUSTranslationReady(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		a    model.NewsArticle
		want bool
	}{
		{
			name: "english pending",
			a:    model.NewsArticle{Market: "us", Headline: "Nasdaq rises on tech earnings"},
			want: false,
		},
		{
			name: "translated headline",
			a: model.NewsArticle{
				Market:     "us",
				Headline:   "Nasdaq rises on tech earnings",
				HeadlineKo: "나스닥, 기술주 실적으로 상승",
			},
			want: true,
		},
		{
			name: "domestic ignored",
			a:    model.NewsArticle{Market: "domestic", Headline: "코스피 상승"},
			want: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isUSTranslationReady(tc.a); got != tc.want {
				t.Fatalf("isUSTranslationReady() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestNeedsEnglishTranslation(t *testing.T) {
	t.Parallel()

	if needsEnglishTranslation("코스피 상승", "") {
		t.Fatal("expected Korean headline to skip translation")
	}
	if !needsEnglishTranslation("Nasdaq rises", "") {
		t.Fatal("expected English headline to need translation")
	}
}

// 번역기가 안 붙어 있으면 제목은 **영어로 남는다** — 사라지는 것이 아니다.
func TestApplyUSTranslationLeavesEnglishWithoutTranslator(t *testing.T) {
	t.Parallel()

	st := &NewsStore{}
	article := model.NewsArticle{Market: "us", Headline: "Nasdaq rises on tech earnings"}
	st.applyUSTranslation(t.Context(), &article)
	if article.HeadlineKo != "" {
		t.Fatalf("번역기가 없으면 영어 제목이 그대로 남아야 한다, got %q", article.HeadlineKo)
	}
}

func TestApplyUSTranslationSetsKoreanHeadlineWithoutTranslator(t *testing.T) {
	t.Parallel()

	st := &NewsStore{}
	article := model.NewsArticle{
		Market:   "us",
		Headline: "코스피 연동 해외 증시 속보",
	}
	st.applyUSTranslation(t.Context(), &article)
	if article.HeadlineKo != article.Headline {
		t.Fatalf("expected headline copied to headlineKo, got %q", article.HeadlineKo)
	}
}

/*
번역 엔진 흉내 — 몇 건을 **한 번에** 받았는지 센다.

이 파일이 세는 것이 「요청 수」인 이유는 2026-09-13~14 사고다. 제목 한 건이 곧
요청 한 번이어서 스물넷을 옮기는 데 왕복이 스물네 번 났다(하루 12,559회).
*/
type countingTranslator struct {
	name  string
	out   []string
	err   error
	calls int
	sizes []int
}

func (c *countingTranslator) Name() string    { return c.name }
func (c *countingTranslator) Available() bool { return true }
func (c *countingTranslator) TranslateENtoKO(_ context.Context, texts []string) ([]string, error) {
	c.calls++
	c.sizes = append(c.sizes, len(texts))
	if c.err != nil {
		return make([]string, len(texts)), c.err
	}
	out := make([]string, len(texts))
	copy(out, c.out)
	return out, nil
}

// 제목 셋이면 요청은 **하나**다.
func TestTranslateHeadlinesSendsOneRequestForTheWholeBatch(t *testing.T) {
	t.Parallel()

	eng := &countingTranslator{
		name: "fake",
		out:  []string{"엔비디아 주가 하락", "연준 금리 동결", "애플 신제품 공개"},
	}
	st := &NewsStore{translator: translate.NewChain(eng)}

	out, blocked := st.translateHeadlines(t.Context(),
		[]string{"Nvidia stock slips", "Fed holds rates steady", "Apple unveils new product"},
		nil)

	if blocked {
		t.Fatal("정상 응답인데 blocked 로 쳤다")
	}
	if eng.calls != 1 {
		t.Fatalf("요청이 %d번 나갔다 — 묶음 하나면 한 번이어야 한다 (%v)", eng.calls, eng.sizes)
	}
	if len(eng.sizes) != 1 || eng.sizes[0] != 3 {
		t.Fatalf("한 요청에 실린 건수: %v", eng.sizes)
	}
	for i, want := range []string{"엔비디아 주가 하락", "연준 금리 동결", "애플 신제품 공개"} {
		if out[i] != want {
			t.Fatalf("%d번 결과 %q, want %q", i, out[i], want)
		}
	}
}

/*
회사 이름은 잠기고 되돌아온다 — 그리고 **되돌리지 못하면 그 자리는 비운다.**

첫 제목은 자리표시자가 온전하고, 둘째는 번역기가 그것을 지웠다. 지운 쪽을 그대로
내보내면 「…의 신호가 위험을 시사한다」처럼 주어 없는 제목이 된다.
*/
func TestTranslateHeadlinesRestoresNamesAndDropsWhatItCannot(t *testing.T) {
	t.Parallel()

	masked, _ := translate.MaskNames("Nvidia stock slips",
		[]translate.NamePair{{English: "Nvidia", Korean: "엔비디아"}})
	placeholder := strings.TrimSuffix(masked, " stock slips")

	eng := &countingTranslator{
		name: "fake",
		out: []string{
			placeholder + " 주가 하락", // 자리표시자를 그대로 옮겨 왔다
			"주가 하락",                // 자리표시자를 지웠다 — 어느 회사인지 사라졌다
		},
	}
	st := &NewsStore{translator: translate.NewChain(eng)}

	pairs := [][]translate.NamePair{
		{{English: "Nvidia", Korean: "엔비디아"}},
		{{English: "Micron", Korean: "마이크론"}},
	}
	out, blocked := st.translateHeadlines(t.Context(),
		[]string{"Nvidia stock slips", "Micron stock slips"}, pairs)

	if blocked {
		t.Fatal("blocked 가 아니어야 한다")
	}
	if out[0] != "엔비디아 주가 하락" {
		t.Fatalf("이름을 못 되돌렸다: %q", out[0])
	}
	if out[1] != "" {
		t.Fatalf("자리표시자가 사라졌는데 내보냈다: %q", out[1])
	}
}

/*
예산에 걸려 **못 보낸** 회차는 blocked 다.

이 구분이 재시도 상한의 전부다. 한도가 찬 날을 「이 제목은 옮길 수 없다」로 세면,
하루 형편 때문에 그 제목들의 기회가 영구히 타 버린다.
*/
func TestTranslateHeadlinesReportsBlockedWhenTheEngineNeverAnswered(t *testing.T) {
	t.Parallel()

	eng := &countingTranslator{name: "fake", err: errors.New("azure translate budget: exhausted")}
	st := &NewsStore{translator: translate.NewChain(eng)}

	out, blocked := st.translateHeadlines(t.Context(), []string{"Nvidia stock slips"}, nil)
	if !blocked {
		t.Fatal("엔진이 대답을 못 했는데 blocked 가 아니다")
	}
	if out[0] != "" {
		t.Fatalf("못 옮긴 자리는 비어야 한다: %q", out[0])
	}

	// 번역기가 아예 없을 때도 같다 — 「시도했다」로 세면 안 된다.
	if _, blocked := (&NewsStore{}).translateHeadlines(t.Context(), []string{"x"}, nil); !blocked {
		t.Fatal("번역기가 없는데 blocked 가 아니다")
	}
}
