package repository

import "testing"

func TestScreenScopeClause(t *testing.T) {
	c, args := screenAssetTypeClause(ScreenScope{AssetTypes: []string{"domestic"}, Sector: "information_technology"})
	if c != " AND s.asset_type IN (?) AND s.symbol IN (SELECT ss.symbol FROM stock_sectors ss WHERE ss.sector = ?)" || len(args) != 2 {
		t.Fatalf("clause %q args %v", c, args)
	}
	if c, args := screenAssetTypeClause(ScreenScope{}); c != "" || len(args) != 0 {
		t.Fatalf("빈 범위는 절이 없다: %q", c)
	}
}
