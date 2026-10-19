package repository

import (
	"context"
	"database/sql"
)

// MigrateCommunityIfNeeded creates the 커뮤니티 tables (088_community.sql) on an
// existing database. 마지막 표(community_reports)를 마커로 본다 — 파일이 전부
// CREATE TABLE IF NOT EXISTS라 중간에 끊겼어도 다시 돌리면 빈 표만 채운다.
func MigrateCommunityIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "community_reports")
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	return applyMigrationFile(ctx, db, migrationsDir, "088_community.sql", "커뮤니티")
}

// MigrateContentMentionsIfNeeded creates content_mentions (092) — 글 안의 데이터 태그.
func MigrateContentMentionsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "content_mentions")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "092_content_mentions.sql", "데이터 태그")
}

// MigrateExternalSignalsIfNeeded creates 093 — 해외 지표 보조·애널리스트·Reddit 언급.
func MigrateExternalSignalsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "social_mentions")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "093_external_signals.sql", "외부 신호")
}

// MigrateSocialRedditIfNeeded creates 094 — Reddit 커뮤니티 탭(ApeWisdom 요약 · 글 · AI 요약).
func MigrateSocialRedditIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "social_summaries")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "094_social_reddit.sql", "Reddit 커뮤니티")
}

// MigrateAnalystHistoryIfNeeded creates 099 — 애널리스트 의견 이력 · 증권사 리포트 목록(2026-10-07).
func MigrateAnalystHistoryIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	exists, err := tableExists(ctx, db, "stock_analyst_reports")
	if err != nil || exists {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "099_analyst_history.sql", "애널리스트 이력")
}

// MigrateRequests1009IfNeeded applies 100 — 변화 시가총액 열(+지난 행 이관) · 뉴스 topic · 자산 칩 ·
// 실적 AI 요약 표 (2026-10-09). 파일 전체가 다시 돌아도 같은 결과라(중복 열·키 무시, NULL만 채움,
// IF NOT EXISTS) 마커 둘 중 하나라도 없으면 통째로 돌린다.
func MigrateRequests1009IfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasCol, err := columnExists(ctx, db, "news_articles", "asset_tags_json")
	if err != nil {
		return err
	}
	hasTable, err := tableExists(ctx, db, "earnings_digests")
	if err != nil || (hasCol && hasTable) {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "100_requests_1009.sql", "변화 시총 · 뉴스 카테고리")
}

// MigrateNewsTopicKeywordsIfNeeded applies 101 — 주제 차선 검색어를 관리자 키워드 표로(2026-10-09).
func MigrateNewsTopicKeywordsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := columnExists(ctx, db, "news_poll_keywords", "topic")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "101_news_topic_keywords.sql", "뉴스 주제 검색어")
}

// MigrateContentTopicsIfNeeded applies 102 — 커뮤니티·리포트·메모의 주제(주식·부동산·금융상품, 2026-10-09 요청 9).
// 마커는 마지막 ALTER(journal_notes.topic) — 중간에 끊겼어도 다시 돌면 중복 열·키는 건너뛴다.
func MigrateContentTopicsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := columnExists(ctx, db, "journal_notes", "topic")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "102_content_topics.sql", "글 주제")
}

// MigrateDBCleanup1010IfNeeded applies 103 — 접두사 중복 인덱스 넷 · 문장 검색 표 DROP(2026-10-10 사용자 승인).
// 마커는 문장 검색 표 — 남아 있으면 통째로 돌린다(없는 인덱스·표는 건너뛴다).
func MigrateDBCleanup1010IfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := tableExists(ctx, db, "screener_phrase_dict")
	if err != nil || !has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "103_db_cleanup.sql", "DB 정리(중복 인덱스·문장 검색 표)")
}

// MigrateNotificationsIfNeeded applies 104 — 앱 안 알림 · 알림 커서 · 자산→글 인덱스(2026-10-10).
func MigrateNotificationsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := tableExists(ctx, db, "notify_cursors")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "104_notifications.sql", "앱 안 알림")
}

// MigrateBotDraftsIfNeeded applies 105 — 봇 초안 표 · users.badge(2026-10-16). 두 마커 중 하나라도 없으면
// 통째로 돈다(CREATE IF NOT EXISTS · 중복 열은 마이그레이터가 건너뛴다).
func MigrateBotDraftsIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	hasTable, err := tableExists(ctx, db, "bot_drafts")
	if err != nil {
		return err
	}
	hasCol, err := columnExists(ctx, db, "users", "badge")
	if err != nil || (hasTable && hasCol) {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "105_bot_drafts.sql", "봇 초안 · 게시 계정")
}

// MigrateBotLLMIfNeeded applies 106 — 게시 계정 말투 · 뱃지 키 · 초안 출처(2026-10-19). 마지막 열(note)을 마커로 본다 —
// 중간에 끊겨도 다시 돌면 중복 열은 건너뛰고 MODIFY는 같은 결과다.
func MigrateBotLLMIfNeeded(ctx context.Context, db *sql.DB, migrationsDir string) error {
	has, err := columnExists(ctx, db, "bot_drafts", "note")
	if err != nil || has {
		return err
	}
	return applyMigrationFile(ctx, db, migrationsDir, "106_bot_llm.sql", "봇 초안 LLM · 게시 계정 말투")
}
