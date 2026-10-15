package translate

import (
	"context"
	"strings"
	"sync/atomic"
)

/*
Chain — 앞의 엔진이 내놓지 못하면 뒤의 엔진에 묻는다.

「못 내놓았다」는 오류만이 아니다. **`Accept`를 통과하지 못한 번역도 못 내놓은
것으로 친다.** 그게 이 구조의 요점이다 — 자체 모델이 살아 있는 채로 이상한 문장을
꾸준히 뱉는 상황이 제일 위험한데, 오류로는 그게 안 잡힌다.

순서는 설정이 정한다(`TRANSLATE_PROVIDER`). 지금 엔진은 자체 모델 하나뿐이라
사슬의 길이가 1이지만 구조는 그대로 둔다 — 엔진을 하나 더 붙일 때 이 파일을 고칠
일이 없어야 한다.

**엔진이 하나뿐이라는 말은 폴백이 없다는 뜻이다.** 자체 모델이 실패하거나 검증을
통과하지 못한 제목은 **영어로 남는다** — 목록에서 사라지는 것이 아니라 원문 그대로
나온다. `Snapshot()`의 Rejected가 올라가면 그만큼 영어 제목이 쌓이고 있는 것이다.
*/
type Chain struct {
	engines []Translator
	stats   Stats
}

// Stats는 「이 구조가 지금 실제로 무엇을 하고 있나」다.
//
// Rejected가 올라가면 모델의 품질이 나빠진 것이고, **그만큼 영어 제목이 화면에
// 쌓이고 있다는 뜻이다.** 이 수를 안 보면 그게 조용히 늘어난다 — 화면은 「번역이
// 안 된 기사」와 「원래 영어인 기사」를 구분해 보여 주지 않는다.
type Stats struct {
	// Accepted[i]는 i번째 엔진이 내놓은 번역이 받아들여진 횟수.
	accepted []atomic.Int64
	rejected []atomic.Int64
	failed   []atomic.Int64
}

// StatSnapshot is one engine's counters at a moment.
type StatSnapshot struct {
	Engine   string `json:"engine"`
	Accepted int64  `json:"accepted"`
	Rejected int64  `json:"rejected"`
	Failed   int64  `json:"failed"`
}

func NewChain(engines ...Translator) *Chain {
	live := make([]Translator, 0, len(engines))
	for _, e := range engines {
		if e != nil && e.Available() {
			live = append(live, e)
		}
	}
	return &Chain{
		engines: live,
		stats: Stats{
			accepted: make([]atomic.Int64, len(live)),
			rejected: make([]atomic.Int64, len(live)),
			failed:   make([]atomic.Int64, len(live)),
		},
	}
}

func (c *Chain) Name() string {
	if c == nil || len(c.engines) == 0 {
		return "none"
	}
	names := make([]string, 0, len(c.engines))
	for _, e := range c.engines {
		names = append(names, e.Name())
	}
	return strings.Join(names, ",")
}

func (c *Chain) Available() bool { return c != nil && len(c.engines) > 0 }

// Snapshot returns the per-engine counters for the admin console and logs.
func (c *Chain) Snapshot() []StatSnapshot {
	if c == nil {
		return nil
	}
	out := make([]StatSnapshot, 0, len(c.engines))
	for i, e := range c.engines {
		out = append(out, StatSnapshot{
			Engine:   e.Name(),
			Accepted: c.stats.accepted[i].Load(),
			Rejected: c.stats.rejected[i].Load(),
			Failed:   c.stats.failed[i].Load(),
		})
	}
	return out
}

/*
TranslateENtoKO는 아직 못 채운 자리만 다음 엔진에 넘긴다.

스물넷 중 셋만 실패했으면 다음 엔진에는 **그 셋만** 간다. 엔진이 하나뿐인 지금은
그 셋이 그대로 빈 채로 남아 다음 주기에 다시 시도된다.
*/
func (c *Chain) TranslateENtoKO(ctx context.Context, texts []string) ([]string, error) {
	out := make([]string, len(texts))
	if c == nil || len(texts) == 0 {
		return out, nil
	}

	pending := make([]int, 0, len(texts))
	for i, t := range texts {
		if strings.TrimSpace(t) != "" {
			pending = append(pending, i)
		}
	}

	var lastErr error
	for engineIdx, engine := range c.engines {
		if len(pending) == 0 {
			break
		}
		batch := make([]string, len(pending))
		for j, i := range pending {
			batch[j] = texts[i]
		}

		results, err := engine.TranslateENtoKO(ctx, batch)
		if err != nil {
			lastErr = err
			c.stats.failed[engineIdx].Add(int64(len(pending)))
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			continue
		}
		/*
			이 엔진은 **대답했다**. 앞 엔진의 오류(Azure 예산 소진)를 끝까지 들고 가면,
			뒤 엔진이 대답했는데 전부 거절된 묶음이 「예산 소진」으로 보고된다 —
			호출부는 그것을 「못 보냈다」로 읽어 시도 횟수를 안 세고, 같은 제목을
			2분마다 영원히 다시 보낸다(10-15, 하루 56줄). 대답은 왔고 못 쓴 것이다.
		*/
		lastErr = nil

		next := pending[:0:0]
		for j, i := range pending {
			var got string
			if j < len(results) {
				got = strings.TrimSpace(results[j])
			}
			switch {
			case got == "":
				c.stats.failed[engineIdx].Add(1)
				next = append(next, i)
			case !Accept(texts[i], got):
				// 받아들일 수 없는 번역. 다음 엔진이 받고, 없으면 영어로 남는다.
				c.stats.rejected[engineIdx].Add(1)
				next = append(next, i)
			default:
				c.stats.accepted[engineIdx].Add(1)
				out[i] = ApplyGlossary(texts[i], got)
			}
		}
		pending = next
	}

	if len(pending) == len(texts) && lastErr != nil {
		return out, lastErr
	}
	return out, nil
}
