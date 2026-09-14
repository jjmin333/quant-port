package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode"

	"github.com/go-chi/chi/v5"

	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
종목 별칭 관리 (`/admin/aliases`) — 2026-09-14 사용자 요청 1.

# 왜 관리 화면이 필요한가

별칭은 **사람만 알 수 있는 자료**다. 「구글」의 상장명은 「알파벳 A」이고, NAVER의
한글 읽기는 DB 어디에도 없다. 어떤 문자열 매칭으로도 이어지지 않으므로 누군가 적어
두는 수밖에 없고, 새 종목·새 별명이 생길 때마다 늘어난다.

2026-09-13에 이 자료를 Go 소스의 map에서 표로 옮겼다. 옮긴 이유는 속도가 아니라
**한 줄 고치려면 배포를 해야 했다**는 것이었다. 그런데 표만 만들고 화면을 안 만들면
그 이유가 절반만 이뤄진다 — 여전히 SQL을 직접 쳐야 한다. 이 파일이 나머지 절반이다.

# 왜 DB 콘솔(`/admin/database`)로 충분하지 않은가

그 화면은 어떤 표든 같은 격자로 보여 준다. 별칭에는 그 격자가 모르는 규칙이 셋 있다.

  · **갈래가 둘이고 규칙이 다르다.** `brand`의 target은 영문 표기(소문자로 쓰인다)
    이고 `symbol`의 target은 종목 심볼(대문자)이다. 격자는 둘을 구분 못 한다.
  · **심볼 오타는 조용히 실패한다.** 없는 심볼을 적으면 오류가 안 나고 그냥 아무
    기사도 안 붙는다 — 이 저장소가 되풀이해 만난 가장 비싼 실패 모양이다.
  · **캐시를 다시 읽어야 한다.** 표를 고쳐도 스냅샷이 10분 낡아 있으면 관리자는
    「안 됐다」로 읽고 한 번 더 고친다.

# 검증은 관용보다 거부

들어온 값을 고쳐서 받아 주지 않는다(대소문자 정규화만 한다). 「대충 받아서 맞춰
주기」는 쓰는 사람이 자기가 무엇을 넣었는지 모르게 만들고, 별칭은 틀려도 화면에
오류가 안 뜨는 자료다 — 틀린 채 저장되면 아무도 모른다.
*/

// SetStockAliases wires the alias table. 생성자 인자가 이미 열한 개라 세터로 붙인다
// (`SetIncidents`와 같은 이유).
func (h *AdminHandler) SetStockAliases(repo *repository.StockAliasRepo, stocks *repository.StockRepo) {
	if h == nil {
		return
	}
	h.aliases = repo
	h.aliasStocks = stocks
}

func (h *AdminHandler) ListStockAliases(w http.ResponseWriter, r *http.Request) {
	if h.aliases == nil {
		response.Error(w, http.StatusServiceUnavailable, "alias table not available")
		return
	}
	q := r.URL.Query()
	kind := strings.TrimSpace(q.Get("kind"))
	if kind != "" && !validAliasKind(kind) {
		response.Error(w, http.StatusBadRequest, "kind must be brand or symbol")
		return
	}
	items, err := h.aliases.List(r.Context(), repository.StockAliasFilter{
		Kind:  kind,
		Query: strings.TrimSpace(q.Get("q")),
		// 관리 화면은 **언제나** 꺼진 행까지 본다. 끈 행과 없는 행이 같아 보이면
		// 관리자가 「왜 안 잡히지」를 처음부터 다시 조사한다.
		IncludeDisabled: true,
	})
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	brand, symbol, disabled, err := h.aliases.Counts(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	/*
		요약을 `data` **안에** 넣는다. FE의 `apiRequest`는 봉투에서 `data`만 꺼내
		돌려주므로(api-client.ts), 형제 키로 두면 화면까지 오지 못한다.
	*/
	response.JSON(w, http.StatusOK, map[string]any{
		"data": map[string]any{
			"items": items,
			"summary": map[string]int{
				"brand": brand, "symbol": symbol, "disabled": disabled,
			},
		},
	})
}

type stockAliasBody struct {
	Alias   string `json:"alias"`
	Kind    string `json:"kind"`
	Target  string `json:"target"`
	Note    string `json:"note"`
	Enabled *bool  `json:"enabled"`
}

func (h *AdminHandler) CreateStockAlias(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decodeAlias(w, r)
	if !ok {
		return
	}
	item, err := h.aliases.Create(r.Context(), in)
	if err != nil {
		writeAliasError(w, err)
		return
	}
	h.aliases.ReloadCache(r.Context())
	response.JSON(w, http.StatusCreated, map[string]any{"data": item})
}

func (h *AdminHandler) UpdateStockAlias(w http.ResponseWriter, r *http.Request) {
	id, ok := aliasParamID(w, r)
	if !ok {
		return
	}
	in, ok := h.decodeAlias(w, r)
	if !ok {
		return
	}
	item, err := h.aliases.Update(r.Context(), id, in)
	if err != nil {
		writeAliasError(w, err)
		return
	}
	h.aliases.ReloadCache(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": item})
}

func (h *AdminHandler) DeleteStockAlias(w http.ResponseWriter, r *http.Request) {
	if h.aliases == nil {
		response.Error(w, http.StatusServiceUnavailable, "alias table not available")
		return
	}
	id, ok := aliasParamID(w, r)
	if !ok {
		return
	}
	if err := h.aliases.Delete(r.Context(), id); err != nil {
		writeAliasError(w, err)
		return
	}
	h.aliases.ReloadCache(r.Context())
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]int64{"id": id}})
}

/*
decodeAlias reads and **validates** one row. 고쳐서 받아 주는 것은 대소문자뿐이다.

	alias   소문자로 저장한다 — 매칭이 소문자 인덱스에서 일어난다(`ReloadStockAliases`).
	        한글 별칭에는 대소문자가 없으니 실질적으로 영문 별칭에만 작용한다.
	target  brand 면 소문자(영문 표기), symbol 이면 대문자(심볼). 스냅샷이 그렇게
	        담으므로 여기서 맞춰 두면 화면에 보이는 것과 실제로 쓰이는 것이 같아진다.
*/
func (h *AdminHandler) decodeAlias(w http.ResponseWriter, r *http.Request) (repository.StockAliasInput, bool) {
	var empty repository.StockAliasInput
	if h.aliases == nil {
		response.Error(w, http.StatusServiceUnavailable, "alias table not available")
		return empty, false
	}
	var body stockAliasBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid json body")
		return empty, false
	}

	alias := strings.ToLower(strings.TrimSpace(body.Alias))
	target := strings.TrimSpace(body.Target)
	kind := strings.TrimSpace(body.Kind)
	note := strings.TrimSpace(body.Note)

	if alias == "" {
		response.Error(w, http.StatusBadRequest, "별칭을 입력하세요")
		return empty, false
	}
	if len([]rune(alias)) > 64 {
		response.Error(w, http.StatusBadRequest, "별칭은 64자를 넘을 수 없습니다")
		return empty, false
	}
	/*
		한 글자 별칭은 받지 않는다. 「A」 하나가 기사 제목 어디에나 있어서, 붙는
		순간 그 종목에 세상의 절반이 태깅된다 — 낱말 경계 규칙으로도 못 막는다
		(실측으로 「AMID」·「블록」·「타깃」을 이미 뺐다).
	*/
	if len([]rune(alias)) < 2 {
		response.Error(w, http.StatusBadRequest, "별칭은 두 글자 이상이어야 합니다 — 한 글자는 아무 제목에나 걸립니다")
		return empty, false
	}
	if !validAliasKind(kind) {
		response.Error(w, http.StatusBadRequest, "갈래는 brand 또는 symbol 이어야 합니다")
		return empty, false
	}
	if target == "" {
		response.Error(w, http.StatusBadRequest, "대상을 입력하세요")
		return empty, false
	}
	if len([]rune(target)) > 64 {
		response.Error(w, http.StatusBadRequest, "대상은 64자를 넘을 수 없습니다")
		return empty, false
	}
	if len([]rune(note)) > 255 {
		response.Error(w, http.StatusBadRequest, "메모는 255자를 넘을 수 없습니다")
		return empty, false
	}

	if kind == "symbol" {
		target = strings.ToUpper(target)
		if !looksLikeSymbol(target) {
			response.Error(w, http.StatusBadRequest,
				"심볼은 영문·숫자와 . - 만 쓸 수 있습니다 (예: GOOGL, 005930)")
			return empty, false
		}
		/*
			**없는 심볼은 거부한다.** 이 검사가 없으면 오타가 조용히 저장되고,
			그 뒤로 그 별칭은 아무 기사에도 안 붙는다 — 오류가 없으니 아무도
			모르고, 「왜 구글 뉴스가 안 뜨지」를 다시 조사하게 된다.
		*/
		if h.aliasStocks != nil {
			rows, err := h.aliasStocks.ListBySymbols(r.Context(), []string{target})
			if err != nil {
				response.Error(w, http.StatusInternalServerError, err.Error())
				return empty, false
			}
			if len(rows) == 0 {
				response.Error(w, http.StatusBadRequest,
					"카탈로그에 없는 심볼입니다: "+target)
				return empty, false
			}
		}
	} else {
		// brand 의 target 은 영문 **표기**다. 스냅샷이 소문자로 담는다.
		target = strings.ToLower(target)
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}
	return repository.StockAliasInput{
		Alias: alias, Kind: kind, Target: target, Note: note, Enabled: enabled,
	}, true
}

func validAliasKind(kind string) bool {
	for _, k := range repository.AliasKinds {
		if kind == k {
			return true
		}
	}
	return false
}

// looksLikeSymbol — 티커 모양인가. 국내는 숫자 여섯 자리, 해외는 영문에 . - 가 섞인다.
func looksLikeSymbol(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
		case unicode.IsDigit(r):
		case r == '.' || r == '-':
		default:
			return false
		}
	}
	return true
}

func aliasParamID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		response.Error(w, http.StatusBadRequest, "invalid id")
		return 0, false
	}
	return id, true
}

func writeAliasError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, repository.ErrStockAliasDuplicate):
		response.Error(w, http.StatusConflict, "같은 별칭·갈래·대상 조합이 이미 있습니다")
	case errors.Is(err, repository.ErrStockAliasNotFound):
		response.Error(w, http.StatusNotFound, "stock alias not found")
	default:
		response.Error(w, http.StatusInternalServerError, err.Error())
	}
}
