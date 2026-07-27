package server

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/enact/quant-port/be/internal/middleware"
)

func (s *Server) routes() http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.CORS(s.cfg.CORSAllowedOrigins))

	r.Get("/health", s.health.Liveness)
	r.Get("/health/api-quotas", s.health.APIQuotas)
	r.Get("/ready", s.health.Readiness)

	r.Route("/api/v1", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/signup", s.authHandler.Signup)
			auth.Post("/login", s.authHandler.Login)
			auth.Post("/refresh", s.authHandler.Refresh)
			auth.With(middleware.RequireAuth(s.auth)).Post("/logout", s.authHandler.Logout)
			auth.With(middleware.RequireAuth(s.auth)).Get("/me", s.authHandler.Me)
		})

		api.Route("/broker", func(broker chi.Router) {
			broker.Use(middleware.RequireAuth(s.auth))
			broker.Post("/toss/link", s.broker.LinkToss)
			broker.Get("/toss/status", s.broker.TossStatus)
			broker.Delete("/toss/unlink", s.broker.UnlinkToss)
		})

		// Public market data (optional auth enriches watchlist/held flags)
		api.Group(func(public chi.Router) {
			public.Use(middleware.OptionalAuth(s.auth))

			public.Get("/quotes/indices", s.quotes.ListIndices)
			public.Get("/quotes", s.quotes.ListQuotes)
			public.Get("/stocks/search", s.quotes.SearchStocksCatalog)
			public.Get("/quotes/{symbol}", s.quotes.GetQuote)
			public.Get("/quotes/{symbol}/orderbook", s.quotes.GetOrderBook)
			public.Get("/quotes/{symbol}/trades", s.quotes.GetTrades)
			public.Get("/quotes/{symbol}/candles", s.quotes.GetCandles)
			public.Get("/quotes/{symbol}/candles/snapshot", s.quotes.GetCandleSnapshot)
			public.Get("/quotes/{symbol}/terminal-snapshot", s.quotes.GetTerminalSnapshot)
			public.Get("/quotes/{symbol}/stream", s.quotes.Stream)

			public.Get("/exchange-rate", s.market.ExchangeRate)
			public.Get("/market-calendar/KR", s.market.KrCalendar)
			public.Get("/market-calendar/US", s.market.UsCalendar)
			public.Get("/rankings", s.market.Rankings)
			public.Get("/market-indicators/prices", s.market.MarketIndicatorPrices)
			public.Get("/stocks", s.market.ListStocks)
			public.Get("/stocks/{symbol}", s.market.GetStock)
			public.Get("/stocks/{symbol}/fundamentals", s.fundamentals.Get)
			public.Get("/stocks/{symbol}/warnings", s.market.StockWarnings)

			public.Get("/news/domestic", s.news.ListDomestic)
			public.Get("/news/us/demo", s.news.UsDemo)
			public.Get("/news/reader", s.news.Reader)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.RequireAuth(s.auth))

			protected.Get("/portfolio/summary", s.portfolio.Summary)
			protected.Get("/portfolio/live", s.portfolio.Live)
			protected.Get("/portfolio/history", s.portfolio.History)
			protected.Get("/portfolio/diagnostics", s.portfolio.Diagnostics)

			protected.Get("/insights/dashboard", s.insights.Dashboard)
			protected.Get("/insights/diagnostics", s.insights.Diagnostics)

			protected.Get("/watchlist", s.watchlist.List)
			protected.Post("/watchlist", s.watchlist.Add)
			protected.Delete("/watchlist/{symbol}", s.watchlist.Remove)

			protected.Get("/disclosures/today", s.disclosures.Today)

			protected.Get("/accounts", s.accounts.List)
			protected.Get("/holdings", s.holdings.Get)

			protected.Get("/orders", s.orders.List)
		})
	})

	return r
}
