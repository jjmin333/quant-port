package translate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

/*
자체 모델 번역기 — llama.cpp 서버의 OpenAI 호환 엔드포인트에 붙는다.

**우리 래퍼를 하나 더 만들지 않았다.** llama.cpp가 이미 `/v1/chat/completions`를
내주고, 그건 vLLM·Ollama·TGI도 내는 사실상의 표준이라 엔진을 바꿔도 여기 코드가
아니라 URL만 바뀐다. 중간에 FastAPI를 한 겹 두면 Python 런타임과 실패 지점이
하나씩 더 늘 뿐이다.

모델은 NiuTrans **LMT-60** — Qwen3 기반의 번역 전용 모델(Apache-2.0). 프롬프트는
모델 카드가 정한 형태 그대로다:

	Translate the following text from English into Korean:
	English: <원문>
	Korean:

**0.6B가 아니라 1.7B를 쓴다.** 설계서에서는 0.6B(T1)로 시작하고 모자라면 올린다고
적었는데, 실측에서 0.6B가 회사 이름을 지어냈다 — "Nvidia stock slips…"가
「에버넷의 주가가 하락」이 되고 "Broadcom"이 「보트콤」이 됐다. "Fed holds rates
steady"는 「금리 인상」으로 **뜻이 뒤집혔다**. 1.7B는 같은 세 문장을 엔비디아·
브로드컴·「금리 안정 유지」로 옳게 옮겼고, **속도는 오히려 비슷했다**(실측 856~1,158ms
대 988~1,615ms). 배치라서 초 단위 차이는 값이 아니다 — 틀린 번역이 값이다.
*/
type Local struct {
	baseURL string
	client  *http.Client
	// slots는 동시에 던지는 요청 수. 컨테이너의 `--parallel`과 같아야 의미가 있다 —
	// 더 던지면 llama.cpp가 큐에 세울 뿐이고, 적게 던지면 슬롯이 논다.
	slots int
}

func NewLocal(baseURL string, slots int, timeout time.Duration) *Local {
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if slots <= 0 {
		slots = 2
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Local{
		baseURL: baseURL,
		client:  &http.Client{Timeout: timeout},
		slots:   slots,
	}
}

func (l *Local) Name() string { return "local" }

func (l *Local) Available() bool { return l != nil && l.baseURL != "" }

// Healthy reports whether the container is up and has the model loaded.
//
// 부르는 쪽이 이걸 먼저 보고 폴백을 고르지는 않는다 — 그러면 요청마다 헬스체크가
// 하나씩 더 붙는다. 관리 화면과 기동 로그용이다.
func (l *Local) Healthy(ctx context.Context) bool {
	if !l.Available() {
		return false
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.baseURL+"/health", nil)
	if err != nil {
		return false
	}
	res, err := l.client.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	_, _ = io.Copy(io.Discard, res.Body)
	return res.StatusCode == http.StatusOK
}

// TranslateENtoKO translates each text, running up to `slots` requests at once.
//
// 한 건이 실패해도 나머지는 그대로 간다 — 스물넷을 묶어 보내는데 하나 때문에
// 전부를 버리면, 그 배치는 다음 2분에도 같은 하나에서 또 죽는다.
func (l *Local) TranslateENtoKO(ctx context.Context, texts []string) ([]string, error) {
	out := make([]string, len(texts))
	if !l.Available() || len(texts) == 0 {
		return out, nil
	}

	sem := make(chan struct{}, l.slots)
	var wg sync.WaitGroup
	for i, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}
		wg.Add(1)
		go func(i int, text string) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }()
			if ko, err := l.translateOne(ctx, text); err == nil {
				out[i] = ko
			}
		}(i, text)
	}
	wg.Wait()
	return out, ctx.Err()
}

type chatRequest struct {
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	TopP        float64       `json:"top_p"`
	MaxTokens   int           `json:"max_tokens"`
	Stream      bool          `json:"stream"`
	/*
		프롬프트 캐시를 끈다. 제목마다 프롬프트가 통째로 다르므로 캐시가 맞을 일이
		없는데, 켜 두면 슬롯이 직전 요청의 KV를 붙들고 있다가 병렬 처리에서 서로를
		밀어낸다.
	*/
	CachePrompt bool `json:"cache_prompt"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// LMT-60의 프롬프트. 모델 카드가 정한 형태를 그대로 쓴다 — 여기를 바꾸면 품질이
// 조용히 떨어진다(학습 때 본 모양과 달라진다).
func lmtPrompt(text string) string {
	return "Translate the following text from English into Korean:\nEnglish: " +
		strings.TrimSpace(text) + "\nKorean:"
}

func (l *Local) translateOne(ctx context.Context, text string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Messages: []chatMessage{{Role: "user", Content: lmtPrompt(text)}},
		// 번역에 창의성은 필요 없다. 같은 제목이 매번 같게 나와야 비교도 된다.
		Temperature: 0,
		TopP:        1,
		MaxTokens:   256,
		CachePrompt: false,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		l.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := l.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", err
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("translator status %d: %s", res.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("translator: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("translator: empty response")
	}
	return cleanModelOutput(parsed.Choices[0].Message.Content), nil
}

/*
모델이 덧붙이는 껍데기를 벗긴다.

지시를 따르도록 학습된 모델은 가끔 "Korean:"을 다시 적거나 따옴표로 감싸거나
설명을 한 줄 더 단다. 번역 전용 모델이라 드물지만, 드물게 일어나는 일이 화면에
그대로 나가면 이상해 보이는 것은 똑같다.
*/
func cleanModelOutput(s string) string {
	s = strings.TrimSpace(s)
	// 여러 줄이면 첫 줄만 쓴다 — 제목은 한 줄이다.
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = strings.TrimSpace(s[:i])
	}
	for _, prefix := range []string{"Korean:", "한국어:", "번역:", "Translation:"} {
		if rest, ok := strings.CutPrefix(s, prefix); ok {
			s = strings.TrimSpace(rest)
		}
	}
	// 통짜로 감싼 따옴표만 벗긴다. 안쪽 인용부호는 원문의 일부다.
	if len(s) >= 2 {
		for _, pair := range [][2]string{{`"`, `"`}, {"'", "'"}, {"「", "」"}, {"“", "”"}} {
			if strings.HasPrefix(s, pair[0]) && strings.HasSuffix(s, pair[1]) &&
				!strings.Contains(s[len(pair[0]):len(s)-len(pair[1])], pair[0]) {
				s = strings.TrimSpace(s[len(pair[0]) : len(s)-len(pair[1])])
				break
			}
		}
	}
	return s
}
