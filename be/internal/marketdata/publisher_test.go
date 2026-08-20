package marketdata

import "testing"

func TestResolvePublisher_TheFact(t *testing.T) {
	pub := ResolvePublisher("https://www.tf.co.kr/news/article/123")
	if pub.ID != "the-fact" || pub.Name != "더팩트" {
		t.Fatalf("got %+v", pub)
	}
}

func TestResolvePublisher_OriginalNotNaver(t *testing.T) {
	pub := ResolvePublisher("https://news.naver.com/main/read.naver?oid=001&aid=0001234567")
	if pub.ID != "naver-news" {
		t.Fatalf("portal should be naver-news, got %+v", pub)
	}
	pub2 := ResolvePublisher("https://www.mk.co.kr/news/economy/123")
	if pub2.ID != "maeil" {
		t.Fatalf("got %+v", pub2)
	}
}

func TestReaderAllowedHost(t *testing.T) {
	if ReaderAllowedHost("https://news.naver.com/foo") {
		t.Fatal("naver portal should be blocked")
	}
	if !ReaderAllowedHost("https://www.tf.co.kr/news/1") {
		t.Fatal("the-fact should be allowed")
	}
}
