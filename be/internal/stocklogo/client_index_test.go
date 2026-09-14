package stocklogo

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildClientIndex(t *testing.T) {
	idx := BuildClientIndex(&Manifest{Entries: map[string]ManifestEntry{
		"005930": {Region: "domestic", Source: "naver", Path: "/logos/kr/005930.png"},
		"000660": {Region: "domestic", Source: "naver", Path: "/logos/kr/000660.png"},
		"AAPL":   {Region: "us", Source: "fmp", Path: "/logos/us/AAPL.png"},
		"SOXL":   {Region: "us", Source: "fmp", Path: "/logos/us/SOXL.png", Tone: "light"},
		"SOXS":   {Region: "us", Source: "fmp", Path: "/logos/us/SOXS.png", Tone: "blank"},
		// 받아 두지 못한 종목은 어느 목록에도 없다 — 화면이 원격으로 떨어진다.
		"NVDA": {Region: "us", Source: "fmp", Reason: "fmp 404"},
	}})

	if idx.KR != "000660,005930" {
		t.Errorf("KR = %q", idx.KR)
	}
	// blank은 파일이 있어도 us 목록에 넣지 않는다 — 「로고 없음」으로 친다.
	if idx.US != "AAPL,SOXL" {
		t.Errorf("US = %q", idx.US)
	}
	if idx.Light != "SOXL" {
		t.Errorf("Light = %q", idx.Light)
	}
	if idx.Blank != "SOXS" {
		t.Errorf("Blank = %q", idx.Blank)
	}
}

// SaveManifest는 화면용 목록을 **같이** 쓴다. 하나만 갱신되면 어긋난다.
func TestSaveManifestWritesClientIndex(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	m := &Manifest{Entries: map[string]ManifestEntry{
		"005930": {Region: "domestic", Source: "naver", Path: "/logos/kr/005930.png"},
		"AAPL":   {Region: "us", Source: "fmp", Path: "/logos/us/AAPL.png"},
	}}
	if err := SaveManifest(path, m); err != nil {
		t.Fatalf("SaveManifest: %v", err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "logo-index.json"))
	if err != nil {
		t.Fatalf("logo-index.json not written: %v", err)
	}
	var idx ClientIndex
	if err := json.Unmarshal(raw, &idx); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if idx.KR != "005930" || idx.US != "AAPL" {
		t.Fatalf("client index = %+v", idx)
	}
}
