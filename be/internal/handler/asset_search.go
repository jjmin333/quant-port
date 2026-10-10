package handler

import (
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
AssetSearchHandler — 헤더 검색의 「자산」 칸 (2026-10-09 사용자 요청: 아파트·대출·금리 상품도 검색으로 조회).

종목 자동완성과 나란히 부른다. 아파트는 단지 목록(re_complexes — 점수 없는 단지까지, 이름·도로명·지번),
금융상품·금리·지역은 글 태그와 같은 이름 색인(메모리)에서. 조회만 하고 외부 호출·LLM은 없다.
*/
type AssetSearchHandler struct {
	data   *repository.PublicDataRepo
	assets *service.AssetMentionService
}

func NewAssetSearchHandler(data *repository.PublicDataRepo, assets *service.AssetMentionService) *AssetSearchHandler {
	return &AssetSearchHandler{data: data, assets: assets}
}

// AssetSearchItem is one row of the header's asset group. Href is where a click goes.
type AssetSearchItem struct {
	Kind string `json:"kind"` // apt · product · rate · region
	Key  string `json:"key"`
	Name string `json:"name"`
	Sub  string `json:"sub,omitempty"`
	Href string `json:"href"`
}

// Search serves GET /search/assets?q= — 2글자 이상.
func (h *AssetSearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if n := utf8.RuneCountInString(q); n < 2 || n > 40 {
		response.JSON(w, http.StatusOK, map[string]any{"data": []AssetSearchItem{}})
		return
	}
	out := []AssetSearchItem{}
	hits, err := h.data.SearchComplexDirectory(r.Context(), "", q, 5)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, c := range hits {
		sub := strings.TrimSpace(c.Region + " " + c.UmdNm)
		if c.Road != "" {
			sub += " · " + c.Road
		}
		out = append(out, AssetSearchItem{Kind: model.AssetKindApt, Key: c.Key, Name: c.Name, Sub: sub,
			Href: "/real-estate?lawd=" + c.LawdCd + "&complex=" + c.Key})
	}
	for _, kind := range []string{model.AssetKindProduct, model.AssetKindRate, model.AssetKindSido, model.AssetKindRegion} {
		for _, s := range h.assets.Search(r.Context(), q, kind, 3) {
			out = append(out, AssetSearchItem{Kind: s.Kind, Key: s.Key, Name: s.Name, Sub: s.Sub, Href: assetHref(s.Kind, s.Key)})
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func assetHref(kind, key string) string {
	switch kind {
	case model.AssetKindProduct:
		return "/finance/products/" + key
	case model.AssetKindRate:
		return "/finance/rates"
	case model.AssetKindRegion:
		return "/real-estate?lawd=" + key
	case model.AssetKindSido:
		if key == "00" {
			return "/real-estate"
		}
		return "/real-estate?sido=" + key
	}
	return "/"
}
