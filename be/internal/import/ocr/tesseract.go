package ocr

import (
	"bytes"
	"fmt"
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
	// PSM 11 (sparse text): mobile holdings lists are multi-line blocks, not uniform paragraphs.
	args := []string{imagePath, "stdout", "-l", e.Langs, "--psm", psm, "--oem", "1"}
	cmd := exec.Command(e.Binary, args...)
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
