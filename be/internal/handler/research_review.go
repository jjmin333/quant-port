package handler

import (
	"net/http"

	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

// ResearchReviewHandler serves the stored 「AI 검토 노트」 (LLM 0 on read).
type ResearchReviewHandler struct {
	svc *service.ResearchReviewService
}

func NewResearchReviewHandler(svc *service.ResearchReviewService) *ResearchReviewHandler {
	return &ResearchReviewHandler{svc: svc}
}

// Get — GET /research/calls/{id}/review. 공개 리포트만, 아직 없으면 data=null.
func (h *ResearchReviewHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := researchIDParam(w, r)
	if !ok {
		return
	}
	row, err := h.svc.Get(r.Context(), id)
	if err != nil {
		response.Error(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": row})
}
