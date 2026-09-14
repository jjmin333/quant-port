/*
버튼에 매달린 패널을 좁은 화면에서 **화면에 매다는** 규칙.

# 왜 필요한가 (실측 375×812)

`absolute right-0 w-[292px]` 같은 패널은 **버튼 위치**를 기준으로 편다. 모바일 헤더에서
스탬프 버튼의 오른쪽 끝이 x=246이면 패널의 왼쪽 끝은 246 − 292 = **−46**, 곧 화면
밖이다. 게다가 이 앱의 본문 껍데기(`page-shell-wide`)가 `overflow:hidden`이라 넘친
46px은 스크롤로도 닿지 않았다 — 예상 주가 입력칸의 왼쪽이 잘린 채 열렸다.

폭을 줄여도 안 고쳐진다. 기준이 버튼인 한, 버튼이 화면 가장자리에 가까우면 어떤 폭이든
넘친다. `left-0`으로 여는 패널은 같은 일이 오른쪽에서 벌어진다.

# 규칙

`sm`(640px) 미만에서는 `fixed`로 바꿔 **화면**의 좌우 여백 12px에 맞추고 아래쪽에
붙인다. 세로로 긴 패널(스탬프 폼은 467px)이 작은 화면을 넘길 수 있으므로 높이를 재고
안에서 스크롤한다. `overflow:hidden` 조상은 `position:fixed` 자손을 자르지 않는다.

폭 지정(`w-[292px]` 등)은 각 사용처가 그대로 들고 있으면 된다 — `max-sm:w-auto`가
그 미디어 쿼리 안에서 이긴다.
*/
export const ANCHORED_POPOVER_MOBILE =
  "max-sm:fixed max-sm:inset-x-3 max-sm:bottom-3 max-sm:top-auto max-sm:mb-0 max-sm:mt-0 " +
  "max-sm:max-h-[85vh] max-sm:w-auto max-sm:overflow-y-auto";
