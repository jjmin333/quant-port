package ocr

import (
	"os"
	"runtime"
	"strconv"
	"strings"
)

/*
동시에 실행되는 tesseract 프로세스 수의 상한.

**왜 필요한가.** 스크린샷 가져오기는 이미지당 고루틴 하나를 띄우고(`service.processJob`),
각 고루틴이 `pipeline.ParseHoldingsImage`에서 tesseract를 최대 10회 부른다. 장수 상한
(`maxImportImages = 10`)은 이미 있지만, **그 10장이 동시에 돌면 tesseract 프로세스 10개가
같이 뜬다.** 운영 VM은 2 OCPU다 — 사용자 한 명의 업로드가 박스를 통째로 가져가고, 그동안
같은 프로세스가 서빙하는 API 응답과 시세 폴러가 함께 밀린다.

**왜 상한을 코어 수로 잡는가.** OCR은 순수 CPU 작업이라 코어보다 많이 돌려도 총 처리량이
늘지 않는다 — 컨텍스트 스위치와 캐시 미스만 는다. Go 1.25는 컨테이너 cgroup의 CPU 상한에서
GOMAXPROCS를 자동으로 잡으므로, `runtime.GOMAXPROCS(0)`이 곧 "이 컨테이너가 실제로 쓸 수
있는 코어 수"다. compose에서 `cpus`를 바꾸면 여기도 따라온다.

**트레이드오프.** 10장 업로드의 총 처리 시간은 늘어난다(병렬 10 → 코어 수). 사용자 한 명의
대기가 길어지는 대신 다른 모든 요청의 응답이 유지된다 — 진행률 UI가 이미 있어 대기가 보인다.

`OCR_MAX_CONCURRENCY=0`이면 상한을 끈다(롤백용).
*/
var recognizeSlots chan struct{}

// maxConcurrency는 결정된 상한을 기억한다. 0이면 무제한.
var maxConcurrency int

func init() {
	SetMaxConcurrency(envMaxConcurrency())
}

func envMaxConcurrency() int {
	raw := strings.TrimSpace(os.Getenv("OCR_MAX_CONCURRENCY"))
	if raw == "" {
		return runtime.GOMAXPROCS(0)
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return runtime.GOMAXPROCS(0)
	}
	return n
}

// SetMaxConcurrency는 상한을 다시 잡는다. 0은 무제한.
// 테스트와 기동 시 설정 외에는 부르지 않는다 — 실행 중 교체는 대기 중인 호출을 잃는다.
func SetMaxConcurrency(n int) {
	if n < 0 {
		n = 0
	}
	maxConcurrency = n
	if n == 0 {
		recognizeSlots = nil
		return
	}
	recognizeSlots = make(chan struct{}, n)
}

// MaxConcurrency는 현재 상한을 돌려준다. 0은 무제한.
// 호출부가 자신의 팬아웃을 같은 폭으로 맞출 때 쓴다(service.processJob).
func MaxConcurrency() int { return maxConcurrency }

func acquire() func() {
	slots := recognizeSlots
	if slots == nil {
		return func() {}
	}
	slots <- struct{}{}
	return func() { <-slots }
}
