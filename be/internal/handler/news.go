package handler

import (
	"net/http"
	"strconv"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/service"
	"github.com/enact/quant-port/be/pkg/response"
)

type NewsHandler struct {
	store  *service.NewsStore
	reader *service.NewsReaderService
}

func NewNewsHandler(store *service.NewsStore, reader *service.NewsReaderService) *NewsHandler {
	return &NewsHandler{store: store, reader: reader}
}

func (h *NewsHandler) ListDomestic(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.JSON(w, http.StatusOK, map[string]any{
			"data": []model.NewsArticle{},
			"meta": map[string]any{"count": 0},
		})
		return
	}

	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	filter := model.NewsListFilter{
		Market:   "domestic",
		SourceID: q.Get("source"),
		Sort:     q.Get("sort"),
		Query:    q.Get("q"),
		Symbol:   q.Get("symbol"),
		Limit:    limit,
	}
	if filter.Sort == "" {
		filter.Sort = "latest"
	}
	if filter.SourceID == "" {
		filter.SourceID = "all"
	}

	articles, updatedAt := h.store.List(r.Context(), filter)
	meta := map[string]any{"count": len(articles)}
	if !updatedAt.IsZero() {
		meta["updatedAt"] = updatedAt
	}
	response.JSON(w, http.StatusOK, map[string]any{
		"data": articles,
		"meta": meta,
	})
}

func (h *NewsHandler) UsDemo(w http.ResponseWriter, r *http.Request) {
	if h.store == nil {
		response.Error(w, http.StatusServiceUnavailable, "news store not configured")
		return
	}
	article, ok := h.store.FirstRSSArticle(r.Context())
	if !ok {
		response.Error(w, http.StatusNotFound, "no rss articles available yet")
		return
	}
	tags := append([]string{}, article.Tags...)
	tags = appendUniqueTag(tags, "Reader API")
	tags = appendUniqueTag(tags, "RSS")
	article.Tags = tags
	response.JSON(w, http.StatusOK, map[string]any{
		"data": model.NewsUsDemoArticle{
			NewsArticle: article,
			ReaderMode:  true,
		},
	})
}

func appendUniqueTag(tags []string, tag string) []string {
	for _, t := range tags {
		if t == tag {
			return tags
		}
	}
	return append(tags, tag)
}

func (h *NewsHandler) Reader(w http.ResponseWriter, r *http.Request) {
	if h.reader == nil {
		response.Error(w, http.StatusServiceUnavailable, "reader not configured")
		return
	}
	rawURL := r.URL.Query().Get("url")
	content, err := h.reader.Extract(r.Context(), rawURL)
	if err != nil {
		response.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(w, http.StatusOK, map[string]any{"data": content})
}
