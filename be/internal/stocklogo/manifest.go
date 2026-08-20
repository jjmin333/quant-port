package stocklogo

import (
	"encoding/json"
	"os"
	"sync"
)

const defaultLocalLogoPrefix = "/logos/kr/"

// ManifestEntry describes one migrated or fallback logo row.
type ManifestEntry struct {
	Region string `json:"region"`
	Source string `json:"source"` // naver | fmp
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// ManifestSummary holds aggregate migration stats.
type ManifestSummary struct {
	Domestic struct {
		Total        int `json:"total"`
		NaverSuccess int `json:"naverSuccess"`
		NaverFail    int `json:"naverFail"`
		FMPFallback  int `json:"fmpFallback"`
	} `json:"domestic"`
	US struct {
		Total          int `json:"total"`
		NaverSkipped   int `json:"naverSkipped"`
		FMPAvailable   int `json:"fmpAvailable"`
		FMPUnavailable int `json:"fmpUnavailable"`
	} `json:"us"`
}

// Manifest is written to fe/public/logos/manifest.json by migrate-logos.
type Manifest struct {
	GeneratedAt string                     `json:"generatedAt"`
	Summary     ManifestSummary            `json:"summary"`
	Entries     map[string]ManifestEntry   `json:"entries"`
}

var (
	manifestMu sync.RWMutex
	manifest   *Manifest
)

func SetManifest(m *Manifest) {
	manifestMu.Lock()
	defer manifestMu.Unlock()
	manifest = m
}

func GetManifest() *Manifest {
	manifestMu.RLock()
	defer manifestMu.RUnlock()
	return manifest
}

func LoadManifest(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	if m.Entries == nil {
		m.Entries = map[string]ManifestEntry{}
	}
	SetManifest(&m)
	return nil
}

func SaveManifest(path string, m *Manifest) error {
	if m.Entries == nil {
		m.Entries = map[string]ManifestEntry{}
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o644)
}

func localKRPath(symbol string) string {
	return defaultLocalLogoPrefix + symbol + ".png"
}
