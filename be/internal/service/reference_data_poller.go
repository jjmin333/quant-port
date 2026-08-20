package service

import (
	"context"
	"log"
	"time"
)

const referenceDataPollTick = time.Minute

// RunReferenceDataPoller refreshes shares outstanding and fundamentals on admin-configured intervals.
func RunReferenceDataPoller(
	ctx context.Context,
	settings *MarketDataSettingsStore,
	market *MarketService,
	fundamentals *FundamentalsService,
) {
	if settings == nil {
		return
	}
	lastShares := time.Now()
	lastFundamentals := time.Now()

	ticker := time.NewTicker(referenceDataPollTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			cfg := settings.Get(ctx)
			sharesInterval := time.Duration(cfg.SharesPollHours) * time.Hour
			if lastShares.IsZero() || now.Sub(lastShares) >= sharesInterval {
				if market != nil {
					if n, err := market.RefreshAllSharesOutstanding(ctx); err != nil {
						log.Printf("reference data poller: shares: %v", err)
					} else if n > 0 {
						log.Printf("reference data poller: refreshed shares for %d symbols", n)
					}
				}
				lastShares = now
			}

			fundInterval := time.Duration(cfg.FundamentalsPollHours) * time.Hour
			if lastFundamentals.IsZero() || now.Sub(lastFundamentals) >= fundInterval {
				if fundamentals != nil {
					if n, err := fundamentals.RefreshAllStored(ctx); err != nil {
						log.Printf("reference data poller: fundamentals: %v", err)
					} else if n > 0 {
						log.Printf("reference data poller: refreshed fundamentals for %d symbols", n)
					}
				}
				lastFundamentals = now
			}
		}
	}
}
