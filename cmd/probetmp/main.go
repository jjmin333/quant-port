package main

import (
	"context"
	"fmt"
	"log"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
)

func main() {
	cfg, _ := config.Load()
	m, _ := repository.NewMySQL(cfg.MySQLDSN)
	defer m.Close()
	ctx := context.Background()
	q := func(label, query string, args ...any) {
		rows, err := m.DB().QueryContext(ctx, query, args...)
		if err != nil { fmt.Println(label, "ERR", err); return }
		defer rows.Close()
		cols, _ := rows.Columns()
		fmt.Println("==", label)
		for rows.Next() {
			vals := make([]any, len(cols)); ptrs := make([]any, len(cols))
			for i := range vals { ptrs[i] = &vals[i] }
			rows.Scan(ptrs...)
			out := ""
			for i, c := range cols {
				if b, ok := vals[i].([]byte); ok { out += fmt.Sprintf("%s=%s ", c, string(b)) } else { out += fmt.Sprintf("%s=%v ", c, vals[i]) }
			}
			fmt.Println("  ", out)
		}
	}
	q("005930 1d bars", `SELECT bar_time, DAYNAME(bar_time) dow, close_price, volume FROM candle_bars WHERE symbol='005930' AND bar_interval='1d' AND bar_time>='2026-07-28' ORDER BY bar_time`)
	q("AAPL 1d bars", `SELECT bar_time, DAYNAME(bar_time) dow, close_price FROM candle_bars WHERE symbol='AAPL' AND bar_interval='1d' AND bar_time>='2026-07-28' ORDER BY bar_time`)
	q("weekend 1d bar counts", `SELECT DAYNAME(bar_time) dow, COUNT(*) c, COUNT(DISTINCT symbol) syms FROM candle_bars WHERE bar_interval='1d' AND bar_time>='2026-08-01' GROUP BY DAYNAME(bar_time)`)
}
