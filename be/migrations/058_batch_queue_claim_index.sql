USE quantport;

-- 배치 큐 클레임 인덱스 — 워커끼리의 데드락을 없앤다.
--
-- # 무엇이 문제였나
--
-- 큐 워커는 자기 잡의 다음 대기 행을 이렇게 집는다.
--
--   SELECT id, params FROM batch_job_runs
--    WHERE job_id = ? AND status = 'queued'
--    ORDER BY id ASC LIMIT 1 FOR UPDATE
--
-- 기존 인덱스는 (job_id, started_at)이라 `status`가 들어 있지 않다. 그래서
-- 실행계획이 **그 job_id의 행 전부**를 읽고(실측 rows=13, Using filesort) 그
-- 전부에 FOR UPDATE 락을 건다 — 이미 끝난 `done` 행까지 잠근다. 거기에
-- next-key 락의 갭이 인접한 job_id 구간까지 걸쳐, 서로 다른 잡을 맡은 워커들이
-- 같은 갭을 두고 교착한다.
--
-- 2026-08-23에 실제로 났다:
--   batch worker stock_scores: claim: Error 1213 (40001): Deadlock found
--
-- 큐 워커가 둘일 때는 잘 안 드러났는데, 캔들 롤업과 만료 정리를 더해 셋이 되면서
-- 표면화했다. 워커를 더 붙일 때마다 확률이 올라가는 종류의 결함이다.
--
-- # 왜 이 인덱스인가
--
-- (job_id, status, id)면 WHERE 두 조건이 모두 인덱스 선두에 들어가고 남은 컬럼이
-- id라 ORDER BY까지 인덱스가 준다. 그래서
--   · 읽고 잠그는 행이 **그 잡의 대기 행 하나**로 줄고
--   · filesort가 사라지고
--   · 잡마다 인덱스 구간이 완전히 갈라져 갭이 겹치지 않는다.
--
-- 코드 쪽에는 FOR UPDATE SKIP LOCKED를 함께 넣었다(batch_job_runs.go). 인덱스가
-- 경합 범위를 좁히고, SKIP LOCKED가 남은 경합에서 **기다리는 대신 건너뛰게** 한다
-- — 큐에서는 남이 집은 행을 기다릴 이유가 없다.

CREATE INDEX idx_batch_job_runs_claim
  ON batch_job_runs (job_id, status, id);
