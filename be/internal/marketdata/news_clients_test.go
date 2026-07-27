package marketdata

import "testing"

func TestStripHTML_decodesNumericEntities(t *testing.T) {
	got := stripHTML(`반도체 &#34;급락&#34;...&#39;순환매&#39;`)
	want := `반도체 "급락"...'순환매'`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStripHTML_decodesNamedEntitiesAndTags(t *testing.T) {
	got := stripHTML(`<b>코스피</b> &quot;상승&quot; &amp; &lt;기록&gt;`)
	want := `코스피 "상승" & <기록>`
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
