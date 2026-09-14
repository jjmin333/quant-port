package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/enact/quant-port/be/internal/metrics"
	"github.com/enact/quant-port/be/internal/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	// chi의 RealIP가 아니라 우리 것을 쓴다 — chi 쪽은 X-Forwarded-For를 조건 없이
	// 믿어서, 레이트 리밋의 키로 삼는 순간 헤더만 바꾸면 우회된다(실측: XFF를 매
	// 요청 바꿔 300발을 보내니 전부 통과했다). 자세한 내용은 middleware/realip.go.
	r.Use(middleware.RealIP(s.cfg.TrustProxyHeaders))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	// 요청 하나가 붙잡을 수 있는 시간의 상한. WriteTimeout은 응답을 쓰기 시작한
	// 뒤부터 재기 때문에 아무것도 쓰지 않고 멈춘 핸들러를 못 막는다 — Redis를
	// 정지시키자 /quotes/{symbol}이 120초를 넘겨도 응답하지 않았다.
	// SSE·업로드·관리자 경로는 제외한다(middleware/timeout.go).
	r.Use(middleware.RequestTimeout(s.cfg.RequestTimeout))
	r.Use(middleware.CORS(s.cfg.CORSAllowedOrigins))

	// 이 API의 응답은 거의 전부 JSON이고 그중 몇은 크다 — 뉴스 목록 98KB, 리포트
	// 목록 72KB. 압축하면 8~15KB로 떨어진다. 로컬에서는 티가 안 나지만 VM 인스턴스의
	// 상행 대역폭과 모바일 회선에서는 이게 체감 속도의 대부분이다.
	//
	// SSE(`text/event-stream`)는 chi 기본 목록에 없어 압축 대상이 아니고, 그대로
	// 흘러간다 — compressResponseWriter가 Flush를 그대로 위임하므로 스트림도 안전하다.
	// `/media/*`가 내보내는 이미지도 이미 압축된 포맷이라 목록 밖이다.
	r.Use(chimw.Compress(5))

	r.Get("/health", s.health.Liveness)
	r.Get("/health/api-quotas", s.health.APIQuotas)
	r.Get("/ready", s.health.Readiness)
	// /metrics는 nginx가 프록시하지 않는다 — 내부 네트워크에서만 닿는다.
	r.Get("/metrics", metrics.Handler(s.mysql.DB(), s.redisPoolStats))

	// 레이트 리밋은 **인증 미들웨어 뒤**에 붙는다. 로그인 사용자는 user_id로,
	// 비로그인은 IP로 세는데, 인증보다 앞에 두면 컨텍스트에 사용자가 아직 없어서
	// 로그인 사용자도 전부 공유 IP 버킷에 떨어진다 — NAT 뒤 사용자들이 서로를
	// 밀어내는, 이 설계가 피하려던 바로 그 상황이 된다.
	// 그래서 전역이 아니라 아래 그룹마다 건다. 임계값과 트레이드오프는
	// internal/middleware/ratelimit.go 주석 참조.
	limit := middleware.RateLimit(middleware.RateLimitConfig{
		Enabled:   s.cfg.RateLimit.Enabled,
		UserRate:  s.cfg.RateLimit.UserRate,
		UserBurst: s.cfg.RateLimit.UserBurst,
		IPRate:    s.cfg.RateLimit.IPRate,
		IPBurst:   s.cfg.RateLimit.IPBurst,
	}, s.bgDone)

	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			// 인증 경로는 로그인 전이라 언제나 IP 기준이다. 무차별 대입에
			// 대한 완전한 방어는 아니지만(계정별 잠금은 별도), 초당 수십 회
			// 시도는 여기서 걸린다.
			auth.Use(limit)
			auth.Post("/signup", s.authHandler.Signup)
			auth.Post("/login", s.authHandler.Login)
			auth.Post("/refresh", s.authHandler.Refresh)
			auth.With(middleware.RequireAuth(s.auth)).Post("/logout", s.authHandler.Logout)
			auth.With(middleware.RequireAuth(s.auth)).Get("/me", s.authHandler.Me)
			// 계정 관리. 셋 다 현재 비밀번호를 다시 받으므로 IP 리미터 아래 둔다.
			auth.With(middleware.RequireAuth(s.auth)).Post("/password", s.authHandler.ChangePassword)
			auth.With(middleware.RequireAuth(s.auth)).Post("/display-name", s.authHandler.ChangeDisplayName)
			auth.With(middleware.RequireAuth(s.auth)).Post("/delete", s.authHandler.DeleteAccount)
		})

		// Public market data (optional auth enriches watchlist/held flags)
		api.Group(func(public chi.Router) {
			public.Use(middleware.OptionalAuth(s.auth))
			public.Use(limit)

			// 브라우저에서 난 오류를 로그로만 받는다. 비로그인 화면에서도
			// 나므로 공개 경로이고, 위의 `limit`이 IP 단위로 폭주를 막는다.
			// 자세한 배경은 handler/client_error.go.
			public.Post("/client-errors", s.clientError.Report)

			public.Get("/quotes/indices", s.quotes.ListIndices)
			public.Get("/quotes/batch", s.quotes.BatchQuotes)
			public.Get("/quotes", s.quotes.ListQuotes)
			public.Get("/stocks/search", s.quotes.SearchStocksCatalog)
			public.Get("/quotes/{symbol}", s.quotes.GetQuote)
			public.Get("/quotes/{symbol}/orderbook", s.quotes.GetOrderBook)
			public.Get("/quotes/{symbol}/trades", s.quotes.GetTrades)
			public.Get("/quotes/{symbol}/candles", s.quotes.GetCandles)
			public.Get("/quotes/{symbol}/candles/snapshot", s.quotes.GetCandleSnapshot)
			public.Get("/quotes/{symbol}/terminal-snapshot", s.quotes.GetTerminalSnapshot)
			public.Get("/quotes/{symbol}/stream", s.quotes.Stream)
			public.Get("/quotes/{symbol}/quote-stream", s.quotes.QuoteStream)
			/*
				목록 화면의 가격 스트림. `/quotes/{symbol}` 계열보다 **위에** 있을
				필요는 없다 — chi는 같은 자리에서 정적 세그먼트(`prices`)를 파라미터
				(`{symbol}`)보다 먼저 맞춘다. 그래도 이웃한 스트림 둘 옆에 둔다.

				경로가 `/stream`으로 끝나므로 요청 타임아웃 미들웨어가 알아서 비켜난다
				(`middleware/timeout.go`) — SSE는 응답이 안 끝나는 것이 정상이다.
			*/
			public.Get("/quotes/prices/stream", s.quotes.ListPriceStream)

			public.Get("/exchange-rate", s.market.ExchangeRate)
			public.Get("/market-calendar/KR", s.market.KrCalendar)
			public.Get("/market-calendar/US", s.market.UsCalendar)
			public.Get("/rankings", s.market.Rankings)
			public.Get("/rankings/prices", s.market.RankingPrices)
			public.Get("/rankings/history", s.market.RankingHistory)
			public.Get("/market/data-settings", s.market.DataSettings)
			public.Get("/market/clocks", s.market.MarketClocks)
			public.Get("/market-indicators/prices", s.market.MarketIndicatorPrices)
			public.Get("/stocks", s.market.ListStocks)
			public.Get("/stocks/{symbol}", s.market.GetStock)
			public.Get("/stocks/{symbol}/fundamentals", s.fundamentals.Get)
			// {symbol} 라우트보다 먼저 등록한다 — 안 그러면 "earnings"가 심볼
			// 이름으로 먹힌다 (user/holdings/trades와 같은 이유).
			public.Get("/stocks/earnings/batch", s.earnings.GetMany)
			public.Get("/stocks/{symbol}/earnings", s.earnings.Get)
			public.Get("/stocks/{symbol}/financials/history", s.financialHistory.Get)
			public.Get("/stocks/{symbol}/insider", s.insider.Get)
			public.Get("/stocks/{symbol}/summary", s.stockSummary.Get)
			public.Get("/indices/scores", s.scores.IndexScores)
			public.Get("/stocks/scores/top", s.scores.TopScores)
			public.Get("/stocks/scores/batch", s.scores.GetMany)
			public.Get("/stocks/{symbol}/score", s.scores.Get)
			public.Get("/stocks/{symbol}/score/history", s.scores.History)
			public.Get("/stocks/{symbol}/daily-closes", s.dailyCloses.StockDailyCloses)
			public.Get("/indices/{indexId}/daily-closes", s.dailyCloses.IndexDailyCloses)
			public.Get("/stocks/{symbol}/warnings", s.market.StockWarnings)

			// 시장 전체 실적 캘린더. 「내 종목」은 인증이 아니라 `symbols=`로
			// 좁힌다 — 화면이 이미 관심·보유 목록을 들고 있다.
			// summary를 {…} 없는 고정 경로 뒤에 두는 순서 문제는 없지만,
			// 더 긴 경로를 먼저 적어 읽는 사람이 헷갈리지 않게 한다.
			public.Get("/earnings/calendar/summary", s.earningsCalendar.Summary)
			public.Get("/earnings/calendar", s.earningsCalendar.Calendar)
			public.Get("/earnings/surprises", s.earningsCalendar.Surprises)
			public.Get("/earnings/stats", s.earningsCalendar.Stats)
			public.Get("/earnings/highlights", s.earningsCalendar.Highlights)

			// Research calls are readable without an account (the feed is the
			// public half of the feature); OptionalAuth still identifies the
			// author so their own private/retracted calls stay visible to them.
			public.Get("/research/calls", s.research.List)
			public.Get("/research/calls/{id}", s.research.Get)
			public.Get("/research/calls/{id}/revisions", s.research.Revisions)
			public.Get("/research/calls/{id}/comments", s.research.Comments)
			// Views are recorded by an explicit call, not as a side effect of
			// the detail fetch — otherwise the counter measures re-renders.
			public.Post("/research/calls/{id}/view", s.research.RecordView)
			public.Get("/research/authors/{userId}", s.research.AuthorProfile)

			// Uploaded images are served without auth: they are embedded in
			// public report bodies, so a logged-out reader must be able to
			// load them. Keys are content hashes, so the URL is the capability
			// — nothing is enumerable from it.
			public.Get("/media/*", s.mediaFiles.Serve)
			public.Get("/research/symbols/{symbol}/summary", s.research.SymbolSummary)

			public.Get("/news/domestic", s.news.ListDomestic)
			public.Get("/news/us", s.news.ListUS)
			public.Get("/news/us/demo", s.news.UsDemo)
			public.Get("/news/{market}/briefing", s.news.Briefing)
			public.Get("/news/{market}/dashboard", s.news.Dashboard)
			public.Get("/news/{market}/keywords", s.news.Keywords)
			public.Get("/news/{market}/symbols", s.news.Symbols)
			public.Get("/news/reader", s.news.Reader)
			public.Post("/news/view", s.news.RecordView)

			// 「내 투자」 메뉴의 예시 데이터. 이 화면들의 진짜 엔드포인트는 전부
			// 인증 뒤에 있어, 공개 경로가 없으면 로그인하지 않은 방문자에게는
			// 보여줄 것이 아무것도 없다 — 빈 화면을 보고 가입 여부를 정하게 된다.
			// 응답은 고정 픽스처라 읽을 사용자 데이터가 없고, 따라서 확인할
			// 세션도 없다.
			public.Get("/sample/stamps", s.sample.Stamps)
			public.Get("/sample/stamps/summary", s.sample.StampSummary)
			public.Get("/sample/watchlist", s.sample.Watchlist)
			public.Get("/sample/holdings", s.sample.Holdings)
			public.Get("/sample/journal/notes", s.sample.Journal)
			public.Get("/sample/price-targets", s.sample.Targets)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(s.auth))
			protected.Use(limit)

			protected.Put("/watchlist/order", s.watchlist.Reorder)
			protected.Get("/watchlist", s.watchlist.List)
			protected.Post("/watchlist", s.watchlist.Add)
			protected.Post("/watchlist/bulk", s.watchlist.BulkAdd)
			protected.Delete("/watchlist/{symbol}", s.watchlist.Remove)

			protected.Get("/journal/notes", s.journal.List)
			protected.Post("/journal/notes", s.journal.Create)
			protected.Put("/journal/notes/{id}", s.journal.Update)
			protected.Delete("/journal/notes/{id}", s.journal.Delete)

			protected.Get("/user/price-targets", s.priceTargets.List)
			protected.Post("/user/price-targets", s.priceTargets.Create)
			protected.Put("/user/price-targets/{id}", s.priceTargets.Update)
			protected.Delete("/user/price-targets/{id}", s.priceTargets.Delete)

			protected.Get("/stamps", s.stamps.List)
			protected.Post("/stamps", s.stamps.Create)
			protected.Get("/stamps/summary", s.stamps.Summary)
			protected.Get("/stamps/state", s.stamps.State)
			protected.Get("/stamps/{id}", s.stamps.Get)
			protected.Patch("/stamps/{id}", s.stamps.Update)
			protected.Patch("/stamps/{id}/prediction", s.stamps.Revise)
			protected.Post("/stamps/{id}/open", s.stamps.Open)
			protected.Delete("/stamps/{id}", s.stamps.Delete)

			// Uploads need an account; the matching GET is public (see above).
			// Not research-specific — any feature that needs an image URL uses
			// this one endpoint.
			protected.Post("/media/images", s.mediaFiles.UploadImage)

			// No DELETE here on purpose — a call can be retracted but never
			// removed, or the hit rates stop meaning anything.
			protected.Post("/research/calls", s.research.Publish)
			protected.Post("/research/preview", s.research.Preview)
			protected.Get("/research/mine", s.research.Mine)
			protected.Get("/research/summary", s.research.Summary)
			protected.Patch("/research/calls/{id}", s.research.Update)
			protected.Post("/research/calls/{id}/retract", s.research.Retract)
			protected.Post("/research/calls/{id}/report", s.research.Report)
			protected.Post("/research/calls/{id}/reactions", s.research.ToggleReaction)
			protected.Post("/research/calls/{id}/comments", s.research.AddComment)
			protected.Put("/research/comments/{commentId}", s.research.UpdateComment)
			protected.Delete("/research/comments/{commentId}", s.research.DeleteComment)
			protected.Post("/research/comments/{commentId}/like", s.research.ToggleCommentLike)
			protected.Post("/research/calls/{id}/ratings", s.research.Rate)

			protected.Get("/user/holdings", s.userHoldings.List)
			protected.Put("/user/holdings", s.userHoldings.Replace)
			// Registered before the {symbol} routes so "trades" and "valuation"
			// are not swallowed as symbol names.
			protected.Get("/user/holdings/trades", s.holdingTrades.List)
			protected.Post("/user/holdings/trades", s.holdingTrades.Create)
			protected.Post("/user/holdings/trades/bulk-register", s.holdingTrades.BulkRegister)
			protected.Delete("/user/holdings/trades/{id}", s.holdingTrades.Delete)
			protected.Get("/user/holdings/valuation", s.holdingTrades.Valuation)
			protected.Delete("/user/holdings/{symbol}", s.userHoldings.Delete)
			protected.Get("/user/cash-balances", s.userCashBalances.Get)
			protected.Put("/user/cash-balances", s.userCashBalances.Replace)

			protected.Post("/import/holdings/screenshots", s.importJobs.UploadScreenshots)
			protected.Get("/import/holdings/jobs/{jobId}", s.importJobs.GetJob)
			protected.Post("/import/holdings/jobs/{jobId}/commit", s.importJobs.CommitJob)
		})

		api.Route("/admin", func(admin chi.Router) {
			admin.Use(middleware.RequireAuth(s.auth))
			admin.Use(middleware.RequireAdmin(s.users))
			admin.Use(limit)

			admin.Get("/news-keywords", s.admin.ListNewsKeywords)
			admin.Post("/news-keywords", s.admin.CreateNewsKeyword)
			admin.Put("/news-keywords/{id}", s.admin.UpdateNewsKeyword)
			admin.Delete("/news-keywords/{id}", s.admin.DeleteNewsKeyword)
			admin.Get("/news/stats", s.admin.NewsStats)
			admin.Get("/news/poll-settings", s.admin.GetNewsPollSettings)
			admin.Put("/news/poll-settings", s.admin.UpdateNewsPollSettings)
			admin.Post("/news/reset-cursors", s.admin.ResetNewsCursors)
			admin.Get("/api-quotas", s.admin.APIQuotas)
			admin.Get("/stock-summary", s.admin.StockSummaryStatus)
			admin.Get("/llm-settings", s.admin.GetLLMSettings)
			admin.Put("/llm-settings", s.admin.UpdateLLMSettings)
			admin.Get("/llm-requests", s.admin.GetLLMRequestLog)
			admin.Post("/llm/refresh", s.admin.TriggerLLMRefresh)
			admin.Get("/llm/refresh", s.admin.LLMRefreshStatus)
			admin.Get("/llm/batch", s.admin.LLMBatchStatus)
			admin.Get("/llm/batch/targets", s.admin.LLMBatchTargets)
			admin.Get("/batch/jobs", s.admin.GetBatchJobs)
			// 사이드바 뱃지가 주기적으로 묻는 가벼운 입구 — 판정만 돌려준다.
			admin.Get("/batch/health", s.admin.GetBatchHealth)
			admin.Get("/ops/incidents", s.admin.GetOpsIncidents)
			// 시간표·수동 실행 안내 문서. 화면이 코드를 대신 설명하지 않도록,
			// 리포지터리의 그 문서를 그대로 내려준다.
			admin.Get("/batch/schedule", s.admin.GetBatchSchedule)
			admin.Put("/batch/schedule", s.admin.PutBatchSchedule)
			admin.Post("/batch/run", s.admin.RunBatchJob)
			admin.Get("/score/settings", s.admin.GetScoreSettings)
			admin.Put("/score/settings", s.admin.UpdateScoreSettings)
			admin.Put("/score/symbol-weights", s.admin.UpsertScoreSymbolWeights)
			admin.Delete("/score/symbol-weights", s.admin.DeleteScoreSymbolWeights)
			admin.Post("/score/recompute", s.admin.RecomputeScores)
			admin.Get("/score/status", s.admin.ScoreStatus)
			admin.Get("/market-data/settings", s.admin.GetMarketDataSettings)
			admin.Put("/market-data/settings", s.admin.UpdateMarketDataSettings)
			// 종목 별칭 관리 (`handler/admin_aliases.go`). 쓰기는 이 프로세스의
			// 스냅샷을 곧바로 다시 읽고, 워커들은 자기 TTL(10분)로 따라온다.
			admin.Get("/aliases", s.admin.ListStockAliases)
			admin.Post("/aliases", s.admin.CreateStockAlias)
			admin.Put("/aliases/{id}", s.admin.UpdateStockAlias)
			admin.Delete("/aliases/{id}", s.admin.DeleteStockAlias)
			admin.Get("/cache/namespaces", s.admin.ListCacheNamespaces)
			admin.Get("/cache/entries", s.admin.ListCacheEntries)
			admin.Get("/cache/entry", s.admin.GetCacheEntry)
			admin.Put("/cache/entry", s.admin.UpdateCacheEntry)
			admin.Delete("/cache/entry", s.admin.DeleteCacheEntry)

			// Research moderation. Hiding is the only admin write — the call's
			// frozen values stay frozen even here.
			admin.Get("/research/calls", s.research.AdminList)
			admin.Put("/research/calls/{id}/hidden", s.research.SetHidden)

			// Database console — table catalog + row CRUD. Tables reachable
			// here are limited to the whitelist in service/db_admin_catalog.go.
			admin.Get("/db/tables", s.admin.ListDBTables)
			admin.Get("/db/tables/{table}", s.admin.DescribeDBTable)
			admin.Get("/db/tables/{table}/rows", s.admin.ListDBRows)
			admin.Post("/db/tables/{table}/rows", s.admin.CreateDBRow)
			admin.Put("/db/tables/{table}/rows", s.admin.UpdateDBRow)
			admin.Delete("/db/tables/{table}/rows", s.admin.DeleteDBRow)
		})
	})

	return r
}
