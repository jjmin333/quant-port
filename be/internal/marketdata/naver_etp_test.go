package marketdata

import "testing"

func TestIsKRXUnauthorized(t *testing.T) {
	if !IsKRXUnauthorized(fmtError("krx: status 401: Unauthorized API Call")) {
		t.Fatal("expected unauthorized")
	}
	if IsKRXUnauthorized(fmtError("timeout")) {
		t.Fatal("expected not unauthorized")
	}
}

type fmtError string

func (e fmtError) Error() string { return string(e) }
