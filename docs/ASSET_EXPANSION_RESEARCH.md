# 자산군 확장 · 데이터 API 조사 (Wealth / Asset Intelligence)

> 2026-10-01. 조사·설계 문서이며 **법률 의견이 아니다** — 「상업적 이용」 판단은 출시 전 전문가 확인이 필요하다.
> ✅ = 이 세션에서 키로 직접 호출해 응답을 확인 · 📄 = 공식 문서/약관만 확인 · ⏳ = 공공데이터포털 활용신청 대기(키 미등록 응답) · ❔ = 공식 근거를 못 찾음.
> 키: `be/.env`의 `DATA_GO_KR_API_KEY`·`ECOS_API_KEY`·`REB_RONE_API_KEY`(커밋 안 함). 아직 어떤 코드도 읽지 않는다.

## 0. 결론 (먼저 읽을 것)

1. **부동산은 「매물」이 아니라 「보유 부동산 추정가」로 시작할 수 있다.** 국토부 실거래가(✅, 이용허락 제한 없음) + 한국부동산원
   R-ONE 주간 지수(✅, 출처표시 조건 자유 이용)만으로 아파트 추정가·범위·신뢰도를 계산했다(§6, 실제 단지 3곳).
2. **매물(네이버·직방·다방)은 쓸 수 없다.** 공식 API가 없고, 네이버는 2025-12-24 특허법원에서 매물 크롤링 업체를 상대로
   데이터베이스권 침해로 이겼다(손해배상 8천만 원 + 서버 데이터 삭제). robots.txt도 막혀 있다(§4.4).
3. **「무료」 ≠ 「상업적 무료」가 이번 조사의 핵심이다.**
   - 상업 OK: 국토부 실거래가 · 온비드 · R-ONE(출처표시) · 공공데이터법상 기관 자체 데이터.
   - **상업 불가/허락 필요**: 금융위 「일반상품시세(금·배출권·유가)」·「채권시세」(공공누리 4유형, 원천 KRX — 유료 구매),
     「채권기본정보」(2유형, 예탁원 계약), 업비트 Open API(약관 제5조 영리목적·배포 금지), CoinGecko Demo·CMC Basic(비상업),
     금감원 finlife(영리 시 사전 협의).
4. **Crypto MVP를 업비트로 만드는 것은 기술적으로 가장 쉽지만 약관상 그대로는 안 된다.** 공개 서비스에 시세를 보여 주려면
   두나무 제휴(약관의 「중요 제휴사」) 문의, 또는 CoinGecko Basic($35/월, 상업 라이선스) 이상이 현실적 경로다(§5).
5. **데이터 모델은 「다 갈아엎기」가 아니라 3개 표 추가로 Net Worth까지 간다**(§7): 시장가 자산은 지금의
   `stocks`/`user_holdings`를 그대로 쓰고(crypto도 여기), 비시장 자산은 `user_assets` + `asset_valuations`(출처·신뢰도),
   부채는 `user_liabilities`.
6. **기존 시스템에서 찾은 라이선스 위험 3건**(§10): 지수 띠의 S&P 500·다우(FRED 경유, 「사전 서면 허락 없는 복제 금지」),
   비트코인(업비트 약관), ETF 카탈로그(KRX Open API: 제3자 제공 금지·「KRX 통계정보」 표시 의무). 결정이 필요하다.

---

## 1. 지금 구조 (조사 전 확인)

| 표 | 역할 | 확장 시 의미 |
|---|---|---|
| `stocks` (004) | 종목 마스터. `asset_type`(domestic/us/…), `currency`, `market` | 시장가 자산(ETF·REIT·crypto)의 공통 마스터로 그대로 쓸 수 있다 |
| `user_holdings` (014·015) | (user, symbol) 수량·평단 | 「symbol이 있는 자산」만 담는다 — 부동산·예금은 못 담는다 |
| `holding_trades` (039) | 매수/매도 이력·환율·수수료 | Total Return의 현금흐름 원장 역할을 이미 한다 |
| `user_cash_balances` (041) | KRW·USD 현금 | Net Worth의 현금 칸 |
| `fin_products` (095) | 금감원 예·적금·대출 공시 | 예금 자산의 「금리 근거」, 대출 부채의 「비교 금리」 |
| 지수 띠 | FRED · Finnhub · 업비트(BTC) · goldprice.dev(금) | 금·BTC 시세 경로가 이미 있다(라이선스는 §10) |

평가(현재가×수량)는 시세 API와 FE에서 이뤄지고, **부채·비시장 자산·평가 출처** 개념은 아직 없다.

---

## 2. 최종 API 조사표

`Free`=돈 안 냄 · `Commercial`=상업 서비스(광고·유료화·간접수익 포함)에서 써도 되는가 · `Re-dist`=가공 안 한 값을 사용자에게 그대로 보여 주거나 넘겨도 되는가.
난이도 하=키만 있으면 됨 · 중=코드 매핑·대량 적재 · 상=계약/유료/공식 경로 없음.

| Asset | Provider | Data | API | Free | Commercial | Re-dist | Update | Rate Limit | Historical | Real-time | Cost | Diff | URL |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| 주거 RE | 국토부(data.go.kr) | 아파트 매매 실거래 **상세**(aptSeq·지번·도로명·층·면적·해제·등기일·거래유형) | `apis.data.go.kr/1613000/RTMSDataSvcAptTradeDev/getRTMSDataSvcAptTradeDev` ✅ | O | O (이용허락 제한 없음) | O (출처표시 권장) | 매일 · 계약 후 30일 내 신고분 | 개발 10,000/일 (운영 증설 가능) | 2006~ | X | 무료 | 하 | [15126468](https://www.data.go.kr/data/15126468/openapi.do) |
| 주거 RE | 국토부 | 아파트 전월세(보증금·월세·계약기간·신규/갱신·종전 조건) | `…/RTMSDataSvcAptRent/getRTMSDataSvcAptRent` ✅ | O | O | O | 매일 | 10,000/일 | 2011~ | X | 무료 | 하 | [15126474](https://www.data.go.kr/data/15126474/openapi.do) |
| 주거 RE | 국토부 | 연립·다세대 매매/전월세 | `…/RTMSDataSvcRHTrade`·`RHRent` ✅ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 주거 RE | 국토부 | 단독·다가구 매매/전월세 | `…/RTMSDataSvcSHTrade`·`SHRent` ⏳ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 주거 RE | 국토부 | 오피스텔 매매/전월세 | `…/RTMSDataSvcOffiTrade`·`OffiRent` ✅ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 주거 RE | 국토부 | 분양·입주권 전매 | `…/RTMSDataSvcSilvTrade` ✅ | O | O | O | 매일 | 10,000/일 | ❔ | X | 무료 | 하 | data.go.kr |
| 토지 | 국토부 | 토지 매매 | `…/RTMSDataSvcLandTrade` ⏳ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 상업 RE | 국토부 | 상업·업무용 매매 | `…/RTMSDataSvcNrgTrade` ⏳ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 상업 RE | 국토부 | 공장·창고 매매 | `…/RTMSDataSvcInduTrade` ⏳ | O | O | O | 매일 | 10,000/일 | 2006~ | X | 무료 | 하 | data.go.kr |
| 주거 RE | 한국부동산원 R-ONE | **(주) 매매·전세가격지수**(시군구, 규모별·연령별, 수급동향) | `reb.or.kr/r-one/openapi/SttsApiTblData.do` `STATBL_ID=T244183132827305` ✅ | O | O (약관: 출처표시 조건 자유 이용) | O (출처표시) | 매주(월요일 기준, 목요일 공표) | 명시 없음 · 1회 1,000행 | 2012-05~ | X | 무료 | 하 | [R-ONE 약관](https://www.reb.or.kr/r-one/portal/intro/userAgreementPage.do) |
| 주거 RE | R-ONE | (월) 매매·전세지수, 평균·중위가격, 월세지수, 전월세전환율, 주택·아파트 거래현황, 미분양·인허가 | 같은 API, 표 738개 ✅ | O | O | O | 월 | 위와 같음 | 표마다 | X | 무료 | 하 | 위 |
| 주거 RE | R-ONE | (월) 오피스텔 매매·전세·월세지수, 수익률, 전월세전환율 | 같은 API ✅ | O | O | O | 월 | 위와 같음 | 2018~ | X | 무료 | 하 | 위 |
| 상업 RE | R-ONE | 상업용 임대동향: 오피스·중대형/소규모/집합 상가 임대가격지수·공실률·투자수익률 | 같은 API ✅ | O | O | O | 분기 | 위와 같음 | 2002~ | X | 무료 | 하 | 위 |
| 주거 RE | 한국부동산원(data.go.kr) | **공동주택 실거래가격지수** (R-ONE 오픈API 목록엔 없음) | 부동산통계 조회 서비스 ⏳ | O | O (제한 없음) | O | 월 | 기관 정책 | 2006~ | X | 무료 | 중 | [15134761](https://www.data.go.kr/data/15134761/openapi.do) |
| 공매 | 캠코 온비드(차세대) | 공고목록·공고상세 물건(재산유형·유찰횟수·주소·입찰기간·감정가)·입찰결과목록·결과상세(낙찰가) | `apis.data.go.kr/B010003/OnbidPbancListSrvc2` · `OnbidPbancCltrDtlSrvc2` · `OnbidPbancBidRsltListSrvc2/getPbancBidRsltList2` · `OnbidCltrBidRsltDtlSrvc2` ⏳ | O | O (제한 없음) | O | 매시간 | 개발 1,000/일 | 공고 단위 | X | 무료 | 중 | [15157216](https://www.data.go.kr/data/15157216/openapi.do) · [15157222](https://www.data.go.kr/data/15157222/openapi.do) · [15157254](https://www.data.go.kr/data/15157254/openapi.do) |
| 경매 | 대법원 법원경매정보 | 매각물건·감정가·최저가·유찰 | **공식 API 없음** ❔ | — | — | — | — | — | — | — | — | 상 | courtauction.go.kr |
| 매물 | 네이버부동산·직방·다방·KB·호갱노노·아실 | 호가 매물 | **공식 API 없음** · robots 차단 · 판례 §4.4 | — | X | X | — | — | — | — | 제휴 | 상 | — |
| Crypto | 업비트 | 현재가·호가·체결·분/일/주/월 캔들, WebSocket, KRW 마켓 290개 | `api.upbit.com/v1/ticker`·`candles/*`·`orderbook` ✅ | O(키 불필요) | **X** (약관 제5조) | **X** | 실시간 | REST 시세 그룹 10/초·600/분(응답 헤더 실측) | 상장 이후 전체(200개씩) | O | 무료 · 제휴 | 하(기술) / 상(약관) | [약관](https://upbit.com/open_api_agreement) · [docs](https://docs.upbit.com) |
| Crypto | 빗썸 | v1 API(업비트와 같은 응답 형식) | `api.bithumb.com/v1/ticker` ✅ | O | ❔ 약관 미확인 | ❔ | 실시간 | 문서 확인 필요 | O | O | 무료 | 하 | apidocs.bithumb.com |
| Crypto | 바이낸스 | 현재가·OHLCV·호가, WebSocket, 대량 과거 파일(Vision) | `api.binance.com/api/v3/*` ✅ · `data-api.binance.vision` ✅ | O | ❔ (공개 재배포 허용 근거 없음) | ❔ | 실시간 | weight 6,000/분(exchangeInfo 실측) | O | O | 무료 | 하 | developers.binance.com |
| Crypto | CoinGecko | 코인 메타·시총·다거래소 가격·과거 | `api.coingecko.com/api/v3/*` ✅(키 없이) | Demo O | **Demo X** / Basic 이상 O | 리셀·재배포 X, 표시는 「Powered by CoinGecko」 | 1~5분 | Demo 10,000/월(분당 한도는 문서마다 30~100) | Demo 1년, Analyst 10년 | X | Basic $35/월 | 하 | [pricing](https://www.coingecko.com/en/api/pricing) · [terms](https://www.coingecko.com/en/api_terms) |
| Crypto | CoinMarketCap | 시세·랭킹·메타 | Pro API 📄 | Basic O | **Basic = 개인용** / Startup 이상 상업 | 독립 재배포 X | 1분 | Basic 15,000/월·50/분 | 유료 플랜 | X | 유료 | 하 | [pricing](https://coinmarketcap.com/api/pricing) |
| 금·원자재 | 금융위(원천 KRX) | KRX 금시장·배출권·석유 일별 시세 | `apis.data.go.kr/1160100/service/GetGeneralProductInfoService` ⏳ | O | **X** (공공누리 4유형, 상업은 KRX Data Marketplace 유료) | **X** (상업 여부 무관 제3자 무단 제공 금지) | 일 1회(익영업일 13시) | 10,000/일 | O | X | KRX 유료 | 상 | [15094805](https://www.data.go.kr/data/15094805/openapi.do) |
| 금 | goldprice.dev | 국제 금 현물 | 기존 사용 중 | 키 | ❔ 약관 미확인 | ❔ | 90분 캐시 | — | — | — | — | — | goldprice.dev |
| 금 | FRED | LBMA 금 | **시리즈 없음** (`GOLDAMGBD228NLBM` 404 실측 — 2022 삭제) | — | — | — | — | — | — | — | — | — | — |
| 채권·금리 | 한국은행 ECOS | 기준금리, 시장금리 일별(국고채 1·2·3·5·10·20·30·50년, 회사채 AA-·BBB-, CD 91일, CP, 통안, KOFR, MMF, CMA), 환율 | `ecos.bok.or.kr/api/StatisticSearch/{KEY}/json/kr/…/817Y002/D/…` ✅ | O | 대체로 O ❔ (ECOS 자체 약관 원문 미확인 — data.go.kr의 한국은행 API는 「이용허락 제한 없음」) | 출처표시 | 일 | 명시 없음 | 수십 년 | X | 무료 | 하 | [ECOS](https://ecos.bok.or.kr/api/) |
| 채권 | 금융위(원천 KRX) | 장내 채권 시세 | `GetBondSecuritiesInfoService/getBondPriceInfo` ⏳ | O | **X** (4유형) | **X** | 일 1회 | 10,000/일 | O | X | KRX 유료 | 상 | [15094784](https://www.data.go.kr/data/15094784/openapi.do) |
| 채권 | 금융위(원천 예탁원) | 채권 기본정보(표면금리·만기·이자지급·옵션) | 채권기본정보 ⏳ | O | **X** (2유형, 예탁원 계약 필요) | 출처표시 | 일 1회 | 10,000/일 | O | X | 계약 | 상 | [15059592](https://www.data.go.kr/data/15059592/openapi.do) |
| 채권 | 금융투자협회 | 최종호가수익률·시가평가(민평) | 웹(채권정보센터) · 공개 API 못 찾음 ❔ | — | — | — | 일 | — | — | — | 평가사 유료 | 상 | kofiabond.or.kr |
| 예금 | 금감원 finlife | 예·적금·대출 공시 | 기존 사용 중 ✅ | O | **영리 시 사전 협의**(저작권보호정책) | 출처 「금융감독원」 | 월 | 명시 없음 | X | X | 무료 | 하 | [저작권정책](https://www.fss.or.kr/fss/main/contents.do?menuNo=200701) |
| REITs(상장) | 기존 주식 경로(토스·네이버·DART) | 상장 리츠 = 상장 주식 | 기존 | — | 기존 경로 약관을 따름 | — | 실시간 | — | O | O | — | 하 | — |
| 해외 주택 | FRED | Case-Shiller 미국 주택가격지수 | `CSUSHPINSA` ✅(07월분) | O | ❔ S&P 저작권 시리즈 — §10과 같은 문제 | ❔ | 월(2개월 지연) | 120/분 | 1987~ | X | 무료 | 하 | fred.stlouisfed.org |
| 미술품·자동차 | — | — | 무료 공식 API 못 찾음 ❔ | — | — | — | — | — | — | — | — | 상 | — |

---

## 3. 자산군별 요약 (요청 §3의 9개 질문)

| 자산군 | 현재가치 | 과거 | 주기 | 매물/거래 | 뉴스 연결 | 포트폴리오 편입 | 평가 방식 | 국내 소스 / 해외 소스 |
|---|---|---|---|---|---|---|---|---|
| **주거 RE** | 추정만(단일 시가 없음) | O | 일(실거래)·주(지수) | 거래 O, 매물 X | 지역·단지명 키워드(기존 네이버 뉴스) | O (수동 등록 + 추정) | 비교사례 + 지수 보정 | 국토부·R-ONE / Case-Shiller·Zillow |
| **Crypto** | O | O | 실시간 | 거래소 체결 O | 코인명·티커 | O (`stocks`에 시장가 자산으로) | 시장가 | 업비트·빗썸 / CoinGecko·바이낸스 |
| **금·원자재** | O(국제)/O(KRX, 상업 X) | O | 일·분 | KRX 금시장 | 「금값」 키워드 | O (그램·온스 단위) | 시장가 × 수량, 실물은 부가세·스프레드 차감 | KRX(유료) / goldprice.dev |
| **채권** | 장내 시세(상업 X), 민평(유료) | O | 일 | 장내 | 발행사 뉴스 | O | 시장가 또는 만기보유: 원금+경과이자 | ECOS 금리곡선 · KRX · KOFIA / FRED |
| **예금·현금** | 원금+경과이자(계산) | — | — | — | 금리 뉴스(기준금리) | O (이미 현금 칸 있음) | 고정수익 공식 | finlife · ECOS |
| **상업용 RE** | 거래 사례 희소 → 지수 | O | 분기 | 매매 O(⏳) | 지역 | O (수동) | 임대수익 환원 or 사용자 입력 | R-ONE 임대동향 · 국토부 상업업무용 |
| **REITs** | O(상장) | O | 실시간 | 상장 | 기존 종목 뉴스 그대로 | **이미 됨**(주식 경로) | 시장가 + 배당 | 기존 |
| **공매** | 감정가·최저가·낙찰가 | 결과 O | 매시간 | 물건 O | 약함 | 보유 자산 아님(관심 목록) | — | 온비드(⏳) |
| **경매** | — | — | — | 공식 API 없음 | — | — | — | 대법원(API 없음) |
| **미술품·자동차** | 사용자 입력 | X | — | X | X | O (수동) | 사용자 추정 + 감가상각(자동차) | 없음 |

---

## 4. 부동산 상세

### 4.1 거래 데이터 — 국토부 실거래가 (공공데이터포털)

공통: `GET https://apis.data.go.kr/1613000/RTMSDataSvc{종류}/getRTMSDataSvc{종류}?serviceKey=&LAWD_CD=(법정동 앞 5자리)&DEAL_YMD=(YYYYMM)&pageNo=&numOfRows=`, XML.
**API마다 따로 「활용신청」**해야 한다(같은 키라도 신청 안 한 API는 `SERVICE_KEY_IS_NOT_REGISTERED_ERROR`).

| 종류 | 상태(10-01) | 실측 메모 |
|---|---|---|
| `AptTradeDev` 아파트 매매 상세 | ✅ | `aptSeq`(단지 키, 예 11680-4394)·`bonbun/bubun`·도로명·`excluUseAr`·`floor`·`dealingGbn`(중개/직거래)·`cdealType/cdealDay`(해제)·`rgstDate`(등기일)·`buyerGbn/slerGbn`. 강남구 2026-08 = 96건 |
| `AptTrade` 아파트 매매 | ✅ | 상세의 부분집합 — **상세만 쓰면 된다** |
| `AptRent` 아파트 전월세 | ✅ | `deposit`·`monthlyRent`·`contractTerm`·`contractType`(신규/갱신)·`preDeposit`(종전)·`useRRRight`(갱신요구권). 강남구 2026-08 = 1,130건 |
| `RHTrade`/`RHRent` 연립다세대 | ✅ | `houseType`·`landAr`(대지권) |
| `OffiTrade`/`OffiRent` 오피스텔 | ✅ | |
| `SilvTrade` 분양·입주권 | ✅ | `ownershipGbn`(분/입) |
| `SHTrade`/`SHRent` 단독다가구 · `LandTrade` 토지 · `NrgTrade` 상업업무용 · `InduTrade` 공장창고 | ⏳ 활용신청 필요 | 문서상 같은 형식. 단독·토지·상업은 **지번 일부 비공개**(개인정보) — 단지 단위 비교가 안 되므로 추정가는 「지역·면적 단가」 수준까지만 |

주의: 신고 기한이 계약 후 **30일**이라 이번 달·지난달 건수는 계속 늘어난다(재수집 필요). 해제 거래는 `cdealType`으로 걸러야 한다.
**물량**: 시군구 ~250 × 월 1호출이면 한 종류 1년치 = 약 3,000호출 — 개발 계정 하루 한도(10,000) 안이다.

### 4.2 시장 지표 — 한국부동산원 R-ONE (✅ 전부)

`GET https://www.reb.or.kr/r-one/openapi/SttsApiTblData.do?KEY=&Type=json&STATBL_ID=&DTACYCLE_CD=(WK|MM|QY)&WRTTIME_IDTFR_ID=(202639 등)&CLS_ID=(지역)&pIndex=&pSize=(≤1000)`
표 목록: `SttsApiTbl.do`(738개). `CLS_ID`로 지역 하나의 전체 시계열을 한 번에 받는다(강남구 주간 749주 = 1호출).

| 지표 | STATBL_ID | 주기 |
|---|---|---|
| (주) 아파트 매매가격지수 | `T244183132827305` | 주 |
| (주) 전세가격지수 | `T247713133046872` | 주 |
| (주) 매매/전세 수급동향 | `T248163133074619` / `T245423133086632` | 주 |
| (월) 지역별 매매지수_아파트 / 전세지수 | `A_2024_00178` / `A_2024_00182` | 월 |
| (월) 매매 평균·중위가격_아파트 | `A_2024_00188` / `A_2024_00189` | 월 |
| 주택유형별 월세가격지수 | `A_2024_00163`~`00166` | 월 |
| 지역별 전월세 전환율 | `A_2024_00155`·`00156` | 월 |
| (월) 행정구역별 아파트 매매거래현황(거래량) | `A_2024_00554` | 월 |
| 오피스텔 매매·전세·월세지수 / 수익률 | `A_2024_00615`·`00618`·`00621` / `T245503133561624` | 월 |
| 상업용 임대가격지수(오피스·상가) · 공실률 | `TT244233134228593`… / `TT244763134428698`… | 분기 |
| 미분양 · 인허가 · 착공 · 준공 | `T237973129847263` 등 | 월 |

실측: 2026-09-21 주 전국 100.98 · 서울 102.53 · 강남구 98.63(3주 연속 하락). **공동주택 실거래가격지수는 R-ONE 오픈API 목록에 없다** → data.go.kr 「한국부동산원_부동산통계 조회 서비스」(⏳).

### 4.3 공매·경매

- **온비드(차세대, 2026-01 신설)**: 공고목록 → 공고상세(물건: 재산유형·처분방식·용도·**유찰횟수**·주소·입찰기간·감정가) →
  입찰결과목록 → 결과상세(낙찰/유찰/취소, **낙찰가**). 필수값: 물건유형 `cltrTypeCd`·재산유형 `prptDivCd`·개찰일 범위. 매시간 갱신, 개발 1,000/일,
  **이용허락 제한 없음**. 구 `openapi.onbid.co.kr` 경로는 응답 없음(실측) — 차세대만 쓸 것. 캠코는 2025-11 민간 경·공매 플랫폼 9곳과 데이터 공유 협약도 맺었다.
- **법원경매**: 대법원이 공개 API를 내지 않는다. 사설 플랫폼(탱크옥션 등)은 별도 계약·자체 수집. **Quant-port는 넣지 않는 것을 권한다**(공식 경로 없음).

### 4.4 매물 데이터 — 쓸 수 없다

| 서비스 | 공식 API | robots.txt(10-01 실측) | 판단 |
|---|---|---|---|
| 네이버부동산 | 없음 | `land.naver.com`: 전체 Disallow · `fin.land.naver.com`: 대부분 Allow(검색엔진용) | **특허법원 2025-12-24 네이버 v. 다윈중개** — 매물 24개 항목 크롤링을 DB권 침해로 인정, 8천만 원 + 데이터 삭제. 허위매물 검증에 약 10억 원 투자를 「상당한 투자」로 봄 |
| 직방 | 없음 | CloudFront 403(봇 차단) | 제휴 외 경로 없음 |
| 다방 | 없음(LH 등 기관 제휴 사례만) | `/room`·`/complex`·`/search` Disallow | 제휴 외 경로 없음 |
| KB부동산 | 없음 | 구글·빙 외 전체 Disallow | KB시세는 유료 데이터 상품 |
| 호갱노노 | 없음 | 「명시적 서면 허락 없는 크롤링 금지」 | — |
| 아실 | 없음 | 전체 Disallow, AI 봇 명시 차단 | — |

법리: 저작권법 DB제작자 권리(제93조) + 부정경쟁방지법(데이터 부정사용). 여기어때 사건은 **형사 무죄(대법 2021도1533)지만
민사는 10억 원 배상** — 「형사로 안 걸린다」가 「해도 된다」가 아니다. 결론: **매물은 제휴 없이는 넣지 않는다.**
대안 = 실거래·전월세(공식)로 「시세」를 만들고, 매물은 해당 플랫폼으로 **링크만**(딥링크 자체는 문제 소지가 낮음).

---

## 5. Crypto 비교 · 업비트 MVP 판단

| | 업비트 | 빗썸 | 바이낸스 | CoinGecko | CMC |
|---|---|---|---|---|---|
| 현재가/OHLCV/호가 | O/O/O | O/O/O | O/O/O | O/O/X | O/유료/X |
| 거래소별 가격 | 자기 | 자기 | 자기 | **다거래소** | 다거래소 |
| WebSocket | O | O | O | X | X |
| 과거 | 상장 이후(200개씩) | O | O + 대량 파일 | Demo 1년 | 유료 |
| 한도 | 시세 10/초(실측) | 문서 확인 | weight 6,000/분(실측) | Demo 10k/월 | Basic 15k/월 |
| 키 | 시세 불필요 | 불필요 | 불필요 | Demo 키 | 키 |
| **상업 이용** | **X**(약관 제5조) | ❔ | ❔ | Basic $35/월~ | Startup~ |
| 표시 의무 | — | — | — | 「Powered by CoinGecko」 | 출처 |

업비트 약관 제5조(실문): 「Open API 서비스 … 모든 데이터 및 내용에 대한 저작권은 두나무에 있으므로 사용자가 이를 무단으로 사용, 복제, 배포, 변경하거나 영리목적으로 이용하는 것은 금지」.
**판단**: 기술적으로는 업비트가 최선(원화 가격 = 국내 사용자의 실제 체결 기준, 이미 클라이언트 있음).
그러나 다수 사용자에게 시세를 보여 주는 공개 서비스는 「배포」에 가깝다 → **두나무 제휴 문의가 1순위**, 안 되면 **CoinGecko Basic**(원화 환산 가격, 상업 라이선스, 출처 표시).
보유 평가만 하고 시세 자체를 노출하지 않는 설계(「평가금액만 보여 줌」)도 「사용」에는 해당하므로 회피책이 아니다.
바이낸스는 국내 미신고 거래소라 국내 사용자 대상 화면의 가격 기준으로 쓰기 어렵다(김치프리미엄 계산용 참고값 정도). 실측 10-01: 업비트 113,890,000원 vs 바이낸스 83,663 USDT × 1,352.8 = 113,179,000원 → +0.6%.

---

## 6. 부동산 추정가 MVP — 실측 PoC

방법(가장 단순한 것): **같은 단지(`aptSeq`) · 전용 ±3㎡ · 최근 12개월 · 해제·직거래 제외** → 각 거래가를
**구 주간 매매지수로 오늘 시점에 보정**(가격 × 지수_오늘 / 지수_거래주) → **중앙값 = 추정가, 25~75% = 범위**.
신뢰도: 높음 = 비교 ≥5건 · 최근 거래 ≤90일 · 범위 폭 ≤8% / 보통 = ≥2건 · ≤180일 / 그 외 낮음. PoC는 일회성 node 스크립트(단지 하나에 호출 26회·1.7초)였고 저장소에 넣지 않았다 — 2단계에서 Go 배치로 다시 짠다.

| 단지 (84㎡) | 비교 | 최근 실거래 | 최근 3개월 평균 | 동일면적 중앙값 | **추정 현재가 (범위)** | 신뢰도 | 전세 중앙값(6개월) |
|---|---|---|---|---|---|---|---|
| 래미안대치팰리스(강남 대치) | 8건 | 44.00억 (120일 전) | — | 42.75억 | **42.80억** (41.22~43.72) | 보통 | 18.19억 (전세가율 42%) |
| 헬리오시티(송파 가락) | 92건 | 29.00억 (47일 전) | 29.68억 (6건) | 28.60억 | **29.55억** (28.83~31.36) | 보통(폭 8.5%) | 11.53억 (39%) |
| 마포래미안푸르지오4단지(마포 아현) | 27건 | 25.20억 (27일 전) | 25.33억 (3건) | 24.80억 | **26.01억** (25.24~26.18) | 높음 | 10.50억 (40%) |

PoC에서 배운 것 (설계에 반영할 것):
- **직거래는 반드시 뺀다** — 래미안대치팰리스의 25억 직거래(가족 간 추정)가 중앙값을 흔들었다.
- **지수 보정은 「틈」만 메워야 한다** — 마포는 구 지수가 오른 만큼 오래된 거래를 올려 26.01억이 됐지만, 최근 실제 거래는 25.2~25.3억.
  → 규칙: **최근 6개월 비교가 5건 이상이면 그 창만** 쓰고 지수 보정은 최소화, 모자랄 때만 12개월 + 지수.
- (구현 중 추가) 거래가 많은 단지에선 6개월 중앙값도 오르는 장에서 뒤처졌다(헬리오시티 28.1억 vs 최근 3개월 평균 29.7억) →
  **3개월에 5건이면 3개월 → 6개월에 5건이면 6개월 → 아니면 12개월 + 지수** 순으로 창을 고른다.
- 층·향·동 차이는 반영 안 했다(저층 할인 등). 첫 판은 범위로 보여 주고, 층 보정은 사용자 피드백 뒤.
- 거래가 0건인 단지 → 같은 동·비슷한 연식·면적의 ㎡당 가격 × 면적 × 지수 → 신뢰도 「낮음」으로만.

화면 원칙(요청 §11~12): 네 값은 **이름부터 다르게** 쓴다.
`시장가`(체결 시세 — 주식·코인) · `최근 실거래가`(사실, 날짜와 함께) · `추정가`(계산, 범위·근거·신뢰도와 함께) · `직접 입력가`(사용자).
추정가 카드에는 「근거: 동일 단지·면적 실거래 N건 · 최근 거래 D일 전 · ○○구 주간 지수 반영 · 계산일」을 늘 붙인다.
평가손익은 추정가 기준임을 라벨로 밝히고, 주식 손익과 한 숫자로 합칠 때 「추정 포함」 표시.

---

## 7. 데이터 모델 개선안 (최소 변경)

원칙: **시장가 자산은 지금 구조 그대로**, 없는 개념(비시장 자산·평가 이력·부채)만 더한다. 요청의 `Asset` 상위 표로 주식까지 옮기는 리팩터는
얻는 것보다 깨지는 것이 많다(`stocks`를 참조하는 곳이 점수·뉴스·커뮤니티·스크리너 전부).

```text
시장가 자산 (현재 경로)          비시장 자산 (신규)                 부채 (신규)
stocks.asset_type                user_assets                        user_liabilities
  domestic | us | etf |            id, user_id, asset_type            id, user_id, asset_id NULL
  reit(상장, 기존 그대로) |         (real_estate|deposit|bond_htm|      (담보 자산), kind(mortgage|
  crypto ← 새 값                    gold_physical|vehicle|art|other)    jeonse_deposit|credit|other),
user_holdings / holding_trades     name, currency, acquired_at,       principal, balance, rate,
  (symbol 키, 수량·평단·이력)        acquired_price, ownership_ratio,   rate_type, repayment_type,
                                   valuation_method, detail JSON      started_at, maturity_at
                                   (주소·aptSeq·면적·층 / 금리·만기…)
                                 asset_valuations
                                   asset_id, as_of, value, kind
                                   (market|last_trade|estimate|
                                    user_input|accrual),
                                   source, confidence, low, high,
                                   inputs JSON(비교 거래 id·지수값)
```

- Crypto는 `stocks`에 `asset_type='crypto'`, `market='UPBIT'`, `symbol='KRW-BTC'`로 넣으면 보유·매매이력·차트·뉴스 태깅이 그대로 돈다.
- 전세 **보증금을 받은 집주인**은 부채(`jeonse_deposit`), **세입자**는 자산(`user_assets` deposit 유사)이다 — 한국 Net Worth의 핵심 차이.
- `asset_valuations`는 행을 지우지 않고 쌓는다 → 추정가 추이 차트와 「왜 바뀌었나」(inputs) 설명이 공짜로 나온다.
- 탈퇴 시 `UserRepo.Delete` 소유 표 목록에 세 표 추가(금융상품 때와 같은 패턴).

## 8. Valuation Engine · Net Worth · Total Return

| valuation_method | 대상 | 식 | 기록 kind |
|---|---|---|---|
| market_price | 주식·ETF·REIT·crypto | 시세 × 수량 | market |
| comparable_index | 아파트·오피스텔·연립 | §6 (비교 중앙값, 지수로 틈 보정) | estimate (+범위·신뢰도) |
| last_trade | 단독·토지·상업(사례 희소) | 지역 ㎡단가 × 면적 × 지수 | estimate(낮음) |
| fixed_income | 예금·적금·만기보유 채권 | 원금 + 경과이자(단리/복리, 세후 옵션) | accrual |
| market_or_accrual | 장내 채권 | 시세가 있으면 시세, 없으면 ECOS 같은 만기 금리로 할인 | market / estimate |
| user_estimated | 미술품·자동차·기타 | 사용자 값(자동차는 선택적 연 감가율) | user_input |

**Net Worth** = Σ 시장가 자산 + 현금 + Σ 최신 `asset_valuations` × 지분율 − Σ 부채 잔액.
화면은 「총자산 · 부채 · 순자산」 세 줄 + 자산군 비중(주식/ETF/부동산/예금/Crypto/금/기타). 부동산 1채가 비중의 50%를 넘는 게 보통이라
비중 차트는 「부동산 포함/제외」 토글이 있어야 투자 포트폴리오 분석(기존 진단·위험 카드)이 무의미해지지 않는다.

**Total Return** (요청 §15 — 문서가 Equity에서 끊겨 나머지는 제안):

| 자산 | 수익 = | 비고 |
|---|---|---|
| Equity/ETF/REIT | 가격 변화 + 배당(현금흐름) | 이미 `holding_trades`에 매수·매도, 배당만 추가 |
| Bond | 가격 변화 + 쿠폰 + 경과이자 | 만기보유면 YTM |
| Deposit | 경과이자(세후) | finlife 금리를 기본값으로 제안 |
| Real estate | (추정가 − 취득가 − 취득세·중개비) + 순임대수익 | 대출이 있으면 **자기자본 수익률** = (가치 변화 + 임대 − 이자) / 자기자본 |
| Crypto | 가격 변화 (+ 스테이킹 보상) | |
| Gold | 가격 변화 | 실물은 매입 시 부가세 10%·스프레드 |

포트폴리오 전체는 두 숫자를 같이: **TWR**(시간가중, 벤치마크 비교용) · **MWR/XIRR**(내 돈 기준 실제 수익률).
현금흐름(매수·매도·배당·이자·임대·상환)을 한 원장으로 모으는 게 엔진의 전부다 — `holding_trades`를 일반화한 `asset_cashflows` 하나면 된다(2단계).

## 9. 단계별 제안 (UI는 페이지별로 다시 제안 → 승인 뒤 착수)

| 단계 | 내용 | 외부 데이터 | 결정 필요 |
|---|---|---|---|
| 0 | 라이선스 정리(§10) | — | 업비트 제휴 문의 · CoinGecko 유료 · 지수 띠 S&P/다우 |
| 1 | **Net Worth 골격**: `user_assets`·`asset_valuations`·`user_liabilities` + 수동 입력(부동산·예금·대출·기타) + 「총자산/부채/순자산」 | 없음 | 메뉴 위치(보유종목 확장 vs 새 「자산」) |
| 2 | **아파트 추정가**: 단지 검색(aptSeq) → 등록 → 배치가 **등록된 단지의 시군구만** 매매·전월세를 주 1회 적재 + R-ONE 주간 지수 → 추정가·전세 시세·신뢰도 | 국토부(✅)·R-ONE(✅) | 조회 시 외부 호출 0(배치 적재 원칙) |
| 3 | 금리·채권: ECOS 금리곡선(예금·채권 경과이자·할인율), 기준금리 이벤트를 뉴스·변화 피드에 | ECOS(✅) | — |
| 4 | Crypto 보유(라이선스 해결 시) | 업비트 제휴 or CoinGecko | 0단계 결과 |
| 5 | 오피스텔·연립·단독·토지·상업 추정, 온비드 공매 관심 목록 | ⏳ 활용신청 | — |

운영 메모(인프라 메모리 기준): DB가 작으므로 전국 전량 적재는 하지 않는다. 시군구-월 1호출이라 「등록 지역만」이면 하루 수십 호출이다.

## 10. 기존 시스템 개선점 — 이번 조사에서 찾은 것

| # | 무엇 | 근거 | 제안 | 상태 |
|---|---|---|---|---|
| 1 | 지수 띠 **S&P 500·다우**(FRED `SP500`·`DJIA`) | FRED 시리즈 노트: 「Reproduction of S&P 500 in any form is prohibited except with the prior written permission of S&P Dow Jones Indices」 | 지수 대신 추종 ETF(SPY·DIA 등, 기존 시세 경로) 표시, 또는 라이선스 | **결정 필요** |
| 2 | 지수 띠 **나스닥·VIX**(FRED) | 노트에 NASDAQ OMX·CBOE 저작권 표기 | 1과 함께 검토 | 결정 필요 |
| 3 | 지수 띠 **비트코인**(업비트 ticker) | 업비트 Open API 약관 제5조 | 제휴 문의 or CoinGecko 유료 | 결정 필요 |
| 4 | ETF/ETN 카탈로그(KRX Open API) | KRX 약관: 비상업만 · 「제공 데이터를 제3자에게 제공 불가」 · 결과 페이지에 「KRX 통계정보」 표시 의무 · 키당 10,000/일 | 최소한 출처 표시, 장기적으로 다른 카탈로그 경로 | 결정 필요 |
| 5 | 금감원 finlife(금융상품) | 저작권보호정책: 영리 목적은 사전 협의 | 유료화·광고 전 문의. `FINANCE_PRODUCTS.md` §3·§8 갱신함 | **문서 반영** |
| 6 | 금 시세 goldprice.dev | 약관 미확인 | 약관 확인. 국제 금은 무료·상업 가능한 경로가 드묾(FRED LBMA는 삭제됨) | 확인 필요 |
| 7 | `.env.example`에 `FSS_FINLIFE_API_KEY` 누락 | 코드는 읽는데 예시에 없음 | 추가 + 새 키 3개 자리 | **고침** |

공공데이터법 제3조는 공공데이터의 영리 이용을 원칙적으로 허용하지만 **제3자 권리가 붙은 데이터(KRX·예탁원 원천)는 예외**다 —
그래서 같은 data.go.kr 안에서도 국토부·캠코는 「제한 없음」, 금융위(KRX 원천)는 4유형이다. 새 소스는 「어느 기관 API냐」가 아니라 「원천이 누구냐」로 판단할 것.

## 11. 남은 확인 · 사용자 몫

- [ ] data.go.kr 활용신청(10-01 16:40 기준 미반영): 단독다가구 매매·전월세, 토지, 상업업무용, 공장창고, 온비드 차세대 4종, 한국부동산원 부동산통계 조회 서비스. 반영되면 §2의 ⏳를 다시 호출해 확인.
- [ ] 빗썸·바이낸스·ECOS 약관 원문(이번엔 못 읽음), goldprice.dev 약관.
- [ ] §10 1~4 결정. 업비트 제휴는 사용자(운영 주체) 명의로 문의해야 한다.
- [ ] 1단계 UI 제안서(페이지별 컴포넌트) — 승인 뒤 착수.

## 12. 구현 (2026-10-01 사용자 결정 「전체 개발 일괄」)

**라이선스 결정**: ① 지수 띠에 FRED 권리자 표기(S&P·다우 © S&P Dow Jones Indices · 나스닥 © Nasdaq · VIX © Cboe) ② 비트코인은 업비트 그대로,
상업 제휴·유료 구매는 하지 않음 ③ ETF 점검 카드·랭킹에 「출처: KRX 통계정보」 ④ 지수는 그대로 두고 **추종 ETF(SPY·DIA) 시세를 지수 띠에 추가**
(기존 QQQ·SOXX와 같은 Finnhub 배치, 이력은 `stock_daily_closes`). ETF 퀵룩의 뉴스는 기초지수 기사.

| 층 | 무엇 |
|---|---|
| 스키마 | `096_wealth.sql` — `user_assets`·`asset_valuations`·`user_liabilities` · `re_regions`·`re_trades`·`re_sync_state`·`re_index` · `macro_rates` · `onbid_items`·`onbid_notices` |
| 적재(worker-asset, 2단계에서 이전) | `re_trades` 03:30 · `re_index` 06:00 · `onbid` 07:40 · `ecos_rates` 09:20 (`service/public_data_sync.go`). 버킷 `molit_rtms`·`reb_rone`·`bok_ecos`·`kamco_onbid`·`mois_region`·`upbit_board` |
| 평가 | `service/wealth_valuation.go`(순수 함수 + 시험) · `service/wealth.go`(WealthValuer). 부동산 비교사례(§6 규칙) → 없으면 같은 동 ㎡ 단가(신뢰도 낮음) → 없으면 직접 입력. 예금·적금 원리금(세후), 채권 = 국고채 곡선 보간 + 신용 스프레드 할인, 코인 = 업비트 원화 전체 시세(1분 캐시), 금 = 국제 현물 × 원/달러(지수 일별 종가 표), 자동차 = 직접 입력 × 연 감가. 매일 `re_trades` 끝에 **전 자산 재평가**(`asset_valuations` 하루 한 줄 → 추이) |
| API | 보호 `/wealth`·`/wealth/assets[/{id}]`·`/wealth/liabilities[/{id}]` / 공개 `/realestate/{regions,complexes,complex,market}`·`/macro/rates`·`/auctions/onbid[/stats]`·`/crypto/markets` (`handler/wealth.go`, 입력 검증은 4xx) |
| 화면 | 사이드바 「자산」: 순자산 `/wealth`(총자산·부채·순자산, 자산 구성 막대(부동산 포함/제외), 자산·부채 목록, 평가손익, 금리 카드) · 자산 상세 `/wealth/assets/[id]`(추정가·최근 실거래·3개월 평균·동일 면적 중앙값·전세 시세 + 근거·비교 거래·실거래 점그래프·평가 추이·지역 시장) · 부동산 시세 `/wealth/real-estate` · 부동산 공매 `/wealth/auctions` · 금리 `/wealth/rates` |

**지역 범위**: 법정동코드 API(행안부, 활용신청 대기)가 열리면 전국 시군구의 아파트 매매·전월세를, 그 전까지는 서울 25구만. 12종 전부는 서울 + 사용자가 부동산을
등록한 지역(등록 즉시 `full`로 올림). 하루 호출 상한 6,000이라 첫 바퀴(전국 13개월)는 며칠에 나눠 채워진다.

**남은 것(다음 단계 후보)**: 현금흐름 원장(배당·이자·임대료)과 TWR/XIRR — 지금 손익은 가격 차익만 · 공매 관심 목록 · 법정동 단위 지번 매칭(단독·토지 추정 정밀화) ·
층 보정 · 순자산 일별 스냅샷(주식 포함 추이).

## 13. 2단계 (2026-10-01~02 요청: 포트폴리오 통합 · 벤치마킹 · 점수·AI · 워커 통합)

**메뉴**: 「순자산」을 없애고 **「내 포트폴리오」(/holdings) 하나**로 합쳤다. 맨 위 「내 자산 전체」(총자산·부채·순자산·자산 구성, 부동산 포함/제외)와
「자산 추가」 메뉴(주식·부동산·예금·적금·채권·코인·금·자동차·기타·대출), 그 아래 주식 목록 + 부동산·금융상품 목록 + 부채 + 금리.
예·적금·대출 입력에서 **금감원 공시 상품을 고르면 금융회사·금리·기간이 채워진다**. 옛 /wealth/* 주소는 새 주소로 넘긴다.
부동산 시세 `/real-estate` · 공매 `/real-estate/auctions` · 자산 상세 `/holdings/assets/[id]` · 금리 `/finance/rates`.

**벤치마킹으로 바꾼 것**
| 화면 | 참고 | 바꾼 것 |
|---|---|---|
| 부동산 시세 | 호갱노노·아실·네이버부동산 | 시도→시군구 칩 · 전국 지역표(1·4·12주 지수, 거래량 추세, 점수 정렬) · 지역 화면(점수·AI 한 줄 + 단지 랭킹 5종: 점수·많이 오른·거래 많은·전세가율·평당가 + 이름 검색) · 단지 화면(평형 칩, 추정가·최근 실거래·기간 최고가·전세 시세/전세가율/갭 요약, 1년/전체 실거래 점그래프, 실거래표에 최고가·해제·직거래 표시, 전월세표, 「내 포트폴리오에 등록」). 주소에 `?lawd=&complex=` |
| 공매 | 온비드·탱크옥션·지지옥션 | 표 → 카드(지역·용도·유찰 칩, 주의 표지, 점수, D-day, 감정가 대비 최저가 막대) · 칩 필터 · 정렬 5종(마감·점수·할인·유찰·감정가) · **회차 이력**(최저가가 회차마다 어떻게 내려왔나) |
| 예·적금·대출 | 뱅크샐러드·토스·네이버 | 최고·기본금리를 크게 · 지난 공시월 대비 ▲▼ · 점수 배지 · **우대조건 AI 정리**(난이도 쉬움/보통/어려움 + 조건 칩 +가산 %p) · 정렬(받는 이자·최고금리·점수) · 「우대조건 쉬운 것만」 |
| 홈 | — | 금리(bp 변화·AI 한 줄) · 아파트 시장(전국·서울 주간 지수, 4주 상승 지역) · 정기예금 TOP3 · 공매 마감 임박 |
| 지수 띠 | — | 한국 기준금리 · 국고채 3년·10년(ECOS) 추가, 금리 항목은 `%`·bp로 표시 |

**점수(서버 규칙, `asset_scores` 09:50)** — 같은 묶음 안 백분위 가중. 「추천이 아니라 시장 지표」라는 말을 배지 툴팁·화면 각주에 늘 붙인다.
단지(모멘텀 35·거래 25·전세 받침 20(80% 넘으면 감점)·고점 위치 20) · 시군구(4주 30·12주 30·거래량 20·전세 20) · 예·적금(기본 50·최고 15·우대 의존 15·가입 제한 10·권역 10) ·
대출(금리 60·폭 20·권역 20) · 공매(할인 60·유찰 20(1~3회 가점, 6회 넘으면 감점)·주의 표지 20).

**AI(`asset_ai` 10:20)** — 주식 LLM과 겹치지 않게 **별도 키 `GEMINI_SUB_API_KEY`(버킷 `llm_sub`) + 기능 버킷 `llm_feat_asset`(120콜·40만 토큰/일)** 둘 다 통과해야 부른다.
토큰 절약: 사실표만 압축 JSON(짧은 키), 여러 건을 한 콜에(지역·단지 6, 상품 8), `max_tokens` 상한, **입력 해시가 같으면 건너뜀**(바뀐 것만 다시).
금리 1콜 · 지역/단지 한 줄 · 예·적금 우대조건 구조화. 실패·한도면 지역·단지·금리는 규칙 문장(「지표 요약」으로 표시), 우대조건은 다음 회차에.

**워커 통합** — 9개 → **4개**: worker-market · worker-stock · worker-asset · worker-content(API 바이너리 `SERVER_ROLE=batch`). api는 `SERVER_ROLE=api`.
바이너리 하나(`cmd/worker`)가 `WORKER_ROLES`의 역할을 한 프로세스에서 돌려 MySQL·Redis 연결과 메모리를 나눠 쓴다.

**최초 일괄 적재(2026-10-02 사용자 결정)**: 실거래 **전국 12종**(전에는 서울만 12종·전국 아파트 계획) + **아파트 매매 3년**(보관 48개월).
첫 적재 약 4.5만 호출(API마다 개발계정 1만 안, 0.26초 간격 약 3~4시간), 평소 하루 약 1만. 공매는 개발계정(오퍼레이션당 일 1,000)이라
일괄이 안 되고 매일 900씩 — 늘리려면 공공데이터포털 **운영계정 신청**(활용사례 등록, 사용자 몫). 금융상품은 이미 전 상품(1,034)이고
금리 이력은 금감원 API가 당월만 줘서 소급 불가(이제부터 월별로 쌓임). R-ONE·ECOS는 이미 전체 이력.
AI는 전용 키 한도의 절반(하루 250콜) 안에서 전국 시군구·시군구별 상위 5단지·우대조건 — 첫 채우기 이틀, 이후 바뀐 것만.

**다음 후보**: 단지 좌표(지도) — 국토부 실거래엔 좌표가 없어 건축물대장·주소 API 활용신청이 필요 · 공매 관심 목록·마감 알림 ·
지역별 「예금 금리 vs 대출 금리 vs 전세가율」 같은 교차 지표 · 우대조건 난이도를 사용자 조건(급여이체 여부 등)으로 거르기.

### 출처 (공식)
- 국토부 실거래가: [아파트 매매 상세 15126468](https://www.data.go.kr/data/15126468/openapi.do) · [전월세 15126474](https://www.data.go.kr/data/15126474/openapi.do)
- 한국부동산원: [R-ONE 이용약관](https://www.reb.or.kr/r-one/portal/intro/userAgreementPage.do) · [부동산통계 조회 서비스 15134761](https://www.data.go.kr/data/15134761/openapi.do)
- 캠코 온비드 차세대: [공고목록 15157216](https://www.data.go.kr/data/15157216/openapi.do) · [공고상세 물건 15157220](https://www.data.go.kr/data/15157220/openapi.do) · [입찰결과목록 15157222](https://www.data.go.kr/data/15157222/openapi.do) · [결과상세 15157254](https://www.data.go.kr/data/15157254/openapi.do)
- 금융위: [일반상품시세 15094805](https://www.data.go.kr/data/15094805/openapi.do) · [채권시세 15094784](https://www.data.go.kr/data/15094784/openapi.do) · [채권기본정보 15059592](https://www.data.go.kr/catalog/15059592/openapi.json)
- [금감원 저작권보호정책](https://www.fss.or.kr/fss/main/contents.do?menuNo=200701) · [KRX Open API 약관](https://openapi.krx.co.kr/contents/OPP/INFO/OPPINFO005.jsp) · [ECOS Open API](https://ecos.bok.or.kr/api/)
- [업비트 Open API 이용약관](https://upbit.com/open_api_agreement) · [CoinGecko API Terms](https://www.coingecko.com/en/api_terms) · [CoinGecko 요금제](https://www.coingecko.com/en/api/pricing) · [CoinMarketCap 요금제](https://coinmarketcap.com/api/pricing)
- 판례 보도: [네이버 v. 다윈중개 항소심(한국경제)](https://www.hankyung.com/article/202601040488i) · [여기어때 대법원 무죄(비즈한국)](https://bizhankook.com/articles/27377.html)
