package ocr

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Engine struct {
	Binary string
	Langs  string
}

func DefaultEngine() Engine {
	bin := "tesseract"
	if runtime.GOOS == "windows" {
		bin = "tesseract.exe"
	}
	return Engine{Binary: bin, Langs: "kor+eng"}
}

func (e Engine) Available() bool {
	cmd := exec.Command(e.Binary, "--version")
	return cmd.Run() == nil
}

func (e Engine) RecognizeFile(imagePath string) (string, error) {
	return e.RecognizeFilePSM(imagePath, "11")
}

func (e Engine) RecognizeFilePSM(imagePath, psm string) (string, error) {
	if strings.TrimSpace(psm) == "" {
		psm = "11"
	}
	// 동시에 도는 tesseract 수를 코어 수로 묶는다 — 근거는 limit.go 머리말.
	release := acquire()
	defer release()

	// PSM 11 (sparse text): mobile holdings lists are multi-line blocks, not uniform paragraphs.
	args := []string{imagePath, "stdout", "-l", e.Langs, "--psm", psm, "--oem", "1"}
	cmd := exec.Command(e.Binary, args...)
	// tesseract는 OpenMP로 코어 수만큼 스레드를 띄운다. 우리가 이미 프로세스 단위로
	// 코어 수만큼 병렬화하므로, 그 위에 스레드까지 겹치면 2 OCPU에서 스케줄러만 바쁘다.
	// 운영자가 명시했으면 그것이 이긴다.
	if os.Getenv("OMP_THREAD_LIMIT") == "" {
		cmd.Env = append(os.Environ(), "OMP_THREAD_LIMIT=1")
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("tesseract: %s", msg)
	}
	return stdout.String(), nil
}
