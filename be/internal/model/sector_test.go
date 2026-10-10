package model

import "testing"

// 대형주 몇 개의 실제 SIC로 경계를 고정한다 — 표를 고치다 이 종목들이 칸을 옮기면
// 그 변화는 의도한 것이어야 한다.
func TestSectorFromSIC(t *testing.T) {
	cases := []struct {
		name string
		sic  string
		want Sector
	}{
		{"AAPL 전자계산기", "3571", SectorIT},
		{"MSFT 패키지 소프트웨어", "7372", SectorIT},
		{"NVDA 반도체", "3674", SectorIT},
		{"GOOGL 컴퓨터 서비스", "7370", SectorIT},
		{"JPM 국법은행", "6021", SectorFinancials},
		{"XOM 석유 정제", "2911", SectorEnergy},
		{"JNJ 의약품", "2834", SectorHealthCare},
		{"MDT 의료기기", "3841", SectorHealthCare},
		{"PG 비누·세제", "2840", SectorConsumerStaple},
		{"KO 음료", "2080", SectorConsumerStaple},
		{"AMZN 무점포 소매", "5961", SectorConsumerDisc},
		{"TSLA 자동차", "3711", SectorConsumerDisc},
		{"BA 항공기", "3721", SectorIndustrials},
		{"LIN 산업용 가스", "2810", SectorMaterials},
		{"NEE 전력", "4911", SectorUtilities},
		{"T 전화 통신", "4813", SectorCommunication},
		{"O 리츠", "6798", SectorRealEstate},
		{"SPAC 백지수표", "6770", SectorFinancials},
		{"분류 불능", "9995", SectorUnknown},
		{"빈 값", "", SectorUnknown},
	}
	for _, c := range cases {
		if got := SectorFromSIC(c.sic); got != c.want {
			t.Errorf("%s (%s): got %q want %q", c.name, c.sic, got, c.want)
		}
	}
}

func TestSectorFromKSIC(t *testing.T) {
	cases := []struct {
		name string
		code string
		want Sector
	}{
		{"반도체·전자부품", "261", SectorIT},
		{"통신·방송장비", "264", SectorIT},
		{"의약품", "21102", SectorHealthCare},
		{"포털", "63120", SectorCommunication},
		{"화장품", "20423", SectorConsumerStaple},
		{"기초 화학", "20111", SectorMaterials},
		{"자동차", "30121", SectorConsumerDisc},
		{"가정용 기기", "28511", SectorConsumerDisc},
		{"축전지", "28202", SectorIndustrials},
		{"의료기기", "27112", SectorHealthCare},
		{"측정기기", "27213", SectorIT},
		{"의약학 연구개발", "70113", SectorHealthCare},
		{"공학 연구개발", "70121", SectorIndustrials},
		{"게임 소프트웨어 출판", "58211", SectorIT},
		{"전기업", "35111", SectorUtilities},
		{"은행", "64110", SectorFinancials},
		{"부동산 임대", "68111", SectorRealEstate},
		{"한 자리", "2", SectorUnknown},
		{"숫자 아님", "ab", SectorUnknown},
	}
	for _, c := range cases {
		if got := SectorFromKSIC(c.code); got != c.want {
			t.Errorf("%s (%s): got %q want %q", c.name, c.code, got, c.want)
		}
	}
}

// 지주회사 코드는 이름으로 가른다 — 실측 행(2026-09-23)으로 고정.
func TestSectorFromKSICNamed(t *testing.T) {
	cases := []struct {
		code, name string
		want       Sector
	}{
		{"64992", "신한지주", SectorFinancials},
		{"64992", "iM금융지주", SectorFinancials},
		{"64992", "삼양홀딩스", SectorUnknown},
		{"64992", "한국앤컴퍼니", SectorUnknown},
		{"649", "HDC", SectorUnknown},
		{"649", "효성", SectorUnknown},
		{"649", "큐캐피탈", SectorFinancials},
		{"649", "교보20호스팩", SectorFinancials},
		{"649", "경동인베스트", SectorFinancials},
		{"264", "삼성전자", SectorIT}, // 지주 코드가 아니면 이름을 보지 않는다
		{"64121", "제주은행", SectorFinancials},
	}
	for _, c := range cases {
		if got := SectorFromKSICNamed(c.code, c.name); got != c.want {
			t.Errorf("%s %s: got %q want %q", c.code, c.name, got, c.want)
		}
	}
}

func TestSectorLabelsCoverEverySector(t *testing.T) {
	for _, s := range Sectors {
		if s.Label() == "" {
			t.Errorf("%s has no label", s)
		}
	}
	if len(Sectors) != len(sectorLabels) {
		t.Errorf("Sectors lists %d, labels %d", len(Sectors), len(sectorLabels))
	}
}
