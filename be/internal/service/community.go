package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

/*
커뮤니티 (2026-09-27 사용자 요청 4 · 결정: docs/mockups/community-ai-v1.html).

이 커뮤니티가 다른 주식 게시판과 다른 점은 **글이 우리 데이터를 들고 다닌다**는 것이다.
종목을 붙이면 그 순간의 가격·등락·점수를 스냅샷으로 얼리고, 읽을 때는 지금 점수와 저장된
AI 요약을 곁들인다(LLM 호출 0 — 배치가 써 둔 글만 읽는다). 기사를 붙이면 제목·출처를 복사해
둔다.

지키는 선:
  - 뱃지는 **활동·습관**만 — 적중률·순위·보상은 없다(리포트 방침 2026-08-10을 그대로).
  - 신고 3명 → 관리자 확인 전까지 숨김. 욕설·리딩방 유도 문구는 쓰기 단계에서 거절.
  - 글 1분에 1개, 댓글 10초에 1개 — 도배를 서버가 막는다.
*/

const (
	communityTitleMin    = 2
	communityTitleMax    = 100
	communityBodyMax     = 5000
	communityCommentMax  = 1000
	communityReasonMax   = 200
	communityMaxSymbols  = 3
	communityMaxArticles = 3
	communityMaxImages   = 4
	communityPageSize    = 20
	communityPageMax     = 50
	communityExcerptMax  = 140

	communityPostCooldown    = time.Minute
	communityCommentCooldown = 10 * time.Second
	communityPopularWindow   = 7 * 24 * time.Hour
	// communityPopularLabel — 목록의 「인기」 표시 문턱(좋아요 3개 + 댓글 2개쯤).
	communityPopularLabel = 12
)

// communityImagePathPrefix — 붙일 수 있는 이미지는 우리 업로드뿐이다. 외부 주소를 받으면
// 추적 픽셀이나 사라지는 이미지가 글에 박힌다.
const communityImagePathPrefix = "/api/v1/media/"

// CommunityCooldownError tells the client how long to wait.
type CommunityCooldownError struct {
	What             string
	RemainingSeconds int
}

func (e *CommunityCooldownError) Error() string {
	return fmt.Sprintf("%s: wait %ds", e.What, e.RemainingSeconds)
}

var ErrCommunityBlockedWords = errors.New("community: blocked words")

type CommunityService struct {
	repo     *repository.CommunityRepo
	quotes   *QuotesService
	scores   *ScoreService
	mentions *MentionService
	now    func() time.Time
}

func NewCommunityService(repo *repository.CommunityRepo) *CommunityService {
	return &CommunityService{repo: repo, now: time.Now}
}

func (s *CommunityService) SetQuotes(q *QuotesService) { s.quotes = q }
func (s *CommunityService) SetScores(sc *ScoreService) { s.scores = sc }
func (s *CommunityService) SetMentions(m *MentionService) { s.mentions = m }

// AIOverviews exposes the stored AI line reader for the mention cards.
func (s *CommunityService) AIOverviews(ctx context.Context, symbols []string) (map[string]repository.CommunityAIOverview, error) {
	return s.repo.AIOverviews(ctx, symbols)
}

// ------------------------------------------------------------------- write

type CommunityCreateRequest struct {
	Title      string                 `json:"title"`
	Topic      string                 `json:"topic"`
	Body       string                 `json:"body"`
	Symbols    []string               `json:"symbols"`
	ArticleIDs []string               `json:"articleIds"`
	Images     []model.CommunityImage `json:"images"`
}

func (s *CommunityService) CreatePost(ctx context.Context, req CommunityCreateRequest) (model.CommunityPost, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.CommunityPost{}, err
	}
	title, body, err := validateCommunityText(req.Title, req.Body)
	if err != nil {
		return model.CommunityPost{}, err
	}
	topic, ok := model.ParseTopic(req.Topic)
	if !ok {
		return model.CommunityPost{}, invalidInput("알 수 없는 주제입니다")
	}
	if last, ok, err := s.repo.LastPostAt(ctx, userID); err != nil {
		return model.CommunityPost{}, err
	} else if ok {
		if wait := communityCooldownLeft(s.now(), last, communityPostCooldown); wait > 0 {
			return model.CommunityPost{}, &CommunityCooldownError{What: "post", RemainingSeconds: wait}
		}
	}

	symbols := normalizeCommunitySymbols(req.Symbols)
	if len(symbols) > communityMaxSymbols {
		return model.CommunityPost{}, invalidInput("종목은 %d개까지 붙일 수 있습니다", communityMaxSymbols)
	}
	known, err := s.repo.KnownSymbols(ctx, symbols)
	if err != nil {
		return model.CommunityPost{}, err
	}
	for _, sym := range symbols {
		if !known[sym] {
			return model.CommunityPost{}, invalidInput("알 수 없는 종목입니다: %s", sym)
		}
	}

	articleIDs := dedupeStrings(req.ArticleIDs)
	if len(articleIDs) > communityMaxArticles {
		return model.CommunityPost{}, invalidInput("기사는 %d개까지 붙일 수 있습니다", communityMaxArticles)
	}
	found, err := s.repo.ArticlesByIDs(ctx, articleIDs)
	if err != nil {
		return model.CommunityPost{}, err
	}
	articles := make([]model.CommunityArticle, 0, len(articleIDs))
	for _, id := range articleIDs {
		a, ok := found[id]
		if !ok {
			return model.CommunityPost{}, invalidInput("찾을 수 없는 기사입니다")
		}
		articles = append(articles, a)
	}

	images, err := validateCommunityImages(req.Images)
	if err != nil {
		return model.CommunityPost{}, err
	}
	if err := s.mentions.Check(ctx, body); err != nil {
		return model.CommunityPost{}, err
	}

	id, err := s.repo.CreatePost(ctx, repository.CommunityNewPost{
		UserID:   userID,
		Title:    title,
		Topic:    topic,
		Body:     body,
		Symbols:  s.snapshotSymbols(ctx, symbols),
		Articles: articles,
		Images:   images,
	})
	if err != nil {
		return model.CommunityPost{}, err
	}
	s.mentions.SyncLogged(ctx, model.MentionTargetCommunityPost, id, body)
	return s.GetPost(ctx, id)
}

// snapshotSymbols freezes price·change·score at posting time. 시세를 못 받으면 그 칸만 비운다 —
// 스냅샷이 없다고 글을 거절하면 장 마감·점검 시간에 글을 못 쓴다.
func (s *CommunityService) snapshotSymbols(ctx context.Context, symbols []string) []repository.CommunityNewSymbol {
	out := make([]repository.CommunityNewSymbol, 0, len(symbols))
	scores := map[string]int{}
	if s.scores != nil && len(symbols) > 0 {
		if views, err := s.scores.GetMany(ctx, symbols); err == nil {
			for _, v := range views {
				if v.Score != nil {
					scores[strings.ToUpper(v.Symbol)] = *v.Score
				}
			}
		}
	}
	for _, sym := range symbols {
		ns := repository.CommunityNewSymbol{Symbol: sym}
		if s.quotes != nil {
			if q, ok := s.quotes.GetQuote(ctx, sym); ok && q.CurrentPrice > 0 {
				price, rate := q.CurrentPrice, q.ChangeRate
				ns.Price, ns.ChangeRate, ns.Currency = &price, &rate, string(q.Currency)
			}
		}
		if sc, ok := scores[sym]; ok {
			v := sc
			ns.Score = &v
		}
		out = append(out, ns)
	}
	return out
}

// rawTopic이 빈 값이면 주제를 바꾸지 않는다(옛 클라이언트).
func (s *CommunityService) UpdatePost(ctx context.Context, id int64, rawTitle, rawTopic, rawBody string) (model.CommunityPost, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.CommunityPost{}, err
	}
	title, body, err := validateCommunityText(rawTitle, rawBody)
	if err != nil {
		return model.CommunityPost{}, err
	}
	topic := rawTopic
	if topic == "" {
		cur, err := s.repo.GetPost(ctx, id)
		if err != nil {
			return model.CommunityPost{}, err
		}
		topic = cur.Topic
	} else if _, ok := model.ParseTopic(topic); !ok {
		return model.CommunityPost{}, invalidInput("알 수 없는 주제입니다")
	}
	if err := s.mentions.Check(ctx, body); err != nil {
		return model.CommunityPost{}, err
	}
	if err := s.repo.UpdatePost(ctx, id, userID, title, topic, body, authctx.IsAdmin(ctx)); err != nil {
		return model.CommunityPost{}, err
	}
	s.mentions.SyncLogged(ctx, model.MentionTargetCommunityPost, id, body)
	return s.GetPost(ctx, id)
}

func (s *CommunityService) DeletePost(ctx context.Context, id int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.DeletePost(ctx, id, userID, authctx.IsAdmin(ctx))
}

func (s *CommunityService) ToggleLike(ctx context.Context, targetType string, id int64) (bool, int, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return false, 0, err
	}
	return s.repo.ToggleLike(ctx, targetType, id, userID)
}

func (s *CommunityService) RecordView(ctx context.Context, postID int64, viewerKey string) error {
	if viewerKey == "" {
		return nil
	}
	userID, _ := authctx.UserID(ctx)
	return s.repo.RecordView(ctx, postID, viewerKey, userID)
}

func (s *CommunityService) AddComment(ctx context.Context, postID int64, parentID *int64, raw string) (model.CommunityComment, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.CommunityComment{}, err
	}
	body := strings.TrimSpace(raw)
	if body == "" {
		return model.CommunityComment{}, invalidInput("댓글 내용을 입력해 주세요")
	}
	if utf8.RuneCountInString(body) > communityCommentMax {
		return model.CommunityComment{}, invalidInput("댓글은 %d자까지 쓸 수 있습니다", communityCommentMax)
	}
	if CommunityBlockedPhrase(body) != "" {
		return model.CommunityComment{}, ErrCommunityBlockedWords
	}
	if last, ok, err := s.repo.LastCommentAt(ctx, userID); err != nil {
		return model.CommunityComment{}, err
	} else if ok {
		if wait := communityCooldownLeft(s.now(), last, communityCommentCooldown); wait > 0 {
			return model.CommunityComment{}, &CommunityCooldownError{What: "comment", RemainingSeconds: wait}
		}
	}
	id, err := s.repo.AddComment(ctx, postID, parentID, userID, body)
	if err != nil {
		return model.CommunityComment{}, err
	}
	comments, err := s.Comments(ctx, postID)
	if err != nil {
		return model.CommunityComment{}, err
	}
	for _, c := range comments {
		if c.ID == id {
			return c, nil
		}
	}
	return model.CommunityComment{ID: id, PostID: postID, Body: body, Status: model.CommunityStatusVisible, Mine: true}, nil
}

func (s *CommunityService) DeleteComment(ctx context.Context, id int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.DeleteComment(ctx, id, userID, authctx.IsAdmin(ctx))
}

func (s *CommunityService) Report(ctx context.Context, targetType string, id int64, reason string) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	reason = truncateRunes(strings.TrimSpace(reason), communityReasonMax)
	hid, err := s.repo.Report(ctx, targetType, id, userID, reason)
	if err != nil {
		return err
	}
	if hid {
		log.Printf("community: %s %d auto-hidden after %d reports", targetType, id, repository.CommunityAutoHideReports)
	}
	return nil
}

// SetStatus — 관리자 숨김·복구.
func (s *CommunityService) SetStatus(ctx context.Context, targetType string, id int64, status string) error {
	return s.repo.SetStatus(ctx, targetType, id, status)
}

// -------------------------------------------------------------------- read

// GetPost returns one post decorated for the viewer. Hidden posts are visible to
// their author (and admins) only; deleted ones to nobody.
func (s *CommunityService) GetPost(ctx context.Context, id int64) (model.CommunityPost, error) {
	viewerID, _ := authctx.UserID(ctx)
	row, err := s.repo.GetPost(ctx, id)
	if err != nil {
		return model.CommunityPost{}, err
	}
	if !communityReadable(row.Status, row.UserID, viewerID, false) {
		return model.CommunityPost{}, repository.ErrCommunityNotFound
	}
	rows := []repository.CommunityPostRow{row}
	if err := s.decoratePosts(ctx, rows, viewerID, false); err != nil {
		return model.CommunityPost{}, err
	}
	post := rows[0].CommunityPost
	s.attachLiveSymbolData(ctx, post.Symbols)
	post.Mentions = s.mentions.Load(ctx, model.MentionTargetCommunityPost, post.ID, post.Body)
	return post, nil
}

type CommunityListRequest struct {
	Sort   string
	Symbol string
	Topic  string
	Mine   bool
	Cursor string
	Limit  int
}

func (s *CommunityService) ListPosts(ctx context.Context, req CommunityListRequest) (model.CommunityPostPage, error) {
	viewerID, _ := authctx.UserID(ctx)
	limit := req.Limit
	if limit <= 0 {
		limit = communityPageSize
	}
	if limit > communityPageMax {
		limit = communityPageMax
	}
	q := repository.CommunityListQuery{
		Sort:         req.Sort,
		Symbol:       strings.ToUpper(strings.TrimSpace(req.Symbol)),
		Topic:        req.Topic,
		Limit:        limit + 1,
		PopularSince: s.now().Add(-communityPopularWindow),
	}
	if req.Mine {
		if viewerID <= 0 {
			return model.CommunityPostPage{}, authctx.ErrNoUser
		}
		q.AuthorID = viewerID
	}
	before, offset := parseCommunityCursor(req.Cursor)
	q.BeforeID, q.Offset = before, offset

	rows, err := s.repo.ListPosts(ctx, q)
	if err != nil {
		return model.CommunityPostPage{}, err
	}
	page := model.CommunityPostPage{Items: []model.CommunityPost{}}
	more := len(rows) > limit
	if more {
		rows = rows[:limit]
	}
	if err := s.decoratePosts(ctx, rows, viewerID, true); err != nil {
		return model.CommunityPostPage{}, err
	}
	for _, r := range rows {
		p := r.CommunityPost
		p.Excerpt = CommunityExcerpt(p.Body, communityExcerptMax)
		p.Body = ""
		page.Items = append(page.Items, p)
	}
	if more && len(rows) > 0 {
		if q.Sort == "popular" && q.AuthorID == 0 {
			page.NextCursor = "o" + strconv.Itoa(offset+limit)
		} else {
			page.NextCursor = "b" + strconv.FormatInt(rows[len(rows)-1].ID, 10)
		}
	}
	return page, nil
}

// Comments returns the thread. Removed comments that still have replies stay as
// tombstones (본문·작성자 비움) so the replies keep their place.
func (s *CommunityService) Comments(ctx context.Context, postID int64) ([]model.CommunityComment, error) {
	viewerID, _ := authctx.UserID(ctx)
	post, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	if !communityReadable(post.Status, post.UserID, viewerID, false) {
		return nil, repository.ErrCommunityNotFound
	}
	rows, err := s.repo.Comments(ctx, postID)
	if err != nil {
		return nil, err
	}
	hasReplies := map[int64]bool{}
	for _, c := range rows {
		if c.ParentID != nil && c.Status == model.CommunityStatusVisible {
			hasReplies[*c.ParentID] = true
		}
	}
	kept := rows[:0]
	for _, c := range rows {
		if c.Status != model.CommunityStatusVisible && !hasReplies[c.ID] {
			continue
		}
		kept = append(kept, c)
	}
	rows = kept

	userIDs := []int64{}
	ids := []int64{}
	for _, c := range rows {
		userIDs = append(userIDs, c.UserID)
		ids = append(ids, c.ID)
	}
	postSymbols := []string{}
	if syms, err := s.postSymbols(ctx, postID); err == nil {
		postSymbols = syms
	}
	authors, err := s.authors(ctx, dedupeInt64(userIDs), map[int64][]string{0: postSymbols}, true)
	if err != nil {
		return nil, err
	}
	liked, err := s.repo.LikedBy(ctx, model.CommunityTargetComment, ids, viewerID)
	if err != nil {
		return nil, err
	}
	out := make([]model.CommunityComment, 0, len(rows))
	for _, c := range rows {
		cm := c.CommunityComment
		cm.Author = authors[c.UserID]
		cm.Liked = liked[c.ID]
		cm.Mine = viewerID > 0 && viewerID == c.UserID
		cm.ReportCount = 0
		if c.Status != model.CommunityStatusVisible {
			cm.Body = ""
			cm.Author = model.CommunityAuthor{Badges: []model.CommunityBadge{}}
			cm.Mine = false
		}
		out = append(out, cm)
	}
	return out, nil
}

// Moderation — 관리자 화면: 신고됐거나 숨겨진 글·댓글.
func (s *CommunityService) Moderation(ctx context.Context) (map[string]any, error) {
	posts, err := s.repo.ModerationPosts(ctx, 100)
	if err != nil {
		return nil, err
	}
	comments, err := s.repo.ModerationComments(ctx, 100)
	if err != nil {
		return nil, err
	}
	userIDs := []int64{}
	for _, p := range posts {
		userIDs = append(userIDs, p.UserID)
	}
	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	authors, err := s.authors(ctx, dedupeInt64(userIDs), nil, false)
	if err != nil {
		return nil, err
	}
	outPosts := make([]model.CommunityPost, 0, len(posts))
	for _, p := range posts {
		v := p.CommunityPost
		v.Author = authors[p.UserID]
		v.Symbols, v.Articles, v.Images = []model.CommunitySymbol{}, []model.CommunityArticle{}, []model.CommunityImage{}
		outPosts = append(outPosts, v)
	}
	outComments := make([]model.CommunityComment, 0, len(comments))
	for _, c := range comments {
		v := c.CommunityComment
		v.Author = authors[c.UserID]
		outComments = append(outComments, v)
	}
	return map[string]any{"posts": outPosts, "comments": outComments}, nil
}

func (s *CommunityService) SitemapPosts(ctx context.Context) ([]repository.CommunitySitemapRow, error) {
	return s.repo.SitemapPosts(ctx, 1000)
}

// ------------------------------------------------------------- decoration

func (s *CommunityService) decoratePosts(ctx context.Context, rows []repository.CommunityPostRow, viewerID int64, list bool) error {
	if len(rows) == 0 {
		return nil
	}
	if err := s.repo.AttachSymbols(ctx, rows); err != nil {
		return err
	}
	if err := s.repo.AttachArticles(ctx, rows); err != nil {
		return err
	}
	if err := s.repo.AttachImages(ctx, rows, list); err != nil {
		return err
	}
	ids := make([]int64, 0, len(rows))
	userIDs := make([]int64, 0, len(rows))
	symbolsByPost := map[int64][]string{}
	for _, r := range rows {
		ids = append(ids, r.ID)
		userIDs = append(userIDs, r.UserID)
		for _, sym := range r.Symbols {
			symbolsByPost[r.UserID] = append(symbolsByPost[r.UserID], sym.Symbol)
		}
	}
	authors, err := s.authors(ctx, dedupeInt64(userIDs), symbolsByPost, false)
	if err != nil {
		return err
	}
	liked, err := s.repo.LikedBy(ctx, model.CommunityTargetPost, ids, viewerID)
	if err != nil {
		return err
	}
	for i := range rows {
		p := &rows[i].CommunityPost
		// 「주주」 뱃지는 **이 글에 붙인 종목**을 들고 있을 때만 — 다른 글의 종목으로 달면 거짓이다.
		p.Author = communityAuthorForPost(authors[rows[i].UserID], p.Symbols)
		p.Liked = liked[p.ID]
		p.Mine = viewerID > 0 && viewerID == rows[i].UserID
		p.Popular = repository.CommunityHotScore(p.LikeCount, p.CommentCount, p.ViewCount) >= communityPopularLabel
		if !p.Mine {
			p.ReportCount = 0
		}
		if p.Symbols == nil {
			p.Symbols = []model.CommunitySymbol{}
		}
		if p.Articles == nil {
			p.Articles = []model.CommunityArticle{}
		}
		if p.Images == nil {
			p.Images = []model.CommunityImage{}
		}
	}
	return nil
}

// authors builds each author with the full badge set. heldFor maps userID →
// candidate symbols for the 「주주」 badge (key 0 = same list for everyone).
func (s *CommunityService) authors(
	ctx context.Context, userIDs []int64, heldFor map[int64][]string, sameForAll bool,
) (map[int64]model.CommunityAuthor, error) {
	out := map[int64]model.CommunityAuthor{}
	if len(userIDs) == 0 {
		return out, nil
	}
	stats, err := s.repo.AuthorStats(ctx, userIDs)
	if err != nil {
		return nil, err
	}
	candidates := map[string]bool{}
	for _, syms := range heldFor {
		for _, sym := range syms {
			candidates[sym] = true
		}
	}
	symList := make([]string, 0, len(candidates))
	for sym := range candidates {
		symList = append(symList, sym)
	}
	held, err := s.repo.HeldSymbols(ctx, userIDs, symList)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for _, id := range userIDs {
		st := stats[id]
		if st == nil {
			st = &repository.CommunityAuthorStats{}
		}
		var heldSyms []string
		want := heldFor[id]
		if sameForAll {
			want = heldFor[0]
		}
		for _, sym := range want {
			if held[id][sym] {
				heldSyms = append(heldSyms, sym)
			}
		}
		out[id] = model.CommunityAuthor{
			ID:          id,
			DisplayName: st.DisplayName,
			Badges:      CommunityBadges(st, dedupeStrings(heldSyms), now),
		}
	}
	return out, nil
}

// communityAuthorForPost keeps only the 「주주」 badges for symbols on this post.
func communityAuthorForPost(a model.CommunityAuthor, symbols []model.CommunitySymbol) model.CommunityAuthor {
	on := map[string]bool{}
	for _, s := range symbols {
		on[s.Symbol] = true
	}
	badges := make([]model.CommunityBadge, 0, len(a.Badges))
	for _, b := range a.Badges {
		if b.Key == "holder" && !on[b.Symbol] {
			continue
		}
		badges = append(badges, b)
	}
	a.Badges = badges
	return a
}

func (s *CommunityService) postSymbols(ctx context.Context, postID int64) ([]string, error) {
	row, err := s.repo.GetPost(ctx, postID)
	if err != nil {
		return nil, err
	}
	rows := []repository.CommunityPostRow{row}
	if err := s.repo.AttachSymbols(ctx, rows); err != nil {
		return nil, err
	}
	out := []string{}
	for _, sym := range rows[0].Symbols {
		out = append(out, sym.Symbol)
	}
	return out, nil
}

// attachLiveSymbolData adds today's score and the stored AI overview (detail view).
func (s *CommunityService) attachLiveSymbolData(ctx context.Context, symbols []model.CommunitySymbol) {
	if len(symbols) == 0 {
		return
	}
	list := make([]string, 0, len(symbols))
	for _, sym := range symbols {
		list = append(list, sym.Symbol)
	}
	if s.scores != nil {
		if views, err := s.scores.GetMany(ctx, list); err == nil {
			byID := map[string]model.StockScoreView{}
			for _, v := range views {
				byID[strings.ToUpper(v.Symbol)] = v
			}
			for i := range symbols {
				if v, ok := byID[symbols[i].Symbol]; ok && v.Score != nil {
					sc := *v.Score
					symbols[i].Score = &sc
					symbols[i].ScoreGrade = v.Grade
				}
			}
		}
	}
	if ai, err := s.repo.AIOverviews(ctx, list); err == nil {
		for i := range symbols {
			if o, ok := ai[symbols[i].Symbol]; ok {
				symbols[i].AIOverview, symbols[i].AIAt = o.Text, o.At
			}
		}
	}
}

// -------------------------------------------------------------- pure rules

// CommunityBadges derives badges from activity. Order is display priority.
func CommunityBadges(st *repository.CommunityAuthorStats, heldSymbols []string, now time.Time) []model.CommunityBadge {
	out := []model.CommunityBadge{}
	if st == nil {
		return out
	}
	if st.Role == model.RoleAdmin {
		out = append(out, model.CommunityBadge{Key: "admin", Emoji: "🛡", Label: "운영자"})
	}
	for _, sym := range heldSymbols {
		out = append(out, model.CommunityBadge{Key: "holder", Emoji: "🌱", Label: "주주", Symbol: sym})
	}
	activity := false
	add := func(ok bool, key, emoji, label string) {
		if ok {
			out = append(out, model.CommunityBadge{Key: key, Emoji: emoji, Label: label})
			activity = true
		}
	}
	add(st.LikesReceived >= 100, "liked", "❤", "인기 글쓴이")
	add(st.Posts >= 10, "writer", "✍", "글쟁이")
	add(st.Comments >= 50, "talker", "💬", "수다쟁이")
	add(st.Reads >= 500, "reader", "🔍", "눈팅 장인")
	add(st.JudgedStamps >= 10, "forecaster", "🎯", "예측 기록가")
	add(st.Notes >= 30, "journaler", "📝", "메모 습관")
	if !activity && !st.JoinedAt.IsZero() && now.Sub(st.JoinedAt) < 14*24*time.Hour {
		out = append(out, model.CommunityBadge{Key: "newbie", Emoji: "🐣", Label: "새내기"})
	}
	return out
}

func communityReadable(status string, ownerID, viewerID int64, admin bool) bool {
	switch status {
	case model.CommunityStatusVisible:
		return true
	case model.CommunityStatusHidden:
		return admin || (viewerID > 0 && viewerID == ownerID)
	}
	return false
}

func validateCommunityText(rawTitle, rawBody string) (string, string, error) {
	title := strings.Join(strings.Fields(rawTitle), " ")
	body := strings.TrimSpace(strings.ReplaceAll(rawBody, "\r\n", "\n"))
	if n := utf8.RuneCountInString(title); n < communityTitleMin || n > communityTitleMax {
		return "", "", invalidInput("제목은 %d~%d자입니다", communityTitleMin, communityTitleMax)
	}
	if body == "" {
		return "", "", invalidInput("본문을 입력해 주세요")
	}
	if utf8.RuneCountInString(body) > communityBodyMax {
		return "", "", invalidInput("본문은 %d자까지 쓸 수 있습니다", communityBodyMax)
	}
	if CommunityBlockedPhrase(title+"\n"+body) != "" {
		return "", "", ErrCommunityBlockedWords
	}
	return title, body, nil
}

func validateCommunityImages(images []model.CommunityImage) ([]model.CommunityImage, error) {
	if len(images) > communityMaxImages {
		return nil, invalidInput("이미지는 %d장까지 붙일 수 있습니다", communityMaxImages)
	}
	out := make([]model.CommunityImage, 0, len(images))
	for _, img := range images {
		url := strings.TrimSpace(img.URL)
		if !strings.HasPrefix(url, communityImagePathPrefix) || strings.Contains(url, "..") {
			return nil, invalidInput("이미지는 이곳에서 올린 것만 붙일 수 있습니다")
		}
		out = append(out, model.CommunityImage{URL: url, Width: img.Width, Height: img.Height})
	}
	return out, nil
}

var communitySymbolRe = regexp.MustCompile(`^[A-Z0-9.\-]{1,32}$`)

func normalizeCommunitySymbols(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		sym := strings.ToUpper(strings.TrimSpace(s))
		if sym == "" || seen[sym] || !communitySymbolRe.MatchString(sym) {
			continue
		}
		seen[sym] = true
		out = append(out, sym)
	}
	return out
}

func dedupeStrings(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out
}

func dedupeInt64(in []int64) []int64 {
	out := []int64{}
	seen := map[int64]bool{}
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}

func communityCooldownLeft(now, last time.Time, cooldown time.Duration) int {
	elapsed := now.Sub(last)
	if elapsed >= cooldown || elapsed < -time.Minute {
		return 0
	}
	left := int((cooldown - elapsed + time.Second - 1) / time.Second)
	if left < 1 {
		left = 1
	}
	return left
}

// parseCommunityCursor reads "b<id>" (latest) or "o<offset>" (popular).
func parseCommunityCursor(c string) (before int64, offset int) {
	if len(c) < 2 {
		return 0, 0
	}
	switch c[0] {
	case 'b':
		if v, err := strconv.ParseInt(c[1:], 10, 64); err == nil && v > 0 {
			return v, 0
		}
	case 'o':
		if v, err := strconv.Atoi(c[1:]); err == nil && v > 0 && v <= 10000 {
			return 0, v
		}
	}
	return 0, 0
}

// CommunityExcerpt collapses whitespace and cuts to max characters with 「…」.
func CommunityExcerpt(body string, max int) string {
	flat := strings.Join(strings.Fields(plainMentionText(body)), " ")
	r := []rune(flat)
	if len(r) <= max {
		return flat
	}
	return strings.TrimSpace(string(r[:max])) + "…"
}

/*
communityBlockedPhrases — 쓰기 단계에서 거절하는 말.

두 갈래다: 욕설, 그리고 **리딩방·유료 추천 유도**. 주식 게시판의 스팸 대부분은 후자라
「무료 추천주 드립니다 → 오픈채팅」 한 줄이 글 수백 개로 불어난다. 공백·특수문자를 넣어
피하는 흔한 우회(「리 딩 방」)는 비교 전에 공백과 기호를 지워 막는다.

넓게 잡으면 멀쩡한 글이 막힌다 — 「새끼손가락」 같은 오탐이 생기는 낱말(새끼)은 넣지 않았다.
*/
var communityBlockedPhrases = []string{
	// 욕설
	"씨발", "시발", "씨바", "ㅅㅂ", "ㅆㅂ", "병신", "ㅂㅅ", "좆", "개새끼", "지랄", "미친놈", "미친년",
	"엠창", "느금마", "니애미", "니미럴",
	// 리딩방·유료 추천 유도
	"리딩방", "오픈채팅", "오픈톡", "카톡방", "텔레그램방", "무료추천주", "급등주추천", "수익보장", "원금보장",
}

var communityStripRe = regexp.MustCompile(`[\s\p{P}\p{S}]+`)

// CommunityBlockedPhrase returns the matched blocked phrase, or "".
func CommunityBlockedPhrase(text string) string {
	flat := strings.ToLower(communityStripRe.ReplaceAllString(text, ""))
	for _, p := range communityBlockedPhrases {
		if strings.Contains(flat, p) {
			return p
		}
	}
	return ""
}
