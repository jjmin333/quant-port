-- 배치 잡을 독립 워커 프로세스로 분리하면서, API 서버는 더 이상 잡을 직접
-- 실행하지 않고 이 테이블에 큐잉만 한다. 각 잡 전용 워커 프로세스가
-- status='queued' 행을 폴링해 가져간다(job_id당 워커 1개이므로 단일 UPDATE
-- ORDER BY LIMIT 로 충분히 원자적).
ALTER TABLE batch_job_runs
  ADD COLUMN params JSON NULL AFTER trigger_kind,
  MODIFY COLUMN status VARCHAR(16) NOT NULL DEFAULT 'queued';
