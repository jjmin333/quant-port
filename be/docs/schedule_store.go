package docs

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

// 관리자가 실수로 통째로 지우는 것을 막는다. 시간표 문서의 정체성이 이 제목이다.
const batchScheduleAnchor = "## 시간표"

// BatchScheduleMaxBytes is the PUT ceiling. 운영 메모를 적는 문서이지 첨부함이 아니다.
const BatchScheduleMaxBytes = 256 << 10

// BatchSchedulePath is where the live document is read and written.
//
// embed는 배포 누락을 막기 위한 폴백이다. 관리자 화면에서 고친 내용은 이 경로의
// 파일로 나가야 리포지터리와 화면이 한 벌로 남는다. 비어 있으면 몇 가지 상대
// 경로를 시도하고, 그래도 없으면 읽기는 embed, 쓰기는 거절한다.
func BatchSchedulePath() string {
	if p := strings.TrimSpace(os.Getenv("BATCH_SCHEDULE_PATH")); p != "" {
		return p
	}
	for _, p := range []string{
		"docs/BATCH_SCHEDULE.md",
		"be/docs/BATCH_SCHEDULE.md",
		"/app/docs/BATCH_SCHEDULE.md",
	} {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// BatchScheduleDoc is what the admin API returns.
type BatchScheduleDoc struct {
	Markdown string
	Path     string
	Source   string // "file" | "embed"
	Writable bool
}

// LoadBatchSchedule prefers the on-disk file so an admin save (and an IDE
// edit) show up without rebuilding the binary. Missing/empty file → embed.
func LoadBatchSchedule() BatchScheduleDoc {
	path := BatchSchedulePath()
	if path != "" {
		body, err := os.ReadFile(path)
		if err == nil && len(strings.TrimSpace(string(body))) > 0 {
			return BatchScheduleDoc{
				Markdown: string(body),
				Path:     path,
				Source:   "file",
				Writable: fileWritable(path),
			}
		}
	}
	display := path
	if display == "" {
		display = "be/docs/BATCH_SCHEDULE.md"
	}
	return BatchScheduleDoc{
		Markdown: batchSchedule,
		Path:     display,
		Source:   "embed",
		Writable: path != "" && fileWritable(path),
	}
}

// SaveBatchSchedule writes the operator-edited markdown to disk.
func SaveBatchSchedule(markdown string) (BatchScheduleDoc, error) {
	body, err := normalizeBatchSchedule(markdown)
	if err != nil {
		return BatchScheduleDoc{}, err
	}
	path := BatchSchedulePath()
	if path == "" {
		return BatchScheduleDoc{}, fmt.Errorf("batch schedule: BATCH_SCHEDULE_PATH is not set and no docs/BATCH_SCHEDULE.md was found")
	}
	if err := os.WriteFile(path, []byte(body), 0o664); err != nil {
		return BatchScheduleDoc{}, fmt.Errorf("batch schedule: write %s: %w", path, err)
	}
	return BatchScheduleDoc{
		Markdown: body,
		Path:     path,
		Source:   "file",
		Writable: true,
	}, nil
}

func normalizeBatchSchedule(markdown string) (string, error) {
	body := strings.ReplaceAll(markdown, "\r\n", "\n")
	body = strings.TrimSpace(body) + "\n"
	if body == "\n" {
		return "", fmt.Errorf("batch schedule: empty document")
	}
	if !utf8.ValidString(body) {
		return "", fmt.Errorf("batch schedule: not valid UTF-8")
	}
	if len(body) > BatchScheduleMaxBytes {
		return "", fmt.Errorf("batch schedule: document exceeds %d bytes", BatchScheduleMaxBytes)
	}
	if !strings.Contains(body, batchScheduleAnchor) {
		return "", fmt.Errorf("batch schedule: missing %q heading — refusing to save a different document", batchScheduleAnchor)
	}
	return body, nil
}

func fileWritable(path string) bool {
	f, err := os.OpenFile(path, os.O_WRONLY, 0)
	if err != nil {
		// 파일이 아직 없으면 디렉터리가 쓰이는지로 판단한다.
		if os.IsNotExist(err) {
			dir := path
			if i := strings.LastIndexAny(path, `/\`); i >= 0 {
				dir = path[:i]
			}
			info, statErr := os.Stat(dir)
			return statErr == nil && info.IsDir()
		}
		return false
	}
	_ = f.Close()
	return true
}
