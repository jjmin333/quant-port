-- 문장 검색 사전 넓히기 (2026-09-27 사용자 결정 C ③ — 「LLM은 사전 후보만, 관리자 승인」)
--
--   1. 화면이 알아듣지 못한 **낱말**만 모은다(문장 전체가 아니다, 40자 이하·요청당 5개).
--   2. 주 1회(월 07:40) 자주 나온 낱말을 LLM이 「기존 조건 중 무엇인가」로 옮긴 **후보**를 만든다.
--   3. 관리자가 승인한 것만 사전(phrase → 조건)으로 나가고, 화면 파서가 읽는다.
-- LLM은 조회 경로에 없다. 사전이 틀려도 칩으로 보이므로 사용자가 고칠 수 있다.

USE quantport;

CREATE TABLE IF NOT EXISTS screener_phrase_log (
  phrase     VARCHAR(40)  NOT NULL PRIMARY KEY,
  hits       INT UNSIGNED NOT NULL DEFAULT 1,
  first_seen TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  last_seen  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  KEY idx_spl_hits (hits)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- status: pending | approved | rejected. mapping: {"rules":[{field,min,max}], "sector": "..."} 또는 {"none": true}
CREATE TABLE IF NOT EXISTS screener_phrase_dict (
  phrase      VARCHAR(40)  NOT NULL PRIMARY KEY,
  mapping     JSON         NOT NULL,
  reason      VARCHAR(200) NOT NULL DEFAULT '',
  status      VARCHAR(12)  NOT NULL DEFAULT 'pending',
  source      VARCHAR(8)   NOT NULL DEFAULT 'llm',
  created_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  decided_at  TIMESTAMP(3) NULL,
  KEY idx_spd_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
