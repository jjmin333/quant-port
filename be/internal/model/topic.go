package model

// 글 주제 — 커뮤니티·리포트·메모 공통(2026-10-09 요청 9, migrations/102).
const (
	TopicStock      = "stock"
	TopicRealEstate = "real_estate"
	TopicFinance    = "finance"
)

// ParseTopic — 빈 값은 주식(옛 클라이언트·기존 글). 모르는 값은 false: 쓰기는 400으로 막는다.
func ParseTopic(v string) (string, bool) {
	switch v {
	case "", TopicStock:
		return TopicStock, true
	case TopicRealEstate, TopicFinance:
		return v, true
	}
	return "", false
}

// TopicFilter — 읽기 필터. 빈 값·모르는 값은 전체("")로 본다(쓰기만 거부한다).
func TopicFilter(v string) string {
	if v == "" {
		return ""
	}
	if t, ok := ParseTopic(v); ok {
		return t
	}
	return ""
}
