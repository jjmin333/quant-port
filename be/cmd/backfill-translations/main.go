/*
Command backfill-translations는 **밀린 해외 제목을 한 번에 비운다.**

주기 경로(`NewsStore.backfillUSTranslations`)는 2분마다 스물넷이라, 밀린 것을
비우는 도구가 아니다. 2026-09-15 기준 미번역 2,852건이면 네 시간이 걸리고,
그중 578건은 **7일 창 밖이라 아예 안 걸린다**(`translationWindowDays`). 창은
문자에 돈이 붙는 엔진을 전제로 한 절약이므로, 자체 모델로 도는 동안에는 지킬
것이 없다.

	go run ./cmd/backfill-translations                   # 창 없이 전부
	go run ./cmd/backfill-translations -batch 96         # 묶음을 키운다
	go run ./cmd/backfill-translations -window 7 -dry-run # 무엇이 남았는지만 본다

번역기는 **호스트에서 보이는 주소**로 붙는다(기본 127.0.0.1:8100) — 컨테이너
안에서라면 translator:8100이지만 이 명령은 호스트에서 `go run`으로 돈다.
`cmd/compare-translation`과 같은 규칙이다.

쓰는 것은 `news_articles.headline_ko` 한 칸뿐이다. 원문(`headline`)은 건드리지
않는다 — 그래서 몇 번을 다시 돌려도 잃는 것이 없다.
*/
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/internal/translate"
)

// noWindow는 「창 없음」을 뜻하는 큰 값이다. 0을 주면 리포가 기본값(7일)으로
// 되돌리므로, 전부를 뜻하려면 실제로 큰 수를 줘야 한다.
const noWindow = 3650

func main() {
	batch := flag.Int("batch", 48, "한 회차에 옮길 제목 수")
	window := flag.Int("window", noWindow, "며칠 이내 발행 기사까지 옮길 것인가")
	localURL := flag.String("local", "http://127.0.0.1:8100", "번역기 주소")
	dryRun := flag.Bool("dry-run", false, "남은 건수만 세고 아무것도 쓰지 않는다")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mysql, err := repository.NewMySQLForCLI(cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer mysql.Close()

	// 스키마는 확인하지 않는다 — 이 명령이 만지는 칸(`headline_ko`·`translate_attempts`)은
	// 이미 있는 것이고, **마이그레이션의 주인은 api다**(워커도 읽기 전용으로만 본다).
	// 스택이 도는 중에 CLI가 ALTER를 걸 여지를 두지 않는다.
	newsRepo := repository.NewNewsRepo(mysql.DB())
	stockRepo := repository.NewStockRepo(mysql.DB())

	remaining, err := newsRepo.ListMissingTranslationWindow(ctx, "us", 100000, *window)
	if err != nil {
		log.Fatalf("남은 건수: %v", err)
	}
	log.Printf("미번역 해외 기사 %d건 (창 %d일 · 묶음 %d)", len(remaining), *window, *batch)
	if *dryRun || len(remaining) == 0 {
		return
	}

	/*
		사슬을 **로컬 하나로** 만든다.

		`translate.Build`를 쓰면 TRANSLATE_PROVIDER를 읽는데, 그 값이 azure면 이
		명령이 한도가 말라붙은 엔진에 붙는다. 여기서 하려는 일은 「자체 모델로
		비우기」라고 이름에 적혀 있으므로, 고를 여지를 두지 않는다.
	*/
	// 슬롯 수만 설정에서 가져온다 — 컨테이너의 `--parallel`과 같아야 의미가 있고,
	// 그 둘을 맞추는 자리가 TRANSLATE_LOCAL_SLOTS다.
	local := translate.NewLocal(*localURL, translate.ParseOptions(os.Getenv).LocalSlots, 120*time.Second)
	if !local.Healthy(ctx) {
		log.Fatalf("번역기가 %s 에서 응답하지 않는다 — docker compose --profile legacy-translator up -d translator", *localURL)
	}
	chain := translate.NewChain(local)

	store := service.NewNewsStore(cfg.News, nil, nil, nil, nil, nil, nil, newsRepo, nil, nil, stockRepo, nil, nil, nil)
	store.SetTranslator(chain)

	started := time.Now()
	last := started
	done, failed, err := store.DrainUSTranslations(ctx, *batch, *window, func(done, failed int) {
		// 회차마다 한 줄은 너무 잦다(묶음 48이면 몇 초에 한 번). 10초마다 찍는다.
		if time.Since(last) < 10*time.Second {
			return
		}
		last = time.Now()
		rate := float64(done) / time.Since(started).Seconds()
		log.Printf("  %d건 완료 · %d건 실패 · %.1f건/초 · 남은 %d건 예상 %s",
			done, failed, rate, len(remaining)-done, eta(len(remaining)-done, rate))
	})
	elapsed := time.Since(started)

	log.Printf("끝 — %d건 완료 · %d건 실패 · %s (%.1f건/초)",
		done, failed, elapsed.Round(time.Second), float64(done)/elapsed.Seconds())
	if err != nil {
		log.Fatalf("중단: %v", err)
	}
}

func eta(left int, rate float64) time.Duration {
	if left <= 0 || rate <= 0 {
		return 0
	}
	return (time.Duration(float64(left)/rate) * time.Second).Round(time.Second)
}
