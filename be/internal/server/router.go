package server

import (
	"net/http"
	"time"

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
			// 비로그인도 200 — 첫 방문자 콘솔에 401 두 줄이 찍히던 자리 (handler/auth.go Session).
			auth.With(middleware.OptionalAuth(s.auth)).Get("/session", s.authHandler.Session)
			// 계정 관리. 셋 다 현재 비밀번호를 다시 받으므로 IP 리미터 아래 둔다.
			auth.With(middleware.RequireAuth(s.auth)).Post("/password", s.authHandler.ChangePassword)
			auth.With(middleware.RequireAuth(s.auth)).Post("/display-name", s.authHandler.ChangeDisplayName)
			auth.With(middleware.RequireAuth(s.auth)).Post("/delete", s.authHandler.DeleteAccount)
		})


		/*
			communityGate — 커뮤니티·투자 리포트를 관리자에게만 연다(2026-10-12 사용자 결정: 가상 데이터로
			화면을 시험하는 동안 일반 방문자·검색엔진에 닫는다). 공개로 되돌릴 때는 이 줄을 통과 미들웨어로
			바꾸고 FE lib/hidden-features.ts의 ADMIN_ONLY_ROUTE_PREFIXES를 비운다.
		*/
		communityGate := middleware.RequireAdmin(s.users)

		// Public market data (optional auth enriches watchlist/held flags)
		api.Group(func(public chi.Router) {
			public.Use(middleware.OptionalAuth(s.auth))
			public.Use(limit)

			/*
				Cloudflare 엣지 캐시 등급 (middleware/edge_cache.go, docs/EDGE_CACHE.md).
				**요청자와 무관한 공개 조회에만** 붙인다. 처리 중 사용자 신원을 한 번이라도
				보면 미들웨어가 알아서 private으로 떨어뜨리므로(/quotes/*가 그렇다 — 관심·보유
				표시를 덧씌운다) 목록 실수가 개인화 응답을 퍼뜨리지는 않는다. 그래도 그런
				경로는 여기 넣지 않는다: 헤더만 private이 될 뿐 얻는 것이 없다.

				첫 숫자는 엣지가 「신선하다」고 보는 시간, 둘째는 낡은 사본을 주며 뒤에서
				갱신해도 되는 시간이다. 서버 쪽 갱신 주기보다 길게 잡지 않는다.
			*/
			edgeLive := middleware.EdgeCache(15*time.Second, time.Minute)   // 지수·순위·환율·뉴스판 — 폴러 1~2분. 지수는 서버 60초 + 엣지 75초 < FE 낡음 판정 180초
			edgeNews := middleware.EdgeCache(30*time.Second, 5*time.Minute) // 브리핑·키워드
			edgeDaily := middleware.EdgeCache(time.Minute, 10*time.Minute)  // 배치가 하루 한두 번 채우는 값
			edgeSlow := middleware.EdgeCache(5*time.Minute, time.Hour)      // 달력·백테스트

			// 브라우저에서 난 오류를 로그로만 받는다. 비로그인 화면에서도
			// 나므로 공개 경로이고, 위의 `limit`이 IP 단위로 폭주를 막는다.
			// 자세한 배경은 handler/client_error.go.
			public.Post("/client-errors", s.clientError.Report)

			// 화면 제목·설명 덮어쓰기 (handler/page_meta.go). 읽는 쪽이 Next 서버라
			// 공개 경로다 — 렌더 시점에는 로그인한 사람이 없다(크롤러일 수도 있다).
			public.Get("/page-meta", s.pageMeta.List)

			public.With(edgeLive).Get("/quotes/indices", s.quotes.ListIndices)
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

			public.With(edgeLive).Get("/exchange-rate", s.market.ExchangeRate)
			public.With(edgeSlow).Get("/market-calendar/KR", s.market.KrCalendar)
			public.With(edgeSlow).Get("/market-calendar/US", s.market.UsCalendar)
			public.With(edgeLive).Get("/rankings", s.market.Rankings)
			public.Get("/rankings/prices", s.market.RankingPrices)
			public.With(edgeDaily).Get("/rankings/history", s.market.RankingHistory)
			public.With(edgeNews).Get("/market/data-settings", s.market.DataSettings)
			public.With(edgeDaily).Get("/market/clocks", s.market.MarketClocks)
			public.Get("/market-indicators/prices", s.market.MarketIndicatorPrices)
			public.Get("/stocks", s.market.ListStocks)
			public.With(edgeDaily).Get("/stocks/{symbol}", s.market.GetStock)
			public.With(edgeDaily).Get("/stocks/{symbol}/fundamentals", s.fundamentals.Get)
			// {symbol} 라우트보다 먼저 등록한다 — 안 그러면 "earnings"가 심볼
			// 이름으로 먹힌다 (user/holdings/trades와 같은 이유).
			public.Get("/stocks/earnings/batch", s.earnings.GetMany)
			public.With(edgeDaily).Get("/stocks/{symbol}/earnings", s.earnings.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/financials/history", s.financialHistory.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/insider", s.insider.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/summary", s.stockSummary.Get)
			public.With(edgeDaily).Get("/indices/scores", s.scores.IndexScores)
			public.With(edgeDaily).Get("/stocks/scores/top", s.scores.TopScores)
			public.Get("/stocks/scores/batch", s.scores.GetMany)
			public.With(edgeDaily).Get("/stocks/scores/changes", s.scores.Changes)
			public.With(edgeSlow).Get("/stocks/scores/backtest", s.scoreBacktest.Run)
			public.With(edgeSlow).Get("/stocks/scores/backtest/benchmarks", s.scoreBacktest.Benchmarks)
			/*
				스크리너. 조건 목록은 GET이고 조회는 POST다 — 조건 12개를
				쿼리스트링에 담으면 URL 상한에 걸리고, 사용자마다 다른 조건이라
				URL을 키로 잡는 캐시는 적중률이 0에 가깝다 (handler 머리글).
			*/
			public.With(edgeDaily).Get("/stocks/screen/fields", s.screener.Fields)
			public.With(edgeDaily).Get("/stocks/screen/presets", s.screener.Presets)
			public.Post("/stocks/screen", s.screener.Screen)
			public.With(edgeDaily).Get("/stocks/{symbol}/screen-values", s.screener.SymbolValues)
			public.With(edgeDaily).Get("/stocks/{symbol}/signals", s.externalSignals.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/analyst", s.externalSignals.Analyst)
			// 홈 「미국 개미 분위기」(2026-10-01 요청 3) — Reddit 시장 단위 집계, 배치가 하루 한 번 채운다.
			public.With(edgeDaily).Get("/social/market-mood", s.externalSignals.MarketMood)
			public.With(edgeDaily).Get("/stocks/{symbol}/score", s.scores.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/score/history", s.scores.History)
			public.With(edgeDaily).Get("/stocks/{symbol}/score/explain", s.scores.Explain)
			public.With(edgeDaily).Get("/stocks/{symbol}/qa", s.seoStocks.QA)
			public.With(edgeDaily).Get("/stocks/{symbol}/timeline", s.stockTimeline.Get)
			// 「변화」 메뉴 — 점수 배치가 매일 채운 이벤트를 읽기만 한다.
			public.With(edgeDaily).Get("/changes", s.changes.List)
			// AI 점수 성적표 — 축별 IC. 점수 배치가 매일 쌓은 값을 요약만 한다.
			public.With(edgeDaily).Get("/stocks/scores/report-card", s.reportCard.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/etf-check", s.etfScores.Get)
			public.With(edgeDaily).Get("/etf/scores/top", s.etfScores.Top)
			// 금융상품 비교 — 금감원 공시를 worker-filings가 하루 한 번 채운다(05:10). 읽기만.
			public.With(edgeDaily).Get("/fin/products", s.finProducts.List)
			public.With(edgeDaily).Get("/fin/products/{id}", s.finProducts.Get)
			// 순자산 확장 공공데이터 — worker-filings가 채운 표를 읽기만(코인 시세만 1분 캐시).
			public.With(edgeDaily).Get("/realestate/regions", s.wealth.Regions)
			public.With(edgeDaily).Get("/realestate/complexes", s.wealth.Complexes)
			// 헤더 검색의 자산 칸(아파트·금융상품·금리·지역, 2026-10-09).
			public.With(edgeDaily).Get("/search/assets", s.assetSearch.Search)
			public.With(edgeDaily).Get("/realestate/complex", s.wealth.Complex)
			public.With(edgeDaily).Get("/realestate/market", s.wealth.Market)
			public.With(edgeDaily).Get("/macro/rates", s.wealth.Rates)
			public.With(edgeDaily).Get("/auctions/onbid", s.wealth.Onbid)
			public.With(edgeDaily).Get("/auctions/onbid/stats", s.wealth.OnbidStats)
			public.Get("/crypto/markets", s.wealth.CryptoMarkets)
			// 자산 점수·AI 해설(worker-asset의 asset_scores·asset_ai가 채운다).
			public.With(edgeDaily).Get("/realestate/overview", s.wealth.RealEstateOverview)
			public.With(edgeDaily).Get("/realestate/rankings", s.wealth.RealEstateRankings)
			public.With(edgeDaily).Get("/auctions/onbid/rounds", s.wealth.OnbidRounds)
			public.With(edgeDaily).Get("/home/assets", s.wealth.HomeAssets)
			// 이 단지·지역·상품·금리를 태그한 공개 글(2026-10-10 요청 4).
			public.Get("/assets/posts", s.notifications.AssetPosts)
			public.With(edgeDaily).Get("/assets/peek", s.mentions.PeekAsset)
			// 종목 페이지 SEO — 색인 대상(시총 상위 1,000)과 서버 렌더 요약.
			public.Get("/seo/stocks", s.seoStocks.List)
			public.Get("/seo/stocks/{symbol}", s.seoStocks.Get)
			// 「내 기준」 — 저장된 축 점수를 사용자 가중치로 다시 합산한 랭킹.
			public.Get("/stocks/scores/custom-ranking", s.scoreCustom.Ranking)
			// 목표가 시나리오 — 약세·중립·강세 셋과 그 방법의 실측 성적.
			public.With(edgeDaily).Get("/stocks/{symbol}/scenario", s.priceScenario.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/timeline/news", s.stockTimeline.DayNews)
			// 여러 종목을 한 번에(포트폴리오 위험 카드). 정적 경로라 {symbol}보다 먼저 맞는다.
			public.Get("/stocks/daily-closes/batch", s.closesBatch.Get)
			public.With(edgeDaily).Get("/stocks/{symbol}/daily-closes", s.dailyCloses.StockDailyCloses)
			public.With(edgeDaily).Get("/indices/{indexId}/daily-closes", s.dailyCloses.IndexDailyCloses)
			public.With(edgeDaily).Get("/stocks/{symbol}/warnings", s.market.StockWarnings)

			// 시장 전체 실적 캘린더. 「내 종목」은 인증이 아니라 `symbols=`로
			// 좁힌다 — 화면이 이미 관심·보유 목록을 들고 있다.
			// summary를 {…} 없는 고정 경로 뒤에 두는 순서 문제는 없지만,
			// 더 긴 경로를 먼저 적어 읽는 사람이 헷갈리지 않게 한다.
			public.With(edgeDaily).Get("/earnings/calendar/summary", s.earningsCalendar.Summary)
			public.With(edgeDaily).Get("/earnings/calendar", s.earningsCalendar.Calendar)
			public.With(edgeDaily).Get("/earnings/surprises", s.earningsCalendar.Surprises)
			public.With(edgeDaily).Get("/earnings/stats", s.earningsCalendar.Stats)
			// 실적 관전 포인트 — 배치가 저장한 것만 읽는다(2026-09-27 결정 C ②).
			public.With(edgeDaily).Get("/earnings/previews", s.earningsPreview.List)
			public.With(edgeDaily).Get("/earnings/digest", s.earningsPreview.Digest)
			public.With(edgeDaily).Get("/earnings/highlights", s.earningsCalendar.Highlights)

			// Research calls are readable without an account (the feed is the
			// public half of the feature); OptionalAuth still identifies the
			// author so their own private/retracted calls stay visible to them.
			public.With(communityGate).Get("/research/calls", s.research.List)
			public.With(communityGate).Get("/research/calls/{id}", s.research.Get)
			public.With(communityGate).Get("/research/calls/{id}/revisions", s.research.Revisions)
			public.With(communityGate).Get("/research/calls/{id}/comments", s.research.Comments)
			// AI 검토 노트 — 배치가 저장한 것만(2026-09-27 결정 C ④).
			public.With(communityGate).Get("/research/calls/{id}/review", s.researchReview.Get)
			// Views are recorded by an explicit call, not as a side effect of
			// the detail fetch — otherwise the counter measures re-renders.
			public.With(communityGate).Post("/research/calls/{id}/view", s.research.RecordView)
			public.With(communityGate).Get("/research/authors/{userId}", s.research.AuthorProfile)

			// Uploaded images are served without auth: they are embedded in
			// public report bodies, so a logged-out reader must be able to
			// load them. Keys are content hashes, so the URL is the capability
			// — nothing is enumerable from it.
			public.Get("/media/*", s.mediaFiles.Serve)
			public.With(communityGate).Get("/research/symbols/{symbol}/summary", s.research.SymbolSummary)
			// 커뮤니티 읽기 — 비로그인도(검색 색인 허용, 2026-09-27 결정). 좋아요 여부·내 글 표시 때문에
			// 신원을 읽으므로 엣지 캐시는 걸지 않는다(IdentityProbe가 어차피 private로 돌린다).
			public.With(communityGate).Get("/community/posts", s.community.List)
			public.With(communityGate).Get("/community/posts/{id}", s.community.Get)
			public.With(communityGate).Get("/community/posts/{id}/comments", s.community.Comments)
			public.With(communityGate).Post("/community/posts/{id}/view", s.community.RecordView)
			public.With(communityGate).Get("/community/sitemap", s.community.Sitemap)

			public.With(edgeLive).Get("/news/domestic", s.news.ListDomestic)
			public.With(edgeLive).Get("/news/us", s.news.ListUS)
			public.Get("/news/us/demo", s.news.UsDemo)
			public.With(edgeNews).Get("/news/{market}/briefing", s.news.Briefing)
			public.With(edgeLive).Get("/news/{market}/dashboard", s.news.Dashboard)
			public.With(edgeNews).Get("/news/{market}/keywords", s.news.Keywords)
			public.With(edgeNews).Get("/news/{market}/symbols", s.news.Symbols)
			public.Get("/news/reader", s.news.Reader)
			public.Post("/news/view", s.news.RecordView)

			// 「내 투자」 메뉴의 예시 데이터. 이 화면들의 진짜 엔드포인트는 전부
			// 인증 뒤에 있어, 공개 경로가 없으면 로그인하지 않은 방문자에게는
			// 보여줄 것이 아무것도 없다 — 빈 화면을 보고 가입 여부를 정하게 된다.
			// 응답은 고정 픽스처라 읽을 사용자 데이터가 없고, 따라서 확인할
			// 세션도 없다.
			public.With(edgeDaily).Get("/sample/stamps", s.sample.Stamps)
			public.With(edgeDaily).Get("/sample/stamps/summary", s.sample.StampSummary)
			public.With(edgeDaily).Get("/sample/watchlist", s.sample.Watchlist)
			public.With(edgeDaily).Get("/sample/holdings", s.sample.Holdings)
			public.With(edgeDaily).Get("/sample/journal/notes", s.sample.Journal)
			public.With(edgeDaily).Get("/sample/price-targets", s.sample.Targets)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(s.auth))
			protected.Use(limit)

			protected.Put("/watchlist/order", s.watchlist.Reorder)
			protected.Get("/watchlist", s.watchlist.List)
			protected.Post("/watchlist", s.watchlist.Add)
			protected.Post("/watchlist/bulk", s.watchlist.BulkAdd)
			protected.Delete("/watchlist/{symbol}", s.watchlist.Remove)
			// 관심 금융상품 — 주식 관심 목록과 별개(fin_watchlist).
			protected.Get("/fin/watchlist", s.finProducts.Watchlist)
			protected.Put("/fin/watchlist/{id}", s.finProducts.Watch)
			protected.Delete("/fin/watchlist/{id}", s.finProducts.Unwatch)
			// 순자산 — 시세 기호가 없는 자산(부동산·예금·채권·코인·금·기타)과 부채.
			protected.Get("/wealth", s.wealth.Get)
			protected.Get("/realestate/watch", s.wealth.ListWatch)
			protected.Put("/realestate/watch", s.wealth.AddWatch)
			protected.Delete("/realestate/watch", s.wealth.RemoveWatch)
			protected.Post("/wealth/assets", s.wealth.SaveAsset)
			protected.Get("/wealth/assets/{id}", s.wealth.AssetDetail)
			protected.Put("/wealth/assets/{id}", s.wealth.SaveAsset)
			protected.Delete("/wealth/assets/{id}", s.wealth.DeleteAsset)
			protected.Post("/wealth/liabilities", s.wealth.SaveLiability)
			protected.Put("/wealth/liabilities/{id}", s.wealth.SaveLiability)
			protected.Delete("/wealth/liabilities/{id}", s.wealth.DeleteLiability)

			// 앱 안 알림 — 헤더 종 아이콘(2026-10-10 요청 4, 웹 푸시 아님).
			protected.Get("/notifications", s.notifications.List)
			protected.Get("/notifications/unread", s.notifications.Unread)
			protected.Post("/notifications/read", s.notifications.MarkRead)

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
			// 커뮤니티 쓰기. 이미지는 저장 전에 다시 굽는다(≤1MB · 평균 100~200KB).
			protected.With(communityGate).Post("/community/images", s.mediaFiles.UploadCommunityImage)
			protected.With(communityGate).Post("/community/posts", s.community.Create)
			protected.Post("/mentions/preview", s.mentions.Preview)
			protected.Get("/mentions/assets", s.mentions.SearchAssets)
			// 작성자 본인 + 관리자가 고치고 지우는 경로에만 관리자 표시를 붙인다(2026-10-09 요청 1).
			markAdmin := middleware.MarkAdmin(s.users)
			protected.With(communityGate, markAdmin).Patch("/community/posts/{id}", s.community.Update)
			protected.With(communityGate, markAdmin).Delete("/community/posts/{id}", s.community.Delete)
			protected.With(communityGate).Post("/community/posts/{id}/like", s.community.LikePost)
			protected.With(communityGate).Post("/community/posts/{id}/comments", s.community.AddComment)
			protected.With(communityGate, markAdmin).Delete("/community/comments/{commentId}", s.community.DeleteComment)
			protected.With(communityGate).Post("/community/comments/{commentId}/like", s.community.LikeComment)
			protected.With(communityGate).Post("/community/reports", s.community.Report)

			// 삭제는 완전 삭제다 — 성적에서도 빠진다(2026-10-09 사용자 결정). 철회는 행이 남는다.
			protected.With(communityGate).Post("/research/calls", s.research.Publish)
			protected.With(communityGate).Post("/research/preview", s.research.Preview)
			protected.With(communityGate).Get("/research/mine", s.research.Mine)
			protected.With(communityGate).Get("/research/summary", s.research.Summary)
			protected.With(communityGate, markAdmin).Patch("/research/calls/{id}", s.research.Update)
			protected.With(communityGate, markAdmin).Delete("/research/calls/{id}", s.research.Delete)
			protected.With(communityGate).Post("/research/calls/{id}/retract", s.research.Retract)
			protected.With(communityGate).Post("/research/calls/{id}/report", s.research.Report)
			protected.With(communityGate).Post("/research/calls/{id}/reactions", s.research.ToggleReaction)
			protected.With(communityGate).Post("/research/calls/{id}/comments", s.research.AddComment)
			protected.With(communityGate, markAdmin).Put("/research/comments/{commentId}", s.research.UpdateComment)
			protected.With(communityGate, markAdmin).Delete("/research/comments/{commentId}", s.research.DeleteComment)
			protected.With(communityGate).Post("/research/comments/{commentId}/like", s.research.ToggleCommentLike)
			protected.With(communityGate).Post("/research/calls/{id}/ratings", s.research.Rate)

			protected.Get("/user/holdings", s.userHoldings.List)
			protected.Put("/user/holdings", s.userHoldings.Replace)
			// Registered before the {symbol} routes so "trades" and "valuation"
			// are not swallowed as symbol names.
			protected.Get("/user/holdings/trades", s.holdingTrades.List)
			protected.Post("/user/holdings/trades", s.holdingTrades.Create)
			protected.Post("/user/holdings/trades/bulk-register", s.holdingTrades.BulkRegister)
			protected.Delete("/user/holdings/trades/{id}", s.holdingTrades.Delete)
			protected.Get("/user/holdings/valuation", s.holdingTrades.Valuation)
			protected.Get("/user/holdings/diagnosis", s.holdingsDiag.Get)
			protected.Put("/user/holdings/{symbol}/thesis", s.holdingThesis.Put)
			// 저장한 스크리너 조건. 같은 이름으로 다시 저장하면 덮어쓴다.
			// 「이 종목에 대한 내 기록」 — 발견→근거→판단→반영→추적을 잇는 한 줄.
			protected.Get("/user/symbols/{symbol}/context", s.symbolContext.Get)
			protected.Get("/user/changes", s.changes.Mine)
			protected.Get("/user/changes/summary", s.changes.Summary)
			// 홈 「내 종목」 카드 — 관심·보유 · 점수 변화 · 24시간 뉴스 · 오늘의 변화 한 줄.
			protected.Get("/user/today", s.myToday.Get)
			protected.Get("/user/score-weights", s.scoreCustom.GetWeights)
			protected.Put("/user/score-weights", s.scoreCustom.PutWeights)
			protected.Get("/user/screens", s.screener.ListScreens)
			protected.Post("/user/screens", s.screener.SaveScreen)
			protected.Delete("/user/screens/{id}", s.screener.DeleteScreen)
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
			admin.Post("/batch/run-overdue", s.admin.RunOverdueBatches)
			// 검색 유입 보고서(worker-growth가 만든다 · 읽기만).
			admin.Get("/growth/reports", s.adminGrowth.List)
			admin.Get("/growth/reports/{id}", s.adminGrowth.Get)
			admin.Get("/score/settings", s.admin.GetScoreSettings)
			admin.Put("/score/settings", s.admin.UpdateScoreSettings)
			admin.Put("/score/symbol-weights", s.admin.UpsertScoreSymbolWeights)
			admin.Delete("/score/symbol-weights", s.admin.DeleteScoreSymbolWeights)
			admin.Post("/score/recompute", s.admin.RecomputeScores)
			admin.Get("/score/status", s.admin.ScoreStatus)
			admin.Get("/score/explain-weights", s.admin.ScoreExplainWeights)
			admin.Get("/score/weight-suggestion", s.reportCard.WeightSuggestion)
			admin.Get("/score/ranking-history", s.admin.ScoreRankingHistoryStatus)
			admin.Post("/score/ranking-history/backfill", s.admin.BackfillScoreRankingHistory)
			admin.Get("/market-data/settings", s.admin.GetMarketDataSettings)
			admin.Put("/market-data/settings", s.admin.UpdateMarketDataSettings)
			// 종목 별칭 관리 (`handler/admin_aliases.go`). 쓰기는 이 프로세스의
			// 스냅샷을 곧바로 다시 읽고, 워커들은 자기 TTL(10분)로 따라온다.
			admin.Get("/aliases", s.admin.ListStockAliases)
			admin.Post("/aliases", s.admin.CreateStockAlias)
			admin.Put("/aliases/{id}", s.admin.UpdateStockAlias)
			admin.Delete("/aliases/{id}", s.admin.DeleteStockAlias)
			// 화면 메타 관리 (`handler/admin_page_meta.go`). 빈 값을 저장하면 그 행이
			// 지워지고 화면은 코드 기본값으로 돌아간다 — 쓰기와 삭제가 한 길이다.
			admin.Get("/page-meta", s.admin.ListPageMeta)
			admin.Put("/page-meta", s.admin.PutPageMeta)
			admin.Delete("/page-meta", s.admin.DeletePageMeta)
			admin.Get("/cache/namespaces", s.admin.ListCacheNamespaces)
			admin.Get("/cache/entries", s.admin.ListCacheEntries)
			admin.Get("/cache/entry", s.admin.GetCacheEntry)
			admin.Put("/cache/entry", s.admin.UpdateCacheEntry)
			admin.Delete("/cache/entry", s.admin.DeleteCacheEntry)

			// Research moderation. Hiding is the only admin write — the call's
			// frozen values stay frozen even here.
			admin.Get("/research/calls", s.research.AdminList)
			admin.Put("/research/calls/{id}/hidden", s.research.SetHidden)
			admin.Get("/community/moderation", s.community.AdminModeration)
			admin.Put("/community/{type}/{id}/status", s.community.AdminSetStatus)

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
