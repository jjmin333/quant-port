package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/enact/quant-port/be/internal/marketdata"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// HasCorruptDisplayName reports UTF-8 replacement chars from mis-decoded legacy rows.
func HasCorruptDisplayName(name string) bool {
	return strings.Contains(name, "\uFFFD")
}

// RepairDomesticETPNames overwrites corrupted catalog names using KRX Open API (authoritative UTF-8).
func RepairDomesticETPNames(ctx context.Context, repo *repository.StockRepo, krx *marketdata.KRXClient, basDD string) (int, error) {
	if repo == nil || krx == nil || !krx.Enabled() {
		return 0, fmt.Errorf("repair not configured")
	}
	rows, err := krx.ListDomesticETP(ctx, basDD)
	if err != nil {
		return 0, err
	}
	bySym := make(map[string]model.Stock, len(rows))
	for _, st := range rows {
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		if sym != "" {
			bySym[sym] = st
		}
	}

	existing, err := repo.ListETPStocks(ctx)
	if err != nil {
		return 0, err
	}

	var fixes []model.Stock
	for _, st := range existing {
		if st.Currency != model.CurrencyKRW {
			continue
		}
		sym := strings.ToUpper(strings.TrimSpace(st.Symbol))
		src, ok := bySym[sym]
		if !ok {
			continue
		}
		if !HasCorruptDisplayName(st.Name) && !HasCorruptDisplayName(st.EnglishName) {
			continue
		}
		if strings.TrimSpace(src.Name) == "" {
			continue
		}
		st.Name = strings.TrimSpace(src.Name)
		en := strings.TrimSpace(src.EnglishName)
		if en == "" {
			en = st.Name
		}
		st.EnglishName = en
		fixes = append(fixes, st)
	}

	for i := 0; i < len(fixes); i += insertBatchSize {
		end := i + insertBatchSize
		if end > len(fixes) {
			end = len(fixes)
		}
		if err := repo.UpsertBatch(ctx, fixes[i:end]); err != nil {
			return 0, fmt.Errorf("repair batch: %w", err)
		}
	}
	return len(fixes), nil
}
