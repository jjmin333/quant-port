-- 테스트 계정 회수.
--
-- 왜 필요한가: 계정 삭제 API가 없다. 관리자 DB 콘솔에서도 `users`는 read-only라
-- (`service/db_admin_catalog.go`) 화면으로도 못 지운다. 테스트는 실행마다 계정을
-- 새로 만들므로(그래야 리서치 하루 5건·심볼 30일 쿨다운, 스탬프 5분 쿨다운에
-- 걸리지 않는다) 행이 쌓인다. 이 파일이 유일한 회수 수단이다.
--
-- 패턴을 qa-e2e- 가 아니라 qa- 로 둔 것은 손으로 만든 탐색용 계정(qa-probe-…)도
-- 같이 회수하기 위해서다. 실제 사용자가 qa-…@quant-port.test 를 쓸 일은 없다.
--
-- 접두사는 `e2e/src/env.ts`의 TEST_EMAIL_PREFIX / TEST_EMAIL_DOMAIN 과 한 벌이다.
-- 한쪽을 바꾸면 다른 쪽도 바꿔야 한다.
--
-- 사용:
--   mysql -u quantport -p quantport < e2e/scripts/cleanup.sql
--
-- 먼저 무엇이 지워질지 확인할 것:
--   SELECT id, email, created_at FROM users
--    WHERE email LIKE 'qa-%@quant-port.test';

SET @qa_pattern = 'qa-%@quant-port.test';
-- 2026-09-27부터 가입은 아이디라 새 테스트 계정은 예약 접두사로 고른다(env.ts TEST_LOGIN_PREFIX).
SET @qa_login_pattern = 'qae2e%';

-- 자식 행부터. FK가 없는 표도 있어 순서를 손으로 지킨다.
DELETE t FROM holding_trades      t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM user_holdings       t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM user_cash_balances  t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM watchlist_items     t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM journal_notes       t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM user_price_targets  t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM stamps              t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE t FROM user_refresh_tokens t JOIN users u ON u.id = t.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);

-- 리서치 콜과 그 부속. 콜은 제품에서는 지울 수 없지만(적중률이 광고가 되지
-- 않으려면 그래야 한다) 테스트가 만든 행까지 영구 보존할 이유는 없다.
DELETE r FROM research_reactions r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE r FROM research_ratings   r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE r FROM research_comments  r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE r FROM research_reports   r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE r FROM research_revisions r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE r FROM research_views     r JOIN research_calls c ON c.id = r.call_id JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);
DELETE c FROM research_calls     c JOIN users u ON u.id = c.user_id WHERE (u.email LIKE @qa_pattern OR u.email LIKE @qa_login_pattern);

DELETE FROM users WHERE email LIKE @qa_pattern OR email LIKE @qa_login_pattern;
