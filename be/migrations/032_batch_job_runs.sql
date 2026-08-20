-- 배치 실행 이력 (관리자 > 배치 관리)
--
-- Until now every batch — daily close projection, score recompute, index
-- backfill — left only a log line, so there was no way to answer "did it run,
-- and what did it produce" from the admin console. One row per run, keyed by
-- job id, makes the last outcome and the coverage it reached durable.

CREATE TABLE IF NOT EXISTS batch_job_runs (
  id           BIGINT UNSIGNED NOT NULL AUTO_INCREMENT PRIMARY KEY,
  job_id       VARCHAR(48)  NOT NULL,
  status       VARCHAR(16)  NOT NULL,           -- running | done | error
  trigger_kind VARCHAR(16)  NOT NULL DEFAULT 'manual', -- manual | schedule
  started_at   TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  finished_at  TIMESTAMP(3) NULL,
  -- Human-readable outcome shown in the console.
  message      TEXT         NOT NULL DEFAULT (''),
  -- Structured result (rows written, coverage, distribution) for later audit.
  result       JSON         NULL,
  KEY idx_batch_job_runs_job (job_id, started_at DESC),
  KEY idx_batch_job_runs_started (started_at DESC)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
