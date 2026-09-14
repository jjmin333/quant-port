package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
Azure AI Translator — 해외 제목 EN→KO (2026-09-11 사용자 결정).

# 왜 자체 모델을 걷어 냈나

`local.go`는 llama.cpp 컨테이너(LMT-60 1.7B, CPU 전용, --parallel 2)에 붙어 있었다.
그 구조의 문제는 품질이 아니라 **운영**이었다: 제목 한 건에 0.9~2.8초가 들어 2분
주기가 24건을 겨우 넘기고, 컨테이너가 상시 1.5코어·2GB를 잡고, 모델이 막 올라오는
중이면 그 주기의 제목이 통째로 영어로 남았다. 09-08 호스트 크래시 조사에서도 유일한
실질 CPU 소비자로 지목된 것이 이 컨테이너였다.

# 무엇이 달라지나

한 요청에 제목을 **묶어서** 싣는다(최대 100건). 왕복 한 번이 곧 한 주기 전체라
슬롯 나눠 쓰기가 필요 없고, 실패는 요청 단위로 한 번만 판정된다.

대신 **한도가 생겼다.** F0 무료 등급은 월 200만 자이고 일일 한도가 없다. 실측
(2026-09-08~11) 제목 번역량은 하루 59K~111K자 = 월 약 3.3M자로 그 한도를 넘으므로,
수집량을 한도에 맞춰 줄이기로 했다(사용자 결정). 이 파일은 그 결정을 **강제하지
않는다** — 대신 `apibudget`에 문자를 세어 두고, 넘으면 그 묶음을 보내지 않는다.
보내지 않은 제목은 영어로 남는다(`Chain` 머리글의 규칙 그대로).

# 문자를 언제 세나

**보내기 전에** 센다(`CheckTokens`의 estimate). Azure는 응답에 사용량을 담지 않고,
과금은 **보낸 원문 문자 수** 기준이다 — 그래서 우리가 보낸 길이가 곧 청구서다.
성공 여부와 무관하게 기록하는 이유도 같다: 400을 받아도 상대는 이미 셌을 수 있다.

# 왜 Accept를 이 파일에서 부르지 않나

`Chain`이 부른다. 엔진은 「옮긴 결과」만 돌려주고 받아들일지는 사슬이 정한다 —
그래야 엔진을 갈아 끼울 때 검증 규칙이 갈라지지 않는다.
*/
type Azure struct {
	endpoint string
	key      string
	region   string
	client   *http.Client
	budget   *apibudget.Manager
}

const (
	azureDefaultEndpoint = "https://api.cognitive.microsofttranslator.com"
	// 한 요청에 실을 제목 수. Azure의 문서 상한은 1,000건/50,000자인데, 100건으로
	// 낮춘 이유는 실패 단위를 작게 두기 위해서다 — 한 요청이 실패하면 그 안의 제목이
	// 전부 영어로 남으므로, 한 번에 다 싣는 것이 곧 「한 번 실패하면 전부 실패」다.
	azureMaxBatch = 100
	// 한 요청에 보낼 원문 문자 수 상한. 상대 상한 50,000의 절반 — 제목 하나가
	// 평균 71자(실측)라 100건이면 7천 자 남짓이고, 이 값에 걸리는 일은 사실상 없다.
	azureMaxChars = 25_000
)

// NewAzure builds the engine. An empty key makes it unavailable, which keeps
// it out of the chain entirely (NewChain drops unavailable engines).
func NewAzure(endpoint, key, region string, timeout time.Duration, budget *apibudget.Manager) *Azure {
	endpoint = strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if endpoint == "" {
		endpoint = azureDefaultEndpoint
	}
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &Azure{
		endpoint: endpoint,
		key:      strings.TrimSpace(key),
		region:   strings.TrimSpace(region),
		client:   &http.Client{Timeout: timeout},
		budget:   budget,
	}
}

func (a *Azure) Name() string { return "azure" }

func (a *Azure) Available() bool { return a != nil && a.key != "" }

type azureTranslateResponse struct {
	Translations []struct {
		Text string `json:"text"`
		To   string `json:"to"`
	} `json:"translations"`
}

type azureErrorResponse struct {
	Error struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

/*
TranslateENtoKO — 입력과 **같은 길이**의 슬라이스를 돌려준다.

빈 입력 자리는 빈 출력이다. 예산에 걸리거나 요청이 실패한 자리도 빈 출력이라,
호출부는 「옮기지 못했다」를 한 가지 모양으로만 만난다.
*/
func (a *Azure) TranslateENtoKO(ctx context.Context, texts []string) ([]string, error) {
	out := make([]string, len(texts))
	if !a.Available() || len(texts) == 0 {
		return out, nil
	}

	// 빈 문자열은 보내지 않는다 — 상대가 400을 준다.
	idx := make([]int, 0, len(texts))
	for i, t := range texts {
		if strings.TrimSpace(t) != "" {
			idx = append(idx, i)
		}
	}
	if len(idx) == 0 {
		return out, nil
	}

	var firstErr error
	for _, chunk := range azureChunks(texts, idx) {
		if ctx.Err() != nil {
			return out, ctx.Err()
		}
		translated, err := a.translateChunk(ctx, texts, chunk)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		for n, at := range chunk {
			if n < len(translated) {
				out[at] = translated[n]
			}
		}
	}
	return out, firstErr
}

// azureChunks splits the non-empty indices into request-sized groups, bounded
// by both count and source characters.
func azureChunks(texts []string, idx []int) [][]int {
	var chunks [][]int
	var cur []int
	chars := 0
	for _, at := range idx {
		n := len([]rune(texts[at]))
		if len(cur) > 0 && (len(cur) >= azureMaxBatch || chars+n > azureMaxChars) {
			chunks = append(chunks, cur)
			cur, chars = nil, 0
		}
		cur = append(cur, at)
		chars += n
	}
	if len(cur) > 0 {
		chunks = append(chunks, cur)
	}
	return chunks
}

func (a *Azure) translateChunk(ctx context.Context, texts []string, chunk []int) ([]string, error) {
	body := make([]map[string]string, 0, len(chunk))
	chars := int64(0)
	for _, at := range chunk {
		body = append(body, map[string]string{"Text": texts[at]})
		chars += int64(len([]rune(texts[at])))
	}

	// 예산은 **보내기 전에** 본다. 문자를 토큰 카운터로 센다 — 창(월)과 셈이 같다.
	if a.budget != nil {
		if err := a.budget.CheckTokens(ctx, apibudget.ProviderAzureTranslate, chars); err != nil {
			return nil, fmt.Errorf("azure translate budget: %w", err)
		}
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("azure translate marshal: %w", err)
	}

	u := a.endpoint + "/translate?" + url.Values{
		"api-version": {"3.0"},
		"from":        {"en"},
		"to":          {"ko"},
		"textType":    {"plain"},
	}.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("azure translate request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")
	req.Header.Set("Ocp-Apim-Subscription-Key", a.key)
	if a.region != "" {
		// 전역 엔드포인트가 아닌 지역 리소스는 이 헤더가 없으면 401을 준다.
		req.Header.Set("Ocp-Apim-Subscription-Region", a.region)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("azure translate do: %w", err)
	}
	defer resp.Body.Close()

	// 보낸 문자는 성공 여부와 무관하게 기록한다 — 상대는 이미 셌을 수 있다.
	if a.budget != nil {
		a.budget.Record(ctx, apibudget.ProviderAzureTranslate)
		a.budget.RecordTokens(ctx, apibudget.ProviderAzureTranslate, chars)
	}

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("azure translate read: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		var apiErr azureErrorResponse
		if json.Unmarshal(raw, &apiErr) == nil && apiErr.Error.Message != "" {
			return nil, fmt.Errorf("azure translate %d: %s (code %d)", resp.StatusCode, apiErr.Error.Message, apiErr.Error.Code)
		}
		return nil, fmt.Errorf("azure translate %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var decoded []azureTranslateResponse
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, fmt.Errorf("azure translate decode: %w", err)
	}
	if len(decoded) != len(chunk) {
		// 길이가 어긋나면 제목이 엉뚱한 기사에 붙는다 — Translator 인터페이스의 규약.
		return nil, fmt.Errorf("azure translate: got %d results for %d texts", len(decoded), len(chunk))
	}

	out := make([]string, len(chunk))
	for i, item := range decoded {
		if len(item.Translations) > 0 {
			out[i] = strings.TrimSpace(item.Translations[0].Text)
		}
	}
	return out, nil
}

// ErrAzureNotConfigured is returned by Verify when there is no key to test.
var ErrAzureNotConfigured = errors.New("azure translator key not configured")

/*
Verify — 키·지역이 실제로 먹는지 한 번 물어본다.

기동 로그와 관리자 화면용이다. 요청마다 부르지 않는다 — 그러면 왕복이 두 배가 되고,
무엇보다 문자를 두 번 센다.
*/
func (a *Azure) Verify(ctx context.Context) error {
	if !a.Available() {
		return ErrAzureNotConfigured
	}
	out, err := a.TranslateENtoKO(ctx, []string{"stock"})
	if err != nil {
		return err
	}
	if strings.TrimSpace(out[0]) == "" {
		return errors.New("azure translator returned an empty translation")
	}
	return nil
}
