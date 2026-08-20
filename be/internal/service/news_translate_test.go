package service

import (
	"testing"

	"github.com/enact/quant-port/be/internal/model"
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

func TestApplyUSTranslationLeavesEnglishWhenPapagoOff(t *testing.T) {
	t.Parallel()

	st := &NewsStore{}
	article := model.NewsArticle{Market: "us", Headline: "Nasdaq rises on tech earnings"}
	st.applyUSTranslation(t.Context(), &article)
	if article.HeadlineKo != "" {
		t.Fatalf("english headline without papago must stay untranslated, got %q", article.HeadlineKo)
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
