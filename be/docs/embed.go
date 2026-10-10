// Package docs serves operator-facing markdown that the app itself renders.
//
// 문서를 이미지에 따로 복사하지 않고 embed하는 이유: 복사는 배포마다 빠뜨릴 수
// 있고, 빠뜨려도 기동은 성공한다 — 관리자 화면에서만 조용히 빈 문서가 뜬다.
// embed는 빌드 시점에 끊어지므로 그런 실패가 불가능하다. 문서의 원본은 여전히
// 이 디렉터리의 마크다운 파일 하나뿐이라 사본이 생기지도 않는다.
package docs

import _ "embed"

//go:embed BATCH_SCHEDULE.md
var batchSchedule string
