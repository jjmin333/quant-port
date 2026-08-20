package model

import "time"

type NewsPollSettings struct {
	DomesticNaverKeywordsPerTick int       `json:"domesticNaverKeywordsPerTick"`
	DomesticKakaoKeywordsPerTick int       `json:"domesticKakaoKeywordsPerTick"`
	DomesticKakaoPollMinutes     int       `json:"domesticKakaoPollMinutes"`
	USGoogleKeywordsPerTick      int       `json:"usGoogleKeywordsPerTick"`
	UpdatedAt                    time.Time `json:"updatedAt"`
}

func DefaultNewsPollSettings() NewsPollSettings {
	return NewsPollSettings{
		DomesticNaverKeywordsPerTick: 18,
		DomesticKakaoKeywordsPerTick: 10,
		DomesticKakaoPollMinutes:     6,
		USGoogleKeywordsPerTick:      8,
	}
}
