package repository

import (
	"strings"
	"testing"
)

func TestDSNTimeoutsAddedWhenAbsent(t *testing.T) {
	got := withDefaultDSNTimeouts("user:pw@tcp(host:3306)/db?parseTime=true")
	for _, want := range []string{"timeout=5s", "readTimeout=30s", "writeTimeout=30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %q", want, got)
		}
	}
	// 기존 파라미터가 살아 있어야 한다.
	if !strings.Contains(got, "parseTime=true") {
		t.Errorf("existing params dropped: %q", got)
	}
	if strings.Count(got, "?") != 1 {
		t.Errorf("malformed query string: %q", got)
	}
}

// 운영자가 DSN에 적어 둔 값이 이겨야 한다.
func TestDSNTimeoutsDoNotOverrideExplicit(t *testing.T) {
	got := withDefaultDSNTimeouts("u:p@tcp(h:3306)/db?readTimeout=90s&timeout=1s")
	if strings.Contains(got, "readTimeout=30s") {
		t.Errorf("explicit readTimeout overridden: %q", got)
	}
	if strings.Contains(got, "timeout=5s") {
		t.Errorf("explicit timeout overridden: %q", got)
	}
	if !strings.Contains(got, "writeTimeout=30s") {
		t.Errorf("absent writeTimeout should still be filled: %q", got)
	}
}

func TestDSNWithoutQueryStringGetsOne(t *testing.T) {
	got := withDefaultDSNTimeouts("u:p@tcp(h:3306)/db")
	if !strings.Contains(got, "?timeout=5s") {
		t.Errorf("first param should start the query string: %q", got)
	}
	if strings.Count(got, "?") != 1 {
		t.Errorf("malformed query string: %q", got)
	}
	if strings.Contains(got, "&&") || strings.HasSuffix(got, "&") {
		t.Errorf("malformed separators: %q", got)
	}
}

// 실제 compose에서 쓰는 DSN 모양으로 한 번 더 — loc=Asia%2FSeoul 같은 인코딩이
// 깨지지 않는지.
func TestRealWorldDSNShape(t *testing.T) {
	in := "quantport:quantport@tcp(host.docker.internal:3306)/quantport?parseTime=true&charset=utf8mb4&loc=Asia%2FSeoul"
	got := withDefaultDSNTimeouts(in)
	if !strings.HasPrefix(got, in) {
		t.Errorf("original DSN altered: %q", got)
	}
	if !strings.Contains(got, "loc=Asia%2FSeoul") {
		t.Errorf("encoded value damaged: %q", got)
	}
	for _, want := range []string{"&timeout=5s", "&readTimeout=30s", "&writeTimeout=30s"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s in %q", want, got)
		}
	}
}

// 세션 타임존이 DSN에 들어가야 한다. 관리형 MySQL은 서버 tz가 대개 UTC라
// loc=Asia/Seoul 만으로는 TIMESTAMP 읽기가 9시간 밀린다 — 근거는 mysql.go 주석.
func TestDSNPinsSessionTimeZone(t *testing.T) {
	got := withDefaultDSNTimeouts("u:p@tcp(h:3306)/db?parseTime=true")
	// SET time_zone='+09:00' 이 되도록 따옴표째 URL 인코딩돼야 한다.
	if !strings.Contains(got, "time_zone=%27%2B09%3A00%27") {
		t.Errorf("session time zone not pinned: %q", got)
	}
}

func TestDSNTimeZoneOverridable(t *testing.T) {
	t.Setenv("MYSQL_TIME_ZONE", "UTC")
	got := withDefaultDSNTimeouts("u:p@tcp(h:3306)/db")
	if !strings.Contains(got, "time_zone=%27UTC%27") {
		t.Errorf("MYSQL_TIME_ZONE ignored: %q", got)
	}
}

// 운영자가 DSN에 직접 적었으면 그것이 이긴다 — 다른 파라미터와 같은 규칙.
func TestDSNTimeZoneNotOverriddenWhenExplicit(t *testing.T) {
	got := withDefaultDSNTimeouts("u:p@tcp(h:3306)/db?time_zone=%27%2B00%3A00%27")
	if strings.Contains(got, "%2B09%3A00") {
		t.Errorf("explicit time_zone overridden: %q", got)
	}
}
