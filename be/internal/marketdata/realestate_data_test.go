package marketdata

import "testing"

func TestParseRTMSXML(t *testing.T) {
	body := []byte(`<?xml version="1.0" encoding="utf-8" standalone="yes"?><response><header><resultCode>000</resultCode><resultMsg>OK</resultMsg></header><body><items>` +
		`<item><aptDong> </aptDong><aptNm>한양2</aptNm><aptSeq>11680-379</aptSeq><buildYear>1978</buildYear><cdealDay> </cdealDay><cdealType> </cdealType><dealAmount>790,000</dealAmount><dealDay>29</dealDay><dealMonth>8</dealMonth><dealYear>2026</dealYear><dealingGbn>중개거래</dealingGbn><excluUseAr>147.41</excluUseAr><floor>12</floor><jibun>493</jibun><sggCd>11680</sggCd><umdNm>압구정동</umdNm></item>` +
		`<item><aptNm>상록수</aptNm><dealAmount>100,000</dealAmount><dealDay>3</dealDay><dealMonth>8</dealMonth><dealYear>2026</dealYear><dealingGbn>직거래</dealingGbn><cdealType>O</cdealType><excluUseAr>74.5</excluUseAr><sggCd>11680</sggCd><umdNm>일원동</umdNm><jibun>734</jibun></item>` +
		`</items><numOfRows>10</numOfRows><pageNo>1</pageNo><totalCount>96</totalCount></body></response>`)
	items, h, err := parseRTMSXML(body)
	if err != nil || h.code != "000" || h.total != 96 || len(items) != 2 {
		t.Fatalf("header %+v items %d err %v", h, len(items), err)
	}
	r, ok := rtmsRow(RTMSKind{"apt", "sale", "AptTradeDev"}, items[0])
	if !ok || *r.Price != 790000 || r.Area != 147.41 || *r.Floor != 12 || r.ComplexKey != "11680-379" || r.Direct || r.Cancelled {
		t.Fatalf("row %+v", r)
	}
	if r.DealDate.Format("2006-01-02") != "2026-08-29" {
		t.Fatalf("date %v", r.DealDate)
	}
	r2, _ := rtmsRow(RTMSKind{"apt", "sale", "AptTradeDev"}, items[1])
	if !r2.Direct || !r2.Cancelled || r2.ComplexKey != "11680|일원동|734|상록수" {
		t.Fatalf("row2 %+v", r2)
	}
	// 활용신청 안 된 키 — 오류 머리글.
	_, h, _ = parseRTMSXML([]byte(`<OpenAPI_ServiceResponse><cmmMsgHeader><errMsg>SERVICE_KEY_IS_NOT_REGISTERED_ERROR</errMsg><returnAuthMsg>등록되지 않은 서비스키</returnAuthMsg></cmmMsgHeader></OpenAPI_ServiceResponse>`))
	if h.authMsg == "" {
		t.Fatal("auth error not detected")
	}
}

func TestPickSigungu(t *testing.T) {
	got := pickSigungu([]rawRegion{
		{"4111000000", "110", "000", "00", "경기도 수원시"},
		{"4111100000", "111", "000", "00", "경기도 수원시 장안구"},
		{"1168000000", "680", "000", "00", "서울특별시 강남구"},
		{"1168010100", "680", "101", "00", "서울특별시 강남구 역삼동"},
		{"1100000000", "000", "000", "00", "서울특별시"},
	})
	codes := map[string]string{}
	for _, r := range got {
		codes[r.LawdCd] = r.Name
	}
	if len(codes) != 2 || codes["41111"] != "수원시 장안구" || codes["11680"] != "강남구" {
		t.Fatalf("got %v", codes)
	}
}

func TestRoadAddress(t *testing.T) {
	cases := [][4]string{
		{"동대구로", "00345", "00000", "동대구로 345"},
		{"교학로19길", "00019", "00002", "교학로19길 19-2"},
		{"", "00019", "0", ""},
		{"동대구로", "", "", ""},
	}
	for _, c := range cases {
		if got := roadAddress(c[0], c[1], c[2]); got != c[3] {
			t.Errorf("%v → %q", c, got)
		}
	}
}
