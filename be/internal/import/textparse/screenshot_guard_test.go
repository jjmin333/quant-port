package textparse

import "testing"

func TestRejectNonBrokerageScreenshot_acceptsHoldings(t *testing.T) {
	ocr := []string{`국내주식
삼성전자
8.176.383원
34주
~4,216,61781 (34.02%)`}
	lines := FilterParsedLines(MergeMultiOCRPass(ParseHoldingsText(ocr[0])))
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg != "" {
		t.Fatalf("expected accept, got %q lines=%+v", msg, lines)
	}
}

func TestRejectNonBrokerageScreenshot_acceptsQuoteList(t *testing.T) {
	ocr := []string{`테슬라 $250.12 (+1.23%)
애플 $190.45 (-0.50%)
엔비디아 $880.00 (+2.10%)
마이크로소프트 $420.33 (+0.80%)`}
	lines := FilterQuoteListLines(ParseQuoteListFromOCRTexts(ocr...))
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg != "" {
		t.Fatalf("expected accept, got %q lines=%+v", msg, lines)
	}
}

func TestRejectNonBrokerageScreenshot_rejectsRandomPhoto(t *testing.T) {
	ocr := []string{"sky cloud tree"}
	lines := []ParsedLine{{Name: "sky", Quantity: 0}}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for random photo text")
	}
}

func TestRejectNonBrokerageScreenshot_rejectsDocument(t *testing.T) {
	ocr := []string{`회의록
2026년 1분기 실적 보고
참석자: 김철수, 이영희
다음 주까지 자료를 제출해 주세요.`}
	lines := []ParsedLine{{Name: "회의록", Quantity: 0}, {Name: "김철수", Quantity: 0}}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for generic document")
	}
}

func TestRejectNonBrokerageScreenshot_rejectsSingleGarbageName(t *testing.T) {
	ocr := []string{`Some random caption with numbers 12345 and words`}
	lines := []ParsedLine{{Name: "Some random", Quantity: 0}}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for single garbage row")
	}
}

func TestRejectNonBrokerageScreenshot_rejectsReceiptLikeDollars(t *testing.T) {
	ocr := []string{`Coffee $4.50
Bagel $3.25
Tax $1.00
Total $8.75`}
	lines := []ParsedLine{
		{Name: "Coffee", Quantity: 0},
		{Name: "Bagel", Quantity: 0},
	}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for receipt-like $ lines without quote/holdings signals")
	}
}

func TestRejectNonBrokerageScreenshot_rejectsSocialChat(t *testing.T) {
	ocr := []string{`제노코 094940
메타비아
코미팜
7시간 전
xiobaixu = 미친 흐로_
1441주
될관대`}
	lines := []ParsedLine{
		{Name: "제노코", Quantity: 0},
		{Name: "메타비아", Quantity: 0},
		{Name: "코미팜", Quantity: 0},
		{Name: "7시간 전", Quantity: 0},
		{Name: "xiobaixu = 미친 흐로_", Quantity: 0},
		{Name: "될관대", Quantity: 1441},
	}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for social/chat screenshot")
	}
}

func TestRejectPreviewAfterResolve_rejectsMixedChatAndStocks(t *testing.T) {
	ocr := []string{`제노코
메타비아
코미팜
대동고려삼
7시간 전
xiobaixu = 미친 흐로_
1441주`}
	lines := []ParsedLine{
		{Name: "제노코", Quantity: 0},
		{Name: "메타비아", Quantity: 0},
		{Name: "코미팜", Quantity: 0},
		{Name: "대동고려삼", Quantity: 0},
		{Name: "7시간 전", Quantity: 0},
		{Name: "xiobaixu = 미친 흐로_", Quantity: 0},
		{Name: "점 제억와드0 Boon = Bano", Quantity: 1441},
	}
	preview := []PreviewRowInput{
		{RawName: "제노코", Symbol: "094940", Confidence: 0.9},
		{RawName: "메타비아", Symbol: "089970", Confidence: 0.9},
		{RawName: "코미팜", Symbol: "089890", Confidence: 0.9},
		{RawName: "대동고려삼", Symbol: "046890", Confidence: 0.9},
		{RawName: "7시간 전", Confidence: 0.2},
		{RawName: "xiobaixu = 미친 흐로_", Confidence: 0.2},
		{RawName: "점 제억와드0 Boon = Bano", Quantity: 1441, Confidence: 0.2},
	}
	if msg := RejectPreviewAfterResolve(ocr, lines, preview); msg == "" {
		t.Fatal("expected reject when chat garbage mixed with resolved tickers")
	}
}

func TestRejectPreviewAfterResolve_acceptsRealHoldings(t *testing.T) {
	ocr := []string{`국내주식
삼성전자
8.176.383원
34주
~4,216,61781 (34.02%)`}
	lines := FilterParsedLines(MergeMultiOCRPass(ParseHoldingsText(ocr[0])))
	preview := []PreviewRowInput{
		{RawName: "삼성전자", Symbol: "005930", Confidence: 0.95, Quantity: 34},
	}
	if msg := RejectPreviewAfterResolve(ocr, lines, preview); msg != "" {
		t.Fatalf("expected accept for real holdings, got %q", msg)
	}
}

func TestRejectNonBrokerageScreenshot_rejectsPhantomQtyOnly(t *testing.T) {
	ocr := []string{`random text about stocks 제노코 메타비아
some numbers 1441주 here`}
	lines := []ParsedLine{
		{Name: "제노코", Quantity: 0},
		{Name: "메타비아", Quantity: 1441},
	}
	if msg := RejectNonBrokerageScreenshot(ocr, lines); msg == "" {
		t.Fatal("expected reject for single phantom qty without finance/broker UI")
	}
}
