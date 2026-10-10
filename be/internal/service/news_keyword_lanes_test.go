package service

import (
	"context"
	"testing"
	"time"
)

func laneTestStore(counts map[string]int) *NewsStore {
	return &NewsStore{
		laneCursors:     map[string]int{},
		keywordCounts:   map[string]map[string]int{"domestic": counts},
		keywordCountsAt: map[string]time.Time{"domestic": time.Now()},
	}
}

func symbolKeyword(text string) newsKeyword {
	return newsKeyword{Text: text, Symbol: "0000" + text}
}

// 예산은 hot에 몰리되, 느린 두 차선이 0이 되어서는 안 된다 — 0이면 그 차선은
// 영원히 안 돈다.
func TestSplitLaneBudget(t *testing.T) {
	got := splitLaneBudget(22)
	if got[newsLaneHot] != 14 || got[newsLaneWarm] != 4 || got[newsLaneCold] != 4 {
		t.Fatalf("22 -> hot %d warm %d cold %d, want 14/4/4",
			got[newsLaneHot], got[newsLaneWarm], got[newsLaneCold])
	}
	for _, total := range []int{4, 5, 8, 10, 18, 22, 30} {
		b := splitLaneBudget(total)
		sum := b[newsLaneHot] + b[newsLaneWarm] + b[newsLaneCold]
		if sum != total {
			t.Fatalf("total %d: 차선 합이 %d", total, sum)
		}
		for _, lane := range newsKeywordLaneOrder {
			if b[lane] < 1 {
				t.Fatalf("total %d: %s 차선이 0", total, lane)
			}
		}
		if b[newsLaneHot] < b[newsLaneWarm] {
			t.Fatalf("total %d: hot(%d)이 warm(%d)보다 작다", total, b[newsLaneHot], b[newsLaneWarm])
		}
	}
	if len(splitLaneBudget(0)) != 0 {
		t.Fatal("예산이 없으면 아무 차선도 안 돈다")
	}
}

// 사람이 고른 관리자 키워드는 기사 수가 0이어도 cold로 내려가지 않는다.
func TestLaneOfKeepsAdminKeywordsWarm(t *testing.T) {
	counts := map[string]int{"코스피": 400, "특징주": 12}
	if got := laneOf(newsKeyword{Text: "코스피"}, counts); got != newsLaneHot {
		t.Fatalf("코스피 -> %s, want hot", got)
	}
	if got := laneOf(newsKeyword{Text: "특징주"}, counts); got != newsLaneWarm {
		t.Fatalf("특징주 -> %s, want warm", got)
	}
	// 방금 추가한 관리자 키워드 — 기사 0건이지만 warm.
	if got := laneOf(newsKeyword{Text: "신규키워드"}, counts); got != newsLaneWarm {
		t.Fatalf("신규 관리자 키워드 -> %s, want warm", got)
	}
	// 7일 동안 한 건도 못 낸 종목만 cold다.
	if got := laneOf(symbolKeyword("조용한종목"), counts); got != newsLaneCold {
		t.Fatalf("무소식 종목 -> %s, want cold", got)
	}
}

/*
차선의 요점 — **hot이 훨씬 자주 돈다.**

hot 3개·cold 30개인 큐를 여러 틱 돌려서, hot이 매 틱 전부 불리고 cold는 조금씩만
불리는지 본다. 예전의 단일 커서였다면 hot은 열 틱에 한 번꼴로 왔다.
*/
func TestNextKeywordsInLanesFavorsProductiveKeywords(t *testing.T) {
	counts := map[string]int{"코스피": 500, "증시": 300, "반도체": 200}
	st := laneTestStore(counts)

	var queue []newsKeyword
	for _, hot := range []string{"코스피", "증시", "반도체"} {
		queue = append(queue, newsKeyword{Text: hot})
	}
	for i := 0; i < 30; i++ {
		queue = append(queue, symbolKeyword(string(rune('가'+i))))
	}

	seen := map[string]int{}
	for tick := 0; tick < 5; tick++ {
		batch := st.nextKeywordsInLanes(context.Background(), "domestic", "test:lane", queue, 10)
		if len(batch) != 10 {
			t.Fatalf("tick %d: %d개를 집었다, 예산은 10", tick, len(batch))
		}
		inTick := map[string]bool{}
		for _, kw := range batch {
			if inTick[kw.Text] {
				t.Fatalf("tick %d: 같은 검색어를 두 번 불렀다 (%s)", tick, kw.Text)
			}
			inTick[kw.Text] = true
			seen[kw.Text]++
		}
	}

	for _, hot := range []string{"코스피", "증시", "반도체"} {
		if seen[hot] != 5 {
			t.Fatalf("%s는 5틱 중 %d번 불렸다 — hot은 매 틱 돌아야 한다", hot, seen[hot])
		}
	}
	if seen[string(rune('가'))] > 2 {
		t.Fatalf("cold 검색어가 %d번 불렸다 — 꼬리가 hot의 자리를 먹고 있다", seen[string(rune('가'))])
	}
}

// 커서는 차선마다 따로 이어진다 — 한 바퀴를 돌면 cold도 빠짐없이 불린다.
func TestNextKeywordsInLanesRotatesColdLaneFully(t *testing.T) {
	st := laneTestStore(map[string]int{"코스피": 500})

	queue := []newsKeyword{{Text: "코스피"}}
	for i := 0; i < 12; i++ {
		queue = append(queue, symbolKeyword(string(rune('가'+i))))
	}

	seen := map[string]int{}
	// cold 12개 · 틱당 cold 몫 1 → 12틱이면 한 바퀴.
	for tick := 0; tick < 12; tick++ {
		for _, kw := range st.nextKeywordsInLanes(context.Background(), "domestic", "test:rot", queue, 5) {
			seen[kw.Text]++
		}
	}
	for i := 0; i < 12; i++ {
		text := string(rune('가' + i))
		if seen[text] == 0 {
			t.Fatalf("한 바퀴를 돌았는데 %s가 한 번도 안 불렸다", text)
		}
	}
}

/*
빈 자리는 버리지 않는다.

큐에 hot만 있고 warm·cold가 비어 있으면 그 몫은 hot이 가져간다 — 안 그러면 큐가
작은 초기 상태에서 한도의 일부만 쓰고 만다.
*/
func TestNextKeywordsInLanesRedistributesEmptyLanes(t *testing.T) {
	counts := map[string]int{}
	queue := make([]newsKeyword, 0, 8)
	for i := 0; i < 8; i++ {
		text := string(rune('가' + i))
		counts[text] = 500
		queue = append(queue, newsKeyword{Text: text, Symbol: "S" + text})
	}
	st := laneTestStore(counts)

	batch := st.nextKeywordsInLanes(context.Background(), "domestic", "test:redist", queue, 6)
	if len(batch) != 6 {
		t.Fatalf("hot만 있는 큐에서 %d개만 집었다 — 예산 6을 다 써야 한다", len(batch))
	}
	seen := map[string]bool{}
	for _, kw := range batch {
		if seen[kw.Text] {
			t.Fatalf("한 틱에 같은 검색어가 두 번 (%s)", kw.Text)
		}
		seen[kw.Text] = true
	}
}

// 예산이 큐보다 크면 큐 길이에서 멈춘다 — 같은 말을 두 번 부르지 않는다.
func TestNextKeywordsInLanesCapsAtQueueLength(t *testing.T) {
	st := laneTestStore(map[string]int{})
	queue := []newsKeyword{symbolKeyword("가"), symbolKeyword("나")}
	batch := st.nextKeywordsInLanes(context.Background(), "domestic", "test:cap", queue, 10)
	if len(batch) != 2 {
		t.Fatalf("%d개를 집었다 — 큐가 2개뿐이다", len(batch))
	}
}
