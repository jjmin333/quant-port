-- 검색 유입 · 활성화 감시(worker-growth) — 2026-09-25 사용자 결정: AI 에이전트가 아니라 코드만으로 도는 별도 컨테이너.
--
-- 이 표들은 **worker-growth만 쓴다.** 그 워커는 앱의 다른 표를 읽기만 한다 — 점수 산식 · 금융 원본 ·
-- 백테스트 · 사용자 데이터 · 인증 · 법적 고지를 자동으로 바꾸지 않는다는 약속이 코드 구조로 지켜지게
-- (`internal/growth` 쓰기 경로는 growth_* 표뿐이고, 시험이 그것을 센다).

USE quantport;

-- 사이트맵 URL마다 마지막 크롤 결과 한 줄. 이전 결과와의 차이(새 문제 · 고쳐진 문제)는 덮어쓰기 전에
-- 메모리에서 잰다 — 날마다 1,000줄씩 쌓을 이유가 없다. 사이트맵에서 빠진 URL은 지운다.
CREATE TABLE IF NOT EXISTS growth_crawl_pages (
  url            VARCHAR(600)  NOT NULL,
  page_type      VARCHAR(24)   NOT NULL DEFAULT '',
  status_code    SMALLINT      NOT NULL DEFAULT 0,
  location       VARCHAR(600)  NOT NULL DEFAULT '',
  response_ms    INT           NOT NULL DEFAULT 0,
  html_bytes     INT           NOT NULL DEFAULT 0,
  title          VARCHAR(300)  NOT NULL DEFAULT '',
  description    VARCHAR(600)  NOT NULL DEFAULT '',
  canonical      VARCHAR(600)  NOT NULL DEFAULT '',
  robots         VARCHAR(160)  NOT NULL DEFAULT '',
  h1_count       SMALLINT      NOT NULL DEFAULT 0,
  text_chars     INT           NOT NULL DEFAULT 0,
  links_out      INT           NOT NULL DEFAULT 0,
  links_in       INT           NOT NULL DEFAULT 0,
  has_og_image   TINYINT(1)    NOT NULL DEFAULT 0,
  jsonld_types   VARCHAR(200)  NOT NULL DEFAULT '',
  -- [{"code":"thin_content","severity":"warn","detail":"..."}]
  issues         JSON          NULL,
  first_seen_at  DATETIME      NOT NULL,
  crawled_at     DATETIME      NOT NULL,
  PRIMARY KEY (url),
  KEY idx_growth_crawl_type (page_type)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 검색 성과 원자료. Google은 (날짜·검색어·페이지), Bing은 검색어 행(page='')과 페이지 행(query='')을
-- 따로 받는다. 키는 MD5(query \0 page) — 검색어와 URL이 길어 그대로는 기본 키에 못 넣는다.
-- 드문 검색어는 Google이 익명화해 이 행에서 빠진다. 그래서 **합계는 여기서 더하지 않고**
-- growth_kpi_daily의 google 절(날짜만으로 받은 값)을 쓴다.
CREATE TABLE IF NOT EXISTS growth_search_rows (
  engine       VARCHAR(8)    NOT NULL,
  day          DATE          NOT NULL,
  row_key      BINARY(16)    NOT NULL,
  query        VARCHAR(400)  NOT NULL DEFAULT '',
  page         VARCHAR(600)  NOT NULL DEFAULT '',
  clicks       INT           NOT NULL DEFAULT 0,
  impressions  INT           NOT NULL DEFAULT 0,
  position     DOUBLE        NOT NULL DEFAULT 0,
  PRIMARY KEY (engine, day, row_key),
  KEY idx_growth_search_day (day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Search Console URL 검사 결과(URL마다 최신 한 줄). 하루 한도(속성당 2,000)보다 한참 적게,
-- 오래 안 본 URL부터 돌려 가며 본다. 상태가 바뀌면 이전 값을 prev_에 남긴다.
CREATE TABLE IF NOT EXISTS growth_url_inspections (
  url               VARCHAR(600)  NOT NULL,
  inspected_at      DATETIME      NOT NULL,
  verdict           VARCHAR(24)   NOT NULL DEFAULT '',
  coverage_state    VARCHAR(200)  NOT NULL DEFAULT '',
  indexing_state    VARCHAR(48)   NOT NULL DEFAULT '',
  robots_state      VARCHAR(24)   NOT NULL DEFAULT '',
  fetch_state       VARCHAR(48)   NOT NULL DEFAULT '',
  google_canonical  VARCHAR(600)  NOT NULL DEFAULT '',
  user_canonical    VARCHAR(600)  NOT NULL DEFAULT '',
  last_crawl_at     DATETIME      NULL,
  prev_coverage     VARCHAR(200)  NOT NULL DEFAULT '',
  changed_at        DATETIME      NULL,
  PRIMARY KEY (url),
  KEY idx_growth_inspect_at (inspected_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 하루 KPI. 절(site · db · google · bing · ga)마다 다른 실행이 채우므로 JSON 병합으로 쓴다.
-- day는 「그 숫자가 가리키는 날」이다 — 크롤은 실행일, 검색·방문·가입은 그 활동이 있던 날.
CREATE TABLE IF NOT EXISTS growth_kpi_daily (
  day         DATE      NOT NULL,
  metrics     JSON      NOT NULL,
  updated_at  DATETIME  NOT NULL,
  PRIMARY KEY (day)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- 보고서. 같은 종류·같은 마감일은 덮어쓴다(수동 재실행이 행을 늘리지 않게).
CREATE TABLE IF NOT EXISTS growth_reports (
  id            BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  kind          VARCHAR(8)      NOT NULL,
  period_start  DATE            NOT NULL,
  period_end    DATE            NOT NULL,
  created_at    DATETIME        NOT NULL,
  markdown      MEDIUMTEXT      NOT NULL,
  data          JSON            NULL,
  PRIMARY KEY (id),
  UNIQUE KEY uq_growth_reports_period (kind, period_end)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
