package resolver

import (
	"context"
	"os"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveTSMCXOVR(t *testing.T) {
	if os.Getenv("LIVE_MYSQL") != "1" {
		t.Skip("set LIVE_MYSQL=1")
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
	cases := map[string]string{
		"TSMC(ADR)": "TSM",
		"XOVR":      "XOVR",
	}
	for raw, want := range cases {
		res := r.ResolveName(ctx, raw)
		t.Logf("%q -> %+v", raw, res)
		if res.Symbol != want {
			t.Fatalf("%q: want symbol %s got %q err=%q", raw, want, res.Symbol, res.Error)
		}
	}
}
