-- 금융상품 비교 (2026-09-29 사용자 요청 — 예·적금·대출 비교 MVP)
--
--   fin_products   금융감독원 「금융상품 한눈에」 Open API(finlife.fss.or.kr)의 상품 한 줄.
--                  공통 칸만 열로 두고, 상품 종류마다 다른 기본정보(우대조건·중도상환수수료 …)는
--                  `base`, 기간·금리 옵션 목록은 `options`에 **API 원문 그대로** 둔다 — 옮겨 적다가
--                  숫자를 바꾸는 일이 없게. 계산은 FE(lib/fin-calc.ts)가 이 원문으로 한다.
--                  이번 수집에서 안 보인 상품은 지우지 않고 active=0으로 숨긴다(권역 하나라도
--                  실패한 날은 숨기지 않는다 — 실패가 곧 「상품 없음」이 되지 않게).
--   fin_companies  금융회사 홈페이지·대표전화(같은 API의 companySearch). 외부 링크는 여기와
--                  금감원 비교공시 페이지뿐이다 — 가입 버튼·제휴 링크는 두지 않는다(중개 해석 회피).
--   fin_watchlist  사용자의 관심 금융상품. 주식 관심 목록(watchlist_items)과 섞지 않는다.
--
-- 전부 worker-filings의 `fin_products` 잡이 채우고, 조회는 표만 읽는다.

USE quantport;

CREATE TABLE IF NOT EXISTS fin_products (
  id             BIGINT       NOT NULL AUTO_INCREMENT PRIMARY KEY,
  -- deposit(정기예금) · saving(적금) · mortgage(주택담보대출) · rent(전세자금대출) · credit(개인신용대출)
  kind           VARCHAR(16)  NOT NULL,
  fin_co_no      VARCHAR(20)  NOT NULL,
  fin_prdt_cd    VARCHAR(64)  NOT NULL,
  -- 권역: 020000 은행 · 030200 여신전문 · 030300 저축은행 · 050000 보험 · 060000 금융투자
  top_fin_grp_no CHAR(6)      NOT NULL,
  company_name   VARCHAR(100) NOT NULL,
  product_name   VARCHAR(200) NOT NULL,
  dcls_month     CHAR(6)      NOT NULL DEFAULT '',
  dcls_strt_day  VARCHAR(8)   NOT NULL DEFAULT '',
  dcls_end_day   VARCHAR(8)   NOT NULL DEFAULT '',
  base           JSON         NOT NULL,
  options        JSON         NOT NULL,
  active         TINYINT(1)   NOT NULL DEFAULT 1,
  first_seen_at  TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  fetched_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  UNIQUE KEY uk_fin_products (kind, fin_co_no, fin_prdt_cd),
  KEY idx_fin_products_kind_active (kind, active)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS fin_companies (
  fin_co_no      VARCHAR(20)  NOT NULL PRIMARY KEY,
  top_fin_grp_no CHAR(6)      NOT NULL,
  company_name   VARCHAR(100) NOT NULL,
  homepage_url   VARCHAR(300) NOT NULL DEFAULT '',
  phone          VARCHAR(40)  NOT NULL DEFAULT '',
  fetched_at     TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

CREATE TABLE IF NOT EXISTS fin_watchlist (
  user_id    BIGINT       NOT NULL,
  product_id BIGINT       NOT NULL,
  created_at TIMESTAMP(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
  PRIMARY KEY (user_id, product_id),
  KEY idx_fin_watchlist_product (product_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
