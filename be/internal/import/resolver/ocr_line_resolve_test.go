package resolver

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveSuspiciousOCRFragments(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("LIVE_MYSQL=1")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	mysql, err := repository.NewMySQL(cfg.MySQLDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer mysql.Close()
	r := New(repository.NewStockRepo(mysql.DB()))
	ctx := context.Background()

	candidates := []string{
		"195",
		"645,752",
		"645752",
		"645,752 (13.07%)",
		"195                          645,752 (13.07%)",
		"®  NAVER                               4,294,248",
		"® NAVER",
		"4,294,248",
		"4294248",
		"-645.752원013.07%",
		"19주",
		"NAVER",
		"베레",
		"@ 쁘셔",
		"뭔",
		"-421,586뭔(20.22%0)",
		"6  Let",
		"Let",
		"ae",
		"1",
		"네이버",
		"네버",
		"내버",
		"니버",
		"뭔",
		"안트",
		"트로젠",
		"195",
	}
	for _, raw := range candidates {
		raw = strings.TrimSpace(raw)
		res := r.ResolveName(ctx, raw)
		if res.Symbol == "065660" || res.Symbol == "035420" {
			t.Logf("MATCH %q -> %q (%s) conf=%.2f err=%q", raw, res.Name, res.Symbol, res.Confidence, res.Error)
		}
		if raw == "안트" && res.Symbol == "065660" {
			t.Fatalf("안트 should not resolve to anterogen")
		}
	}
}
