package config

import (
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
	Papago  PapagoConfig
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

type PapagoConfig struct {
	ClientID     string
	ClientSecret string
}

func (c PapagoConfig) Enabled() bool {
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
	APIKey   string
	BaseURL  string
	Model    string

	// Quota overrides for apibudget's ProviderOpenAI limits. Free-tier
	// ceilings move without notice and differ per model and per account, so
	// they must be tunable from the environment rather than pinned in code.
	// 0 means "keep the compiled-in default"; a negative value disables that
	// particular ceiling.
	DailyQuota      int
	PerMinute       int
	TokensPerMinute int
	TokensPerDay    int64
}

// buildLLMConfig prefers an explicit OpenAI key; if unset, falls back to
// GEMINI_API_KEY against Google's OpenAI-compatible endpoint. Either way the
// downstream llm.Client only ever speaks the OpenAI chat-completions shape —
// OPENAI_BASE_URL/OPENAI_MODEL still override the provider-specific default
// if set, so switching providers never requires a code change.
func buildLLMConfig() LLMConfig {
	cfg := LLMConfig{
		DailyQuota:      getEnvInt("LLM_DAILY_QUOTA", 0),
		PerMinute:       getEnvInt("LLM_PER_MINUTE", 0),
		TokensPerMinute: getEnvInt("LLM_TOKENS_PER_MINUTE", 0),
		TokensPerDay:    int64(getEnvInt("LLM_TOKENS_PER_DAY", 0)),
	}
	if apiKey := getEnv("OPENAI_API_KEY", ""); apiKey != "" {
		cfg.Provider = "openai"
		cfg.APIKey = apiKey
		cfg.BaseURL = getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1")
		cfg.Model = getEnv("OPENAI_MODEL", "gpt-4o-mini")
		return cfg
	}
	if apiKey := getEnv("GEMINI_API_KEY", ""); apiKey != "" {
		cfg.Provider = "gemini"
		cfg.APIKey = apiKey
		cfg.BaseURL = getEnv("OPENAI_BASE_URL", "https://generativelanguage.googleapis.com/v1beta/openai/")
		cfg.Model = getEnv("OPENAI_MODEL", "gemini-3.1-flash-lite")
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
}

func (t TossConfig) Enabled() bool {
	return t.ClientID != "" && t.ClientSecret != ""
}

func Load() (*Config, error) {
	_ = godotenv.Load()

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
		Toss: TossConfig{
			BaseURL:      getEnv("TOSS_API_BASE_URL", "https://openapi.tossinvest.com"),
			ClientID:     getEnv("TOSS_CLIENT_ID", ""),
			ClientSecret: getEnv("TOSS_CLIENT_SECRET", ""),
		},
		LLM: buildLLMConfig(),
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
			Papago: PapagoConfig{
				ClientID:     getEnv("PAPAGO_CLIENT_ID", ""),
				ClientSecret: getEnv("PAPAGO_CLIENT_SECRET", ""),
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
		UserRate:  getEnvFloat("RATE_LIMIT_USER_RPS", 10),
		UserBurst: getEnvFloat("RATE_LIMIT_USER_BURST", 20),
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
