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
		/*
			22 = hot 14 · warm 4 · cold 4 (service.splitLaneBudget).

			예전 18은 큐 225개를 **한 줄로** 돌리던 시절의 값이라 한 바퀴가 25분,
			평균 지연이 12.5분이었다. 차선을 나눈 지금은 같은 예산의 대부분이 뉴스를
			내는 검색어로 가므로, 넷을 더해 hot 회전을 4분에 맞춘다. 네이버 일 호출
			13,250 → 15,840(한도 24,000의 66%). 카카오는 이미 한도의 82%를 쓰고
			있어 손대지 않는다(실측 2026-09-17).
		*/
		DomesticNaverKeywordsPerTick: 22,
		DomesticKakaoKeywordsPerTick: 10,
		DomesticKakaoPollMinutes:     6,
		USGoogleKeywordsPerTick:      8,
	}
}
