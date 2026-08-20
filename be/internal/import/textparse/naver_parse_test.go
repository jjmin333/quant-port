package textparse

import (
	"context"
	"os"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/import/resolver"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestParseSuspiciousNAVEROCRLines(t *testing.T) {
	lines := []string{
		"195                          645,752 (13.07%)",
		"4.294.248원",
		"19주",
		"®  NAVER                               4,294,248",
		"645,752 (13.07%)",
		"645,752",
		"4294248",
		"4,294,248",
	}
	for _, text := range lines {
		for _, l := range ParseHoldingsText(text) {
			if l.Name != "" || l.Quantity > 0 {
				t.Logf("input=%q -> name=%q qty=%.0f valid=%v frag=%v",
					text, l.Name, l.Quantity,
					IsValidParsedLine(l.Name, l.Quantity),
					LooksLikeOCRFragment(l.Name))
			}
		}
	}
}

func TestLiveResolveSuspiciousNames(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("LIVE_MYSQL=1")
	}
	cfg, _ := config.Load()
	mysql, _ := repository.NewMySQL(cfg.MySQLDSN)
	defer mysql.Close()
	r := resolver.New(repository.NewStockRepo(mysql.DB()))
	ctx := context.Background()

	for _, text := range []string{
		"195", "645752", "645,752", "4294248", "4,294,248", "4294248원",
	} {
		for _, l := range ParseHoldingsText(text) {
			if l.Name == "" {
				continue
			}
			res := r.ResolveName(ctx, NormalizeDisplayName(l.Name))
			t.Logf("parsed %q qty=%.0f -> %q (%s)", l.Name, l.Quantity, res.Name, res.Symbol)
		}
	}
}
