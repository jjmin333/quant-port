-- 종목별 **산업 코드와 공통 섹터**
--
-- # 왜 필요한가
--
-- 점수의 피어그룹이 (자산유형 × 시장 × 시총 3분위)뿐이다 — 섹터 칸이 어느 표에도
-- 없었다(`docs/STOCK_SCORE_DESIGN.md` §2.5). PER 40은 반도체에서 평범하고 은행에서
-- 비정상인데, 한 줄로 세우면 가치 축은 「업종 판별기」가 된다. 포트폴리오의 섹터
-- 노출도 이 칸이 있어야 잰다.
--
-- # 왜 `stocks`에 컬럼을 더하지 않나
--
-- `stocks`는 카탈로그 동기화가 통째로 다시 쓰는 표다. 거기에 다른 배치가 채우는
-- 칸을 섞으면 두 배치가 서로의 값을 덮는다. 이 표는 섹터 배치만 쓴다.
--
-- # 원천
--
--   · 국내 — DART `company.json`의 `induty_code` (KSIC). 이름은 주지 않는다.
--   · 미국 — SEC `submissions/CIK….json`의 `sic` · `sicDescription`.
--
-- `sector`는 그 코드를 `model.SectorFromKSIC` / `SectorFromSIC`로 옮긴 값이다.
-- 매핑을 고치면 **다시 받지 않고** 원래 코드로 다시 계산할 수 있도록 원 코드를
-- 그대로 남긴다. 옮길 수 없는 코드는 `sector`가 NULL이다(짐작해서 넣지 않는다).

USE quantport;

CREATE TABLE IF NOT EXISTS stock_sectors (
  symbol        VARCHAR(32)  NOT NULL PRIMARY KEY,
  source        VARCHAR(8)   NOT NULL,              -- dart | sec
  industry_code VARCHAR(16)  NOT NULL DEFAULT '',   -- KSIC 또는 SIC 원 코드
  industry_name VARCHAR(160) NOT NULL DEFAULT '',   -- SEC만 준다
  sector        VARCHAR(32)  NULL,                  -- model.Sector, 옮길 수 없으면 NULL
  fetched_at    TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
  KEY idx_stock_sectors_sector (sector)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
