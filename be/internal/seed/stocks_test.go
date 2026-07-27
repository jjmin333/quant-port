package seed

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseUSIPOYear(t *testing.T) {
	t.Parallel()

	cases := []struct {
		in   string
		want string
	}{
		{"2023", "2023-01-01"},
		{"1996", "1996-01-01"},
		{"", ""},
		{"n/a", ""},
		{"1799", ""},
	}
	for _, tc := range cases {
		got := parseUSIPOYear(tc.in)
		if tc.want == "" {
			if got != nil {
				t.Fatalf("parseUSIPOYear(%q) = %v, want nil", tc.in, got)
			}
			continue
		}
		if got == nil {
			t.Fatalf("parseUSIPOYear(%q) = nil, want %s", tc.in, tc.want)
		}
		if got.Format("2006-01-02") != tc.want {
			t.Fatalf("parseUSIPOYear(%q) = %s, want %s", tc.in, got.Format("2006-01-02"), tc.want)
		}
	}
}

func TestLoadUSJSONSetsListDateFromIPOYear(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "sample.json")
	const body = `[
	  {"symbol":"CCJ","name":"Cameco Corporation Common Stock","ipoyear":"1996"},
	  {"symbol":"AMAT","name":"Applied Materials Inc. Common Stock","ipoyear":"1972"},
	  {"symbol":"EMPTY","name":"No IPO Year Corp","ipoyear":""}
	]`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	stocks, err := LoadUSJSON(path, "NYSE")
	if err != nil {
		t.Fatal(err)
	}
	bySymbol := map[string]struct {
		hasDate bool
		date    string
	}{}
	for _, s := range stocks {
		entry := struct {
			hasDate bool
			date    string
		}{}
		if s.ListDate != nil {
			entry.hasDate = true
			entry.date = s.ListDate.Format("2006-01-02")
		}
		bySymbol[s.Symbol] = entry
	}

	if got := bySymbol["CCJ"]; !got.hasDate || got.date != "1996-01-01" {
		t.Fatalf("CCJ listDate = %+v, want 1996-01-01", got)
	}
	if got := bySymbol["AMAT"]; !got.hasDate || got.date != "1972-01-01" {
		t.Fatalf("AMAT listDate = %+v, want 1972-01-01", got)
	}
	if got := bySymbol["EMPTY"]; got.hasDate {
		t.Fatalf("EMPTY listDate = %s, want nil", got.date)
	}
}
