package model

import (
	"strings"
	"time"
)

// 커뮤니티 (2026-09-27 사용자 요청 4) — migrations/088_community.sql.

const (
	CommunityStatusVisible = "visible"
	CommunityStatusHidden  = "hidden"
	CommunityStatusDeleted = "deleted"

	CommunityTargetPost    = "post"
	CommunityTargetComment = "comment"
)

// CommunityBadge is one badge next to a nickname. Badges describe **activity and
// habits only** — no hit rates, ranks or rewards (2026-08-10 리포트 방침을 커뮤니티에도).
type CommunityBadge struct {
	Key   string `json:"key"`
	Emoji string `json:"emoji"`
	Label string `json:"label"`
	// Symbol is set for the 「주주」 badge — the tagged symbol the author holds.
	Symbol string `json:"symbol,omitempty"`
}

type CommunityAuthor struct {
	ID          int64            `json:"id"`
	DisplayName string           `json:"displayName"`
	Badges      []CommunityBadge `json:"badges"`
}

// CommunitySymbol is an attached stock: the value frozen at posting time plus
// the value at read time.
type CommunitySymbol struct {
	Symbol    string `json:"symbol"`
	Name      string `json:"name"`
	AssetType string `json:"assetType,omitempty"`

	SnapPrice      *float64  `json:"snapPrice,omitempty"`
	SnapCurrency   string    `json:"snapCurrency,omitempty"`
	SnapChangeRate *float64  `json:"snapChangeRate,omitempty"`
	SnapScore      *int      `json:"snapScore,omitempty"`
	SnapAt         time.Time `json:"snapAt"`

	// Read-time values (detail view). Price is left to the client's live quotes.
	Score      *int   `json:"score,omitempty"`
	ScoreGrade string `json:"scoreGrade,omitempty"`
	AIOverview string `json:"aiOverview,omitempty"`
	AIAt       string `json:"aiAt,omitempty"`
}

type CommunityArticle struct {
	ArticleID   string     `json:"articleId"`
	Title       string     `json:"title"`
	Source      string     `json:"source"`
	URL         string     `json:"url,omitempty"`
	PublishedAt *time.Time `json:"publishedAt,omitempty"`
}

type CommunityImage struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type CommunityPost struct {
	ID           int64              `json:"id"`
	Author       CommunityAuthor    `json:"author"`
	Title        string             `json:"title"`
	Topic        string             `json:"topic"`
	Body         string             `json:"body,omitempty"`
	Excerpt      string             `json:"excerpt,omitempty"`
	Status       string             `json:"status"`
	HiddenReason string             `json:"hiddenReason,omitempty"`
	LikeCount    int                `json:"likeCount"`
	CommentCount int                `json:"commentCount"`
	ViewCount    int                `json:"viewCount"`
	ReportCount  int                `json:"reportCount,omitempty"`
	Liked        bool               `json:"liked"`
	Mine         bool               `json:"mine"`
	Popular      bool               `json:"popular"`
	CreatedAt    time.Time          `json:"createdAt"`
	EditedAt     *time.Time         `json:"editedAt,omitempty"`
	Symbols      []CommunitySymbol  `json:"symbols"`
	Articles     []CommunityArticle `json:"articles"`
	Images       []CommunityImage   `json:"images"`
	// Mentions — 본문의 데이터 태그(상세에서만, 2026-09-28).
	Mentions *MentionSet `json:"mentions,omitempty"`
	// ImageCount / ArticleCount are set on list rows, which carry only the first image.
	ImageCount   int `json:"imageCount"`
	ArticleCount int `json:"articleCount"`
}

type CommunityComment struct {
	ID          int64           `json:"id"`
	PostID      int64           `json:"postId"`
	ParentID    *int64          `json:"parentId,omitempty"`
	Author      CommunityAuthor `json:"author"`
	Body        string          `json:"body"`
	Status      string          `json:"status"`
	LikeCount   int             `json:"likeCount"`
	ReportCount int             `json:"reportCount,omitempty"`
	Liked       bool            `json:"liked"`
	Mine        bool            `json:"mine"`
	CreatedAt   time.Time       `json:"createdAt"`
}

type CommunityPostPage struct {
	Items      []CommunityPost `json:"items"`
	NextCursor string          `json:"nextCursor,omitempty"`
}

// CommunityBadgeCatalog — 서비스에 있는 뱃지 전부(「주주」는 종목마다 달라 뺐다). 활동으로 붙는 뱃지(service.CommunityBadges)와
// 관리자가 게시 계정에 고르는 뱃지(2026-10-19)가 이 한 목록을 쓴다 — 이름·이모지가 두 곳에서 갈라지지 않게.
var CommunityBadgeCatalog = []CommunityBadge{
	{Key: "admin", Emoji: "🛡", Label: "운영자"},
	{Key: "liked", Emoji: "❤", Label: "인기 글쓴이"},
	{Key: "writer", Emoji: "✍", Label: "글쟁이"},
	{Key: "talker", Emoji: "💬", Label: "수다쟁이"},
	{Key: "reader", Emoji: "🔍", Label: "눈팅 장인"},
	{Key: "forecaster", Emoji: "🎯", Label: "예측 기록가"},
	{Key: "journaler", Emoji: "📝", Label: "메모 습관"},
	{Key: "newbie", Emoji: "🐣", Label: "새내기"},
}

// CommunityBadgeByKey — 목록에서 key의 뱃지. 없으면 false.
func CommunityBadgeByKey(key string) (CommunityBadge, bool) {
	for _, b := range CommunityBadgeCatalog {
		if b.Key == key {
			return b, true
		}
	}
	return CommunityBadge{}, false
}

// PersonaBadges — users.badge(쉼표로 이은 키)를 뱃지로. 모르는 키는 버린다.
func PersonaBadges(csv string) []CommunityBadge {
	out := []CommunityBadge{}
	for _, k := range strings.Split(csv, ",") {
		if b, ok := CommunityBadgeByKey(strings.TrimSpace(k)); ok {
			out = append(out, b)
		}
	}
	return out
}

// PersonaBadgeText — 리포트 작성자 칸처럼 칩 하나에 담는 글자(「✍ 글쟁이 · ❤ 인기 글쓴이」).
func PersonaBadgeText(csv string) string {
	parts := []string{}
	for _, b := range PersonaBadges(csv) {
		parts = append(parts, b.Emoji+" "+b.Label)
	}
	return strings.Join(parts, " · ")
}
