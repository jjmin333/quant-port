-- 배치 감시 보고서 한 벌.
--
-- # 왜 한 행인가
--
-- 이 표는 이력이 아니라 **마지막 판정**만 담는다. 판정의 이력은 원장
-- (batch_job_runs)에 워치독 잡의 실행 기록으로 이미 남고, 여기 필요한 것은
-- 「가장 최근 판정이 언제, 무엇이었나」 하나뿐이다.
--
-- # 이 표의 진짜 값은 payload가 아니라 computed_at이다
--
-- 화면은 판정을 저장된 것에서 읽지 않고 API가 그 자리에서 다시 계산한다 — 15분 묵은
-- 숫자를 보여줄 이유가 없고, 워치독이 죽었을 때 화면이 「마지막으로 정상이던 세상」을
-- 계속 보여주는 것이 가장 나쁜 실패다.
--
-- 그래서 이 행이 답하는 질문은 하나다: **워치독이 아직 살아 있는가.** computed_at이
-- 낡으면 감시가 멈춘 것이고, 그 침묵은 보고서 안에 적힐 수 없다(죽은 프로세스는
-- 자기가 죽었다고 쓰지 못한다).
--
-- payload는 사후 확인용이다 — 「어제 새벽에 무엇이 늦었나」를 로그 없이 보고 싶을 때.
--
-- NOTE: 주석 안에 문장 종결 기호를 넣지 않는다. splitSQLStatements가 파싱 없이
-- 그 문자로 파일을 자르기 때문에, 주석 속 한 글자가 statement를 반으로 쪼갠다.

CREATE TABLE IF NOT EXISTS batch_health (
  id          TINYINT      NOT NULL PRIMARY KEY,
  unhealthy   INT          NOT NULL DEFAULT 0,
  payload     JSON         NOT NULL,
  computed_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
