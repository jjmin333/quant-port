-- 종목 별칭 표 (2026-09-13 사용자 요청 1·4)
--
-- # 무엇을 담는가
--
-- 「DB 어디에도 없는 한글 읽기」다 상장명이 영문인 국내 종목(NAVER)과, 통칭이
-- 상장명과 다른 해외 종목(구글 ↔ 알파벳 A)이 그것이다 둘 다 어떤 문자열 매칭으로도
-- 이어지지 않으므로 사람이 적어 두는 수밖에 없다
--
-- 쓰는 곳은 둘이다 앱 검색(`SearchAliasTokens`)과 뉴스 종목 태깅(`usNameIndex`)
--
-- # 왜 표로 옮겼나
--
-- 지금까지 이 자료는 Go 소스 안의 map 두 개였다 옮긴 이유는 속도가 아니라 —
-- map 조회는 애초에 공짜다 — **고치려면 배포를 해야 했다**는 것이다 별칭은 새 종목·
-- 새 별명이 생길 때마다 늘어나는 자료인데, 한 줄 고치자고 이미지를 다시 굽는 것은
-- 자료의 성격에 맞지 않는다
--
-- 씨앗은 `internal/repository/stock_alias_seed.go` 이고 아래 INSERT 행은 거기서
-- **생성**됐다 두 벌이 어긋나지 않는지는 시험이 센다(TestSeedMatchesMigration)
-- 앞으로 늘어나는 별칭은 표에만 넣는다
--
-- # 두 갈래 (kind)
--
--   brand   한글 읽기 → 영문 표기  「에스케이」 하나로 SK·SK하이닉스·SK텔레콤이 잡힌다
--                                종목 코드를 적으면 계열사 상장마다 표를 고쳐야 한다
--   symbol  한글 별칭 → 종목 심볼  브랜드로 풀 수 없는 것 「구글」의 상장명은 「알파벳 A」다
--
-- # source
--
--   seed    이 마이그레이션이 넣은 것
--   toss    나중에 배치가 채울 자리 (지금은 안 쓴다 — stocks.name 을 직접 색인한다)
--   manual  운영 중에 사람이 넣은 것
--
-- 갈래를 컬럼으로 둔 이유는 **지우는 기준**이 갈래마다 다르기 때문이다 배치가
-- 다시 채우는 행은 통째로 갈아 끼워도 되지만 사람이 넣은 행은 그러면 안 된다
--
-- # enabled
--
-- 오태깅이 나면 행을 지우는 대신 끈다 지우면 「왜 뺐는지」가 같이 사라지고, 다음
-- 사람이 같은 별칭을 다시 넣는다 note 에 이유를 적어 두면 그 반복이 끊긴다
--
-- # 인덱스
--
-- 조회는 언제나 kind 로 갈라서 alias 로 찾거나(정확 일치) 접두로 훑는다
-- (검색창이 글자마다 질의를 보내므로 「네이」로도 naver 가 나와야 한다)
-- 그래서 (kind, alias) 복합이 맞고 alias 단독 인덱스는 두지 않는다

CREATE TABLE IF NOT EXISTS stock_aliases (
  id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  alias       VARCHAR(64)     NOT NULL,
  kind        VARCHAR(8)      NOT NULL,
  target      VARCHAR(64)     NOT NULL,
  source      VARCHAR(16)     NOT NULL DEFAULT 'manual',
  note        VARCHAR(255)    NULL,
  enabled     TINYINT(1)      NOT NULL DEFAULT 1,
  created_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at  TIMESTAMP       NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  UNIQUE KEY uniq_alias_kind_target (alias, kind, target),
  KEY idx_kind_alias (kind, alias)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- 씨앗 아래 행은 stock_alias_seed.go 에서 생성됐다 손으로 고치지 않는다
--
-- INSERT IGNORE 인 이유는 이 파일이 이미 표가 있는 DB에도 적용될 수 있어서다
-- 운영 중에 사람이 같은 별칭을 먼저 넣어 뒀다면 그 행(과 그 note)이 이긴다
INSERT IGNORE INTO stock_aliases (alias, kind, target, source) VALUES
  ('나이스', 'brand', 'nice', 'seed'),
  ('네이버', 'brand', 'naver', 'seed'),
  ('디비', 'brand', 'db', 'seed'),
  ('디아이', 'brand', 'di', 'seed'),
  ('디에스', 'brand', 'ds', 'seed'),
  ('디엔', 'brand', 'dn', 'seed'),
  ('디엘', 'brand', 'dl', 'seed'),
  ('라이즈', 'brand', 'rise', 'seed'),
  ('비엔케이', 'brand', 'bnk', 'seed'),
  ('비지에프', 'brand', 'bgf', 'seed'),
  ('비지에프리테일', 'brand', 'bgf', 'seed'),
  ('솔', 'brand', 'sol', 'seed'),
  ('숲', 'brand', 'soop', 'seed'),
  ('심팩', 'brand', 'simpac', 'seed'),
  ('씨제이', 'brand', 'cj', 'seed'),
  ('아이비케이', 'brand', 'ibk', 'seed'),
  ('아이에스씨', 'brand', 'isc', 'seed'),
  ('아이엠뱅크', 'brand', 'im', 'seed'),
  ('아프리카티비', 'brand', 'soop', 'seed'),
  ('아프리카tv', 'brand', 'soop', 'seed'),
  ('알에프에이치아이씨', 'brand', 'rfhic', 'seed'),
  ('에스비에스', 'brand', 'sbs', 'seed'),
  ('에스에프에이', 'brand', 'sfa', 'seed'),
  ('에스엔티', 'brand', 'snt', 'seed'),
  ('에스엠', 'brand', 'sm', 'seed'),
  ('에스오일', 'brand', 's-oil', 'seed'),
  ('에스지', 'brand', 'sg', 'seed'),
  ('에스케이', 'brand', 'sk', 'seed'),
  ('에스티엑스', 'brand', 'stx', 'seed'),
  ('에쓰오일', 'brand', 's-oil', 'seed'),
  ('에이스', 'brand', 'ace', 'seed'),
  ('에이치디', 'brand', 'hd', 'seed'),
  ('에이치디씨', 'brand', 'hdc', 'seed'),
  ('에이치엘', 'brand', 'hl', 'seed'),
  ('에이치엘비', 'brand', 'hlb', 'seed'),
  ('에이치엠엠', 'brand', 'hmm', 'seed'),
  ('에이치피에스피', 'brand', 'hpsp', 'seed'),
  ('에이케이', 'brand', 'ak', 'seed'),
  ('에프앤에프', 'brand', 'f&f', 'seed'),
  ('에프엔에프', 'brand', 'f&f', 'seed'),
  ('엔에이치', 'brand', 'nh', 'seed'),
  ('엔에이치엔', 'brand', 'nhn', 'seed'),
  ('엘비', 'brand', 'lb', 'seed'),
  ('엘아이지', 'brand', 'lig', 'seed'),
  ('엘아이지넥스원', 'brand', 'lig', 'seed'),
  ('엘에스', 'brand', 'ls', 'seed'),
  ('엘에프', 'brand', 'lf', 'seed'),
  ('엘엑스', 'brand', 'lx', 'seed'),
  ('엘지', 'brand', 'lg', 'seed'),
  ('오씨아이', 'brand', 'oci', 'seed'),
  ('와이비엠', 'brand', 'ybm', 'seed'),
  ('와이지', 'brand', 'yg', 'seed'),
  ('와이티엔', 'brand', 'ytn', 'seed'),
  ('인베니', 'brand', 'inveni', 'seed'),
  ('제이더블유', 'brand', 'jw', 'seed'),
  ('제이비', 'brand', 'jb', 'seed'),
  ('제이와이피', 'brand', 'jyp', 'seed'),
  ('지에스', 'brand', 'gs', 'seed'),
  ('지케이엘', 'brand', 'gkl', 'seed'),
  ('케이비', 'brand', 'kb', 'seed'),
  ('케이비스타', 'brand', 'kbstar', 'seed'),
  ('케이씨', 'brand', 'kc', 'seed'),
  ('케이씨씨', 'brand', 'kcc', 'seed'),
  ('케이알모터스', 'brand', 'kr', 'seed'),
  ('케이지', 'brand', 'kg', 'seed'),
  ('케이티', 'brand', 'kt', 'seed'),
  ('케이티앤지', 'brand', 'kt&g', 'seed'),
  ('케이티엔지', 'brand', 'kt&g', 'seed'),
  ('코덱스', 'brand', 'kodex', 'seed'),
  ('코액트', 'brand', 'koact', 'seed'),
  ('키움', 'brand', 'kiwoom', 'seed'),
  ('타이거', 'brand', 'tiger', 'seed'),
  ('티씨씨', 'brand', 'tcc', 'seed'),
  ('포스코', 'brand', 'posco', 'seed'),
  ('플러스', 'brand', 'plus', 'seed'),
  ('하나로', 'brand', 'hanaro', 'seed'),
  ('골드만삭스', 'symbol', 'GS', 'seed'),
  ('구글', 'symbol', 'GOOGL', 'seed'),
  ('구글씨', 'symbol', 'GOOG', 'seed'),
  ('구글에이', 'symbol', 'GOOGL', 'seed'),
  ('넥스트에라', 'symbol', 'NEE', 'seed'),
  ('노스롭그루먼', 'symbol', 'NOC', 'seed'),
  ('뉴몬트', 'symbol', 'NEM', 'seed'),
  ('니오', 'symbol', 'NIO', 'seed'),
  ('달러제너럴', 'symbol', 'DG', 'seed'),
  ('대만반도체', 'symbol', 'TSM', 'seed'),
  ('데이터독', 'symbol', 'DDOG', 'seed'),
  ('델타항공', 'symbol', 'DAL', 'seed'),
  ('도미노피자', 'symbol', 'DPZ', 'seed'),
  ('도어대시', 'symbol', 'DASH', 'seed'),
  ('디어앤컴퍼니', 'symbol', 'DE', 'seed'),
  ('램리서치', 'symbol', 'LRCX', 'seed'),
  ('레이시온', 'symbol', 'RTX', 'seed'),
  ('로블록스', 'symbol', 'RBLX', 'seed'),
  ('로빈후드', 'symbol', 'HOOD', 'seed'),
  ('로우스', 'symbol', 'LOW', 'seed'),
  ('록히드마틴', 'symbol', 'LMT', 'seed'),
  ('루멘텀', 'symbol', 'LITE', 'seed'),
  ('루시드', 'symbol', 'LCID', 'seed'),
  ('리비안', 'symbol', 'RIVN', 'seed'),
  ('리프트', 'symbol', 'LYFT', 'seed'),
  ('마벨', 'symbol', 'MRVL', 'seed'),
  ('마이크로스트래티지', 'symbol', 'MSTR', 'seed'),
  ('마이크론', 'symbol', 'MU', 'seed'),
  ('마이크론테크놀로지', 'symbol', 'MU', 'seed'),
  ('머크', 'symbol', 'MRK', 'seed'),
  ('메드트로닉', 'symbol', 'MDT', 'seed'),
  ('메르카도리브레', 'symbol', 'MELI', 'seed'),
  ('메리어트', 'symbol', 'MAR', 'seed'),
  ('메타플랫폼', 'symbol', 'META', 'seed'),
  ('모건스탠리', 'symbol', 'MS', 'seed'),
  ('몬델리즈', 'symbol', 'MDLZ', 'seed'),
  ('몽고디비', 'symbol', 'MDB', 'seed'),
  ('바이두', 'symbol', 'BIDU', 'seed'),
  ('뱅크오브아메리카', 'symbol', 'BAC', 'seed'),
  ('버라이즌', 'symbol', 'VZ', 'seed'),
  ('버크셔', 'symbol', 'BRK.B', 'seed'),
  ('버크셔해서웨이', 'symbol', 'BRK.B', 'seed'),
  ('베리사인', 'symbol', 'VRSN', 'seed'),
  ('부킹홀딩스', 'symbol', 'BKNG', 'seed'),
  ('블랙록', 'symbol', 'BLK', 'seed'),
  ('사우스웨스트항공', 'symbol', 'LUV', 'seed'),
  ('서비스나우', 'symbol', 'NOW', 'seed'),
  ('셰브론', 'symbol', 'CVX', 'seed'),
  ('소파이', 'symbol', 'SOFI', 'seed'),
  ('쇼피파이', 'symbol', 'SHOP', 'seed'),
  ('슈퍼마이크로', 'symbol', 'SMCI', 'seed'),
  ('스냅챗', 'symbol', 'SNAP', 'seed'),
  ('스노우플레이크', 'symbol', 'SNOW', 'seed'),
  ('스포티파이', 'symbol', 'SPOT', 'seed'),
  ('시게이트', 'symbol', 'STX', 'seed'),
  ('시스코', 'symbol', 'CSCO', 'seed'),
  ('써모피셔', 'symbol', 'TMO', 'seed'),
  ('쓰리엠', 'symbol', 'MMM', 'seed'),
  ('아날로그디바이스', 'symbol', 'ADI', 'seed'),
  ('아리스타네트웍스', 'symbol', 'ANET', 'seed'),
  ('아이비엠', 'symbol', 'IBM', 'seed'),
  ('아이온큐', 'symbol', 'IONQ', 'seed'),
  ('아처에비에이션', 'symbol', 'ACHR', 'seed'),
  ('아카마이', 'symbol', 'AKAM', 'seed'),
  ('알리바바', 'symbol', 'BABA', 'seed'),
  ('알트리아', 'symbol', 'MO', 'seed'),
  ('알파벳', 'symbol', 'GOOGL', 'seed'),
  ('암젠', 'symbol', 'AMGN', 'seed'),
  ('애브비', 'symbol', 'ABBV', 'seed'),
  ('얌브랜즈', 'symbol', 'YUM', 'seed'),
  ('어플라이드머티리얼즈', 'symbol', 'AMAT', 'seed'),
  ('어플라이드머티어리얼즈', 'symbol', 'AMAT', 'seed'),
  ('에이에스엠엘', 'symbol', 'ASML', 'seed'),
  ('에이엠디', 'symbol', 'AMD', 'seed'),
  ('에이치피', 'symbol', 'HPQ', 'seed'),
  ('엑슨모빌', 'symbol', 'XOM', 'seed'),
  ('엔페이즈', 'symbol', 'ENPH', 'seed'),
  ('온세미컨덕터', 'symbol', 'ON', 'seed'),
  ('워크데이', 'symbol', 'WDAY', 'seed'),
  ('웨스턴디지털', 'symbol', 'WDC', 'seed'),
  ('웰스파고', 'symbol', 'WFC', 'seed'),
  ('유나이티드항공', 'symbol', 'UAL', 'seed'),
  ('유나이티드헬스', 'symbol', 'UNH', 'seed'),
  ('유니티', 'symbol', 'U', 'seed'),
  ('유피에스', 'symbol', 'UPS', 'seed'),
  ('인튜이트', 'symbol', 'INTU', 'seed'),
  ('일라이릴리', 'symbol', 'LLY', 'seed'),
  ('제너럴다이내믹스', 'symbol', 'GD', 'seed'),
  ('제이피모간', 'symbol', 'JPM', 'seed'),
  ('제이피모건', 'symbol', 'JPM', 'seed'),
  ('조비에비에이션', 'symbol', 'JOBY', 'seed'),
  ('존슨앤드존슨', 'symbol', 'JNJ', 'seed'),
  ('존슨앤존슨', 'symbol', 'JNJ', 'seed'),
  ('질로우', 'symbol', 'Z', 'seed'),
  ('징둥닷컴', 'symbol', 'JD', 'seed'),
  ('치폴레', 'symbol', 'CMG', 'seed'),
  ('캐터필러', 'symbol', 'CAT', 'seed'),
  ('컴캐스트', 'symbol', 'CMCSA', 'seed'),
  ('케이엘에이', 'symbol', 'KLAC', 'seed'),
  ('코노코필립스', 'symbol', 'COP', 'seed'),
  ('코히런트', 'symbol', 'COHR', 'seed'),
  ('콜게이트', 'symbol', 'CL', 'seed'),
  ('크라우드스트라이크', 'symbol', 'CRWD', 'seed'),
  ('클라우드플레어', 'symbol', 'NET', 'seed'),
  ('텍사스인스트루먼트', 'symbol', 'TXN', 'seed'),
  ('티에스엠씨', 'symbol', 'TSM', 'seed'),
  ('티에스엠씨에이디알', 'symbol', 'TSM', 'seed'),
  ('팔로알토네트웍스', 'symbol', 'PANW', 'seed'),
  ('퍼스트솔라', 'symbol', 'FSLR', 'seed'),
  ('페덱스', 'symbol', 'FDX', 'seed'),
  ('페북', 'symbol', 'META', 'seed'),
  ('페이스북', 'symbol', 'META', 'seed'),
  ('포티넷', 'symbol', 'FTNT', 'seed'),
  ('프록터앤갬블', 'symbol', 'PG', 'seed'),
  ('프록터앤드갬블', 'symbol', 'PG', 'seed'),
  ('프리포트맥모란', 'symbol', 'FCX', 'seed'),
  ('핀둬둬', 'symbol', 'PDD', 'seed'),
  ('핀터레스트', 'symbol', 'PINS', 'seed'),
  ('필립모리스', 'symbol', 'PM', 'seed'),
  ('허니웰', 'symbol', 'HON', 'seed'),
  ('홈디포', 'symbol', 'HD', 'seed'),
  ('화이자', 'symbol', 'PFE', 'seed');
