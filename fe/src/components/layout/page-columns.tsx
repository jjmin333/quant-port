import { LabRailColumn } from "@/components/layout/page-rail-lab";
import { cn } from "@/lib/utils";

/**
 * 전체 폭 화면의 컬럼 조립 규칙.
 *
 * 08-11에 본문이 뷰포트를 다 쓰게 바뀌면서(`page-shell-wide`), 예전 중앙 컬럼용으로
 * 만들어진 화면들이 **가로로 늘어나기만** 했다. 표 한 줄이 1,500px을 가로지르면 읽는
 * 눈이 되돌아올 곳을 잃는다. 그래서 본문 폭을 다시 좁히는 대신, 남는 폭을 **관련 위젯**
 * 으로 채운다 — 같은 맥락의 정보를 옆에 두는 쪽이 여백보다 낫다.
 *
 * 규칙:
 *  - 레일은 `xl`(1280px) 이상에서만 나타난다. 그 아래에서는 본문 밑으로 쌓인다.
 *    데스크탑 좁은 창에서 레일을 억지로 유지하면 본문이 먼저 눌린다.
 *  - 레일 폭은 고정, 본문은 `minmax(0,1fr)`. `1fr`만 쓰면 안쪽의 긴 표가 컬럼을
 *    밀어내 레일이 잘린다(그리드 아이템의 min-width가 auto라서).
 *  - 좌·우 레일을 동시에 쓰는 화면(종목 분석)은 `2xl`에서만 3단이 된다.
 */

type ColumnsProps = {
  children: React.ReactNode;
  /** 우측(또는 3단에서 우측) 레일 */
  rail?: React.ReactNode;
  /** 좌측 레일. 목록에서 본문을 고르는 화면(랭킹 → 상세)에만 쓴다. */
  leadRail?: React.ReactNode;
  /**
   * 좌측 레일을 넓게(420px) 세운다.
   *
   * 종목 랭킹 `넓게` 보기처럼 한 줄에 이름·시세·스파크라인·등락·점수를 함께 놓는 목록은
   * 300px에서 이름 몫이 20px대로 남아 한 글자로 잘린다. 그때는 본문에서 폭을 빌려온다.
   */
  wideLead?: boolean;
  className?: string;
};

export function PageColumns({ children, rail, leadRail, wideLead, className }: ColumnsProps) {
  const hasLead = Boolean(leadRail);
  const hasRail = Boolean(rail);

  return (
    <div
      className={cn(
        "grid min-w-0 items-start gap-3",
        hasLead &&
          !hasRail &&
          (wideLead ? "xl:grid-cols-[420px_minmax(0,1fr)]" : "xl:grid-cols-[300px_minmax(0,1fr)]"),
        !hasLead && hasRail && "xl:grid-cols-[minmax(0,1fr)_300px] 2xl:grid-cols-[minmax(0,1fr)_340px]",
        hasLead &&
          hasRail &&
          (wideLead
            ? "xl:grid-cols-[420px_minmax(0,1fr)] 2xl:grid-cols-[420px_minmax(0,1fr)_340px]"
            : "xl:grid-cols-[300px_minmax(0,1fr)] 2xl:grid-cols-[300px_minmax(0,1fr)_340px]"),
        className,
      )}
    >
      {hasLead && (
        <RailColumn sticky className="xl:order-1">
          {leadRail}
        </RailColumn>
      )}
      <div className={cn("min-w-0", hasLead && "xl:order-2")}>{children}</div>
      {hasRail &&
        (hasLead ? (
          /*
           * 3단 화면(종목 분석)의 오른쪽 레일. `2xl` 미만에서는 세 번째 컬럼을 세울 폭이
           * 없어 본문 아래로 내려가 두 컬럼을 가로지른다. 그때는 세로로 쌓지 않고 격자로
           * 흘린다 — 폭이 1,200px인데 카드를 한 줄에 하나만 두면 스크롤만 길어진다.
           * 붙박이(sticky)도 그 상태에서는 걸지 않는다. 본문 아래에 있는 블록을 붙여두면
           * 스크롤이 거기서 멈춘 것처럼 보인다.
           */
          <RailColumn
            className="xl:order-3 xl:col-span-2 xl:grid xl:grid-cols-2 xl:items-start 2xl:col-span-1 2xl:col-start-3 2xl:row-start-1 2xl:flex"
            stickyFrom="2xl"
          >
            {rail}
          </RailColumn>
        ) : (
          /*
            좌측 레일이 없는 평범한 2단 화면. 마크업은 `RailColumn sticky`와 같고,
            **좁은 화면에서 레일을 어디에 세울지**만 레이아웃 실험대가 정한다
            (`lib/layout-lab`). 실서비스 기본값(`stack`)에서는 예전과 한 글자도
            다르지 않다 — 확정안이 정해지면 실험대와 함께 이 한 줄을 되돌린다.
          */
          <LabRailColumn>{rail}</LabRailColumn>
        ))}
    </div>
  );
}

/**
 * 레일 한 줄. 화면 높이만큼 붙어 따라온다 — 본문이 길어도 위젯이 시야에서 사라지지 않게.
 * 붙박이는 컬럼으로 서 있을 때만 건다. 쌓인 상태에서 붙이면 본문 위를 덮는다.
 */
function RailColumn({
  children,
  className,
  sticky = false,
  stickyFrom,
}: {
  children: React.ReactNode;
  className?: string;
  sticky?: boolean;
  stickyFrom?: "2xl";
}) {
  return (
    <aside
      className={cn(
        "flex min-w-0 flex-col gap-3",
        sticky &&
          "xl:sticky xl:top-3 xl:max-h-[calc(100dvh-5.5rem)] xl:overflow-y-auto xl:overscroll-contain",
        stickyFrom === "2xl" &&
          "2xl:sticky 2xl:top-3 2xl:max-h-[calc(100dvh-5.5rem)] 2xl:overflow-y-auto 2xl:overscroll-contain",
        className,
      )}
    >
      {children}
    </aside>
  );
}

/**
 * 본문 상단의 지표 타일 줄. 화면마다 `grid-cols-2 sm:grid-cols-4`를 다시 쓰던 것을 모았다.
 */
export function StatRow({
  children,
  className,
}: {
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <div className={cn("grid min-w-0 grid-cols-2 gap-2 sm:grid-cols-4", className)}>
      {children}
    </div>
  );
}
