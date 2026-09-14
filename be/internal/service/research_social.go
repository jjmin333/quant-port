package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// Social layer for 리서치 콜 (docs/RESEARCH_CALL_DESIGN.md §10).

const researchCommentMaxLen = 2000

var (
	ErrResearchRatingClosed = errors.New("판정이 끝난 콜은 평가할 수 없습니다")
	ErrResearchSelfRating   = errors.New("자기 콜은 평가할 수 없습니다")
)

// ToggleReaction flips a like/bookmark and returns whether it is now on.
func (s *ResearchService) ToggleReaction(
	ctx context.Context, id int64, kind string,
) (bool, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return false, err
	}
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return false, err
	}
	if !researchReadable(call, userID) {
		return false, repository.ErrResearchNotFound
	}
	return s.repo.ToggleReaction(ctx, id, userID, normalizeReactionKind(kind))
}

func normalizeReactionKind(v string) model.ResearchReactionKind {
	if model.ResearchReactionKind(strings.ToLower(strings.TrimSpace(v))) == model.ResearchBookmark {
		return model.ResearchBookmark
	}
	return model.ReactionLike
}

// ---------------------------------------------------------------- comments

func (s *ResearchService) Comments(ctx context.Context, id int64) ([]model.ResearchComment, error) {
	viewerID, _ := authctx.UserID(ctx)
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if !researchReadable(call, viewerID) {
		return nil, repository.ErrResearchNotFound
	}
	return s.repo.Comments(ctx, id, viewerID)
}

func (s *ResearchService) AddComment(
	ctx context.Context, id int64, parentID *int64, body string,
) (model.ResearchComment, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchComment{}, err
	}
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return model.ResearchComment{}, err
	}
	if !researchReadable(call, userID) {
		return model.ResearchComment{}, repository.ErrResearchNotFound
	}
	body = truncateRunes(strings.TrimSpace(body), researchCommentMaxLen)
	if body == "" {
		return model.ResearchComment{}, errors.New("comment body required")
	}
	return s.repo.AddComment(ctx, id, userID, parentID, body)
}

func (s *ResearchService) UpdateComment(
	ctx context.Context, commentID int64, body string,
) (model.ResearchComment, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return model.ResearchComment{}, err
	}
	body = truncateRunes(strings.TrimSpace(body), researchCommentMaxLen)
	if body == "" {
		return model.ResearchComment{}, errors.New("comment body required")
	}
	return s.repo.UpdateComment(ctx, commentID, userID, body)
}

func (s *ResearchService) DeleteComment(ctx context.Context, commentID int64) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	return s.repo.DeleteComment(ctx, commentID, userID)
}

/*
ToggleCommentLike 은 댓글 추천을 켜고 끈다 (2026-09-14 사용자 요청 3).

# 왜 여기서 콜의 공개 범위를 다시 안 보나

글 추천(`ToggleReaction`)은 콜을 먼저 읽어 `researchReadable`을 확인한다. 여기서는
그 검사가 **댓글 자체에** 들어 있다 — 댓글 목록은 같은 검사를 통과해야만 나오므로
(`Comments`), 비공개 콜의 댓글 id를 알아낼 길이 애초에 없다. 그리고 id 하나로는
아무것도 드러나지 않는다: 추천은 성공해도 그 글의 어떤 내용도 돌려주지 않는다.

리포지토리가 지워진 댓글을 거부하므로 「삭제된 댓글입니다」에 추천이 붙지 않는다.
*/
func (s *ResearchService) ToggleCommentLike(ctx context.Context, commentID int64) (bool, error) {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return false, err
	}
	return s.repo.ToggleCommentLike(ctx, commentID, userID)
}

// ---------------------------------------------------------------- ratings

// Rate records a reader's 3-axis score of the *argument*.
//
// It is refused once the verdict is in. Rating a settled call is not judging
// the reasoning any more — it is re-scoring the outcome, which the alpha
// numbers already do, better.
func (s *ResearchService) Rate(ctx context.Context, id int64, clarity, evidence, risk int) error {
	userID, err := authctx.RequireUserID(ctx)
	if err != nil {
		return err
	}
	call, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if !researchReadable(call, userID) {
		return repository.ErrResearchNotFound
	}
	if call.UserID == userID {
		return ErrResearchSelfRating
	}
	if !researchRatable(call) {
		return ErrResearchRatingClosed
	}
	return s.repo.UpsertRating(ctx, model.ResearchRating{
		CallID:   id,
		UserID:   userID,
		Clarity:  clampRating(clarity),
		Evidence: clampRating(evidence),
		Risk:     clampRating(risk),
	})
}

// researchRatable is true only while the call is still running.
func researchRatable(c model.ResearchCall) bool {
	return c.Status == model.StatusOpen
}

func clampRating(v int) int {
	if v < 1 {
		return 1
	}
	if v > 5 {
		return 5
	}
	return v
}

// ---------------------------------------------------------------- views

// ViewerKey identifies a reader for view de-duplication without storing
// anything that identifies them: logged-in readers key on their id, anonymous
// ones on a hash of IP + user agent. Neither value is recoverable from the row.
func ViewerKey(userID int64, remoteAddr, userAgent string) string {
	if userID > 0 {
		sum := sha256.Sum256([]byte("u:" + strconv.FormatInt(userID, 10)))
		return hex.EncodeToString(sum[:])
	}
	host := remoteAddr
	if idx := strings.LastIndex(host, ":"); idx > 0 {
		host = host[:idx]
	}
	sum := sha256.Sum256([]byte("a:" + host + "|" + userAgent))
	return hex.EncodeToString(sum[:])
}

// RecordView bumps a call's counter, at most once per viewer per 30 minutes.
func (s *ResearchService) RecordView(ctx context.Context, id int64, viewerKey string) error {
	if viewerKey == "" {
		return nil
	}
	_, err := s.repo.RecordView(ctx, id, viewerKey)
	return err
}

// ---------------------------------------------------------------- decoration

// attachSocial fills the social half of a batch of views in a fixed number of
// queries, regardless of how many calls are on screen.
func (s *ResearchService) attachSocial(
	ctx context.Context, viewerID int64, views []model.ResearchView,
) {
	if len(views) == 0 {
		return
	}
	ids := make([]int64, 0, len(views))
	for _, v := range views {
		ids = append(ids, v.ID)
	}
	counts, err := s.repo.CountsFor(ctx, ids)
	if err != nil {
		return
	}
	mine, _ := s.repo.MyReactions(ctx, viewerID, ids)

	for i := range views {
		c := counts[views[i].ID]
		views[i].LikeCount = c.Likes
		views[i].BookmarkCount = c.Bookmarks
		views[i].CommentCount = c.Comments
		if r := mine[views[i].ID]; r != nil {
			views[i].Liked = r[string(model.ReactionLike)]
			views[i].Bookmarked = r[string(model.ResearchBookmark)]
		}
		views[i].CanRate = viewerID > 0 &&
			viewerID != views[i].UserID &&
			researchRatable(views[i].ResearchCall)
	}
}

// attachRating adds the rating summary. Only the detail view needs it, so it is
// separate from attachSocial rather than firing on every feed row.
func (s *ResearchService) attachRating(ctx context.Context, viewerID int64, view *model.ResearchView) {
	if summary, err := s.repo.RatingSummary(ctx, view.ID); err == nil {
		view.Rating = summary
	}
	if mine, err := s.repo.MyRating(ctx, view.ID, viewerID); err == nil {
		view.MyRating = mine
	}
}
