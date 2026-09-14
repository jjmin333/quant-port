package translate

import (
	"strconv"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
)

/*
설정.

	TRANSLATE_PROVIDER=azure          Azure AI Translator (기본값)
	AZURE_TRANSLATOR_KEY=…            리소스 키 (없으면 이 엔진은 안 붙는다)
	                                  AZURE_TRANSLATOR_API_KEY_FIRST/_SECOND 도 받는다
	AZURE_TRANSLATOR_REGION=…         리소스 지역 (예: koreacentral). 전역 리소스면 비움
	AZURE_TRANSLATOR_ENDPOINT=…       기본값 https://api.cognitive.microsofttranslator.com
	TRANSLATE_LOCAL_URL=…             (레거시) 자체 모델 컨테이너 주소

# 기본값이 azure로 바뀌었다 (2026-09-11 사용자 결정)

자체 모델(translator 컨테이너)을 걷어 내기로 했다 — 이유는 `azure.go` 머리글에 적혀
있다(제목 한 건에 0.9~2.8초, 상시 1.5코어·2GB, 모델 적재 중에는 그 주기가 통째로
영어). `local`은 **지우지 않고 남겨** 둔다: 이름을 그대로 두면 `TRANSLATE_PROVIDER=local`
한 줄로 되돌릴 수 있고, 되돌리는 길이 한 줄이어야 갈아타는 판단이 싸다.

**Papago는 없다.** NCP 자격증명이 만료됐고 갱신하지 않기로 했다(2026-09-03).
죽은 키를 폴백으로 남겨 두면 거른 건마다 실패하는 호출이 하나씩 더 나가고,
무엇보다 「폴백이 있다」는 착각을 준다.

# 키가 없으면 어떻게 되나

그 엔진은 **안 붙는다**(`Available()` false → `NewChain`이 버린다). 사슬이 비면
해외 제목은 영어로 남고, 기동 로그가 그 사실을 찍는다(`server.go`). 조용히 자체
모델로 떨어지지 않는 이유는 그 컨테이너를 내리기로 했기 때문이다 — 있지도 않은
폴백을 기대하게 만들면 안 된다.
*/

// Options is the parsed TRANSLATE_* configuration.
type Options struct {
	// Providers는 순서 그대로다. 비어 있으면 기본값이 들어간다.
	Providers []string
	LocalURL  string
	// LocalSlots는 컨테이너의 `--parallel`과 같아야 한다. 더 던지면 큐에 설 뿐이다.
	LocalSlots   int
	LocalTimeout time.Duration

	AzureKey      string
	AzureRegion   string
	AzureEndpoint string
	// AzureTimeout은 묶음 한 번의 왕복 상한이다. 자체 모델의 60초보다 훨씬 짧은
	// 이유는 여기서 60초가 걸린다면 기다려서 나아지는 상황이 아니기 때문이다 —
	// 제목 100건을 한 요청에 싣고 실측 왕복은 1초 미만이다.
	AzureTimeout time.Duration
}

const (
	// 기본 엔진은 azure다 (2026-09-11 사용자 결정 — 머리글 참조).
	defaultProviders = "azure"
	defaultLocalURL  = "http://translator:8100"
	defaultSlots     = 2
	// 제목 한 건 실측 0.9~2.8초(1.7B). 60초는 모델이 막 올라오는 중이거나 슬롯이
	// 밀렸을 때를 덮는 값이고, 이걸 넘기면 그 제목은 영어로 남아 다음 주기에 다시 온다.
	defaultTimeout      = 60 * time.Second
	defaultAzureTimeout = 20 * time.Second
)

// ParseOptions reads the TRANSLATE_* environment through the given lookup.
func ParseOptions(getEnv func(string) string) Options {
	opts := Options{
		LocalURL:     firstNonEmpty(getEnv("TRANSLATE_LOCAL_URL"), defaultLocalURL),
		LocalSlots:   atoiOr(getEnv("TRANSLATE_LOCAL_SLOTS"), defaultSlots),
		LocalTimeout: defaultTimeout,
		// 키 이름을 셋 받는다 — Azure 포털이 키를 「KEY 1 / KEY 2」로 내주므로
		// 그 화면을 그대로 옮겨 적은 이름도 받아 준다. 순서는 적은 사람의 의도
		// 순서다: 명시적인 AZURE_TRANSLATOR_KEY가 있으면 그것이 이긴다.
		//
		// 실제로 2026-09-13에 키가 `AZURE_TRANSLATOR_API_KEY_FIRST`로 들어와
		// 있었고, 그동안 번역은 0건이었다(`engines=local`). 이름 하나가 안 맞아
		// 기능이 통째로 꺼져 있었는데 **아무 오류도 나지 않는다** — 키 없는 엔진은
		// 조용히 사슬에서 빠지기 때문이다. 받아 주는 이름을 늘려 그 침묵을 줄인다.
		AzureKey: firstNonEmpty(
			getEnv("AZURE_TRANSLATOR_KEY"),
			getEnv("AZURE_TRANSLATOR_API_KEY_FIRST"),
			getEnv("AZURE_TRANSLATOR_API_KEY_SECOND"),
		),
		AzureRegion:   strings.TrimSpace(getEnv("AZURE_TRANSLATOR_REGION")),
		AzureEndpoint: strings.TrimSpace(getEnv("AZURE_TRANSLATOR_ENDPOINT")),
		AzureTimeout:  defaultAzureTimeout,
	}
	if raw := strings.TrimSpace(getEnv("TRANSLATE_TIMEOUT_SEC")); raw != "" {
		if n, err := strconv.Atoi(raw); err == nil && n > 0 {
			opts.LocalTimeout = time.Duration(n) * time.Second
		}
	}
	for _, name := range strings.Split(firstNonEmpty(getEnv("TRANSLATE_PROVIDER"), defaultProviders), ",") {
		if name = strings.ToLower(strings.TrimSpace(name)); name != "" {
			opts.Providers = append(opts.Providers, name)
		}
	}
	return opts
}

/*
Build assembles the chain in the configured order.

목록에 없는 엔진은 만들지 않는다. 키가 없는 엔진은 만들어도 `NewChain`이 버린다
(`Available()`) — 그래서 「이름은 적혔지만 키가 없다」와 「이름이 안 적혔다」가 같은
결과가 되고, 기동 로그가 그 둘을 구분해 준다.

`budget`은 문자 수를 세는 자리다. nil이어도 동작하지만(시험 경로), 운영에서 nil이면
월 한도를 아무도 안 보게 되므로 `server.go`가 반드시 넘긴다.
*/
func Build(opts Options, budget *apibudget.Manager) *Chain {
	engines := make([]Translator, 0, len(opts.Providers))
	for _, name := range opts.Providers {
		switch name {
		case "azure":
			engines = append(engines, NewAzure(
				opts.AzureEndpoint, opts.AzureKey, opts.AzureRegion, opts.AzureTimeout, budget,
			))
		case "local":
			engines = append(engines, NewLocal(opts.LocalURL, opts.LocalSlots, opts.LocalTimeout))
		}
	}
	return NewChain(engines...)
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return ""
}

func atoiOr(raw string, fallback int) int {
	if n, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil && n > 0 {
		return n
	}
	return fallback
}
