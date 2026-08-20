package textparse

func financeScore(line ParsedLine) int {
	score := 0
	if line.MarketValue != nil {
		score += 2
	}
	if line.ProfitLoss != nil {
		score += 2
	}
	if line.ReturnRate != nil {
		score += 1
	}
	if line.Currency != "" {
		score += 1
	}
	return score
}

func pickRicherFinance(a, b ParsedLine) ParsedLine {
	if financeScore(b) > financeScore(a) {
		b.Quantity = a.Quantity
		if b.Name == "" {
			b.Name = a.Name
		}
		return b
	}
	return a
}

// mergeCombinedFinance merges finance from two OCR rows of the same holding (e.g. two screenshots).
func mergeCombinedFinance(existing, incoming ParsedLine, totalQty float64) ParsedLine {
	out := pickRicherFinance(existing, incoming)
	out.Quantity = totalQty

	_, okA := lineCostBasis(existing)
	_, okB := lineCostBasis(incoming)
	switch {
	case okA && okB:
		eval := lineEvalSum(existing) + lineEvalSum(incoming)
		pnl := linePnLSum(existing) + linePnLSum(incoming)
		if eval > 0 {
			out.MarketValue = &eval
			out.ProfitLoss = &pnl
			out.ReturnRate = nil
		}
	case okA:
		out = existing
		out.Quantity = totalQty
	case okB:
		out = incoming
		out.Quantity = totalQty
	}

	if out.Currency == "" {
		if incoming.Currency != "" {
			out.Currency = incoming.Currency
		} else if existing.Currency != "" {
			out.Currency = existing.Currency
		}
	}
	return out
}

func lineCostBasis(line ParsedLine) (float64, bool) {
	if line.Quantity <= 0 {
		return 0, false
	}
	if line.MarketValue != nil && line.ProfitLoss != nil {
		cost := *line.MarketValue - *line.ProfitLoss
		if cost > 0 {
			return cost, true
		}
	}
	if line.MarketValue != nil && line.ReturnRate != nil {
		cur := *line.MarketValue / line.Quantity
		denom := 1 + *line.ReturnRate
		if cur > 0 && denom != 0 {
			avg := cur / denom
			cost := avg * line.Quantity
			if cost > 0 {
				return cost, true
			}
		}
	}
	return 0, false
}

func lineEvalSum(line ParsedLine) float64 {
	if line.MarketValue != nil {
		return *line.MarketValue
	}
	return 0
}

func linePnLSum(line ParsedLine) float64 {
	if line.ProfitLoss != nil {
		return *line.ProfitLoss
	}
	return 0
}
