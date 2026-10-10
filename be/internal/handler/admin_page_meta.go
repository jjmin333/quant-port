package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
화면 메타 관리 (`/admin/seo`) — 2026-09-15 사용자 요청 2.

# 왜 관리 화면이 필요한가

브라우저 탭 이름과 검색 결과에 나가는 글은 **운영하면서 다듬는 글**이다. 그런데
지금까지는 Next 소스에 문자열로 박혀 있어서 한 줄 고치자고 이미지를 다시 굽고
배포를 해야 했다 — 2026-09-13에 종목 별칭을 표로 옮긴 것과 정확히 같은 이유다.

# 덮어쓰기지 정답이 아니다

빈 칸은 저장되지 않고, 아무것도 안 적은 행은 **지워진다**(`PageMeta.IsEmpty`).
그래서 이 표가 통째로 비어도 사이트는 지금 그대로 돌아간다. 화면 쪽 규칙은
FE `lib/page-meta.ts`에 있다.

# 검증은 관용보다 거부

경로는 정규화만 하고(소문자·앞 슬래시), 길이를 넘기거나 모양이 아닌 값은 **거절한다**.
이 자료는 틀려도 화면에 오류가 안 뜬다 — `/markett`에 저장된 제목은 조용히 아무
화면에도 안 나오고, 관리자는 「저장은 됐는데 왜 안 바뀌지」를 처음부터 다시 조사한다.
길이 상한은 컬럼과 같아야 한다. DB가 잘라 주기를 기대하면 STRICT 모드에서는 저장
자체가 실패하고, 아니면 **말이 중간에서 끊긴 채** 검색 결과에 나간다.
*/

const (
	pageMetaMaxPath        = 191
	pageMetaMaxTitle       = 255
	pageMetaMaxDescription = 500
)

// SetPageMeta wires the override table.
func (h *AdminHandler) SetPageMeta(repo *repository.PageMetaRepo) {
	if h != nil {
		h.pageMeta = repo
	}
}

func (h *AdminHandler) ListPageMeta(w http.ResponseWriter, r *http.Request) {
	if h.pageMeta == nil {
		response.Error(w, http.StatusServiceUnavailable, "page meta table not available")
		return
	}
	items, err := h.pageMeta.List(r.Context())
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	if items == nil {
		items = []model.PageMeta{}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}

type pageMetaBody struct {
	Path          string `json:"path"`
	Title         string `json:"title"`
	Description   string `json:"description"`
	OGTitle       string `json:"ogTitle"`
	OGDescription string `json:"ogDescription"`
	OGImage       string `json:"ogImage"`
	NoIndex       *bool  `json:"noindex"`
}

func (h *AdminHandler) PutPageMeta(w http.ResponseWriter, r *http.Request) {
	if h.pageMeta == nil {
		response.Error(w, http.StatusServiceUnavailable, "page meta table not available")
		return
	}
	var in pageMetaBody
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
		response.Error(w, http.StatusBadRequest, "invalid body")
		return
	}
	item, msg := decodePageMeta(in)
	if msg != "" {
		response.Error(w, http.StatusBadRequest, msg)
		return
	}
	if err := h.pageMeta.Upsert(r.Context(), item); err != nil {
		writePageMetaError(w, err)
		return
	}
	/*
		저장한 뒤 **목록 전체를 돌려준다.** 빈 값을 저장하면 행이 지워지므로, 방금
		무엇이 남았는지는 응답 하나로 보이는 편이 정직하다 — 화면이 자기 초안을
		정답이라고 믿으면 「지워진 행」이 화면에만 남는다.
	*/
	h.ListPageMeta(w, r)
}

func (h *AdminHandler) DeletePageMeta(w http.ResponseWriter, r *http.Request) {
	if h.pageMeta == nil {
		response.Error(w, http.StatusServiceUnavailable, "page meta table not available")
		return
	}
	path := repository.NormalizePath(r.URL.Query().Get("path"))
	if path == "" {
		response.Error(w, http.StatusBadRequest, "path is required")
		return
	}
	if err := h.pageMeta.Delete(r.Context(), path); err != nil {
		writePageMetaError(w, err)
		return
	}
	h.ListPageMeta(w, r)
}

/*
decodePageMeta — 들어온 값을 검사하고 저장 형태로 옮긴다.

공백만 다듬고 **내용은 고치지 않는다.** 길이를 넘치면 자르지 않고 거절하는 이유가
위 머리글에 있다. 빈 문자열은 그대로 통과한다 — 그것이 「이 칸은 코드 기본값을
쓴다」는 뜻이고, 전부 비면 저장 대신 삭제가 된다.
*/
func decodePageMeta(in pageMetaBody) (model.PageMeta, string) {
	path := repository.NormalizePath(in.Path)
	if path == "" {
		return model.PageMeta{}, "path is required"
	}
	if len(path) > pageMetaMaxPath {
		return model.PageMeta{}, "path is too long"
	}
	if strings.ContainsAny(path, " \t\n?#") {
		return model.PageMeta{}, "path must be a plain app path like /market"
	}

	item := model.PageMeta{
		Path:          path,
		Title:         strings.TrimSpace(in.Title),
		Description:   strings.TrimSpace(in.Description),
		OGTitle:       strings.TrimSpace(in.OGTitle),
		OGDescription: strings.TrimSpace(in.OGDescription),
		OGImage:       strings.TrimSpace(in.OGImage),
		NoIndex:       in.NoIndex,
	}

	/*
		길이는 **글자가 아니라 바이트**로 센다 — 컬럼 상한이 바이트이고 한글은
		utf8mb4 에서 세 바이트다. 글자로 세면 관리자 화면에서는 통과한 글이 저장
		직전에 MySQL 에서 터진다.
	*/
	for _, check := range []struct {
		label string
		value string
		max   int
	}{
		{"title", item.Title, pageMetaMaxTitle},
		{"ogTitle", item.OGTitle, pageMetaMaxTitle},
		{"description", item.Description, pageMetaMaxDescription},
		{"ogDescription", item.OGDescription, pageMetaMaxDescription},
		{"ogImage", item.OGImage, pageMetaMaxDescription},
	} {
		if len(check.value) > check.max {
			return model.PageMeta{}, check.label + " is too long"
		}
		if !utf8.ValidString(check.value) {
			return model.PageMeta{}, check.label + " is not valid text"
		}
	}
	return item, ""
}

func writePageMetaError(w http.ResponseWriter, err error) {
	if errors.Is(err, repository.ErrPageMetaUnavailable) {
		response.Error(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	response.Error(w, http.StatusInternalServerError, err.Error())
}
