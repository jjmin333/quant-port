package marketdata

import "time"

type Quote struct {
	Value      float64
	Change     float64
	ChangeRate float64
	Sparkline  []float64
	AsOf       time.Time // provider observation or quote timestamp (UTC)
}
