-- 운영 사건 원장 — 「무엇이 깨졌고, 언제부터, 아직 깨져 있나」
--
-- # 왜 필요했나
--
-- 이 저장소에는 이미 배치 원장(batch_job_runs)과 감시 보고서(batch_health)가 있는데
-- 둘 다 이 질문에 답하지 못한다
--
--   batch_job_runs  실행된 것만 안다 — 안 돌아간 슬롯은 아무 흔적도 남기지 않는다
--   batch_health    단 한 행이다 — 지금 상태만 있고 이력이 없다
--
-- 그래서 2026-09-10 DART 한도 소진처럼 사람이 나중에 물어보는 질문들이 로그를 뒤지는
-- 일이 됐다 — 「언제부터 깨져 있었나」 「몇 번째 재발인가」 「그때 무엇이 함께 죽었나」
-- 로그는 컨테이너가 재시작하면 사라지고 실제로 그 조사에서 사라져 있었다
--
-- # 설계 규칙 하나 — 사건은 **줄이 아니라 상태**다
--
-- 워치독은 15분마다 도는데, 늦은 잡 하나가 회차마다 한 행을 만들면 하루 96행이
-- 쌓이고 「몇 건인가」가 「몇 번 쳐다봤나」를 뜻하게 된다 실측으로 그 로그가 그랬다
-- financials_domestic 한 건이 같은 줄을 18번 넘게 찍었다
--
-- 그래서 (kind, subject) 당 **열린 사건은 최대 하나**다 (uniq_open) 같은 일이 다시
-- 보이면 새 행이 아니라 last_seen_at 과 occurrences 를 올린다 조건이 풀리면 그 행을
-- resolved 로 닫고, 다음에 또 깨지면 **새 행**이 열린다 — 그래야 재발이 세어진다
--
-- # status 를 generated column 으로 두지 않는 이유
--
-- resolved_at IS NULL 로 유도할 수 있지만, 그러면 부분 유니크 인덱스를 만들 수 없다
-- (MySQL 에는 WHERE 절 인덱스가 없다) 열린 사건 하나를 DB가 보장하게 하려면 닫힌 행이
-- 인덱스에서 빠져야 하고, 그래서 resolved_key 를 쓴다 — 열려 있으면 상수 문자열,
-- 닫히면 그 행의 id 라서 서로 충돌하지 않는다
--
-- NOTE 주석 안에 문장 종결 기호를 넣지 않는다 splitSQLStatements 가 파싱 없이
-- 그 문자로 파일을 자르기 때문에, 주석 속 한 글자가 statement 를 반으로 쪼갠다

CREATE TABLE IF NOT EXISTS ops_incidents (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  -- kind 는 사건의 종류 batch_missed · batch_failed · batch_stuck · quota_exhausted
  -- · upstream_degraded 코드의 model.IncidentKind 와 같은 값이어야 한다
  kind          VARCHAR(32)  NOT NULL,
  -- subject 는 그 종류 안에서 무엇인지 배치면 job_id, 한도면 provider 이름
  subject       VARCHAR(64)  NOT NULL,
  severity      VARCHAR(16)  NOT NULL DEFAULT 'warn',
  -- 사람이 읽는 한 줄 가장 최근에 본 상태를 적는다
  message       VARCHAR(512) NOT NULL DEFAULT '',
  -- 기계가 읽는 부속 정보 기대 시각 · 마지막 성공 · 한도와 사용량 등
  detail        JSON         NULL,
  occurrences   INT UNSIGNED NOT NULL DEFAULT 1,
  first_seen_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_seen_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  resolved_at   TIMESTAMP(3) NULL DEFAULT NULL,
  -- 열린 행은 'open', 닫힌 행은 자기 id 문자열 위 머리글의 uniq_open 설명 참조
  resolved_key  VARCHAR(32)  NOT NULL DEFAULT 'open',
  UNIQUE KEY uniq_open (kind, subject, resolved_key),
  KEY idx_open (resolved_at, last_seen_at),
  KEY idx_subject (subject, first_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
