/*
 * 타이포 스케일 견본 — v1.0.0에서 정한 계단을 실물로 보는 자리.
 *
 * 크기 결정을 눈으로 하려면 목업이 아니라 **같은 폰트·같은 색 토큰**을 쓰는 화면이
 * 있어야 한다. 후보를 고를 때 이 페이지로 골랐고, 고른 뒤에는 "그 값이 실제로 어떻게
 * 보이는지"를 되짚는 자리로 남긴다 — 다음에 `globals.css`의 여섯 줄을 만질 때
 * 여기부터 열어 보면 된다.
 *
 * 인라인 style을 쓰지 않는다: 토큰 자체를 그려야 토큰이 바뀌면 이 페이지도 바뀐다.
 */

const STEPS: Array<{ cls: string; px: string; use: string }> = [
  { cls: "text-micro", px: "11px", use: "배지·칩·각주·메타 — 이 앱의 하한" },
  { cls: "text-cell", px: "12px", use: "표 셀·목록 본문·기본 UI" },
  { cls: "text-note", px: "13px", use: "한 단 강조된 보조" },
  { cls: "text-read", px: "14px / 1.7", use: "읽는 글 — 브리핑·요약·AI 해설·메모" },
  { cls: "text-article", px: "15px / 1.85", use: "문서 본문 — 리포트·기사 원문" },
  { cls: "text-section", px: "15px", use: "섹션·카드 제목" },
  { cls: "text-page", px: "19px", use: "페이지 제목" },
];

const PROSE =
  "중동 지정학적 리스크와 미국 국채 금리 급등의 여파로 국내 증시는 장중 약세 흐름을 지속하고 있습니다. 코스피는 6,792.49로 0.40% 하락했고, 코스닥은 1.85% 급락하며 투자 심리가 위축된 모습입니다. 외국인과 기관이 각각 6,222억 원, 1,010억 원 규모의 순매도를 기록하며 지수 하락을 주도하고 있습니다.";

const ROWS: Array<[string, string, string, string, string]> = [
  ["1", "삼성전자", "74,300", "+1.64%", "93"],
  ["2", "SK하이닉스", "1,684,000", "-1.23%", "91"],
  ["3", "NAVER", "259,500", "+0.19%", "88"],
];

export default function TypographySamplePage() {
  return (
    <div className="min-h-screen bg-background px-6 py-8 text-foreground">
      <div className="mx-auto max-w-[880px] space-y-6">
        <header>
          <h1 className="text-page font-semibold">타이포 스케일 (v1.0.0)</h1>
          <p className="mt-1 text-cell text-muted-foreground">
            정의는 <code className="font-code">src/app/globals.css</code>의{" "}
            <code className="font-code">@theme inline</code> 안에 있다. 숫자는 Pretendard +{" "}
            <code className="font-code">tabular-nums</code>(<code className="font-code">font-num</code>)를
            쓴다.
          </p>
        </header>

        {/* 계단 */}
        <section className="rounded-xl border bg-card p-4">
          <h2 className="text-section font-semibold">계단</h2>
          <div className="mt-3 divide-y">
            {STEPS.map((s) => (
              <div key={s.cls} className="flex flex-wrap items-baseline gap-x-3 gap-y-1 py-2.5">
                <code className="w-32 shrink-0 font-code text-micro text-[var(--brand)]">
                  {s.cls}
                </code>
                <span className="w-24 shrink-0 font-num text-micro text-muted-foreground">
                  {s.px}
                </span>
                <span className={s.cls}>다람쥐 헌 쳇바퀴에 타고파 · 0123456789</span>
                <span className="basis-full text-micro text-muted-foreground">{s.use}</span>
              </div>
            ))}
          </div>
        </section>

        {/* 읽는 글 */}
        <section className="rounded-xl border bg-card p-4">
          <h2 className="text-section font-semibold">읽는 글 — text-read</h2>
          <p className="mt-2 text-read">{PROSE}</p>
          <p className="mt-3 text-micro text-muted-foreground">
            비교: 아래는 v1.0.0 이전의 크기(12px)다. 같은 문단을 표 셀과 같은 크기로 읽고
            있었다.
          </p>
          <p className="mt-1 text-[12px] leading-relaxed text-muted-foreground">{PROSE}</p>
        </section>

        {/* 밀도가 기능인 자리 */}
        <section className="rounded-xl border bg-card p-4">
          <h2 className="text-section font-semibold">표 — text-cell · text-micro · font-num</h2>
          <div className="mt-3 overflow-hidden rounded-lg border">
            <div className="border-b bg-muted px-2.5 py-1 text-micro font-semibold text-muted-foreground">
              AI 추천 랭킹 TOP
            </div>
            {ROWS.map((r) => (
              <div
                key={r[0]}
                className="flex items-center gap-2.5 border-b px-2.5 py-1.5 text-cell last:border-b-0"
              >
                <span className="w-3 font-num text-micro text-muted-foreground">{r[0]}</span>
                <span className="min-w-0 flex-1 truncate">{r[1]}</span>
                <span className="w-24 text-right font-num">{r[2]}</span>
                <span
                  className={`w-16 text-right font-num ${
                    r[3].startsWith("+") ? "text-[var(--profit)]" : "text-[var(--loss)]"
                  }`}
                >
                  {r[3]}
                </span>
                <span className="w-12 text-right font-num text-micro text-muted-foreground">
                  {r[4]} A+
                </span>
              </div>
            ))}
          </div>
          <p className="mt-2 text-micro text-muted-foreground">
            자릿수는 <code className="font-code">tabular-nums</code>가 맞춘다 — 고정폭 글꼴이
            아니라 고정폭 숫자가 정렬의 조건이다.
          </p>
        </section>
      </div>
    </div>
  );
}
