package service

import (
	"testing"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/toss"
)

func TestStockNeedsTossReference(t *testing.T) {
	t.Parallel()

	year := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	shares := int64(100)

	cases := []struct {
		name string
		st   model.Stock
		want bool
	}{
		{
			name: "missing list date",
			st:   model.Stock{SharesOutstanding: &shares},
			want: true,
		},
		{
			name: "missing shares",
			st:   model.Stock{ListDate: &year},
			want: true,
		},
		{
			name: "complete",
			st:   model.Stock{ListDate: &year, SharesOutstanding: &shares},
			want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := stockNeedsTossReference(tc.st); got != tc.want {
				t.Fatalf("stockNeedsTossReference() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMergeTossReference(t *testing.T) {
	t.Parallel()

	seedDate := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	tossDate := "2023-09-14"
	existing := model.Stock{
		Symbol:   "ARM",
		Name:     "Arm Holdings",
		ListDate: &seedDate,
	}

	merged := mergeTossReference(existing, toss.StockInfo{
		Symbol:            "ARM",
		ListDate:          &tossDate,
		SharesOutstanding: "1050000000",
	})

	if merged.ListDate == nil || merged.ListDate.Format("2006-01-02") != tossDate {
		t.Fatalf("listDate = %v, want %s", merged.ListDate, tossDate)
	}
	if merged.SharesOutstanding == nil || *merged.SharesOutstanding != 1050000000 {
		t.Fatalf("shares = %v, want 1050000000", merged.SharesOutstanding)
	}
	if merged.Name != "Arm Holdings" {
		t.Fatalf("name changed to %q", merged.Name)
	}
}

func TestMergeTossReferenceSkipsEmptyTossValues(t *testing.T) {
	t.Parallel()

	seedDate := time.Date(1996, 1, 1, 0, 0, 0, 0, time.UTC)
	existing := model.Stock{Symbol: "CCJ", ListDate: &seedDate}

	merged := mergeTossReference(existing, toss.StockInfo{
		Symbol:            "CCJ",
		ListDate:          nil,
		SharesOutstanding: "",
	})

	if merged.ListDate == nil || !merged.ListDate.Equal(seedDate) {
		t.Fatalf("listDate overwritten: %v", merged.ListDate)
	}
	if merged.SharesOutstanding != nil {
		t.Fatalf("shares set from empty toss value: %v", merged.SharesOutstanding)
	}
}
