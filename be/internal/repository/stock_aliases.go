package repository

import (
	"context"
	"database/sql"
	"log"
	"strings"
	"sync/atomic"
	"time"
)

/*
`stock_aliases` 표를 읽어 **프로세스 전역 스냅샷**으로 들고 있는다.

# 왜 캐시가 필요한가

읽는 쪽이 둘인데 둘 다 뜨겁다.

  · **뉴스 태깅 인덱스** — 기사 제목마다 별칭을 훑는다. 목록 요청 하나가 기사
    100건을 만들고 기사마다 두 번 훑으므로, 여기에 SQL을 넣으면 지금보다 **느려진다.**
  · **앱 검색** — 검색창이 글자마다 질의를 보낸다.

그래서 표는 **출처**이고 매칭은 메모리에서 한다. 사용자가 「검색할 때마다 쓰지 말고
한 번 일괄로」라고 한 요구가 가리키는 것이 정확히 이 구조다 — 다만 「한 번」이 아니라
**주기적으로** 다시 읽는다. 한 번만 읽으면 표를 고쳐도 재기동 전까지 반영되지 않아,
배포 없이 고치려고 표로 옮긴 이유가 사라진다.

# 왜 패키지 전역인가

`SearchAliasTokens`가 이미 패키지 함수다(점수 계산이 리포지토리 인스턴스를 들고
다니지 않는다). 여기에 인스턴스를 끼워 넣으려면 검색 점수 경로 전체가 바뀌는데,
별칭 하나 때문에 그럴 일이 아니다. 대신 스냅샷은 **통째로 교체**만 하고
(`atomic.Pointer`) 안을 고치지 않는다 — 읽는 쪽은 잠금 없이 map을 읽는다.

# 못 읽으면 어떻게 되나

씨앗(`stock_alias_seed.go`)이 그대로 남는다. 별칭이 통째로 사라지면 「네이버」가
0건이 되는데 **아무 오류도 안 난다** — 그런 실패는 눈에 안 띄어 오래 간다.
그래서 DB 실패는 캐시를 비우지 않는다.
*/

// aliasSnapshot is immutable once published.
type aliasSnapshot struct {
	// brand: 한글 읽기 -> 영문 표기들
	brand map[string][]string
	/*
		symbol: 한글 별칭 -> 심볼**들**.

		# 왜 목록인가 (2026-09-14 사용자 요청 1)

		표는 처음부터 한 별칭에 대상을 여럿 담을 수 있었다 — 고유 키가
		`(alias, kind, target)`이다. 못 담은 것은 **이 map**이었다. 값이 문자열
		하나라 「구글 → GOOGL」 다음에 「구글 → GOOG」가 들어오면 나중 행이 앞
		행을 덮었고, 어느 쪽이 남는지는 표를 읽는 순서가 정했다. 사용자가 적은
		증상이 그것이다 — 구글 별칭에 둘을 달아 뒀는데 검색에는 GOOG만 나왔다.

		목록의 **첫 칸은 특별하다**(`PrimarySymbolNameAliases`). 검색은 전부를
		쓰지만 뉴스 태깅은 하나만 쓸 수 있어서다 — 이유는 그 함수 머리글에.
		순서는 표의 `id` 오름차순으로 확정한다(`ReloadStockAliases`의 ORDER BY).
	*/
	symbol map[string][]string
	// fromDB — DB를 한 번이라도 성공적으로 읽었나. 로그에만 쓴다.
	fromDB  bool
	builtAt time.Time
}

var (
	aliasSnap    atomic.Pointer[aliasSnapshot]
	aliasDB      atomic.Pointer[sql.DB]
	aliasLoading atomic.Bool
)

// aliasCacheTTL — 표를 고치고 나서 서비스에 반영되기까지의 최대 지연.
//
// 별칭은 사람이 가끔 고치는 자료라 초 단위 반영이 필요 없고, 반대로 재기동을
// 기다려야 한다면 표로 옮긴 뜻이 없다. 10분은 그 사이다.
const aliasCacheTTL = 10 * time.Minute

func seedSnapshot() *aliasSnapshot {
	brand := make(map[string][]string, len(seedBrandAliases))
	for k, v := range seedBrandAliases {
		brand[k] = append([]string(nil), v...)
	}
	// 씨앗은 별칭마다 대상이 하나다 — 어느 주식인지는 사람이 정한다
	// (`stock_alias_seed.go` 머리글 ②). 여럿을 붙이는 것은 표의 일이다.
	symbol := make(map[string][]string, len(seedSymbolAliases))
	for k, v := range seedSymbolAliases {
		symbol[k] = []string{v}
	}
	return &aliasSnapshot{brand: brand, symbol: symbol, builtAt: time.Now()}
}

// currentAliases returns the live snapshot, kicking off a background refresh
// when it is stale. It never blocks on the database.
func currentAliases() *aliasSnapshot {
	// **매번 nil을 확인한다** — `sync.Once`로 한 번만 채우면 그 뒤에 nil이 들어오는
	// 경로(시험의 되돌리기 등) 하나에 전체가 패닉한다. 여기는 검색·태깅의 한가운데라
	// 「없으면 씨앗」이 항상 성립해야 하는 자리다. 경합해서 둘이 만들어도 손해는
	// 씨앗 map 한 벌이고, 이긴 쪽만 남는다.
	snap := aliasSnap.Load()
	if snap == nil {
		fresh := seedSnapshot()
		if aliasSnap.CompareAndSwap(nil, fresh) {
			snap = fresh
		} else {
			snap = aliasSnap.Load()
		}
	}
	if db := aliasDB.Load(); db != nil && time.Since(snap.builtAt) > aliasCacheTTL {
		go refreshAliasCacheAsync(db)
	}
	return snap
}

func refreshAliasCacheAsync(db *sql.DB) {
	// 스냅샷이 낡은 동안 요청이 여럿 들어오면 전부 여기로 온다. 한 번만 읽는다.
	if !aliasLoading.CompareAndSwap(false, true) {
		return
	}
	defer aliasLoading.Store(false)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := ReloadStockAliases(ctx, db); err != nil {
		log.Printf("stock aliases: reload failed, keeping previous snapshot: %v", err)
		// 실패해도 다음 요청이 곧바로 또 시도하지 않도록 시각만 밀어 둔다.
		if snap := aliasSnap.Load(); snap != nil {
			bumped := *snap
			bumped.builtAt = time.Now()
			aliasSnap.Store(&bumped)
		}
	}
}

/*
SetStockAliasSource wires the database that backs the alias cache and loads it once.

배선이 없으면 씨앗만 쓴다 — 시험과 DB 없이 도는 명령들이 그 경로다.
*/
func SetStockAliasSource(ctx context.Context, db *sql.DB) {
	if db == nil {
		return
	}
	aliasDB.Store(db)
	if err := ReloadStockAliases(ctx, db); err != nil {
		log.Printf("stock aliases: initial load failed, using seed only: %v", err)
	}
}

/*
ReloadStockAliases reads the table and publishes a new snapshot.

씨앗 **위에** 얹는다. 표에 없는 씨앗 행은 살아남고, 같은 별칭이 표에 있으면 표가
이긴다 — 운영에서 고친 것이 소스에 적힌 것을 덮어야 배포 없이 고치는 의미가 있다.

`enabled=0` 행은 **씨앗에서도 뺀다.** 오태깅이 나서 끈 별칭이 씨앗에 있다는 이유로
계속 살아 있으면 끄는 스위치가 스위치가 아니게 된다.
*/
func ReloadStockAliases(ctx context.Context, db *sql.DB) error {
	if db == nil {
		return nil
	}
	/*
		**순서를 고정한다** (2026-09-14 사용자 요청 1). 한 별칭에 대상이 여럿일 때
		목록의 첫 칸이 뉴스 태깅이 쓰는 「대표」가 되는데(`PrimarySymbolNameAliases`),
		정렬이 없으면 그 대표가 회차마다 바뀔 수 있다 — 같은 기사에 어제는 GOOGL이,
		오늘은 GOOG이 붙는다. `id` 오름차순이면 **먼저 적어 둔 것이 대표**다.
	*/
	rows, err := db.QueryContext(ctx,
		`SELECT alias, kind, target, enabled FROM stock_aliases ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	fold := newAliasFold()
	for rows.Next() {
		var alias, kind, target string
		var enabled bool
		if err := rows.Scan(&alias, &kind, &target, &enabled); err != nil {
			return err
		}
		fold.add(alias, kind, target, enabled)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	snap, disabled := fold.snap, fold.disabled
	snap.fromDB = true
	snap.builtAt = time.Now()
	prev := aliasSnap.Load()
	aliasSnap.Store(snap)
	/*
		**달라졌을 때만 적는다.** 이 함수는 10분마다 도는데(`aliasCacheTTL`) 별칭은
		사람이 가끔 고치는 자료라 대부분의 회차는 같은 수를 말한다. 같은 줄이 프로세스
		아홉 개에서 10분마다 나오면 시간당 50줄이고, 그런 로그는 곧 안 보는 로그가 된다
		(`ops_incidents`가 같은 이유로 「처음 열릴 때만」 찍는다).
	*/
	if prev == nil || !prev.fromDB || len(prev.brand) != len(snap.brand) || len(prev.symbol) != len(snap.symbol) {
		log.Printf("stock aliases: %d brand · %d symbol (표에서 끈 것 %d)",
			len(snap.brand), len(snap.symbol), disabled)
	}
	return nil
}

/*
aliasFold — 표의 행들을 씨앗 스냅샷 **위에** 접어 넣는 상태.

# 왜 함수 밖으로 꺼냈나 (2026-09-14 사용자 요청 1)

이 규칙은 `sql.Rows` 반복문 안에 있었다. 그래서 시험이 닿으려면 DB가 필요했고,
실제로 「끈 행 하나가 별칭 전체를 지운다」는 버그가 그 안에서 조용히 살아 있었다 —
대상이 하나뿐이던 동안에는 증상이 없었기 때문이다. 규칙만 떼어 두면 DB 없이 센다.

# 규칙 (갈래 공통)

	· 표가 씨앗을 **이긴다.** 한 별칭의 첫 행이 오면 씨앗 값을 비우고 그때부터 쌓는다.
	  그러지 않으면 「씨앗 naver + 표 naver-new」가 둘 다 남는다.
	· 끄기는 **행 단위가 아니라 목록에서 빠지는 것**이다. 끈 행은 안 얹을 뿐이고,
	  같은 별칭의 켜진 행은 그대로 남는다. 예전에는 `delete`를 불러 별칭을 통째로
	  지웠는데, 대상이 여럿이 되면 그건 「GOOG만 껐는데 GOOGL까지 사라진다」가 된다.
	· 그 별칭의 **모든** 행이 꺼져 있으면 결과적으로 씨앗까지 사라진다(첫 행에서
	  비우고 아무것도 안 쌓이므로). 끄는 스위치가 스위치이려면 그래야 한다.

순서는 부르는 쪽이 정한다 — `ReloadStockAliases`가 `id` 오름차순으로 읽으므로,
목록의 첫 칸은 언제나 **먼저 적어 둔 대상**이다.
*/
type aliasFold struct {
	snap     *aliasSnapshot
	seen     map[string]bool // "kind\x00alias" — 이 별칭의 첫 행을 이미 봤나
	disabled int
}

func newAliasFold() *aliasFold {
	return &aliasFold{snap: seedSnapshot(), seen: map[string]bool{}}
}

func (f *aliasFold) add(alias, kind, target string, enabled bool) {
	alias = strings.ToLower(strings.TrimSpace(alias))
	target = strings.TrimSpace(target)
	if alias == "" || target == "" {
		return
	}
	if kind != "brand" && kind != "symbol" {
		return
	}
	key := kind + "\x00" + alias
	first := !f.seen[key]
	f.seen[key] = true

	if kind == "brand" {
		if first {
			delete(f.snap.brand, alias)
		}
		if !enabled {
			f.disabled++
			return
		}
		f.snap.brand[alias] = append(f.snap.brand[alias], strings.ToLower(target))
		return
	}
	if first {
		delete(f.snap.symbol, alias)
	}
	if !enabled {
		f.disabled++
		return
	}
	f.snap.symbol[alias] = append(f.snap.symbol[alias], strings.ToUpper(target))
}

// MigrateStockAliasesIfNeeded creates and seeds the table on an existing database.
func MigrateStockAliasesIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_aliases")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "070_stock_aliases.sql", "종목 별칭")
}

/*
HangulNameAliases 는 별칭 표를 **읽기 전용 사본**으로 내준다.

뉴스 종목 태깅이 같은 표를 쓴다(`service/news_symbol_alias.go`). 「네이버」가 기사
제목에 있어도 태깅되지 않던 이유가 검색에서와 같은 문제였다 — 상장명이 `NAVER`라
한글 읽기가 DB 어디에도 없다. 표를 두 벌 두면 한쪽만 고쳐지므로 여기를 단일
출처로 삼는다.

사본을 주는 이유는 스냅샷이 **교체만 되고 수정되지 않아야** 하기 때문이다.
*/
func HangulNameAliases() map[string][]string {
	snap := currentAliases()
	out := make(map[string][]string, len(snap.brand))
	for k, v := range snap.brand {
		out[k] = append([]string(nil), v...)
	}
	return out
}

/*
SymbolNameAliases 는 한글 별칭 → 미국 종목 심볼**들**을 읽기 전용 사본으로 내준다.

한 별칭이 여러 종목을 가리킬 수 있다 — 「구글」은 GOOGL과 GOOG 둘 다이고, 어느
쪽을 찾는지는 검색한 사람만 안다. 그래서 **검색은 전부를 본다.**
*/
func SymbolNameAliases() map[string][]string {
	snap := currentAliases()
	out := make(map[string][]string, len(snap.symbol))
	for k, v := range snap.symbol {
		out[k] = append([]string(nil), v...)
	}
	return out
}

/*
PrimarySymbolNameAliases 는 별칭마다 **대표 심볼 하나**만 골라 내준다.

# 왜 하나만인가

부르는 쪽이 뉴스 종목 태깅이고, 그 자리의 규칙은 「정밀도가 재현율보다 중요하다」다
(`service/news_symbol_match.go` 머리말). 검색은 후보를 넓게 뽑아 점수로 줄 세우면
되지만 태깅은 붙거나 안 붙거나 둘뿐이라, 「구글 실적 발표」 한 줄에 GOOGL과 GOOG을
**둘 다** 달면 목록에 같은 회사가 두 번 서고 시세도 두 벌이 붙는다. 기사는 회사
이야기이지 주식 종류 이야기가 아니다.

대표는 **표에 먼저 적힌 것**이다(`ReloadStockAliases`가 `id` 순으로 읽는다).
나중에 대상을 덧붙여도 이미 붙어 있던 태그가 다른 종목으로 옮겨 가지 않는다 —
그쪽이 조용히 바뀌면 아무도 알아채지 못한다.
*/
func PrimarySymbolNameAliases() map[string]string {
	snap := currentAliases()
	out := make(map[string]string, len(snap.symbol))
	for k, v := range snap.symbol {
		if len(v) > 0 {
			out[k] = v[0]
		}
	}
	return out
}
