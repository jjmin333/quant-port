package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
AssetMentionService — 글 안의 자산 태그(2026-10-03 사용자 결정 ③). 단지·지역·금융상품·금리.

이름 → 키는 **메모리 색인**(30분)으로 푼다: 단지 1.3만·상품 1천·지역 254·금리 계열 — 자동완성이 글자마다
DB를 훑지 않게. 값은 저장된 점수 facts·실거래·금리 표에서 읽는다(LLM 0, 외부 호출 0).
쓴 순간 값은 content_asset_mentions에 얼리고, 읽을 때 지금 값을 나란히 붙인다 — 종목 태그와 같은 규칙.
*/
type AssetMentionService struct {
	intel *repository.AssetIntelRepo
	data  *repository.PublicDataRepo
	store *repository.AssetWatchRepo

	mu       sync.Mutex
	idx      *assetNameIndex
	loadedAt time.Time
}

func NewAssetMentionService(intel *repository.AssetIntelRepo, data *repository.PublicDataRepo, store *repository.AssetWatchRepo) *AssetMentionService {
	return &AssetMentionService{intel: intel, data: data, store: store}
}

const assetIndexTTL = 30 * time.Minute

type assetEntry struct {
	kind, key, name, sub, token string
	score                       int
}

// assetNameIndex — 종류별 「대괄호 안 글자」 변형 → 키. 같은 변형이 둘이면 점수 높은 쪽(먼저 넣은 쪽)이 이긴다.
type assetNameIndex struct {
	byText  map[string]map[string]string // kind → 정규화한 글자 → key
	entries map[string]assetEntry        // kind|key → 표시
	list    []assetEntry                 // 자동완성 순서(점수 높은 순)
	// 기사 제목 칩(2026-10-09) — 전국에 하나뿐인 법정동 이름 → 시군구, 전국에 하나뿐인 단지 이름 → 단지 키·시군구.
	// 여럿이면 넣지 않는다(「신사동」이 강남·관악 둘이면 어느 쪽인지 모른다).
	headlineDong map[string]string
	headlineApt  map[string][2]string
}

func normAssetText(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), "")) }

func (ix *assetNameIndex) add(e assetEntry, variants ...string) {
	if ix.byText[e.kind] == nil {
		ix.byText[e.kind] = map[string]string{}
	}
	for _, v := range variants {
		if n := normAssetText(v); n != "" {
			if _, taken := ix.byText[e.kind][n]; !taken {
				ix.byText[e.kind][n] = e.key
			}
		}
	}
	id := repository.MentionID(e.kind, e.key)
	if _, ok := ix.entries[id]; !ok {
		ix.entries[id] = e
		ix.list = append(ix.list, e)
	}
}

func (s *AssetMentionService) index(ctx context.Context) *assetNameIndex {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.idx != nil && time.Since(s.loadedAt) < assetIndexTTL {
		return s.idx
	}
	ix, err := s.buildIndex(ctx)
	if err != nil {
		log.Printf("asset mentions: index: %v", err)
		if s.idx != nil {
			return s.idx // 낡은 색인이 빈 색인보다 낫다
		}
		return &assetNameIndex{byText: map[string]map[string]string{}, entries: map[string]assetEntry{}}
	}
	s.idx, s.loadedAt = ix, time.Now()
	return ix
}

func (s *AssetMentionService) buildIndex(ctx context.Context) (*assetNameIndex, error) {
	ix := &assetNameIndex{byText: map[string]map[string]string{}, entries: map[string]assetEntry{}}
	regions, err := s.data.Regions(ctx)
	if err != nil {
		return nil, err
	}
	nameCount := map[string]int{}
	for _, g := range regions {
		nameCount[g.Name]++
	}
	regionLabel := map[string]string{} // lawd → 화면 이름(겹치면 시도 약칭을 앞에)
	for _, g := range regions {
		label := g.Name
		if nameCount[g.Name] > 1 {
			label = SidoShort(g.Sido) + " " + g.Name
		}
		regionLabel[g.LawdCd] = label
	}
	// 지역 — 점수 높은 순으로 넣어야 「겹치는 짧은 이름」이 큰 곳에 가지 않는데, 겹치는 이름은 애초에 맨 이름을 받지 않는다.
	for _, g := range regions {
		e := assetEntry{kind: model.AssetKindRegion, key: g.LawdCd, name: SidoShort(g.Sido) + " " + g.Name, sub: g.Sido, token: regionLabel[g.LawdCd]}
		variants := []string{SidoShort(g.Sido) + " " + g.Name, g.Sido + " " + g.Name}
		if nameCount[g.Name] == 1 {
			variants = append(variants, g.Name)
		}
		ix.add(e, variants...)
	}
	// 단지 — 「시군구 단지명」이 정식, 이름만 써도 점수 높은 단지로.
	complexes, err := s.intel.ScoreNames(ctx, "re_complex", "")
	if err != nil {
		return nil, err
	}
	for _, c := range complexes {
		label := regionLabel[c.Group]
		e := assetEntry{kind: model.AssetKindApt, key: c.Key, name: c.Name, sub: label, token: strings.TrimSpace(label + " " + c.Name), score: c.Score}
		ix.add(e, label+" "+c.Name, c.Name)
	}
	// 점수가 없는 단지도(2026-10-09 — 점수는 1년 5건+ 단지만이라 전체의 3분의 1) 단지 목록에서 넣는다.
	// 점수 단지가 먼저 들어가 있으므로 같은 이름이면 점수 단지가 이긴다(add는 먼저 넣은 쪽을 둔다).
	dongLawd := map[string]map[string]bool{}
	type aptName struct {
		top        [2]string // 거래 가장 많은 단지의 키·시군구(목록이 거래 많은 순이라 처음 본 것)
		topTrades  int
		restTrades int
	}
	aptNames := map[string]*aptName{}
	if dir, err := s.data.ComplexDirectoryEntries(ctx); err == nil {
		for _, c := range dir {
			label := regionLabel[c.LawdCd]
			e := assetEntry{kind: model.AssetKindApt, key: c.Key, name: c.Name, sub: label, token: strings.TrimSpace(label + " " + c.Name)}
			ix.add(e, label+" "+c.Name, c.Name)
			if d := normAssetText(c.UmdNm); d != "" {
				if dongLawd[d] == nil {
					dongLawd[d] = map[string]bool{}
				}
				dongLawd[d][c.LawdCd] = true
			}
			n := normAssetText(c.Name)
			if a := aptNames[n]; a == nil {
				aptNames[n] = &aptName{top: [2]string{c.Key, c.LawdCd}, topTrades: c.Trades}
			} else {
				a.restTrades += c.Trades
			}
		}
	} else {
		log.Printf("asset mentions: complex directory: %v", err)
	}
	ix.headlineDong, ix.headlineApt = map[string]string{}, map[string][2]string{}
	for d, lawds := range dongLawd {
		if len(lawds) == 1 && utf8.RuneCountInString(d) >= 3 && strings.HasSuffix(d, "동") {
			for l := range lawds {
				ix.headlineDong[d] = l
			}
		}
	}
	// 이름이 같은 단지가 여럿이어도 한 곳이 거래의 20배 이상이면 기사는 그곳 이야기다(「헬리오시티」 송파 2,606건 vs 4건).
	for n, a := range aptNames {
		if headlineAptName(n) && a.topTrades >= 20*a.restTrades {
			ix.headlineApt[n] = a.top
		}
	}
	/*
		시·도와 전국(2026-10-10) — 「서울 아파트값」·「전국 집값」처럼 시·도 단위로 쓰는 부동산 기사가 대부분이다.
		「경기」는 「경기 침체」와 겹쳐 짧은 이름을 받지 않는다(「경기도」만).
	*/
	if sidos, err := s.intel.ScoreNames(ctx, "re_sido", ""); err == nil {
		for _, g := range sidos {
			short := SidoShort(g.Name)
			e := assetEntry{kind: model.AssetKindSido, key: g.Key, name: g.Name, sub: "시·도", token: short}
			variants := []string{g.Name}
			if short != "경기" {
				variants = append(variants, short)
			}
			ix.add(e, variants...)
		}
	}
	ix.add(assetEntry{kind: model.AssetKindSido, key: "00", name: "전국", sub: "전국", token: "전국"}, "전국")
	// 금융상품 — 「금융회사 상품명」이 정식.
	products, err := s.intel.ScoreNames(ctx, "fin_product", "company")
	if err != nil {
		return nil, err
	}
	for _, p := range products {
		e := assetEntry{kind: model.AssetKindProduct, key: p.Key, name: p.Name, sub: p.Extra, token: strings.TrimSpace(p.Extra + " " + p.Name), score: p.Score}
		ix.add(e, p.Extra+" "+p.Name, p.Name)
	}
	// 금리 계열 — 이름(띄어쓰기 무시) + 흔한 별칭.
	alias := map[string][]string{"base": {"기준금리"}, "mortgage": {"주담대", "주담대금리"}, "creditLoan": {"신용대출"}, "cd91": {"CD", "CD금리"},
		"deposit1y": {"예금금리", "정기예금"}, "jeonseLoan": {"전세대출"}}
	for _, d := range RateSeriesDefs {
		e := assetEntry{kind: model.AssetKindRate, key: d.Key, name: d.Label, sub: "한국은행 ECOS", token: d.Label}
		ix.add(e, append([]string{d.Label, d.Key}, alias[d.Key]...)...)
	}
	return ix, nil
}

// lookup — 대괄호 안 글자 → 키.
func (ix *assetNameIndex) lookup(kind, text string) (string, bool) {
	k, ok := ix.byText[kind][normAssetText(text)]
	return k, ok
}

// AssetSuggestion — 편집기 자동완성 한 줄. Token은 대괄호 안에 넣을 글자.
type AssetSuggestion struct {
	Kind  string `json:"kind"`
	Key   string `json:"key"`
	Name  string `json:"name"`
	Sub   string `json:"sub,omitempty"`
	Token string `json:"token"`
}

// Search — 이름에 q가 들어간 자산(kind가 비면 전부, 종류마다 앞에서 몇 개). 토큰이 60자를 넘는 것은 뺀다(태그가 안 된다).
func (s *AssetMentionService) Search(ctx context.Context, q, kind string, limit int) []AssetSuggestion {
	q = normAssetText(q)
	out := []AssetSuggestion{}
	if q == "" || s == nil {
		return out
	}
	per := map[string]int{}
	perMax := limit
	if kind == "" {
		perMax = max(2, limit/2)
	}
	for _, e := range s.index(ctx).list {
		if (kind != "" && e.kind != kind) || per[e.kind] >= perMax || len([]rune(e.token)) > 60 {
			continue
		}
		if !strings.Contains(normAssetText(e.name), q) && !strings.Contains(normAssetText(e.token), q) {
			continue
		}
		per[e.kind]++
		out = append(out, AssetSuggestion{Kind: e.kind, Key: e.key, Name: e.name, Sub: e.sub, Token: strings.ReplaceAll(e.token, "]", "")})
		if len(out) >= limit*2 {
			break
		}
	}
	// 짧은 이름(정확히 맞는 것)이 앞에.
	sort.SliceStable(out, func(i, j int) bool {
		return normAssetText(out[i].Name) == q && normAssetText(out[j].Name) != q
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// ── 값 ──────────────────────────────────────────────────────────────────────

/*
values — 자산(kind|key)마다 지금 값·표시. 쓴 순간 값(Snap)도 이것으로 얼린다.
한 요청에 종류마다 쿼리 한두 번(태그 10개까지라 작다).
*/
func (s *AssetMentionService) values(ctx context.Context, ids []string) map[string]model.AssetMention {
	out := map[string]model.AssetMention{}
	if len(ids) == 0 {
		return out
	}
	ix := s.index(ctx)
	byKind := map[string][]string{}
	for _, id := range ids {
		kind, key, _ := strings.Cut(id, "|")
		byKind[kind] = append(byKind[kind], key)
		e := ix.entries[id]
		m := model.AssetMention{Kind: kind, Key: key, Name: e.name, Sub: e.sub, Snap: map[string]float64{}, Extra: map[string]string{}}
		if m.Name == "" {
			m.Name = key
		}
		m.Href = assetHref(kind, key)
		out[id] = m
	}
	put := func(kind, key string, f func(m *model.AssetMention)) {
		id := repository.MentionID(kind, key)
		m := out[id]
		f(&m)
		out[id] = m
	}
	setScore := func(m *model.AssetMention, sc repository.AssetScore) {
		v := sc.Score
		m.Score, m.Grade = &v, sc.Grade
		m.Snap[m.Kind+".score"] = float64(sc.Score)
	}

	if keys := byKind[model.AssetKindApt]; len(keys) > 0 {
		scores, _ := s.intel.Scores(ctx, "re_complex", keys)
		notes, _ := s.intel.Notes(ctx, "re_complex", keys)
		sales, _ := s.data.ComplexesSales(ctx, keys, time.Now().AddDate(-1, 0, 0))
		last := map[string]repository.ReTrade{}
		for _, t := range sales {
			if _, ok := last[t.ComplexKey]; !ok && !t.Cancelled && t.Price != nil {
				last[t.ComplexKey] = t
			}
		}
		for _, key := range keys {
			put(model.AssetKindApt, key, func(m *model.AssetMention) {
				if sc, ok := scores[key]; ok {
					f := factsMap(sc.Facts)
					setScore(m, sc)
					for id, fk := range map[string]string{"pyeong": "pyeongPrice", "jeonse": "jeonseRatio", "peak": "peakPct", "mom": "momentumPct", "trades": "trades6m"} {
						if v, ok := f[fk].(float64); ok {
							m.Snap["apt."+id] = v
						}
					}
				}
				if t, ok := last[key]; ok {
					m.Snap["apt.price"] = float64(*t.Price)
					floor := ""
					if t.Floor != nil {
						floor = fmt.Sprintf(" · %d층", *t.Floor)
					}
					m.Extra["lastDeal"] = fmt.Sprintf("%s%s · %.0f㎡", t.DealDate, floor, t.Area)
					if m.Name == key {
						m.Name = t.ComplexName
					}
				}
				if n, ok := notes[key]; ok {
					m.Note = noteText(n.Body)
				}
			})
		}
	}
	if keys := byKind[model.AssetKindRegion]; len(keys) > 0 {
		scores, _ := s.intel.Scores(ctx, "re_region", keys)
		notes, _ := s.intel.Notes(ctx, "re_region", keys)
		for _, key := range keys {
			put(model.AssetKindRegion, key, func(m *model.AssetMention) {
				if sc, ok := scores[key]; ok {
					f := factsMap(sc.Facts)
					setScore(m, sc)
					for id, fk := range map[string]string{"w1": "w1Pct", "w4": "w4Pct", "w12": "w12Pct", "jeonse": "jeonse4Pct", "vol": "volTrendPct"} {
						if v, ok := f[fk].(float64); ok {
							m.Snap["region."+id] = v
						}
					}
				}
				if n, ok := notes[key]; ok {
					m.Note = noteText(n.Body)
				}
			})
		}
	}
	if keys := byKind[model.AssetKindSido]; len(keys) > 0 {
		// 시·도는 re_sido, 전국("00")은 re_nation 점수 행(점수 0 — 백분위를 매길 상대가 없다). 해설은 re_area 노트.
		scores, _ := s.intel.Scores(ctx, "re_sido", keys)
		nation, _ := s.intel.Scores(ctx, "re_nation", []string{"00"})
		notes, _ := s.intel.Notes(ctx, "re_area", keys)
		for _, key := range keys {
			put(model.AssetKindSido, key, func(m *model.AssetMention) {
				sc, ok := scores[key]
				if key == "00" {
					sc, ok = nation["00"]
				} else if ok {
					setScore(m, sc)
				}
				if ok {
					f := factsMap(sc.Facts)
					for id, fk := range map[string]string{"w1": "w1Pct", "w4": "w4Pct", "w12": "w12Pct", "jeonse": "jeonse4Pct", "vol": "volTrendPct"} {
						if v, ok := f[fk].(float64); ok {
							m.Snap["sido."+id] = v
						}
					}
				}
				if n, ok := notes[key]; ok {
					m.Note = noteText(n.Body)
				}
			})
		}
	}
	if keys := byKind[model.AssetKindProduct]; len(keys) > 0 {
		scores, _ := s.intel.Scores(ctx, "fin_product", keys)
		notes, _ := s.intel.Notes(ctx, "fin_conds", keys)
		for _, key := range keys {
			put(model.AssetKindProduct, key, func(m *model.AssetMention) {
				if sc, ok := scores[key]; ok {
					f := factsMap(sc.Facts)
					setScore(m, sc)
					for _, k := range []string{"base", "max"} {
						if v, ok := f[k].(float64); ok {
							m.Snap["product."+k] = v
						}
					}
					if t, ok := f["term"].(float64); ok && t > 0 {
						m.Extra["term"] = fmt.Sprintf("%.0f개월", t)
					}
				}
				if n, ok := notes[key]; ok {
					m.Note = noteText(n.Body)
				}
			})
		}
	}
	if keys := byKind[model.AssetKindRate]; len(keys) > 0 {
		latest, _ := s.data.LatestRates(ctx)
		for _, key := range keys {
			put(model.AssetKindRate, key, func(m *model.AssetMention) {
				for _, d := range RateSeriesDefs {
					if d.Key != key {
						continue
					}
					if p, ok := latest[d.ID()]; ok {
						m.Snap["rate.value"] = p[0].Value
						m.Extra["date"] = p[0].Date
						if p[1].Date != "" {
							m.Extra["prev"] = fmt.Sprintf("%.3f", p[1].Value)
						}
					}
				}
			})
		}
	}
	return out
}

func noteText(body json.RawMessage) string {
	var b struct {
		Note string `json:"note"`
	}
	_ = json.Unmarshal(body, &b)
	return b.Note
}

func assetHref(kind, key string) string {
	switch kind {
	case model.AssetKindApt:
		lawd := key
		if i := strings.IndexAny(key, "|-"); i > 0 {
			lawd = key[:i]
		}
		if len(lawd) > 5 {
			lawd = lawd[:5]
		}
		return "/real-estate?" + url.Values{"lawd": {lawd}, "complex": {key}}.Encode()
	case model.AssetKindRegion:
		return "/real-estate?lawd=" + url.QueryEscape(key)
	case model.AssetKindSido:
		if key == "00" {
			return "/real-estate"
		}
		return "/real-estate?sido=" + url.QueryEscape(key)
	case model.AssetKindProduct:
		return "/finance/products/" + url.PathEscape(key)
	default:
		return "/finance/rates"
	}
}

// ── 쓰기 · 읽기 ─────────────────────────────────────────────────────────────

func (s *AssetMentionService) resolve(ctx context.Context, parsed []parsedMention) ([]model.MentionToken, map[string][]string) {
	has := false
	for _, p := range parsed {
		if p.kind != "" {
			has = true
			break
		}
	}
	if !has {
		return nil, map[string][]string{}
	}
	ix := s.index(ctx)
	return resolveAssetMentions(parsed, ix.lookup)
}

// sync stores a target's asset tags — 새로 태그한 것만 지금 값을 얼린다.
func (s *AssetMentionService) sync(ctx context.Context, targetType string, targetID int64, parsed []parsedMention) error {
	_, names := s.resolve(ctx, parsed)
	have, err := s.store.MentionIDs(ctx, targetType, targetID)
	if err != nil {
		return err
	}
	if len(names) == 0 && len(have) == 0 {
		return nil
	}
	var fresh []string
	for id := range names {
		if !have[id] {
			fresh = append(fresh, id)
		}
	}
	vals := s.values(ctx, fresh)
	all := map[string]repository.AssetMentionRow{}
	freshSet := map[string]bool{}
	for id, ns := range names {
		kind, key, _ := strings.Cut(id, "|")
		row := repository.AssetMentionRow{Kind: kind, Key: key, Names: ns, Name: s.index(ctx).entries[id].name}
		if v, ok := vals[id]; ok {
			row.Snap, row.Name = v.Snap, v.Name
			freshSet[id] = true
		}
		all[id] = row
	}
	return s.store.ReplaceMentions(ctx, targetType, targetID, all, freshSet)
}

// load returns stored asset tags with today's values. 토큰은 저장된 이름으로만 다시 찾는다.
func (s *AssetMentionService) load(ctx context.Context, targetType string, targetID int64, parsed []parsedMention) ([]model.MentionToken, []model.AssetMention) {
	rows, err := s.store.LoadMentions(ctx, targetType, targetID)
	if err != nil || len(rows) == 0 {
		if err != nil {
			log.Printf("asset mentions: load %s %d: %v", targetType, targetID, err)
		}
		return nil, nil
	}
	byName := map[string]map[string]string{}
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		if byName[r.Kind] == nil {
			byName[r.Kind] = map[string]string{}
		}
		for _, n := range r.Names {
			byName[r.Kind][normAssetText(n)] = r.Key
		}
		ids = append(ids, repository.MentionID(r.Kind, r.Key))
	}
	tokens, _ := resolveAssetMentions(parsed, func(kind, text string) (string, bool) {
		k, ok := byName[kind][normAssetText(text)]
		return k, ok
	})
	now := s.values(ctx, ids)
	out := make([]model.AssetMention, 0, len(rows))
	for _, r := range rows {
		id := repository.MentionID(r.Kind, r.Key)
		m := now[id]
		at := r.SnapAt
		m.Now, m.Snap, m.SnapAt = m.Snap, r.Snap, &at
		if m.Name == "" || m.Name == r.Key {
			m.Name = r.Name
		}
		out = append(out, m)
	}
	return tokens, out
}

// preview resolves without storing — 「그때」 = 지금(Now는 비운다).
func (s *AssetMentionService) preview(ctx context.Context, parsed []parsedMention) ([]model.MentionToken, []model.AssetMention) {
	tokens, _ := s.resolve(ctx, parsed)
	order := assetMentionOrder(tokens)
	vals := s.values(ctx, order)
	out := make([]model.AssetMention, 0, len(order))
	for _, id := range order {
		out = append(out, vals[id])
	}
	return tokens, out
}

// Peek — 자산 하나의 지금 값. 홈 칩·뉴스 칩이 페이지를 떠나지 않고 여는 미리보기(2026-10-09 요청 5).
// 색인에 없는 키는 false — 아무 문자열로 DB를 두드리지 못하게 한다.
func (s *AssetMentionService) Peek(ctx context.Context, kind, key string) (model.AssetMention, bool) {
	id := repository.MentionID(kind, key)
	if _, ok := s.index(ctx).entries[id]; !ok {
		return model.AssetMention{}, false
	}
	return s.values(ctx, []string{id})[id], true
}

/*
TagHeadline — 기사 제목에서 시군구·금리 이름을 찾아 칩으로 (2026-10-09 요청 4, 뉴스 주제 기사 전용).
글 태그와 같은 색인·같은 키를 쓴다 — 칩을 누르면 같은 자산 미리보기 모달이 뜬다. 단지·상품은 이름이 일반어와
겹쳐(「래미안」·「정기예금」) 제목 스캔에는 넣지 않는다. 한글이 든 2글자 이상 변형만 본다 — 「base」·「CD」
같은 영문 키가 단어 일부에 걸리지 않게. 제목에 나온 순서로 최대 4개.
*/
func (s *AssetMentionService) TagHeadline(ctx context.Context, text string) []model.NewsAssetTag {
	if s == nil || strings.TrimSpace(text) == "" {
		return nil
	}
	ix := s.index(ctx)
	norm := normAssetText(text)
	type hit struct {
		pos int
		tag model.NewsAssetTag
	}
	best := map[string]hit{} // kind|key → 가장 앞 위치
	// 해외 기사의 금리(「美 주담대 7.28%」)에 국내 금리 칩을 붙이지 않는다.
	foreign := false
	for _, m := range headlineForeignMarkers {
		if strings.Contains(strings.ToLower(text), m) { // 「美주담대」처럼 붙여 쓴다 — 낱말 경계를 보지 않는다
			foreign = true
			break
		}
	}
	// 시·도 칩은 부동산 제목에만 — 「서울 은행 예금」·「전국 신용대출」에 시·도 미리보기는 엉뚱하다.
	realEstate := false
	for _, w := range headlineRealEstateTerms {
		if strings.Contains(norm, w) {
			realEstate = true
			break
		}
	}
	for _, kind := range []string{model.AssetKindSido, model.AssetKindRegion, model.AssetKindRate} {
		if (kind == model.AssetKindRate && foreign) || (kind == model.AssetKindSido && !realEstate) {
			continue
		}
		for variant, key := range ix.byText[kind] {
			if utf8.RuneCountInString(variant) < 2 || !containsHangul(variant) {
				continue
			}
			pos := strings.Index(norm, variant)
			if pos < 0 {
				continue
			}
			pos = utf8.RuneCountInString(norm[:pos]) // 아래 법정동·단지 매칭과 같은 글자 단위로
			id := kind + "|" + key
			if h, ok := best[id]; ok && h.pos <= pos {
				continue
			}
			e := ix.entries[repository.MentionID(kind, key)]
			best[id] = hit{pos, model.NewsAssetTag{Kind: kind, Key: key, Name: e.name}}
		}
	}
	/*
		법정동·단지 이름(2026-10-09) — 「잠실동 아파트값」·「헬리오시티 신고가」처럼 시군구 이름 없이 쓰는 제목이 대부분이라
		시군구만 찾으면 부동산 기사 275건 중 6건에만 칩이 붙었다. 전국에 하나뿐인 이름만 쓴다(색인이 거른다).
		이름 4만 개를 훑지 않고 제목의 부분 문자열(3~20자)을 사전에서 찾는다 — 제목 하나에 수백 번.
	*/
	rs := []rune(norm)
	addHit := func(pos int, kind, key, name string) {
		id := kind + "|" + key
		if h, ok := best[id]; !ok || pos < h.pos {
			best[id] = hit{pos, model.NewsAssetTag{Kind: kind, Key: key, Name: name}}
		}
	}
	regionName := func(lawd string) string { return ix.entries[repository.MentionID(model.AssetKindRegion, lawd)].name }
	for i := range rs {
		for l := 3; l <= 20 && i+l <= len(rs); l++ {
			sub := string(rs[i : i+l])
			if lawd, ok := ix.headlineDong[sub]; ok {
				addHit(i, model.AssetKindRegion, lawd, regionName(lawd))
			}
			if a, ok := ix.headlineApt[sub]; ok {
				addHit(i, model.AssetKindApt, a[0], ix.entries[repository.MentionID(model.AssetKindApt, a[0])].name)
				addHit(i, model.AssetKindRegion, a[1], regionName(a[1]))
			}
		}
	}
	hits := make([]hit, 0, len(best))
	for _, h := range best {
		hits = append(hits, h)
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].pos != hits[j].pos {
			return hits[i].pos < hits[j].pos
		}
		return hits[i].tag.Key < hits[j].tag.Key
	})
	out := make([]model.NewsAssetTag, 0, min(4, len(hits)))
	for _, h := range hits[:min(4, len(hits))] {
		out = append(out, h.tag)
	}
	return out
}

// headlineAptName — 기사 제목에서 단지로 읽어도 되는 이름인가: 한글 5자 이상 · 숫자만 아님 · 흔한 말 아님.
// 「래미안」·「푸르지오」처럼 짧은 브랜드는 단지 수십 곳이라 애초에 유일하지 않고, 「현대아파트」 같은 일반어는 막는다.
func headlineAptName(n string) bool {
	if utf8.RuneCountInString(n) < 5 || !containsHangul(n) {
		return false
	}
	// 「강남아파트」·「서초아파트」 같은 이름은 기사에서 「강남 아파트」라는 일반어로 쓰인다(10-09 실측 오탐) — 끝이 일반 명사면 뺀다.
	for _, w := range []string{"아파트", "주택", "빌라", "맨션", "연립", "타운"} {
		if strings.HasSuffix(n, w) {
			return false
		}
	}
	return true
}

// headlineForeignMarkers — 제목이 해외 이야기임을 말하는 표시.
var headlineForeignMarkers = []string{"美", "미국", "英", "영국", "日", "일본", "中", "중국", "유럽", "연준", "fed"}

// headlineRealEstateTerms — 시·도 칩을 붙일 부동산 제목의 표시(띄어쓰기 없앤 소문자 제목에서 찾는다).
var headlineRealEstateTerms = []string{"아파트", "집값", "부동산", "주택", "전세", "전셋값", "매매", "분양", "청약", "재건축", "재개발", "실거래", "월세"}
