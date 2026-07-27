package marketdata

import (
	"net/http"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/redis/go-redis/v9"
)

type Clients struct {
	Finnhub   *FinnhubClient
	FRED      *FREDClient
	Upbit     *UpbitClient
	GoldPrice *GoldPriceClient
	Budget    *apibudget.Manager
}

func NewClients(cfg config.MarketDataConfig, budget *apibudget.Manager, redisClient *redis.Client) *Clients {
	httpClient := &http.Client{Timeout: 15 * time.Second}
	return &Clients{
		Finnhub:   NewFinnhubClient(cfg.Finnhub.APIKey, httpClient, budget),
		FRED:      NewFREDClient(cfg.FRED.APIKey, httpClient, budget),
		Upbit:     NewUpbitClient(cfg.Upbit.AccessKey, cfg.Upbit.SecretKey, httpClient, budget),
		GoldPrice: NewGoldPriceClient(cfg.GoldPrice.APIKey, httpClient, budget, redisClient),
		Budget:    budget,
	}
}
