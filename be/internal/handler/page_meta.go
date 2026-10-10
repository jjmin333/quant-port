package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/pkg/response"
)

/*
화면 메타 **공개 읽기** — `GET /api/v1/page-meta` (2026-09-15 사용자 요청 2).

# 왜 관리자 입구와 따로인가

이 값을 읽는 쪽이 **Next 서버**다. 화면을 그리기 전에 제목을 알아야 하고, 그 순간에는
로그인한 사람이 없다(크롤러일 수도 있다). 관리자 입구 뒤에 두면 렌더가 관리자
세션을 들고 다녀야 하는데, 그건 제목 하나를 위해 인증 경계를 무너뜨리는 일이다.

내용도 공개해서 곤란한 것이 없다 — **이 표에 있는 글은 전부 검색 결과에 나가라고
적은 글**이다.

# 왜 목록 통째로인가

행이 스무 개 남짓이고, 읽는 쪽은 화면마다 자기 경로 하나를 찾는다. 경로별 입구를
두면 화면 수만큼 왕복이 생기고 캐시도 그만큼 쪼개진다. 한 벌을 접어 두면
(FE `lib/page-meta.ts`의 `revalidate`) 크롤러가 몇 번을 훑든 BE 호출은 주기당 하나다.
*/
type PageMetaHandler struct {
	repo *repository.PageMetaRepo
}

func NewPageMetaHandler(repo *repository.PageMetaRepo) *PageMetaHandler {
	return &PageMetaHandler{repo: repo}
}

// List returns every override. A missing table is an empty list, not an error —
// 덮어쓰기가 없다는 것과 표가 아직 없다는 것은 **화면 입장에서 같은 상황**이다.
func (h *PageMetaHandler) List(w http.ResponseWriter, r *http.Request) {
	items := []model.PageMeta{}
	if h != nil && h.repo != nil {
		if rows, err := h.repo.List(r.Context()); err == nil && rows != nil {
			items = rows
		}
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": map[string]any{"items": items}})
}
