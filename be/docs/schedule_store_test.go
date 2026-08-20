package docs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBatchScheduleFallsBackToEmbed(t *testing.T) {
	t.Setenv("BATCH_SCHEDULE_PATH", filepath.Join(t.TempDir(), "missing.md"))
	doc := LoadBatchSchedule()
	if doc.Source != "embed" {
		t.Fatalf("source = %s, want embed", doc.Source)
	}
	if !strings.Contains(doc.Markdown, batchScheduleAnchor) {
		t.Fatal("embed missing 시간표 heading")
	}
}

func TestSaveAndLoadBatchScheduleRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "BATCH_SCHEDULE.md")
	t.Setenv("BATCH_SCHEDULE_PATH", path)
	body := "# 배치 시간표 · 수동 실행 안내\n\n## 시간표 (KST)\n\n| 시각 | 잡 | 메모 |\n| --- | --- | --- |\n| 06:00 | test | 운영 메모 |\n"
	saved, err := SaveBatchSchedule(body)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Source != "file" || !saved.Writable {
		t.Fatalf("saved = %+v", saved)
	}
	got := LoadBatchSchedule()
	if !strings.Contains(got.Markdown, "운영 메모") {
		t.Fatalf("reload missing memo:\n%s", got.Markdown)
	}
}

func TestSaveBatchScheduleRejectsWrongDoc(t *testing.T) {
	t.Setenv("BATCH_SCHEDULE_PATH", filepath.Join(t.TempDir(), "BATCH_SCHEDULE.md"))
	if _, err := SaveBatchSchedule("# other\n"); err == nil {
		t.Fatal("want error for missing 시간표 heading")
	}
}

func TestSaveBatchScheduleCreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BATCH_SCHEDULE.md")
	t.Setenv("BATCH_SCHEDULE_PATH", path)
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("file should not exist yet")
	}
	if _, err := SaveBatchSchedule("# x\n\n## 시간표\n"); err != nil {
		t.Fatal(err)
	}
}
