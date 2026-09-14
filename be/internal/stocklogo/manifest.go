package stocklogo

import (
	"encoding/json"
	"os"
	"sync"
)

const defaultLocalLogoPrefix = "/logos/kr/"

const defaultLocalUSLogoPrefix = "/logos/us/"

// ManifestEntry describes one migrated or fallback logo row.
type ManifestEntry struct {
	Region string `json:"region"`
	Source string `json:"source"` // naver | fmp
	Path   string `json:"path,omitempty"`
	Reason string `json:"reason,omitempty"`
	/*
		Tone은 이 로고가 **흰 카드 위에서 읽히려면 무엇이 필요한가**다 — tone.go 참고.

		  "light" 투명 배경 위의 흰 그림. 어두운 판을 깔면 드러난다.
		  "blank" 판을 깔아도 안 보인다(불투명한 흰 이미지). 로고가 없는 것으로 친다.
		  ""      정상.
	*/
	Tone string `json:"tone,omitempty"`
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
	// GeneratedAt is when the **first** full migration ran. Delta runs keep it.
	GeneratedAt string `json:"generatedAt"`
	/*
		RefreshedAt는 **마지막으로 돌린 시각**이다.

		델타 실행은 `GeneratedAt`을 보존하므로, 그것만으로는 월간 갱신이 언제 돌았는지
		알 수 없다 — 실측으로 8월 31일에 국내 로고 52개를 새로 받았는데도 매니페스트는
		7월 29일이라고 적혀 있었다. 한 달에 한 번 손으로 도는 작업이라(`refresh-logos.ps1`)
		"마지막이 언제였나"가 그 작업의 유일한 상태값이다.
	*/
	RefreshedAt string                   `json:"refreshedAt,omitempty"`
	Summary     ManifestSummary          `json:"summary"`
	Entries     map[string]ManifestEntry `json:"entries"`
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
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}
	/*
		화면이 읽는 작은 목록을 **같은 자리에서 같이** 쓴다 (client_index.go).
		둘을 따로 만들게 두면 반드시 어긋나고, 어긋난 그 순간부터 화면은 없는
		파일을 가리키거나 있는 파일을 못 쓴다.
	*/
	return SaveClientIndex(path, m)
}

func localKRPath(symbol string) string {
	return defaultLocalLogoPrefix + symbol + ".png"
}

func localUSPath(symbol string) string { return defaultLocalUSLogoPrefix + symbol + ".png" }
