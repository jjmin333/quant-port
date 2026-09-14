/*
Command compare-translation은 **자체 모델과 「예전 Papago가 내놓은 번역」을 나란히 놓는다.**

자체 모델로 갈아타기 전에 반드시 해야 하는 일이다. 기계 번역 품질은 지표로만 보면
멀쩡한데 화면에서 읽으면 이상한 경우가 흔해서, 사람이 몇십 건을 직접 읽어야 한다.
이 명령은 그 읽을거리를 만든다.

**Papago를 지금 부르지 않는다.** NCP 자격증명이 만료돼 401을 낸다(2026-09-03 확인).
대신 DB에 이미 쌓여 있는 `headline_ko`를 쓴다 — 그 값들은 키가 살아 있던 동안
Papago가 실제로 내놓은 번역이라, 비교 대상으로 이보다 정확한 것이 없다. 3만 4천 건이
있으므로 표본이 모자랄 일도 없다.

화면에는 아무 영향이 없다 — DB를 읽기만 하고 아무것도 쓰지 않는다.

	go run ./cmd/compare-translation -n 40
	go run ./cmd/compare-translation -n 200 -csv out.csv   # 표로 받아 읽기
*/
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	_ "github.com/go-sql-driver/mysql"

	"github.com/enact/quant-port/be/internal/config"
	"github.com/enact/quant-port/be/internal/translate"
)

func main() {
	n := flag.Int("n", 40, "비교할 제목 수")
	localURL := flag.String("local", "http://127.0.0.1:8100", "번역기 컨테이너 주소")
	csvPath := flag.String("csv", "", "결과를 CSV로 저장할 경로")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	db, err := sql.Open("mysql", cfg.MySQLDSN)
	if err != nil {
		log.Fatalf("mysql: %v", err)
	}
	defer db.Close()

	pairs, err := loadPairs(db, *n)
	if err != nil {
		log.Fatalf("headlines: %v", err)
	}
	if len(pairs) == 0 {
		log.Fatal("비교할 기사를 못 찾았다 — 번역이 붙은 해외 기사가 없다")
	}
	fmt.Printf("제목 %d건 (기존 Papago 번역이 있는 기사)\n\n", len(pairs))

	ctx := context.Background()
	local := translate.NewLocal(*localURL, 2, 120*time.Second)
	if !local.Healthy(ctx) {
		log.Fatalf("번역기가 %s 에서 응답하지 않는다 — docker compose up -d translator", *localURL)
	}

	/*
		자체 모델에도 **이름 잠그기를 그대로 적용한다.** 이걸 빼고 재면 운영에서 도는
		것과 다른 것을 재게 된다 — 이름 잠그기가 있고 없고가 품질 차이의 큰 몫이다.
	*/
	masked := make([]string, len(pairs))
	restores := make([][]string, len(pairs))
	for i, p := range pairs {
		masked[i], restores[i] = translate.MaskNames(p.English, p.Names)
	}

	start := time.Now()
	raw, _ := local.TranslateENtoKO(ctx, masked)
	localMs := time.Since(start).Milliseconds()

	rows := [][]string{{"#", "english", "local", "papago(저장분)", "verdict"}}
	accepted, restoreFailed, empty := 0, 0, 0
	for i, p := range pairs {
		lo := at(raw, i)
		verdict := "ok"
		switch {
		case strings.TrimSpace(lo) == "":
			empty++
			verdict = "EMPTY"
		default:
			restored, ok := translate.RestoreNames(lo, restores[i])
			if !ok {
				restoreFailed++
				verdict = "NAME-LOST"
			} else if !translate.Accept(p.English, restored) {
				verdict = "REJECT"
			} else {
				accepted++
			}
			lo = translate.ApplyGlossary(p.English, restored)
		}
		rows = append(rows, []string{fmt.Sprint(i + 1), p.English, lo, p.Papago, verdict})

		fmt.Printf("%3d  EN     %s\n", i+1, p.English)
		fmt.Printf("     자체 %-9s %s\n", verdict, lo)
		fmt.Printf("     파파고      %s\n\n", p.Papago)
	}

	fmt.Println(strings.Repeat("─", 72))
	fmt.Printf("자체 모델 : %d건 중 통과 %d · 이름 유실 %d · 빈 응답 %d\n",
		len(pairs), accepted, restoreFailed, empty)
	fmt.Printf("속도      : %s (건당 %s) · 글자 %d자\n",
		ms(localMs), ms(localMs/int64(max(1, len(pairs)))), totalRunes(englishOf(pairs)))
	fmt.Printf("\n두 열이 **다르게** 나온 건은 사람이 읽어야 한다 — 지표로는 안 갈린다.\n")

	if *csvPath != "" {
		if err := writeCSV(*csvPath, rows); err != nil {
			log.Fatalf("csv: %v", err)
		}
		fmt.Printf("\nCSV: %s\n", *csvPath)
	}
}

// Pair는 한 기사의 원문·저장된 Papago 번역·그 기사에 붙은 종목 이름이다.
type Pair struct {
	English string
	Papago  string
	Names   []translate.NamePair
}

func englishOf(pairs []Pair) []string {
	out := make([]string, len(pairs))
	for i, p := range pairs {
		out[i] = p.English
	}
	return out
}

/*
**이미 번역이 붙은** 해외 기사를 최신순으로 가져온다.

그 `headline_ko`는 Papago 키가 살아 있던 동안 Papago가 실제로 내놓은 값이다.
지금은 401이라 다시 부를 수 없으므로, 비교 대상으로 이것을 쓴다.

종목 이름도 함께 읽는다 — 운영과 같은 조건으로 재려면 이름 잠그기가 켜져 있어야 한다.
*/
func loadPairs(db *sql.DB, limit int) ([]Pair, error) {
	/*
		**자체 모델이 방금 넣은 행을 빼야 한다.**

		처음엔 최신순으로 뽑았는데, 번역기를 띄운 뒤로는 최신 번역이 곧 자체 모델의
		결과다 — 그러면 자기 자신과 비교하게 되고 실제로 25건 중 여러 건이 글자까지
		똑같이 나왔다. 비교가 아니라 항등식이었다.

		`updated_at`이 하루 넘게 지난 행만 본다. 그 시점에는 번역기가 없었으므로
		그 `headline_ko`는 Papago가 남긴 것이 확실하다.
	*/
	rows, err := db.Query(`SELECT headline, headline_ko, symbols_json FROM news_articles
	                        WHERE market='us' AND headline <> ''
	                          AND COALESCE(headline_ko,'') <> ''
	                          AND updated_at < NOW() - INTERVAL 1 DAY
	                        ORDER BY published_at DESC LIMIT ?`, limit*3)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	seen := map[string]bool{}
	var out []Pair
	var symbols []string
	for rows.Next() {
		var en, ko string
		var symJSON []byte
		if err := rows.Scan(&en, &ko, &symJSON); err != nil {
			return nil, err
		}
		en, ko = strings.TrimSpace(en), strings.TrimSpace(ko)
		// 원문이 이미 한국어면 Papago도 손대지 않았다 — 비교할 것이 없다.
		if en == "" || ko == "" || seen[en] || !translate.NeedsTranslation(en) {
			continue
		}
		seen[en] = true
		var syms []string
		_ = json.Unmarshal(symJSON, &syms)
		symbols = append(symbols, syms...)
		out = append(out, Pair{English: en, Papago: ko})
		if len(out) >= limit {
			break
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return attachNames(db, out, symbols), nil
}

/*
기사에 붙은 종목의 영어·한국어 이름을 붙인다 — 운영의 `namePairsFor`와 같은 재료다.

여기서는 종목을 기사별로 가르지 않고 **한 번에 모아** 붙인다. 비교용이라 정밀도보다
조건을 같게 맞추는 것이 중요하고, 이름 잠그기는 낱말 경계를 보므로 관련 없는 이름이
섞여도 그 기사에 없으면 잠기지 않는다.
*/
func attachNames(db *sql.DB, pairs []Pair, symbols []string) []Pair {
	if len(symbols) == 0 {
		return pairs
	}
	uniq := map[string]bool{}
	args := make([]any, 0, len(symbols))
	holes := make([]string, 0, len(symbols))
	for _, s := range symbols {
		s = strings.ToUpper(strings.TrimSpace(s))
		if s == "" || uniq[s] {
			continue
		}
		uniq[s] = true
		args = append(args, s)
		holes = append(holes, "?")
	}
	rows, err := db.Query(`SELECT COALESCE(english_name,''), COALESCE(name,'') FROM stocks
	                        WHERE symbol IN (`+strings.Join(holes, ",")+`)`, args...)
	if err != nil {
		return pairs
	}
	defer rows.Close()

	var names []translate.NamePair
	for rows.Next() {
		var en, ko string
		if err := rows.Scan(&en, &ko); err != nil {
			break
		}
		en, ko = strings.TrimSpace(en), strings.TrimSpace(ko)
		if en == "" || ko == "" || en == ko {
			continue
		}
		names = append(names, translate.NamePair{English: en, Korean: ko})
	}
	for i := range pairs {
		pairs[i].Names = names
	}
	return pairs
}
func writeCSV(path string, rows [][]string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	// 엑셀이 UTF-8로 읽게 BOM을 붙인다 — 없으면 한글이 깨져 보여서, 사람이
	// 읽으라고 만든 파일이 못 읽는 파일이 된다. 리터럴로 적으면 Go 소스 중간의
	// BOM이 되어 컴파일이 막히므로 바이트로 쓴다.
	if _, err := f.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return err
	}
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func at(list []string, i int) string {
	if i < len(list) {
		return list[i]
	}
	return ""
}

func ms(v int64) string { return (time.Duration(v) * time.Millisecond).String() }

func totalRunes(texts []string) int {
	n := 0
	for _, t := range texts {
		n += utf8.RuneCountInString(t)
	}
	return n
}
