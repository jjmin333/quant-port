package model

import "testing"

func TestParseTopic(t *testing.T) {
	for in, want := range map[string]string{"": TopicStock, "stock": TopicStock, "real_estate": TopicRealEstate, "finance": TopicFinance} {
		if got, ok := ParseTopic(in); !ok || got != want {
			t.Errorf("ParseTopic(%q) = %q,%v", in, got, ok)
		}
	}
	if _, ok := ParseTopic("crypto"); ok {
		t.Error("unknown topic accepted")
	}
	if TopicFilter("crypto") != "" || TopicFilter("finance") != TopicFinance {
		t.Error("TopicFilter")
	}
}
