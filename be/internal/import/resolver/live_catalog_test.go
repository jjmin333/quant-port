package resolver

import (
	"context"
	"os"
	"testing"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func TestLiveResolveKiwoomETFs(t *testing.T) {
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
	for _, raw := range []string{"KODEX 미국나스.", "TIGER 미국나스.", "TIGER 미국나스닥100", "006×미국나스.", "미국나스"} {
		res := r.ResolveName(ctx, raw)
		t.Logf("%q -> %+v", raw, res)
	}
}
func TestLiveResolveMaplebin(t *testing.T) {
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

	stockRepo := repository.NewStockRepo(mysql.DB())
	ctx := context.Background()
	if st, err := stockRepo.GetBySymbol(ctx, "APP"); err == nil && st != nil {
		t.Logf("APP catalog: name=%q english=%q", st.Name, st.EnglishName)
		t.Logf("scores: name=%.3f english=%.3f",
			catalogMatchScore("맵러빈", st.Name),
			catalogMatchScore("맵러빈", st.EnglishName))
	}
	catalog, err := stockRepo.ListNameCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	entries := make([]catalogEntry, len(catalog))
	for i, row := range catalog {
		entries[i] = catalogEntry{Symbol: row.Symbol, Name: row.Name, EnglishName: row.EnglishName}
	}
	best, score, ok := matchBestCatalog("맵러빈", entries)
	t.Logf("matchBestCatalog: ok=%v best=%+v score=%.3f catalog=%d", ok, best, score, len(entries))

	type hit struct {
		entry catalogEntry
		score float64
	}
	var top []hit
	for _, e := range entries {
		s := catalogMatchScore("맵러빈", e.Name)
		if e.EnglishName != "" {
			if es := catalogMatchScore("맵러빈", e.EnglishName); es > s {
				s = es
			}
		}
		if s >= 0.75 {
			top = append(top, hit{e, s})
		}
	}
	for i := 0; i < len(top); i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].score > top[i].score {
				top[i], top[j] = top[j], top[i]
			}
		}
	}
	limit := 8
	if len(top) < limit {
		limit = len(top)
	}
	for i := 0; i < limit; i++ {
		t.Logf("top[%d] %s name=%q score=%.3f", i, top[i].entry.Symbol, top[i].entry.Name, top[i].score)
	}

	r := New(stockRepo)
	res := r.ResolveName(ctx, "맵러빈")
	t.Logf("result: %+v", res)
	if res.Error != "" || res.Symbol != "APP" {
		t.Fatalf("want APP got %+v err=%q", res, res.Error)
	}
}
