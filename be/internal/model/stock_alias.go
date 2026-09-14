package model

import "time"

/*
StockAlias — 종목 별칭 표 한 행 (`stock_aliases`).

무엇을 담고 왜 표인지는 마이그레이션 머리글(`migrations/070_stock_aliases.sql`)에
있다. 여기서는 **화면이 무엇을 보는가**만 적는다.

  Kind    "brand"  한글 읽기 → 영문 표기  (「에스케이」 → sk)
          "symbol" 한글 별칭 → 종목 심볼  (「구글」 → GOOGL)

  Source  "seed"   마이그레이션이 넣은 것
          "toss"   배치가 채울 자리 (지금은 안 쓴다)
          "manual" 운영 중에 사람이 넣은 것

  Enabled 오태깅이 나면 **지우지 말고 끈다.** 지우면 「왜 뺐는지」가 note와 함께
          사라지고 다음 사람이 같은 별칭을 다시 넣는다.
*/
type StockAlias struct {
	ID        int64     `json:"id"`
	Alias     string    `json:"alias"`
	Kind      string    `json:"kind"`
	Target    string    `json:"target"`
	Source    string    `json:"source"`
	Note      string    `json:"note"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}
