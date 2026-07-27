package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Env  string
	Host string
	Port int

	MySQLDSN string
	RedisURL string

	CORSAllowedOrigins []string

	Auth AuthConfig
	Toss TossConfig

	LLM     LLMConfig
	Dart    DartConfig
	Edgar   EdgarConfig
	Finnhub FinnhubConfig

	MarketData MarketDataConfig
	News       NewsConfig
}

type NewsConfig struct {
	Naver NaverConfig
	Kakao KakaoConfig
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
	JWTSecret           string
	EncryptionMasterKey string
	AccessTokenMinutes  int
	RefreshTokenDays    int
	CookieSecure        bool
	AllowGlobalToss     bool
}

type EdgarConfig struct {
	UserAgent string
}

func (c EdgarConfig) Enabled() bool {
	return strings.TrimSpace(c.UserAgent) != ""
}

type LLMConfig struct {
	APIKey  string
	BaseURL string
	Model   string
}

type DartConfig struct {
	APIKey string
}

type FinnhubConfig struct {
	APIKey string
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
		Auth: AuthConfig{
			JWTSecret:           getEnv("JWT_SECRET", ""),
			EncryptionMasterKey: getEnv("ENCRYPTION_MASTER_KEY", ""),
			AccessTokenMinutes:  accessMinutes,
			RefreshTokenDays:    refreshDays,
			CookieSecure:        getEnv("AUTH_COOKIE_SECURE", "false") == "true",
			AllowGlobalToss:     getEnv("AUTH_ALLOW_GLOBAL_TOSS", "true") == "true",
		},
		Toss: TossConfig{
			BaseURL:      getEnv("TOSS_API_BASE_URL", "https://openapi.tossinvest.com"),
			ClientID:     getEnv("TOSS_CLIENT_ID", ""),
			ClientSecret: getEnv("TOSS_CLIENT_SECRET", ""),
		},
		LLM: LLMConfig{
			APIKey:  getEnv("OPENAI_API_KEY", ""),
			BaseURL: getEnv("OPENAI_BASE_URL", "https://api.openai.com/v1"),
			Model:   getEnv("OPENAI_MODEL", "gpt-4o-mini"),
		},
		Dart: DartConfig{
			APIKey: getEnv("DART_API_KEY", ""),
		},
		Edgar: EdgarConfig{
			UserAgent: getEnv("SEC_EDGAR_USER_AGENT", ""),
		},
		Finnhub: FinnhubConfig{
			APIKey: getEnv("FINNHUB_API_KEY", ""),
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
				ClientID:     getEnv("NAVER_CLIENT_ID", ""),
				ClientSecret: getEnv("NAVER_CLIENT_SECRET", ""),
			},
			Kakao: KakaoConfig{
				RestAPIKey: getEnv("KAKAO_REST_API_KEY", ""),
			},
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
