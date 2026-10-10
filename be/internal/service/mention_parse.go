package service

import (
	"regexp"
	"strings"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
글 태그 해석 — 순수 함수만 둔다(DB는 lookup 함수로 받는다).

FE `lib/mentions.ts`가 **같은 정규식**으로 토큰을 찾고, 서버가 준 `MentionToken{Raw, Len}`을
Raw로 찾아 그린다. 그래서 조사 처리(「@삼성전자가」의 「가」)·지표 이름 같은 판단은 여기 한
곳에만 있다 — 두 언어에 같은 규칙을 두 번 쓰지 않는다.
*/

// 앞 글자가 글자·숫자면 태그가 아니다(이메일 a@b.com). Go regexp에 lookbehind가 없어 그룹 1로 먹는다.
// 2026-10-03: 자산 태그 「@아파트[…]」 등 — 종류 낱말 + 대괄호(60자까지, 단지·상품 이름이 길다)를 맨 앞 대안으로.
var mentionTokenRe = regexp.MustCompile(
	`(^|[^0-9A-Za-z가-힣_])([@#](?:(?:아파트|지역|상품|금리)\[[^\]\n]{1,60}\]|\[[^\]\n]{1,40}\]|[0-9A-Za-z가-힣&.\-]{1,30})(?::[0-9A-Za-z가-힣·%]{1,20})?)`)

// mentionMaxTokens — 한 글에서 보는 토큰 수 상한(이름 후보 조회가 토큰 × 30개라서).
const mentionMaxTokens = 40

type parsedMention struct {
	raw       string
	card      bool
	name      string // 대괄호 안 글자 또는 조사까지 붙은 글자 run
	bracketed bool
	// bracketLen — 대괄호 형식일 때 Raw에서 이름 부분([...])이 차지하는 룬 수.
	bracketLen int
	fieldRun   string
	// kind — 자산 태그면 model.AssetKind*(종류 낱말은 kindLen 룬), 종목이면 빈칸.
	kind    string
	kindLen int
}

func parseMentions(body string) []parsedMention {
	var out []parsedMention
	for _, m := range mentionTokenRe.FindAllStringSubmatch(body, -1) {
		raw := m[2]
		p := parsedMention{raw: raw, card: raw[0] == '#'}
		rest := raw[1:]
		for _, w := range model.AssetKindWords {
			if strings.HasPrefix(rest, w.Word+"[") {
				p.kind, p.kindLen = w.Kind, len([]rune(w.Word))
				rest = rest[len(w.Word):]
				break
			}
		}
		if strings.HasPrefix(rest, "[") {
			end := strings.IndexByte(rest, ']')
			p.bracketed = true
			p.bracketLen = len([]rune(rest[:end+1]))
			p.name = strings.TrimSpace(rest[1:end])
			rest = rest[end+1:]
		} else if i := strings.IndexByte(rest, ':'); i >= 0 {
			p.name, rest = rest[:i], rest[i:]
		} else {
			p.name, rest = rest, ""
		}
		if strings.HasPrefix(rest, ":") {
			p.fieldRun = rest[1:]
		}
		if p.name == "" {
			continue
		}
		out = append(out, p)
		if len(out) >= mentionMaxTokens {
			break
		}
	}
	return out
}

// mentionCandidates — 이름 후보: 대괄호면 그대로, 아니면 앞부분 전부(긴 것부터).
func mentionCandidates(p parsedMention) []string {
	if p.bracketed {
		return []string{p.name}
	}
	runes := []rune(p.name)
	out := make([]string, 0, len(runes))
	for n := len(runes); n > 0; n-- {
		c := string(runes[:n])
		if strings.TrimRight(c, ".-") != c {
			continue // 「@AAPL.」의 마침표는 문장 끝이다
		}
		out = append(out, c)
	}
	return out
}

/*
resolveMentions — 토큰마다 가장 긴 이름을 종목으로. lookup은 이름(그대로) 또는 티커(대문자)로
찾는다. 못 찾은 토큰은 빠진다 — 화면에서는 그냥 글자다. 두 번째 값은 종목별로 본문이 부른 이름들.
*/
func resolveMentions(parsed []parsedMention, lookup func(name string) (string, bool)) ([]model.MentionToken, map[string][]string) {
	var out []model.MentionToken
	names := map[string][]string{}
	for _, p := range parsed {
		if p.kind != "" {
			continue // 자산 태그는 resolveAssetMentions
		}
		for _, cand := range mentionCandidates(p) {
			sym, ok := lookup(cand)
			if !ok {
				continue
			}
			tok := model.MentionToken{Raw: p.raw, Symbol: sym, Card: p.card}
			full := p.bracketed || cand == p.name
			if p.bracketed {
				tok.Len = 1 + p.bracketLen
			} else {
				tok.Len = 1 + len([]rune(cand))
			}
			if full && !p.card && p.fieldRun != "" {
				if id, n := model.MatchScreenFieldAlias(p.fieldRun); id != "" {
					tok.Field = id
					tok.Len += 1 + n
				}
			}
			out = append(out, tok)
			if !containsString(names[sym], cand) {
				names[sym] = append(names[sym], cand)
			}
			break
		}
	}
	return out, names
}

/*
resolveAssetMentions — 자산 태그(종류 낱말 + 대괄호)를 lookup(종류, 대괄호 안 글자)로 키에 붙인다.
두 번째 값은 자산(kind|key)별로 본문이 부른 이름들 — 저장해 두고 읽을 때 DB 이름 조회 없이 다시 찾는다.
*/
func resolveAssetMentions(parsed []parsedMention, lookup func(kind, text string) (string, bool)) ([]model.MentionToken, map[string][]string) {
	var out []model.MentionToken
	names := map[string][]string{}
	for _, p := range parsed {
		if p.kind == "" {
			continue
		}
		key, ok := lookup(p.kind, p.name)
		if !ok {
			continue
		}
		tok := model.MentionToken{Raw: p.raw, Kind: p.kind, Key: key, Card: p.card, Len: 1 + p.kindLen + p.bracketLen}
		if !p.card && p.fieldRun != "" {
			if id, n := model.MatchAssetFieldAlias(p.kind, p.fieldRun); id != "" {
				tok.Field = id
				tok.Len += 1 + n
			}
		}
		out = append(out, tok)
		id := repository.MentionID(p.kind, key)
		if !containsString(names[id], p.name) {
			names[id] = append(names[id], p.name)
		}
	}
	return out, names
}

// assetMentionOrder — 토큰에 나온 자산(kind|key), 처음 나온 순서.
func assetMentionOrder(tokens []model.MentionToken) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens {
		if t.Kind == "" {
			continue
		}
		if id := repository.MentionID(t.Kind, t.Key); !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// mentionSymbolOrder — 토큰에 나온 종목, 처음 나온 순서.
func mentionSymbolOrder(tokens []model.MentionToken) []string {
	seen := map[string]bool{}
	var out []string
	for _, t := range tokens {
		if !seen[t.Symbol] {
			seen[t.Symbol] = true
			out = append(out, t.Symbol)
		}
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

// plainMentionText — 요약·검색 설명용: 「@삼성전자:점수」→「삼성전자 점수」, 「#[TIGER 미국S&P500]」→ 이름.
func plainMentionText(body string) string {
	return mentionTokenRe.ReplaceAllStringFunc(body, func(m string) string {
		sub := mentionTokenRe.FindStringSubmatch(m)
		tok := sub[2][1:]
		for _, w := range model.AssetKindWords {
			if strings.HasPrefix(tok, w.Word+"[") {
				tok = tok[len(w.Word):] // 「@아파트[송파구 헬리오시티]」→「송파구 헬리오시티」
				break
			}
		}
		if strings.HasPrefix(tok, "[") {
			if end := strings.IndexByte(tok, ']'); end > 0 {
				tok = tok[1:end] + tok[end+1:]
			}
		}
		return sub[1] + strings.Replace(tok, ":", " ", 1)
	})
}
