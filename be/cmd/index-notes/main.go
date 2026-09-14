/*
Command index-notes writes the LLM paragraph for each tracked index.

지수 열셋의 「무슨 국면인가」를 한 문단으로 적는다. 숫자는 규칙이 뽑고(20일선·RSI·
누적 등락률), 모델은 그것들을 엮기만 한다 — 프롬프트가 JSON 밖의 수치를 금지한다
(`internal/llm/index_note.go`).

**평소에는 이 명령을 쓸 일이 없다.** 같은 작업이 worker-scores의 하루 한 번 슬롯에서
지수 점수 직후에 돈다. 여기 있는 이유는 둘이다: 프롬프트를 고친 뒤 확인해 보려면 배치를
기다릴 수 없고, 배치가 실패한 날 손으로 메울 길이 있어야 한다.

같은 거래일에 이미 문단이 있는 지수는 건너뛴다. `-force`가 그것을 넘어선다 — 프롬프트를
바꾼 뒤 같은 날 다시 돌릴 때 쓴다. **호출이 지수 수만큼(최대 13) 나가므로** 습관적으로
붙이지 않는다.

	go run ./cmd/index-notes
	go run ./cmd/index-notes -force
*/
package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/llm"
	"github.com/enact/quant-port/be/internal/marketdata/apibudget"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
)

func main() {
	force := flag.Bool("force", false, "rewrite notes that already exist for today")
	timeoutMin := flag.Int("timeout-min", 10, "job timeout in minutes")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*timeoutMin)*time.Minute)
	defer cancel()

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	if err := repository.MigrateIndexScoresIfNeeded(ctx, mysql.DB(), filepath.Join("migrations")); err != nil {
		log.Fatalf("migrate: %v", err)
	}

	/*
		예산 관리자에 Redis를 붙이지 않는다.

		붙이려면 이 명령이 Redis에 닿아야 하는데, 손으로 도는 도구가 그것 때문에
		실패하는 편이 더 나쁘다. 대가는 이 실행분이 관리자 화면의 LLM 사용량에서
		빠진다는 것이고, 실행이 드물어서(프롬프트를 고칠 때뿐) 감당할 만하다.
		매일 도는 몫은 worker-scores가 공유 예산에서 정확히 센다.
	*/
	svc := service.NewIndexScoreService(mysql.DB(), repository.NewIndexDailyCloseRepo(mysql.DB()))
	svc.SetLLM(llm.NewPoolFromKeys(llm.Config{
		Provider: cfg.LLM.Provider,
		BaseURL:  cfg.LLM.BaseURL,
		Model:    cfg.LLM.Model,
	}, cfg.LLM.KeyKR, cfg.LLM.KeyUS, apibudget.NewManager(nil)))

	res, err := svc.GenerateNotes(ctx, *force)
	if err != nil {
		log.Fatalf("index notes: %v", err)
	}
	log.Printf("지수 해설: 생성 %d · 건너뜀 %d · 실패 %d (토큰 %d)",
		res.Generated, res.Skipped, res.Failed, res.Tokens)
	if res.Generated == 0 && res.Failed == 0 {
		log.Printf("생성된 것이 없습니다 — 이미 오늘 것이 있거나(-force로 덮어쓰기) LLM이 꺼져 있습니다.")
	}
}
