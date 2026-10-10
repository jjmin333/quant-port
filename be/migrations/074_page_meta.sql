-- 화면별 제목·설명 덮어쓰기 표 (2026-09-15 사용자 요청 2)
--
-- # 무엇을 담는가
--
-- 브라우저 탭 이름과 **검색 결과에 나가는 글**이다 지금까지 이 값들은 Next 소스에
-- 문자열로 박혀 있었다 — `lib/seo.ts`의 `SITE_TITLE`과 화면마다의 `pageMetadata({...})`
--
-- 고치려면 배포를 해야 했다 별칭 표(070)를 만든 이유와 정확히 같다 검색 문구는
-- 코드가 아니라 **운영하면서 다듬는 글**이라 배포 주기에 묶여 있으면 안 된다
--
-- # 왜 「덮어쓰기」인가 (전체 저장이 아니라)
--
-- 비어 있는 칸은 **코드의 기본값이 이긴다** 표가 비어 있어도 사이트는 지금 그대로
-- 돌아가고, 표가 통째로 사라져도(DB 복구·새 환경) 아무 화면도 제목을 잃지 않는다
--
-- 반대로 하면 — 표를 정답으로 두면 — 새 화면을 만들 때마다 행을 넣어야 하고,
-- 넣는 것을 잊으면 조용히 제목 없는 화면이 나간다
--
-- # path
--
-- 앱 경로 그대로다 `/`, `/market`, `/news/domestic` 동적 경로(`/stocks/{symbol}`,
-- `/reports/{id}`)는 넣지 않는다 그쪽 제목은 종목명·글 제목에서 나오는 것이라
-- 사람이 적을 값이 아니다
--
-- 191 인 이유는 utf8mb4 에서 인덱스 키 상한(767바이트)에 맞춘 관습적인 값이다
-- 경로는 그보다 훨씬 짧다
--
-- # noindex
--
-- 이 화면을 검색에서 빼는가 세 값이다 — 비어 있으면(NULL) 코드의 기본값을 따르고,
-- 0/1 이면 그 값이 이긴다 TINYINT NULL 인 이유가 이것이다 「관리자가 아직 정하지
-- 않았다」와 「색인해도 된다고 정했다」는 다른 상태다
--
-- # og_*
--
-- 검색 결과 문구와 카카오톡·슬랙 카드 문구를 **다르게** 쓰고 싶을 때만 채운다
-- 비어 있으면 위의 title/description 이 그대로 카드에도 쓰인다 (지금 코드가
-- 하는 일 그대로 — `pageMetadata`)

CREATE TABLE IF NOT EXISTS page_meta (
  path           VARCHAR(191) NOT NULL,
  title          VARCHAR(255) NOT NULL DEFAULT '',
  description    VARCHAR(500) NOT NULL DEFAULT '',
  og_title       VARCHAR(255) NOT NULL DEFAULT '',
  og_description VARCHAR(500) NOT NULL DEFAULT '',
  og_image       VARCHAR(500) NOT NULL DEFAULT '',
  noindex        TINYINT(1)   NULL,
  updated_at     TIMESTAMP    NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (path)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
