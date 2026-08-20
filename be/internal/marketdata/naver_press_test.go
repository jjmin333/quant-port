package marketdata

import "testing"

func TestParseNaverPressOID(t *testing.T) {
	oid, ok := ParseNaverPressOID("https://n.news.naver.com/mnews/article/015/0005553272?sid=101")
	if !ok || oid != "015" {
		t.Fatalf("got %q ok=%v", oid, ok)
	}
}

func TestResolvePublisherFromArticle_WireUsesPortal(t *testing.T) {
	portal := "https://n.news.naver.com/mnews/article/015/0005553272"
	original := "https://www.newspim.com/news/view/20260727001234"
	pub := ResolvePublisherFromArticle(portal, original)
	if pub.Name != "한국경제" {
		t.Fatalf("expected 한국경제, got %+v", pub)
	}
}

func TestResolvePublisherFromArticle_DirectOutlet(t *testing.T) {
	portal := "https://n.news.naver.com/mnews/article/015/0005553272"
	original := "https://www.mk.co.kr/news/economy/123"
	pub := ResolvePublisherFromArticle(portal, original)
	if pub.Name != "매일경제" {
		t.Fatalf("expected 매일경제, got %+v", pub)
	}
}

func TestResolvePublisherFromArticle_NewspimOnly(t *testing.T) {
	pub := ResolvePublisherFromArticle("", "https://www.newspim.com/news/view/1")
	if pub.Name != "뉴스핌" {
		t.Fatalf("expected 뉴스핌, got %+v", pub)
	}
}
