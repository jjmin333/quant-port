/**
 * 댓글 한 줄의 **곁다리 표시** — 글쓴이 활동 배지와 상대 시각.
 *
 * 둘 다 순수 함수로 떼어 둔 이유는 같다: 경계값에서만 틀리는 종류라, 화면을 눈으로
 * 봐서는 안 갈린다(99가 「100+」로 뜨는지, 59분 59초가 「1시간 전」이 되는지).
 */

/**
 * 활동 수를 **구간**으로 접는다 — `300+`, `100+`, `10+`.
 *
 * # 왜 정확한 수를 안 쓰나 (2026-09-14 사용자 요청 3)
 *
 * 읽는 사람이 이 배지로 하는 판단은 「처음 보는 사람인가, 오래 써 온 사람인가」
 * 하나뿐이다. 「리포트·댓글 합쳐 137개」는 그 판단에 아무것도 더해 주지 않으면서
 * 그 사람의 활동량을 필요 이상으로 드러낸다. 그리고 정확한 수는 **매일 바뀌므로**,
 * 같은 사람의 같은 댓글이 어제와 다른 숫자를 달고 있게 된다.
 *
 * 10 미만은 배지가 없다. 「2+」는 「처음 보는 사람」과 구분되지 않는데, 배지가 붙는
 * 것 자체가 신호라 아무 신호도 없는 것보다 나쁘다.
 */
export function activityBadge(activity: number | undefined): string | null {
  const n = activity ?? 0;
  for (const step of [1000, 300, 100, 30, 10]) {
    if (n >= step) return `${step}+`;
  }
  return null;
}

/**
 * 「3시간 전」. 하루가 넘으면 날짜를 그대로 쓰는 쪽이 낫다 — 「9일 전」은 그게
 * 언제인지 세어 봐야 알고, 댓글은 보통 글과 같은 주에 몰린다.
 *
 * `now`를 인자로 받는 이유는 시험 때문만이 아니다. 렌더 도중에 시계를 직접 읽으면
 * 같은 입력이 매번 다른 결과를 내는 순수하지 않은 렌더가 된다(`daysUntilDue`가 같은
 * 이유로 컴포넌트 밖에 있다).
 */
export function relativeTime(iso: string, now: number = Date.now()): string | null {
  const at = new Date(iso).getTime();
  if (Number.isNaN(at)) return null;
  const secs = Math.floor((now - at) / 1000);
  // 시계가 어긋나 미래로 보이면 「방금」이라고 한다 — 「-3시간 전」보다 낫다.
  if (secs < 60) return "방금";
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}분 전`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}시간 전`;
  return null;
}
