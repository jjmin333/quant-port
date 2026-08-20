package resolver



import (

	"context"

	"strings"

	"sync"



	"github.com/enact/quant-port/be/internal/model"

	"github.com/enact/quant-port/be/internal/repository"

)



type Result struct {

	Symbol     string

	Name       string

	Confidence float64

	Error      string

}



type Resolver struct {

	stocks *repository.StockRepo



	catalogMu sync.Mutex

	catalog   []catalogEntry

}



func New(stocks *repository.StockRepo) *Resolver {

	return &Resolver{stocks: stocks}

}



func (r *Resolver) ResolveName(ctx context.Context, rawName string) Result {

	name := strings.TrimSpace(rawName)

	if name == "" {

		return Result{Error: "empty name"}

	}



	for _, sym := range symbolLookupCandidates(name) {
		if st, err := r.stocks.GetBySymbol(ctx, sym); err == nil && st != nil {
			display := st.Name
			if display == "" {
				display = st.EnglishName
			}
			return Result{Symbol: st.Symbol, Name: display, Confidence: 0.95}
		}
	}

	if looksLikeSymbol(name) {

		if st, err := r.stocks.GetBySymbol(ctx, strings.ToUpper(name)); err == nil && st != nil {

			display := st.Name

			if display == "" {

				display = st.EnglishName

			}

			return Result{Symbol: st.Symbol, Name: display, Confidence: 0.95}

		}

	}



	queries := buildSearchQueries(name)

	for _, query := range queries {

		if res, ok := r.searchOne(ctx, query, name); ok {

			return res

		}

	}



	if res, ok := r.fuzzyResolve(ctx, name); ok {

		return res

	}

	return Result{Error: "no match"}

}



func buildSearchQueries(name string) []string {

	seen := map[string]struct{}{}

	var out []string

	add := func(q string) {

		q = strings.TrimSpace(q)

		if q == "" {

			return

		}

		if _, dup := seen[q]; dup {

			return

		}

		seen[q] = struct{}{}

		out = append(out, q)

	}



	add(name)
	if base := listingBaseTicker(name); base != name {
		add(base)
	}
	for _, part := range strings.Fields(name) {

		add(part)

	}

	return out

}

func listingBaseTicker(name string) string {
	name = strings.TrimSpace(name)
	if i := strings.Index(name, "("); i > 0 {
		return strings.TrimSpace(name[:i])
	}
	return name
}

func symbolLookupCandidates(name string) []string {
	var out []string
	seen := map[string]struct{}{}
	add := func(sym string) {
		sym = strings.ToUpper(strings.TrimSpace(sym))
		if sym == "" || !looksLikeSymbol(sym) {
			return
		}
		if _, dup := seen[sym]; dup {
			return
		}
		seen[sym] = struct{}{}
		out = append(out, sym)
	}
	add(name)
	add(listingBaseTicker(name))
	if alias := adrKnownSymbol(listingBaseTicker(name)); alias != "" {
		add(alias)
	}
	return out
}

func adrKnownSymbol(base string) string {
	switch strings.ToUpper(strings.TrimSpace(base)) {
	case "TSMC":
		return "TSM"
	default:
		return ""
	}
}



func (r *Resolver) searchOne(ctx context.Context, query, rawName string) (Result, bool) {

	items, err := r.stocks.Search(ctx, model.StockSearchParams{

		Query: query,

		Limit: 5,

	})

	if err != nil || len(items) == 0 {

		return Result{}, false

	}

	best := items[0]

	display := best.Name

	if display == "" {

		display = best.EnglishName

	}

	if strings.EqualFold(strings.TrimSpace(best.Name), rawName) ||

		strings.EqualFold(strings.TrimSpace(best.EnglishName), rawName) ||

		strings.EqualFold(strings.TrimSpace(best.Symbol), strings.ToUpper(strings.Fields(rawName)[0])) {

		return Result{Symbol: best.Symbol, Name: display, Confidence: 0.92}, true

	}

	score := catalogMatchScore(rawName, display)
	if best.EnglishName != "" {
		if s := catalogMatchScore(rawName, best.EnglishName); s > score {
			score = s
		}
	}
	if score < fuzzyHighConfThreshold {
		return Result{}, false
	}

	return Result{Symbol: best.Symbol, Name: display, Confidence: confidenceFromFuzzyScore(score)}, true

}



func (r *Resolver) fuzzyResolve(ctx context.Context, rawName string) (Result, bool) {

	catalog, err := r.loadCatalog(ctx)

	if err != nil || len(catalog) == 0 {

		return Result{}, false

	}



	best, score, ok := matchBestCatalog(rawName, catalog)

	if !ok {

		return Result{}, false

	}



	display := best.Name

	if display == "" {

		display = best.EnglishName

	}

	return Result{

		Symbol:     best.Symbol,

		Name:       display,

		Confidence: confidenceFromFuzzyScore(score),

	}, true

}



func (r *Resolver) loadCatalog(ctx context.Context) ([]catalogEntry, error) {

	r.catalogMu.Lock()

	defer r.catalogMu.Unlock()

	if r.catalog != nil {

		return r.catalog, nil

	}



	rows, err := r.stocks.ListNameCatalog(ctx)

	if err != nil {

		return nil, err

	}

	catalog := make([]catalogEntry, len(rows))

	for i, row := range rows {

		catalog[i] = catalogEntry{

			Symbol:      row.Symbol,

			Name:        row.Name,

			EnglishName: row.EnglishName,

		}

	}

	r.catalog = catalog

	return r.catalog, nil

}



func looksLikeSymbol(s string) bool {

	s = strings.TrimSpace(s)

	if s == "" || len(s) > 12 {

		return false

	}

	for _, r := range s {

		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '-' {

			continue

		}

		return false

	}

	return true

}


