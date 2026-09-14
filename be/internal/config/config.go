package config

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Env  string
	Host string
	Port int

	MySQLDSN string
	RedisURL string

	CORSAllowedOrigins []string
	RateLimit          RateLimitConfig
	// TrustProxyHeaders enables X-Forwarded-For / X-Real-IP parsing, and only
	// for requests arriving from a loopback/private peer. Turn it off when the
	// app is exposed directly with no reverse proxy in front.
	TrustProxyHeaders bool
	// RequestTimeout bounds a single request. 0 disables it. SSE, uploads and
	// admin routes are exempt — see middleware/timeout.go.
	RequestTimeout time.Duration

	Auth AuthConfig
	Toss TossConfig

	LLM     LLMConfig
	Dart    DartConfig
	Edgar   EdgarConfig
	Finnhub FinnhubConfig
	FnGuide FnGuideConfig
	KRX     KRXConfig

	MarketData MarketDataConfig
	News       NewsConfig
	Import     ImportConfig
	Media      MediaConfig
}

// ImportConfig — 스크린샷 OCR import job (Tesseract + temp files).
type ImportConfig struct {
	// TempDir — 업로드·crop·OCR 중간 파일. Docker에서는 tmpfs 마운트 권장.
	TempDir string
}

// MediaConfig — 사용자 업로드 이미지(리포트 본문 등) 저장소.
//
// Import와 달리 **영구 보관**이다. Docker에서 Dir은 반드시 named volume이어야
// 한다 — 컨테이너 writable layer는 recreate마다 사라진다.
type MediaConfig struct {
	// Dir — 로컬 파일 저장소 루트. 객체 스토리지로 옮기면 이 값 대신
	// 해당 백엔드 설정을 쓰고 media.Store 구현만 교체한다.
	Dir string
	// MaxUploadBytes — 파일 1개 상한.
	MaxUploadBytes int64
	// PublicBaseURL — CDN/객체 스토리지 앞단이 생기면 여기에 넣는다. 비어 있으면
	// API가 직접 `/api/v1/media/<key>`로 서빙한다.
	PublicBaseURL string
}

type NewsConfig struct {
	Naver   NaverConfig
	Kakao   KakaoConfig
	Finnhub FinnhubNewsConfig
}

type FinnhubNewsConfig struct {
	APIKey string
}

func (c FinnhubNewsConfig) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

type NaverConfig struct {
	ClientID     string
	ClientSecret string
}

func (c NaverConfig) Enabled() bool {
	return strings.TrimSpace(c.ClientID) != "" && strings.TrimSpace(c.ClientSecret) != ""
}

type KakaoConfig struct {
	RestAPIKey string
}

func (c KakaoConfig) Enabled() bool {
	return strings.TrimSpace(c.RestAPIKey) != ""
}

type UpbitConfig struct {
	AccessKey string
	SecretKey string
}

type FREDConfig struct {
	APIKey string
}

type GoldPriceConfig struct {
	APIKey string
}

type MarketDataConfig struct {
	Upbit     UpbitConfig
	FRED      FREDConfig
	GoldPrice GoldPriceConfig
	Finnhub   FinnhubConfig
}

type AuthConfig struct {
	JWTSecret          string
	AccessTokenMinutes int
	RefreshTokenDays   int
	CookieSecure       bool
}

type EdgarConfig struct {
	UserAgent string
}

func (c EdgarConfig) Enabled() bool {
	return strings.TrimSpace(c.UserAgent) != ""
}

type LLMConfig struct {
	Provider string // "openai" | "gemini" | "" (disabled)
	// KeyKR/KeyUS are the per-market API keys. LLM spend is split by market
	// (국내 종목 요약·국내 브리핑 vs 해외 종목 요약·해외 브리핑) so each
	// key has its own provider quota. A missing regional key falls back to
	// the shared GEMINI_API_KEY / OPENAI_API_KEY; if both regions resolve
	// to the same string they share one budget bucket.
	KeyKR   string
	KeyUS   string
	BaseURL string
	Model   string

	// Quota overrides for apibudget's LLM limits. Free-tier ceilings move
	// without notice and differ per model and per account, so they must be
	// tunable from the environment rather than pinned in code.
	// 0 means "keep the compiled-in default"; a negative value disables that
	// particular ceiling.
	DailyQuota      int
	PerMinute       int
	TokensPerMinute int
	TokensPerDay    int64
}

func (c LLMConfig) Enabled() bool {
	return strings.TrimSpace(c.KeyKR) != "" || strings.TrimSpace(c.KeyUS) != ""
}

// buildLLMConfig prefers an explicit OpenAI key; if unset, falls back to
// Gemini against Google's OpenAI-compatible endpoint. Either way the
// downstream llm.Client only ever speaks the OpenAI chat-completions shape —
// OPENAI_BASE_URL/OPENAI_MODEL still override the provider-specific default
// if set, so switching providers never requires a code change.
//
// Regional keys (GEMINI_KR_API_KEY / GEMINI_US_API_KEY, or the *_API_KEY_KR
// aliases) take precedence over the shared GEMINI_API_KEY. New LLM features
// must keep this KR/US split — see llm.Pool and docs/AGENT_STATE.md.
func buildLLMConfig() LLMConfig {
	cfg := LLMConfig{
		DailyQuota:      getEnvInt("LLM_DAILY_QUOTA", 0),
		PerMinute:       getEnvInt("LLM_PER_MINUTE", 0),
		TokensPerMinute: getEnvInt("LLM_TOKENS_PER_MINUTE", 0),
		TokensPerDay:    int64(getEnvInt("LLM_TOKENS_PER_DAY", 0)),
	}
	openaiKR := firstEnv("OPENAI_KR_API_KEY", "OPENAI_API_KEY_KR")
	openaiUS := firstEnv("OPENAI_US_API_KEY", "OPENAI_API_KEY_US")
	openai := getEnv("OPENAI_API_KEY", "")
	if openaiKR != "" || openaiUS != "" || openai != "" {
		cfg.Provider = "openai"
		cfg.BaseURL = getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1")
		cfg.Model = getEnv("OPENAI_MODEL", "gpt-4o-mini")
		cfg.KeyKR = firstNonEmpty(openaiKR, openai)
		cfg.KeyUS = firstNonEmpty(openaiUS, openai)
		return cfg
	}
	geminiKR := firstEnv("GEMINI_KR_API_KEY", "GEMINI_API_KEY_KR")
	geminiUS := firstEnv("GEMINI_US_API_KEY", "GEMINI_API_KEY_US")
	gemini := getEnv("GEMINI_API_KEY", "")
	if geminiKR != "" || geminiUS != "" || gemini != "" {
		cfg.Provider = "gemini"
		cfg.BaseURL = getEnv("OPENAI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta/openai/")
		cfg.Model = getEnv("OPENAI_MODEL", "gemini-3.1-flash-lite")
		cfg.KeyKR = firstNonEmpty(geminiKR, gemini)
		cfg.KeyUS = firstNonEmpty(geminiUS, gemini)
		return cfg
	}
	return cfg
}

type DartConfig struct {
	APIKey string
}

type FinnhubConfig struct {
	APIKey string
}

// FnGuideConfig — 국내 실적발표 캘린더 소스.
//
// **Enabled는 킬 스위치다.** FnGuide는 이 데이터에 공개 API를 주지 않아
// Company Guide 화면의 내부 JSON을 읽는데, 그건 언제든 바뀔 수 있고 약관과의
// 관계도 우리가 감수하고 택한 것이다. 끄면 국내 실적 화면은 DART 공시 이력으로
// 돌아가되 죽지 않는다 — 코드 배포 없이 되돌릴 길이 있어야 한다.
type FnGuideConfig struct {
	Enabled bool
	BaseURL string
}

type KRXConfig struct {
	APIKey string
}

func (c KRXConfig) Enabled() bool {
	return strings.TrimSpace(c.APIKey) != ""
}

type TossConfig struct {
	BaseURL      string
	ClientID     string
	ClientSecret string
	// Secondary is an overflow OAuth app. Empty means single-key mode
	// (identical to the previous one-credential setup).
	SecondaryClientID     string
	SecondaryClientSecret string

	/*
		StreamEnabled — 실시간 시세를 웹소켓 구독으로 받을지.

		**기본값은 off다.** 켜면 호가·체결이 REST 폴링이 아니라 푸시로 오는데,
		그 경로가 사용자에게 가장 잘 보이는 곳(상세 화면의 호가창·체결·형성봉)이라
		운영자가 의도적으로 켜고 확인하는 편이 안전하다. 끄면 폴링이 그대로 돌아
		즉시 예전 동작으로 돌아온다.

		**웹소켓은 REST와 같은 자격증명을 쓴다.** 한때는 SECONDARY를 전용으로
		떼어 줬는데, 그 근거였던 「연결을 닫으면 토큰이 무효화된다」가 실측으로
		반증됐다(`toss/stream.go` 헤더 · `docs/TOSS_TOKEN_DESIGN.md`).
		공유해도 되므로 SECONDARY는 REST failover로 돌아갔고, 구독은 두 앱을
		모두 써서 슬롯이 200 → 400이 됐다.
	*/
	StreamEnabled bool
}

/*
RESTConfig — REST 클라이언트가 쓸 자격증명 구성.

**두 자격증명을 다 돌려준다.** 한때 웹소켓이 켜져 있으면 SECONDARY를 빼고
돌려줬는데, 그 근거였던 「WS 재연결이 REST 토큰을 죽인다」가 실측으로 반증됐다
(`toss/stream.go` 헤더 · `docs/TOSS_TOKEN_DESIGN.md`). 재연결이 재발급을 부르지
않는 한 공유가 안전하고, 그 규칙은 이제 `TokenManager.TokenAfter`가 강제한다.

그래서 REST는 failover를 되찾고, 구독은 두 앱을 다 써서 슬롯이 400이 된다.
남겨 둔 이유는 **호출부의 의도를 유지**하기 위해서다 — REST 쪽이 어떤 구성을
쓰는지 한 군데서 읽히는 편이, 나중에 다시 갈라야 할 때 찾기 쉽다.
*/
func (t TossConfig) RESTConfig() TossConfig {
	return t
}

// StreamReady reports whether websocket streaming can actually run.
//
// SECONDARY는 더 이상 전제가 아니다 — 앱이 하나뿐이면 연결 2개·슬롯 200으로
// 돌고, 둘이면 4개·400이 된다.
func (t TossConfig) StreamReady() bool {
	return t.StreamEnabled && t.Enabled()
}

func (t TossConfig) Enabled() bool {
	return strings.TrimSpace(t.ClientID) != "" && strings.TrimSpace(t.ClientSecret) != ""
}

func (t TossConfig) SecondaryEnabled() bool {
	id := strings.TrimSpace(t.SecondaryClientID)
	secret := strings.TrimSpace(t.SecondaryClientSecret)
	if id == "" || secret == "" {
		return false
	}
	// Same pair as primary would share Toss's quota — attaching it as
	// "secondary" would 429 twice on one bucket.
	return id != strings.TrimSpace(t.ClientID) || secret != strings.TrimSpace(t.ClientSecret)
}

/*
loadDotEnv은 `.env`를 읽되 **실패를 삼키지 않는다**.

예전에는 `_ = godotenv.Load()` 한 줄이었다. 그런데 godotenv는 파일 하나에서 한 줄만
어긋나도 **전체 파싱을 포기**한다. 실제로 물려 본 적이 있다 — `be/.env` 맨 앞에 UTF-8
BOM이 붙어 있어 파서가 첫 줄에서 멈췄고, 키가 **하나도** 안 실렸다. 그런데 아무 소리도
나지 않았다: `MYSQL_DSN`의 기본값이 마침 파일의 값과 같아 DB는 붙었고, 대신 Toss·FRED·
DART·Gemini가 전부 「not configured」가 되어 `cmd/` 아래 CLI들이 조용히 반쪽으로 돌았다.

BOM을 직접 걷어내는 이유도 같다. 편집기가 한 번 더 붙이면 같은 일이 반복되는데,
그때 다시 반나절을 쓸 이유가 없다. 컨테이너는 compose의 `env_file`이 파일을 직접
읽어 환경변수로 넣어 주므로 이 경로를 타지 않는다 — 그래서 **호스트에서만** 나던
증상이라 더 알아채기 어려웠다.
*/
func loadDotEnv() {
	const path = ".env"
	body, err := os.ReadFile(path)
	if err != nil {
		return // 파일이 없는 건 정상이다(컨테이너·CI는 환경변수로 받는다).
	}
	// BOM은 바이트로 걷어낸다 — 소스에 BOM 문자를 그대로 적으면 컴파일이 막힌다.
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	env, err := godotenv.Unmarshal(string(body))
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %s 를 읽지 못해 **키를 하나도 싣지 못했습니다** — %v\n", path, err)
		return
	}
	// 이미 있는 환경변수가 이긴다 — godotenv.Load와 같은 규칙이고, 컨테이너가
	// 넣어 준 값을 파일이 덮어쓰면 안 된다.
	for k, v := range env {
		if _, ok := os.LookupEnv(k); !ok {
			os.Setenv(k, v)
		}
	}
}

func Load() (*Config, error) {
	loadDotEnv()

	port, err := strconv.Atoi(getEnv("API_PORT", "8080"))
	if err != nil {
		return nil, fmt.Errorf("invalid API_PORT: %w", err)
	}

	accessMinutes, err := strconv.Atoi(getEnv("JWT_ACCESS_TOKEN_MINUTES", "15"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_ACCESS_TOKEN_MINUTES: %w", err)
	}
	refreshDays, err := strconv.Atoi(getEnv("JWT_REFRESH_TOKEN_DAYS", "7"))
	if err != nil {
		return nil, fmt.Errorf("invalid JWT_REFRESH_TOKEN_DAYS: %w", err)
	}

	cfg := &Config{
		Env:                getEnv("APP_ENV", "development"),
		Host:               getEnv("API_HOST", "0.0.0.0"),
		Port:               port,
		MySQLDSN:           getEnv("MYSQL_DSN", "quantport:quantport@tcp(localhost:3306)/quantport?parseTime=true&charset=utf8mb4&loc=Local"),
		RedisURL:           getEnv("REDIS_URL", "redis://localhost:6379/0"),
		CORSAllowedOrigins: splitCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
		RateLimit:          loadRateLimitConfig(),
		TrustProxyHeaders:  getEnvBool("TRUST_PROXY_HEADERS", true),
		// 10초는 정상 최장 경로(뉴스 목록 동시 재구성 1.4초, /rankings 1.1초)의
		// 7배 여유다. 0으로 두면 완전히 끈다.
		RequestTimeout: time.Duration(getEnvInt("REQUEST_TIMEOUT_SECONDS", 10)) * time.Second,
		Auth: AuthConfig{
			JWTSecret:          getEnv("JWT_SECRET", ""),
			AccessTokenMinutes: accessMinutes,
			RefreshTokenDays:   refreshDays,
			CookieSecure:       getEnv("AUTH_COOKIE_SECURE", "false") == "true",
		},
		Toss: loadTossConfig(),
		LLM:  buildLLMConfig(),
		Dart: DartConfig{
			APIKey: getEnv("DART_API_KEY", ""),
		},
		Edgar: EdgarConfig{
			UserAgent: getEnv("SEC_EDGAR_USER_AGENT", ""),
		},
		Finnhub: FinnhubConfig{
			APIKey: getEnv("FINNHUB_API_KEY", ""),
		},
		FnGuide: FnGuideConfig{
			Enabled: getEnvBool("FNGUIDE_ENABLED", true),
			BaseURL: getEnv("FNGUIDE_BASE_URL", "https://wcomp.fnguide.com"),
		},
		KRX: KRXConfig{
			APIKey: firstEnv("KRX_API_KEY", "KRX_OPEN_API_KEY"),
		},
		MarketData: MarketDataConfig{
			Upbit: UpbitConfig{
				AccessKey: getEnv("UPBIT_ACCESS_KEY", ""),
				SecretKey: getEnv("UPBIT_SECRET_KEY", ""),
			},
			FRED: FREDConfig{
				APIKey: getEnv("FRED_API_KEY", ""),
			},
			GoldPrice: GoldPriceConfig{
				APIKey: getEnv("GOLDPRICE_API_KEY", ""),
			},
			Finnhub: FinnhubConfig{
				APIKey: getEnv("FINNHUB_API_KEY", ""),
			},
		},
		News: NewsConfig{
			Naver: NaverConfig{
				ClientID:     firstEnv("NAVER_NEWS_CLIENT_ID", "NAVER_CLIENT_ID"),
				ClientSecret: firstEnv("NAVER_NEWS_CLIENT_SECRET", "NAVER_CLIENT_SECRET"),
			},

			Kakao: KakaoConfig{
				RestAPIKey: getEnv("KAKAO_REST_API_KEY", ""),
			},
			Finnhub: FinnhubNewsConfig{
				APIKey: getEnv("FINNHUB_NEWS_API_KEY", ""),
			},
		},
		Import: ImportConfig{
			TempDir: getEnv("IMPORT_TEMP_DIR", "tmp/import"),
		},
		Media: MediaConfig{
			Dir:            getEnv("MEDIA_DIR", "var/media"),
			MaxUploadBytes: int64(getEnvInt("MEDIA_MAX_UPLOAD_MB", 5)) << 20,
			PublicBaseURL:  getEnv("MEDIA_PUBLIC_BASE_URL", ""),
		},
	}

	return cfg, nil
}

func (c *Config) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// loadTossConfig reads the primary OAuth app (used for every call) and an
// optional secondary used only when a rate-limit group on primary is
// exhausted. Preferred names are TOSS_PRIMARY_* / TOSS_SECONDARY_*.
// TOSS_CLIENT_ID / TOSS_CLIENT_SECRET remain aliases for primary so existing
// single-key env files keep working. TOSS_KR_* / TOSS_US_* are last-resort
// aliases from a brief naming experiment and are not documented.
func loadTossConfig() TossConfig {
	return TossConfig{
		BaseURL: getEnv("TOSS_API_BASE_URL", "https://openapi.tossinvest.com"),
		ClientID: firstEnv(
			"TOSS_PRIMARY_CLIENT_ID",
			"TOSS_CLIENT_ID",
			"TOSS_KR_CLIENT_ID",
		),
		ClientSecret: firstEnv(
			"TOSS_PRIMARY_CLIENT_SECRET",
			"TOSS_CLIENT_SECRET",
			"TOSS_KR_CLIENT_SECRET",
		),
		SecondaryClientID: firstEnv(
			"TOSS_SECONDARY_CLIENT_ID",
			"TOSS_US_CLIENT_ID",
		),
		SecondaryClientSecret: firstEnv(
			"TOSS_SECONDARY_CLIENT_SECRET",
			"TOSS_US_CLIENT_SECRET",
		),
		StreamEnabled: getEnvBool("TOSS_STREAM_ENABLED", false),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvInt reads an integer env var, falling back on unset or unparseable
// input. A negative value is meaningful to some callers ("disable this
// ceiling"), so it is passed through rather than clamped here.
/*
RateLimitConfig — 미들웨어와 같은 모양이지만 여기에 따로 둔다. config가
internal/middleware를 import하면 middleware → service → config로 순환한다.
변환은 server 배선 지점에서 한 번 한다.

임계값 근거와 트레이드오프는 internal/middleware/ratelimit.go 주석에 있다.
*/
type RateLimitConfig struct {
	Enabled   bool
	UserRate  float64
	UserBurst float64
	IPRate    float64
	IPBurst   float64
}

func loadRateLimitConfig() RateLimitConfig {
	return RateLimitConfig{
		// 기본 on. 끄는 것은 사고 대응 수단이지 기본값이 아니다.
		Enabled:   getEnvBool("RATE_LIMIT_ENABLED", true),
		UserRate:  getEnvFloat("RATE_LIMIT_USER_RPS", 20),
		UserBurst: getEnvFloat("RATE_LIMIT_USER_BURST", 40),
		IPRate:    getEnvFloat("RATE_LIMIT_IP_RPS", 30),
		IPBurst:   getEnvFloat("RATE_LIMIT_IP_BURST", 60),
	}
}

func getEnvFloat(key string, fallback float64) float64 {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil || v < 0 {
		return fallback
	}
	return v
}

func getEnvInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return v
}

// getEnvBool reads a boolean env var. 미설정이면 fallback이고, 값이 있으면
// strconv.ParseBool이 아는 표기("1"/"true"/"TRUE"/"0"/"false"…)를 받는다.
// 해석 불가한 값에 fallback을 쓰는 이유는, 오타 하나가 킬 스위치를 반대로
// 돌려 버리는 쪽이 무시하는 쪽보다 위험하기 때문이다.
func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return fallback
	}
	return v
}

func firstEnv(keys ...string) string {
	for _, key := range keys {
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}

	var out []string
	start := 0
	for i := 0; i <= len(value); i++ {
		if i == len(value) || value[i] == ',' {
			part := value[start:i]
			if part != "" {
				out = append(out, part)
			}
			start = i + 1
		}
	}
	return out
}
