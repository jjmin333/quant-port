package service

import (
	"context"
	"strings"
	"time"
)

func (st *NewsStore) runKakaoPoller(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		st.refreshKakao(ctx)
		cfg := st.pollConfig(ctx)
		interval := time.Duration(cfg.DomesticKakaoPollMinutes) * time.Minute
		if interval < time.Minute {
			interval = time.Minute
		}
		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
	}
}

func mergeArticleKeywords(existing, incoming []string) []string {
	if len(incoming) == 0 {
		return existing
	}
	seen := map[string]struct{}{}
	var out []string
	add := func(v string) {
		if v == "" {
			return
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, v)
	}
	for _, v := range existing {
		add(v)
	}
	for _, v := range incoming {
		add(v)
	}
	return out
}
