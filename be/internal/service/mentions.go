package service

import (
	"context"
	"log"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
MentionService — 커뮤니티 글·투자 리포트 본문의 데이터 태그 (2026-09-28 사용자 요청 2).

쓰기(Sync): 본문을 해석해 종목을 찾고, **새로 태그된 종목만** 그 순간 값을 얼린다(스크리너 지표
26개 + 가격 + 점수). 글을 고쳐도 이미 있던 태그의 「그때」 값은 그대로다.
읽기(Load): 저장된 이름으로 토큰을 다시 찾고(DB 이름 조회 없음), 지금 값을 나란히 붙인다.
LLM 0 · 외부 호출 0(가격은 이미 있는 시세 캐시).
*/
type MentionService struct {
	repo     *repository.MentionRepo
	screener *ScreenerService
	quotes   *QuotesService
	scores   *ScoreService
	ai       func(ctx context.Context, symbols []string) (map[string]repository.CommunityAIOverview, error)
	// assets — 자산 태그(단지·지역·상품·금리, 2026-10-03). nil이면 자산 태그는 글자로 남는다.
	assets *AssetMentionService
}

// SetAssets wires the asset tag resolver (@아파트[…] · @지역[…] · @상품[…] · @금리[…]).
func (s *MentionService) SetAssets(a *AssetMentionService) { s.assets = a }

func NewMentionService(repo *repository.MentionRepo, screener *ScreenerService) *MentionService {
	return &MentionService{repo: repo, screener: screener}
}

func (s *MentionService) SetQuotes(q *QuotesService) { s.quotes = q }
func (s *MentionService) SetScores(sc *ScoreService) { s.scores = sc }

// SetAIOverviews wires the stored AI summary reader (카드의 AI 한 줄).
func (s *MentionService) SetAIOverviews(f func(ctx context.Context, symbols []string) (map[string]repository.CommunityAIOverview, error)) {
	s.ai = f
}

func (s *MentionService) resolve(ctx context.Context, body string) ([]model.MentionToken, map[string][]string, repository.MentionLookup, error) {
	parsed := parseMentions(body)
	var cands []string
	for _, p := range parsed {
		cands = append(cands, mentionCandidates(p)...)
	}
	lk, err := s.repo.ResolveNames(ctx, cands)
	if err != nil {
		return nil, nil, lk, err
	}
	tokens, names := resolveMentions(parsed, func(c string) (string, bool) {
		st, ok := lk.Find(c)
		return st.Symbol, ok
	})
	return tokens, names, lk, nil
}

// Check rejects a body that tags too many stocks. 쓰기 전에 부른다(글을 만든 뒤 거절하지 않게).
func (s *MentionService) Check(ctx context.Context, body string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	tokens, _, _, err := s.resolve(ctx, body)
	if err != nil {
		return err
	}
	if n := len(mentionSymbolOrder(tokens)); n > model.MentionMaxSymbols {
		return invalidInput("태그한 종목은 %d개까지입니다(지금 %d개)", model.MentionMaxSymbols, n)
	}
	if s.assets != nil {
		if at, _ := s.assets.resolve(ctx, parseMentions(body)); len(assetMentionOrder(at)) > model.MentionMaxAssets {
			return invalidInput("태그한 부동산·금융상품·금리는 %d개까지입니다(지금 %d개)", model.MentionMaxAssets, len(assetMentionOrder(at)))
		}
	}
	return nil
}

// Sync stores the body's mentions for one target. 실패해도 글은 이미 있다 — 부른 쪽이 로그만 남긴다.
func (s *MentionService) Sync(ctx context.Context, targetType string, targetID int64, body string) error {
	if s == nil || s.repo == nil {
		return nil
	}
	tokens, names, _, err := s.resolve(ctx, body)
	if err != nil {
		return err
	}
	if len(mentionSymbolOrder(tokens)) > model.MentionMaxSymbols {
		return invalidInput("태그한 종목은 %d개까지입니다", model.MentionMaxSymbols)
	}
	have, err := s.repo.Symbols(ctx, targetType, targetID)
	if err != nil {
		return err
	}
	var fresh []string
	for sym := range names {
		if !have[sym] {
			fresh = append(fresh, sym)
		}
	}
	if err := s.repo.Replace(ctx, targetType, targetID, names, s.snapshot(ctx, fresh)); err != nil {
		return err
	}
	if s.assets != nil {
		return s.assets.sync(ctx, targetType, targetID, parseMentions(body))
	}
	return nil
}

// SyncLogged is Sync for write paths that already succeeded.
func (s *MentionService) SyncLogged(ctx context.Context, targetType string, targetID int64, body string) {
	if err := s.Sync(ctx, targetType, targetID, body); err != nil {
		log.Printf("mentions: sync %s %d: %v", targetType, targetID, err)
	}
}

func (s *MentionService) snapshot(ctx context.Context, symbols []string) map[string]model.MentionSnap {
	out := make(map[string]model.MentionSnap, len(symbols))
	if len(symbols) == 0 {
		return out
	}
	var values map[string]map[string]float64
	if s.screener != nil {
		_, values, _, _ = s.screener.Values(ctx, symbols)
	}
	for _, sym := range symbols {
		snap := model.MentionSnap{Fields: values[sym]}
		if snap.Fields == nil {
			snap.Fields = map[string]float64{}
		}
		if v, ok := snap.Fields["score"]; ok {
			sc := int(v + 0.5)
			snap.Score = &sc
		}
		if s.quotes != nil {
			if q, ok := s.quotes.GetQuote(ctx, sym); ok && q.CurrentPrice > 0 {
				price, rate := q.CurrentPrice, q.ChangeRate
				snap.Price, snap.ChangeRate, snap.Currency = &price, &rate, string(q.Currency)
			}
		}
		out[sym] = snap
	}
	return out
}

// Load returns a stored target's mentions with read-time values. 없으면 nil.
func (s *MentionService) Load(ctx context.Context, targetType string, targetID int64, body string) *model.MentionSet {
	if s == nil || s.repo == nil {
		return nil
	}
	rows, err := s.repo.Load(ctx, targetType, targetID)
	if err != nil {
		log.Printf("mentions: load %s %d: %v", targetType, targetID, err)
		rows = nil
	}
	var assetTokens []model.MentionToken
	var assets []model.AssetMention
	if s.assets != nil {
		assetTokens, assets = s.assets.load(ctx, targetType, targetID, parseMentions(body))
	}
	if len(rows) == 0 && len(assets) == 0 {
		return nil
	}
	byName := map[string]string{}
	for _, r := range rows {
		for _, n := range r.Names {
			byName[strings.ToLower(n)] = r.Symbol
		}
	}
	tokens, _ := resolveMentions(parseMentions(body), func(c string) (string, bool) {
		sym, ok := byName[strings.ToLower(c)]
		return sym, ok
	})
	set := &model.MentionSet{Tokens: append(tokens, assetTokens...), Symbols: make([]model.ContentMention, 0, len(rows)), Assets: assets}
	for _, r := range rows {
		m := model.ContentMention{
			CommunitySymbol: model.CommunitySymbol{
				Symbol: r.Symbol, Name: r.Name, AssetType: r.AssetType,
				SnapPrice: r.Snap.Price, SnapCurrency: r.Snap.Currency, SnapChangeRate: r.Snap.ChangeRate,
				SnapScore: r.Snap.Score, SnapAt: r.SnapAt,
			},
			Snap: r.Snap.Fields,
		}
		set.Symbols = append(set.Symbols, m)
	}
	s.attachNow(ctx, set.Symbols)
	set.FillFields()
	return set
}

// Preview resolves a draft without storing — 편집기 미리보기. 「그때」 = 지금.
func (s *MentionService) Preview(ctx context.Context, body string) (model.MentionSet, error) {
	set := model.MentionSet{Tokens: []model.MentionToken{}, Symbols: []model.ContentMention{}}
	if s == nil || s.repo == nil {
		return set, nil
	}
	tokens, _, lk, err := s.resolve(ctx, body)
	if err != nil {
		return set, err
	}
	// 덮어쓰지 않고 붙인다 — 태그가 없으면 tokens가 nil이라 JSON이 null이 되고, 화면이 그 length를 읽다
	// 죽었다(2026-10-19 리포트 본문의 「## 소제목」만 있는 글 → 「#」 때문에 미리보기를 부른다).
	set.Tokens = append(set.Tokens, tokens...)
	order := mentionSymbolOrder(tokens)
	snaps := s.snapshot(ctx, order)
	for _, sym := range order {
		st := lk.BySymbol[sym]
		snap := snaps[sym]
		set.Symbols = append(set.Symbols, model.ContentMention{
			CommunitySymbol: model.CommunitySymbol{
				Symbol: sym, Name: st.Name, AssetType: st.AssetType,
				SnapPrice: snap.Price, SnapCurrency: snap.Currency, SnapChangeRate: snap.ChangeRate, SnapScore: snap.Score,
			},
			Snap: snap.Fields,
		})
	}
	s.attachNow(ctx, set.Symbols)
	for i := range set.Symbols {
		set.Symbols[i].Now = nil // 미리보기에서는 그때 = 지금이다
	}
	if s.assets != nil {
		at, assets := s.assets.preview(ctx, parseMentions(body))
		set.Tokens = append(set.Tokens, at...)
		set.Assets = assets
	}
	set.FillFields()
	return set, nil
}

// attachNow fills today's values, score/grade and the stored AI line (카드).
func (s *MentionService) attachNow(ctx context.Context, list []model.ContentMention) {
	if len(list) == 0 {
		return
	}
	syms := make([]string, 0, len(list))
	for _, m := range list {
		syms = append(syms, m.Symbol)
	}
	if s.screener != nil {
		if _, values, _, err := s.screener.Values(ctx, syms); err == nil {
			for i := range list {
				list[i].Now = values[list[i].Symbol]
			}
		}
	}
	if s.scores != nil {
		if views, err := s.scores.GetMany(ctx, syms); err == nil {
			for _, v := range views {
				for i := range list {
					if strings.EqualFold(list[i].Symbol, v.Symbol) && v.Score != nil {
						sc := *v.Score
						list[i].Score, list[i].ScoreGrade = &sc, v.Grade
					}
				}
			}
		}
	}
	if s.ai != nil {
		if ai, err := s.ai(ctx, syms); err == nil {
			for i := range list {
				if o, ok := ai[list[i].Symbol]; ok {
					list[i].AIOverview, list[i].AIAt = o.Text, o.At
				}
			}
		}
	}
}

// SearchAssets — 자산 태그 자동완성. 자산 해석기가 없으면 빈 목록.
func (s *MentionService) SearchAssets(ctx context.Context, q, kind string, limit int) []AssetSuggestion {
	if s == nil || s.assets == nil {
		return []AssetSuggestion{}
	}
	return s.assets.Search(ctx, q, kind, limit)
}

// PeekAsset — 자산 미리보기 하나. 자산 해석기가 없으면 false.
func (s *MentionService) PeekAsset(ctx context.Context, kind, key string) (model.AssetMention, bool) {
	if s == nil || s.assets == nil {
		return model.AssetMention{}, false
	}
	return s.assets.Peek(ctx, kind, key)
}
