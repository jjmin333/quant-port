package config

import "testing"

func TestLoadTossConfig_primaryPreferredOverLegacy(t *testing.T) {
	t.Setenv("TOSS_API_BASE_URL", "https://example.test")
	t.Setenv("TOSS_PRIMARY_CLIENT_ID", "primary-id")
	t.Setenv("TOSS_PRIMARY_CLIENT_SECRET", "primary-secret")
	t.Setenv("TOSS_CLIENT_ID", "legacy-id")
	t.Setenv("TOSS_CLIENT_SECRET", "legacy-secret")
	t.Setenv("TOSS_KR_CLIENT_ID", "kr-id")
	t.Setenv("TOSS_KR_CLIENT_SECRET", "kr-secret")
	t.Setenv("TOSS_SECONDARY_CLIENT_ID", "secondary-id")
	t.Setenv("TOSS_SECONDARY_CLIENT_SECRET", "secondary-secret")
	t.Setenv("TOSS_US_CLIENT_ID", "us-id")
	t.Setenv("TOSS_US_CLIENT_SECRET", "us-secret")

	cfg := loadTossConfig()
	if cfg.ClientID != "primary-id" || cfg.ClientSecret != "primary-secret" {
		t.Fatalf("primary = %q / %q, want PRIMARY_* vars", cfg.ClientID, cfg.ClientSecret)
	}
	if cfg.SecondaryClientID != "secondary-id" || cfg.SecondaryClientSecret != "secondary-secret" {
		t.Fatalf("secondary = %q / %q, want SECONDARY_* vars", cfg.SecondaryClientID, cfg.SecondaryClientSecret)
	}
	if !cfg.Enabled() || !cfg.SecondaryEnabled() {
		t.Fatal("both keys should be enabled")
	}
}

func TestLoadTossConfig_legacySingleKey(t *testing.T) {
	t.Setenv("TOSS_PRIMARY_CLIENT_ID", "")
	t.Setenv("TOSS_PRIMARY_CLIENT_SECRET", "")
	t.Setenv("TOSS_SECONDARY_CLIENT_ID", "")
	t.Setenv("TOSS_SECONDARY_CLIENT_SECRET", "")
	t.Setenv("TOSS_KR_CLIENT_ID", "")
	t.Setenv("TOSS_KR_CLIENT_SECRET", "")
	t.Setenv("TOSS_US_CLIENT_ID", "")
	t.Setenv("TOSS_US_CLIENT_SECRET", "")
	t.Setenv("TOSS_CLIENT_ID", "legacy-id")
	t.Setenv("TOSS_CLIENT_SECRET", "legacy-secret")

	cfg := loadTossConfig()
	if cfg.ClientID != "legacy-id" || cfg.ClientSecret != "legacy-secret" {
		t.Fatalf("primary = %q / %q, want TOSS_CLIENT_* fallback", cfg.ClientID, cfg.ClientSecret)
	}
	if cfg.SecondaryEnabled() {
		t.Fatal("legacy single-key setup must not invent a secondary")
	}
}

func TestTossConfig_identicalSecondaryDisabled(t *testing.T) {
	cfg := TossConfig{
		ClientID:              "same",
		ClientSecret:          "secret",
		SecondaryClientID:     "same",
		SecondaryClientSecret: "secret",
	}
	if !cfg.Enabled() {
		t.Fatal("primary should be enabled")
	}
	if cfg.SecondaryEnabled() {
		t.Fatal("identical secondary must be treated as absent")
	}
}
