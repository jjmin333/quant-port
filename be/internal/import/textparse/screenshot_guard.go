package textparse

import (
	"regexp"
	"strings"
	"unicode"
)

// NonBrokerageScreenshotMsg is shown when OCR output does not look like a brokerage holdings/quote screen.
const NonBrokerageScreenshotMsg = "증권사 보유 화면을 스크린샷해 주세요."

var brokerageUIKeywords = regexp.MustCompile(`(?i)(잔고|보유\s*종목|평가금|평가\s*손|손익|국내주|해외주|증권|MY\s*투자|내\s*주식|Portfolio|Holdings|Balance|Market\s*Value|Total\s*Value|P&L|Shares|총\s*평가|수익률|평가\s*손익|매입금|평균단가|평단)`)

var socialChatOCRNoise = regexp.MustCompile(`(?i)(시간\s*전|분\s*전|일\s*전|좋아요|댓글|공유하기|팔로우|구독|좋아|조회수|repost|retweet|facebook|instagram|tiktok|youtube|카카오톡|telegram|discord)`)

var garbagePreviewName = regexp.MustCompile(`(?i)(시간\s*전|분\s*전|일\s*전|xiobaixu|Boon\s*=\s*Bano)`)

const minBrokerOCRRunes = 18

// RejectNonBrokerageScreenshot returns a user-facing message when OCR text/lines do not resemble a brokerage screen.
func RejectNonBrokerageScreenshot(ocrTexts []string, lines []ParsedLine) string {
	combined := strings.Join(ocrTexts, "\n")
	trimmed := strings.TrimSpace(combined)
	if len([]rune(trimmed)) < minBrokerOCRRunes {
		return NonBrokerageScreenshotMsg
	}
	if len(lines) == 0 {
		return NonBrokerageScreenshotMsg
	}
	if looksLikeSocialChatScreen(combined) && !hasBrokerageUI(combined) {
		return NonBrokerageScreenshotMsg
	}
	if suspicious := countSuspiciousLineNames(lines); suspicious >= 1 {
		return NonBrokerageScreenshotMsg
	}
	if allLinesHaveQtyWithoutFinance(lines) && !hasBrokerageUI(combined) {
		return NonBrokerageScreenshotMsg
	}

	if InferQuoteListOnly(ocrTexts...) {
		if validateQuoteListScreenshot(ocrTexts, lines) {
			return ""
		}
		return NonBrokerageScreenshotMsg
	}

	if validateHoldingsScreenshot(combined, lines) {
		return ""
	}

	return NonBrokerageScreenshotMsg
}

// RejectPreviewAfterResolve applies stricter checks once symbols are resolved (catches chat/community screenshots).
func RejectPreviewAfterResolve(ocrTexts []string, lines []ParsedLine, preview []PreviewRowInput) string {
	if msg := RejectNonBrokerageScreenshot(ocrTexts, lines); msg != "" {
		return msg
	}
	if len(preview) == 0 {
		return NonBrokerageScreenshotMsg
	}

	combined := strings.Join(ocrTexts, "\n")
	brokerUI := hasBrokerageUI(combined)

	resolved := 0
	garbage := 0
	qtyRows := 0
	for _, row := range preview {
		if row.Symbol != "" && row.Confidence >= 0.65 {
			resolved++
		}
		if looksLikeGarbagePreviewName(row.RawName) {
			garbage++
		}
		if row.Quantity > 0 {
			qtyRows++
		}
	}
	total := len(preview)

	if !brokerUI {
		if resolved < 2 {
			return NonBrokerageScreenshotMsg
		}
		if float64(resolved)/float64(total) < 0.55 {
			return NonBrokerageScreenshotMsg
		}
		if garbage >= 1 {
			return NonBrokerageScreenshotMsg
		}
		if qtyRows > 0 && !holdingsFinanceEvidence(lines) {
			return NonBrokerageScreenshotMsg
		}
	}

	if garbage > 0 && float64(garbage)/float64(total) >= 0.2 {
		return NonBrokerageScreenshotMsg
	}
	return ""
}

// PreviewRowInput is the minimal preview fields used by post-resolve validation.
type PreviewRowInput struct {
	RawName    string
	Symbol     string
	Confidence float64
	Quantity   float64
}

func hasBrokerageUI(combined string) bool {
	return brokerageUIKeywords.MatchString(combined)
}

func looksLikeSocialChatScreen(combined string) bool {
	return socialChatOCRNoise.MatchString(combined)
}

func looksLikeGarbagePreviewName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	if garbagePreviewName.MatchString(name) {
		return true
	}
	if strings.Contains(name, "=") {
		return true
	}
	if looksLikeImportUIArtifact(name) {
		return true
	}
	if LooksLikeOCRFragment(name) {
		return true
	}
	return false
}

func looksLikeImportUIArtifact(name string) bool {
	return strings.Contains(name, "검색/코드") ||
		strings.Contains(name, "종목코드") ||
		strings.Contains(name, "평단가") ||
		strings.Contains(name, "확인 필요") ||
		strings.Contains(name, "스크린샷")
}

func countSuspiciousLineNames(lines []ParsedLine) int {
	n := 0
	for _, l := range lines {
		if looksLikeGarbagePreviewName(l.Name) {
			n++
		}
	}
	return n
}

func allLinesHaveQtyWithoutFinance(lines []ParsedLine) bool {
	if len(lines) < 2 {
		return false
	}
	qty := 0
	for _, l := range lines {
		if l.Quantity > 0 {
			qty++
		}
		if lineHasFinance(l) {
			return false
		}
	}
	return qty >= 2
}

func holdingsFinanceEvidence(lines []ParsedLine) bool {
	finance := 0
	qty := 0
	for _, l := range lines {
		if l.Quantity > 0 {
			qty++
		}
		if lineHasFinance(l) {
			finance++
		}
	}
	return finance >= 1 || (qty >= 2 && finance >= 1)
}

func validateQuoteListScreenshot(ocrTexts []string, lines []ParsedLine) bool {
	names := countQuoteNameLines(lines)
	if names < 2 {
		return false
	}
	combined := strings.Join(ocrTexts, "\n")
	dollars := strings.Count(combined, "$")
	pctCount := len(returnPctInParens.FindAllString(combined, -1))
	if names >= 3 && dollars >= 3 {
		return true
	}
	if names >= 2 && dollars >= 3 && pctCount >= 2 {
		return true
	}
	if hasBrokerageUI(combined) && names >= 2 && dollars >= 2 {
		return true
	}
	return false
}

func validateHoldingsScreenshot(combined string, lines []ParsedLine) bool {
	qtyLines := 0
	financeLines := 0
	for _, l := range lines {
		if l.Quantity > 0 {
			qtyLines++
		}
		if lineHasFinance(l) {
			financeLines++
		}
	}

	brokerUI := hasBrokerageUI(combined)
	hasKRW := strings.Contains(combined, "원") && krwMoneyToken.MatchString(combined)
	hasUSD := strings.Count(combined, "$") >= 1

	if qtyLines >= 1 && financeLines >= 1 {
		return true
	}
	if qtyLines >= 2 && (financeLines >= 1 || hasKRW || brokerUI) {
		return true
	}
	if brokerUI && qtyLines >= 1 && qtyPattern.MatchString(combined) {
		return true
	}
	if qtyLines >= 2 && qtyPattern.MatchString(combined) && hasKRW {
		return true
	}
	if qtyLines >= 1 && hasKRW && financeLines >= 1 {
		return true
	}
	if qtyLines >= 1 && hasUSD && financeLines >= 1 {
		return true
	}
	if financeLines >= 2 && countCredibleLines(lines) >= 2 {
		return true
	}
	return false
}

func countQuoteNameLines(lines []ParsedLine) int {
	n := 0
	for _, l := range lines {
		if l.Quantity != 0 {
			continue
		}
		if !isNameOnlyCandidate(l.Name) {
			continue
		}
		if len([]rune(hangulRunesOnly(l.Name))) >= 2 || latinLetterRunes(l.Name) >= 3 {
			n++
		}
	}
	return n
}

func countCredibleLines(lines []ParsedLine) int {
	n := 0
	for _, l := range lines {
		name := strings.TrimSpace(l.Name)
		if name == "" || !isNameOnlyCandidate(name) {
			continue
		}
		letters := 0
		for _, r := range name {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters >= 2 {
			n++
		}
	}
	return n
}
