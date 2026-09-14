import { PageScrollShell } from "@/components/layout/page-content-shell";

/**
 * 약관·개인정보처리방침 같은 **읽는 문서**의 공통 껍데기.
 *
 * 서버 컴포넌트다 — 이 글들은 상호작용이 없고, 크롤러가 본문을 그대로 읽어야
 * 신뢰 신호(투자 정보는 YMYL이라 운영 주체·약관이 순위에 영향을 준다)로 쓰인다.
 *
 * 폭은 리포트 상세와 같은 `reading`이다. 이 앱에서 「긴 글을 읽는 화면」은 이미
 * 그 폭으로 정해져 있어서, 문서마다 다른 폭을 쓰면 같은 앱처럼 안 읽힌다.
 */
export function LegalDocument({
  title,
  updatedAt,
  children,
}: {
  title: string;
  /** ISO 날짜. 「마지막 개정일」로 보여 준다 — 약관은 언제 것인지가 내용의 일부다. */
  updatedAt: string;
  children: React.ReactNode;
}) {
  return (
    <PageScrollShell width="reading" contentClassName="space-y-6">
      <header className="space-y-2">
        <h1 className="text-2xl font-bold tracking-tight text-[var(--text-primary)]">{title}</h1>
        <p className="text-cell text-[var(--text-muted)]">마지막 개정일 {updatedAt}</p>
      </header>
      <div className="space-y-6">{children}</div>
    </PageScrollShell>
  );
}

export function LegalSection({
  title,
  children,
}: {
  title: string;
  children: React.ReactNode;
}) {
  return (
    <section className="space-y-2">
      <h2 className="text-base font-semibold text-[var(--text-primary)]">{title}</h2>
      <div className="space-y-2 text-sm leading-relaxed text-[var(--text-secondary)]">
        {children}
      </div>
    </section>
  );
}
