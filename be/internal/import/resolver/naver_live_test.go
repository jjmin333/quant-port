package resolver

import (
	"context"
	"os"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveNAVER(t *testing.T) {
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

	for _, raw := range []string{"NAVER", "®  NAVER", "안트로젠", "035420", "065660"} {
		res := r.ResolveName(ctx, raw)
		t.Logf("%q -> sym=%q name=%q conf=%.2f err=%q", raw, res.Symbol, res.Name, res.Confidence, res.Error)
	}

	// Stock search ranking for NAVER
	items, err := repository.NewStockRepo(mysql.DB()).Search(ctx, model.StockSearchParams{
		Query: "NAVER",
		Limit: 5,
	})
	if err != nil {
		t.Fatal(err)
	}
	for i, st := range items {
		t.Logf("search[%d] sym=%q name=%q en=%q", i, st.Symbol, st.Name, st.EnglishName)
	}
}
