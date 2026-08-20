package service

import (
	"sort"
	"strings"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// Daily-close series primitives.
//
// These are market-data mechanics, not the property of any one feature. They
// used to live in the stamp files, which made 리서치 콜 look like it was built
// on top of 스탬프 — the two features are independent and neither should own
// the other's arithmetic. Anything stamp- or research-specific stays in that
// feature's own file.

const seriesDateLayout = "2006-01-02"

// closeSeries is an ascending (date, close) list supporting "the close on or
// before D" lookups without another round trip.
type closeSeries struct {
	points []repository.DailyClosePoint
}

func newCloseSeries(points []repository.DailyClosePoint) closeSeries {
	sorted := make([]repository.DailyClosePoint, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Date < sorted[j].Date })
	return closeSeries{points: sorted}
}

func (s closeSeries) empty() bool { return len(s.points) == 0 }

// onOrBefore returns the latest point at or before date.
func (s closeSeries) onOrBefore(date string) (repository.DailyClosePoint, bool) {
	idx := sort.Search(len(s.points), func(i int) bool { return s.points[i].Date > date })
	if idx == 0 {
		return repository.DailyClosePoint{}, false
	}
	return s.points[idx-1], true
}

// firstAfter returns the earliest point strictly after date.
func (s closeSeries) firstAfter(date string) (repository.DailyClosePoint, bool) {
	idx := sort.Search(len(s.points), func(i int) bool { return s.points[i].Date > date })
	if idx >= len(s.points) {
		return repository.DailyClosePoint{}, false
	}
	return s.points[idx], true
}

func (s closeSeries) latest() (repository.DailyClosePoint, bool) {
	if len(s.points) == 0 {
		return repository.DailyClosePoint{}, false
	}
	return s.points[len(s.points)-1], true
}

func closePctChange(from, to float64) (float64, bool) {
	if from <= 0 {
		return 0, false
	}
	return (to - from) / from * 100, true
}

// horizonGapDays is how far past a target date we still accept the next close.
// A record taken on a Friday has its 1D target on Saturday, where "last close on
// or before target" is the record's own day — without this the 1D bucket would
// stay empty forever. The window is small enough that a symbol which stopped
// trading cannot fill a short bucket with a much later close.
const horizonGapDays = 5

// resolveHorizonClose picks the close representing `target` for a record made on
// baseTradeDate, or reports that the horizon has not elapsed yet.
func resolveHorizonClose(
	prices closeSeries, baseTradeDate string, target time.Time,
) (repository.DailyClosePoint, bool) {
	targetDate := target.Format(seriesDateLayout)
	// A close at or before the record itself is not a result — it is the
	// record's own day (or older), so the horizon has simply not elapsed.
	if point, ok := prices.onOrBefore(targetDate); ok && point.Date > baseTradeDate {
		return point, true
	}
	next, ok := prices.firstAfter(baseTradeDate)
	if !ok {
		return repository.DailyClosePoint{}, false
	}
	if next.Date > target.AddDate(0, 0, horizonGapDays).Format(seriesDateLayout) {
		return repository.DailyClosePoint{}, false
	}
	return next, true
}

// benchmarkForStock picks the index a symbol's alpha is measured against.
// Callers fix it at creation so a later catalog edit cannot silently restate
// history.
func benchmarkForStock(assetType model.QuoteAssetType, market string) string {
	kosdaq := strings.Contains(strings.ToUpper(market), "KOSDAQ")
	switch assetType {
	case model.AssetUS, model.AssetETF:
		return "sp500"
	}
	if kosdaq {
		return "kosdaq"
	}
	return "kospi"
}

// nyLocation is resolved once — the container may ship without tzdata, in which
// case the fixed offset is close enough to pick a calendar date.
var nyLocation = func() *time.Location {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		return time.FixedZone("EST", -5*3600)
	}
	return loc
}()

// marketTradeDate is the market-local calendar date a moment belongs to. US
// symbols must not inherit the KST date: at 09:00 KST the US session is still
// the previous US day, and using KST there would shift every horizon by one.
func marketTradeDate(now time.Time, assetType model.QuoteAssetType) string {
	switch assetType {
	case model.AssetUS, model.AssetETF:
		return now.In(nyLocation).Format(seriesDateLayout)
	default:
		return now.In(model.KST).Format(seriesDateLayout)
	}
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
