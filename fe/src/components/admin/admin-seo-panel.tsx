"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { Loader2, RotateCcw, Save } from "lucide-react";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { useConfirm } from "@/components/providers/confirm-provider";
import { buttonVariants } from "@/components/ui/button";
import { isAdminSession } from "@/lib/admin-features";
import {
  fetchAdminPageMeta,
  resetAdminPageMeta,
  saveAdminPageMeta,
} from "@/lib/admin";
import { toUserMessage } from "@/lib/error-message";
import { PAGE_ROUTES, type PageRoute } from "@/lib/page-meta-routes";
import type { PageMetaOverride } from "@/lib/page-meta-types";
import { SITE_NAME } from "@/lib/seo";
import { cn } from "@/lib/utils";

/**
 * 화면 메타 관리 (`/admin/seo`) — 2026-09-15 사용자 요청 2.
 *
 * # 무엇을 고치는 화면인가
 *
 * 브라우저 탭에 뜨는 이름과 검색 결과에 나가는 설명이다. 지금까지 그 문구는 Next
 * 소스에 상수로 박혀 있어서, 한 줄 고치려면 배포를 해야 했다 — 2026-09-13에 종목
 * 별칭을 표로 옮긴 것과 같은 이유로 이 화면이 있다.
 *
 * # 「비우면 기본값」이 이 화면의 규칙 전부다
 *
 * 칸을 비우면 코드의 기본값이 쓰이고, **전부 비우면 그 행이 지워진다**. 그래서
 * 저장과 되돌리기가 사실상 한 동작이고, 관리자는 「지금 이 화면이 덮어써져 있나」를
 * 목록의 배지 하나로 안다.
 *
 * 입력칸의 회색 글씨(placeholder)가 **그 기본값 자체**다. 「무엇을 덮어쓰는지」를
 * 따로 설명하지 않고 보여 주는 쪽이 짧다.
 *
 * # 미리보기를 왜 두나
 *
 * 제목은 브라우저 탭에서 브랜드가 뒤에 붙고(`%s · Quant Port`), 설명은 검색 결과에서
 * 대략 150자쯤에서 잘린다. 적는 칸만 있으면 그 두 가지가 보이지 않아, 관리자는
 * 저장하고 검색에 반영될 때까지 기다린 뒤에야 길이를 알게 된다.
 */

/** 구글이 스니펫으로 보여 주는 대략적인 상한. 넘으면 뒤가 「…」로 잘린다. */
const SNIPPET_CHARS = 155;

type Draft = {
  title: string;
  description: string;
  ogTitle: string;
  ogDescription: string;
  ogImage: string;
  /** "" = 기본값 따름. */
  noindex: "" | "true" | "false";
};

const EMPTY_DRAFT: Draft = {
  title: "",
  description: "",
  ogTitle: "",
  ogDescription: "",
  ogImage: "",
  noindex: "",
};

function draftOf(override: PageMetaOverride | undefined): Draft {
  if (!override) return EMPTY_DRAFT;
  return {
    title: override.title ?? "",
    description: override.description ?? "",
    ogTitle: override.ogTitle ?? "",
    ogDescription: override.ogDescription ?? "",
    ogImage: override.ogImage ?? "",
    noindex: override.noindex == null ? "" : override.noindex ? "true" : "false",
  };
}

function isEmptyDraft(draft: Draft): boolean {
  return (
    draft.title.trim() === "" &&
    draft.description.trim() === "" &&
    draft.ogTitle.trim() === "" &&
    draft.ogDescription.trim() === "" &&
    draft.ogImage.trim() === "" &&
    draft.noindex === ""
  );
}

const inputClass =
  "mt-1 w-full rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 text-cell text-[var(--text-primary)]";

function Field({
  label,
  hint,
  value,
  placeholder,
  onChange,
  multiline = false,
}: {
  label: string;
  hint?: string;
  value: string;
  placeholder: string;
  onChange: (next: string) => void;
  multiline?: boolean;
}) {
  return (
    <label className="block">
      <span className="text-micro text-[var(--text-muted)]">{label}</span>
      {multiline ? (
        <textarea
          rows={3}
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          className={cn(inputClass, "resize-y leading-relaxed")}
        />
      ) : (
        <input
          value={value}
          placeholder={placeholder}
          onChange={(e) => onChange(e.target.value)}
          className={inputClass}
        />
      )}
      {hint && <span className="mt-1 block text-micro text-[var(--text-muted)]">{hint}</span>}
    </label>
  );
}

export function AdminSeoPanel() {
  const router = useRouter();
  const confirm = useConfirm();
  const { session, loading } = useAuth();

  const [overrides, setOverrides] = useState<PageMetaOverride[]>([]);
  const [selected, setSelected] = useState<string>(PAGE_ROUTES[0]?.path ?? "/");
  const [draft, setDraft] = useState<Draft>(EMPTY_DRAFT);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  const byPath = useMemo(
    () => new Map(overrides.map((item) => [item.path, item])),
    [overrides],
  );

  /*
    목록을 받으면 **지금 고르고 있는 화면의 초안도 다시 깐다.** 저장 응답이 목록
    전체라(빈 값을 저장하면 행이 지워진다) 여기서 맞추지 않으면 화면만 옛 초안을
    들고 「저장했는데 안 지워졌다」로 보인다.
  */
  const applyItems = useCallback((items: PageMetaOverride[], path: string) => {
    setOverrides(items);
    setDraft(draftOf(items.find((item) => item.path === path)));
  }, []);

  const load = useCallback(
    async (path: string) => {
      setBusy(true);
      setError(null);
      try {
        const res = await fetchAdminPageMeta();
        applyItems(res.items ?? [], path);
      } catch (e) {
        setError(toUserMessage(e, "불러오기 실패"));
      } finally {
        setBusy(false);
      }
    },
    [applyItems],
  );

  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    void load(selected);
    // 첫 적재만 한다 — 화면을 바꿀 때는 아래 `pick`이 이미 받아 둔 목록에서 꺼낸다.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [loading, session, router]);

  const route: PageRoute = useMemo(
    () => PAGE_ROUTES.find((r) => r.path === selected) ?? PAGE_ROUTES[0]!,
    [selected],
  );

  const pick = (path: string) => {
    setSelected(path);
    setDraft(draftOf(byPath.get(path)));
    setNotice(null);
    setError(null);
  };

  const handleSave = async () => {
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const res = await saveAdminPageMeta({
        path: route.path,
        title: draft.title.trim(),
        description: draft.description.trim(),
        ogTitle: draft.ogTitle.trim(),
        ogDescription: draft.ogDescription.trim(),
        ogImage: draft.ogImage.trim(),
        noindex: draft.noindex === "" ? null : draft.noindex === "true",
      });
      applyItems(res.items ?? [], route.path);
      setNotice(
        isEmptyDraft(draft)
          ? "덮어쓰기를 지웠습니다. 이 화면은 코드 기본값으로 돌아갑니다."
          : "저장했습니다. 실제 화면에는 1~2분 뒤에 반영됩니다(서버 캐시).",
      );
    } catch (e) {
      setError(toUserMessage(e, "저장 실패"));
    } finally {
      setBusy(false);
    }
  };

  const handleReset = async () => {
    const ok = await confirm({
      title: `「${route.label}」을 기본값으로 되돌릴까요?`,
      description: <p>저장해 둔 문구가 지워지고 코드의 기본값이 다시 쓰입니다.</p>,
      confirmLabel: "되돌리기",
      tone: "danger",
    });
    if (!ok) return;
    setBusy(true);
    setError(null);
    setNotice(null);
    try {
      const res = await resetAdminPageMeta(route.path);
      applyItems(res.items ?? [], route.path);
      setNotice("기본값으로 되돌렸습니다.");
    } catch (e) {
      setError(toUserMessage(e, "되돌리기 실패"));
    } finally {
      setBusy(false);
    }
  };

  if (loading || !isAdminSession(session)) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  // 미리보기는 **저장 전 초안**으로 그린다 — 저장하고 나서야 길이를 아는 것이 문제였다.
  const effectiveTitle = draft.title.trim() || route.title;
  const effectiveDescription = draft.description.trim() || route.description;
  const tabTitle = route.standaloneTitle ? effectiveTitle : `${effectiveTitle} · ${SITE_NAME}`;
  const indexable = draft.noindex === "" ? route.indexable : draft.noindex === "false";
  const overridden = byPath.has(route.path);

  return (
    <>
      <PageHeader
        title="화면 메타 관리"
        subtitle="브라우저 탭 이름과 검색 결과에 나가는 설명을 배포 없이 고칩니다"
      />
      <PageScrollShell bare>
        <div className="page-shell-wide min-w-0 space-y-6 py-4">
          {error && (
            <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-300">
              {error}
            </div>
          )}
          {notice && (
            <div className="rounded-lg border border-[var(--brand)]/30 bg-[var(--brand-muted)] px-3 py-2 text-sm text-[var(--brand)]">
              {notice}
            </div>
          )}

          <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
            {/* ---- 화면 목록 ---- */}
            <section className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-2">
              <p className="px-2 py-1.5 text-micro text-[var(--text-muted)]">
                고칠 화면을 고르세요. 점이 붙은 화면은 이미 덮어써져 있습니다.
              </p>
              <ul className="max-h-[32rem] overflow-y-auto">
                {PAGE_ROUTES.map((item) => (
                  <li key={item.path}>
                    <button
                      type="button"
                      onClick={() => pick(item.path)}
                      className={cn(
                        "flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-cell",
                        item.path === route.path
                          ? "bg-[var(--brand-muted)] text-[var(--brand)]"
                          : "text-[var(--text-primary)] hover:bg-[var(--surface-elevated)]",
                      )}
                    >
                      <span
                        className={cn(
                          "size-1.5 shrink-0 rounded-full",
                          byPath.has(item.path) ? "bg-[var(--brand)]" : "bg-transparent",
                        )}
                        aria-hidden
                      />
                      <span className="min-w-0 flex-1 truncate">{item.label}</span>
                      <span className="font-num shrink-0 text-micro text-[var(--text-muted)]">
                        {item.path}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            </section>

            {/* ---- 편집 ---- */}
            <section className="min-w-0 space-y-4">
              <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
                <div className="flex flex-wrap items-baseline justify-between gap-2">
                  <h2 className="text-section font-semibold text-[var(--text-primary)]">
                    {route.label}
                  </h2>
                  <span className="font-num text-micro text-[var(--text-muted)]">
                    {route.path}
                    {overridden ? " · 덮어쓰기 있음" : " · 기본값"}
                  </span>
                </div>
                <p className="mt-1 text-cell text-[var(--text-muted)]">
                  칸을 비우면 회색 글씨로 보이는 <b>기본값</b>이 쓰입니다. 전부 비우고
                  저장하면 덮어쓰기가 지워집니다.
                </p>

                <div className="mt-3 space-y-3">
                  <Field
                    label="제목"
                    value={draft.title}
                    placeholder={route.title}
                    onChange={(title) => setDraft({ ...draft, title })}
                  />
                  <Field
                    label="설명"
                    hint={`검색 결과에서 대략 ${SNIPPET_CHARS}자까지 보입니다.`}
                    multiline
                    value={draft.description}
                    placeholder={route.description}
                    onChange={(description) => setDraft({ ...draft, description })}
                  />

                  <label className="block">
                    <span className="text-micro text-[var(--text-muted)]">검색 노출</span>
                    <select
                      value={draft.noindex}
                      onChange={(e) =>
                        setDraft({ ...draft, noindex: e.target.value as Draft["noindex"] })
                      }
                      className={inputClass}
                    >
                      <option value="">
                        기본값 따름 ({route.indexable ? "노출함" : "제외함"})
                      </option>
                      <option value="false">노출함</option>
                      <option value="true">제외함</option>
                    </select>
                    <span className="mt-1 block text-micro text-[var(--text-muted)]">
                      제외하면 정규 주소(canonical)도 함께 빠집니다 — 「세지 말라」고 해 놓고
                      정규 주소를 다는 것은 모순이기 때문입니다.
                    </span>
                  </label>
                </div>
              </div>

              <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
                <h3 className="text-section font-semibold text-[var(--text-primary)]">
                  SNS 카드 (선택)
                </h3>
                <p className="mt-1 text-cell text-[var(--text-muted)]">
                  카카오톡·슬랙에 링크를 붙였을 때의 문구입니다. 비우면 위의 제목·설명이
                  그대로 쓰입니다.
                </p>
                <div className="mt-3 space-y-3">
                  <Field
                    label="OG 제목"
                    value={draft.ogTitle}
                    placeholder={
                      route.standaloneTitle ? effectiveTitle : `${effectiveTitle} · ${SITE_NAME}`
                    }
                    onChange={(ogTitle) => setDraft({ ...draft, ogTitle })}
                  />
                  <Field
                    label="OG 설명"
                    multiline
                    value={draft.ogDescription}
                    placeholder={effectiveDescription}
                    onChange={(ogDescription) => setDraft({ ...draft, ogDescription })}
                  />
                  <Field
                    label="OG 이미지 주소"
                    hint="비우면 기본 카드 이미지(/opengraph-image)를 씁니다."
                    value={draft.ogImage}
                    placeholder="/opengraph-image"
                    onChange={(ogImage) => setDraft({ ...draft, ogImage })}
                  />
                </div>
              </div>

              {/* ---- 미리보기 ---- */}
              <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4">
                <h3 className="text-section font-semibold text-[var(--text-primary)]">미리보기</h3>
                <div className="mt-3 space-y-3">
                  <div>
                    <span className="text-micro text-[var(--text-muted)]">브라우저 탭</span>
                    <p className="mt-1 truncate rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-2 py-1.5 text-cell text-[var(--text-primary)]">
                      {tabTitle}
                    </p>
                  </div>
                  <div>
                    <span className="text-micro text-[var(--text-muted)]">
                      검색 결과 {indexable ? "" : "(지금은 노출하지 않습니다)"}
                    </span>
                    <div
                      className={cn(
                        "mt-1 rounded-lg border border-[var(--border)] bg-[var(--surface-elevated)] px-3 py-2",
                        indexable ? "" : "opacity-50",
                      )}
                    >
                      <p className="text-cell text-[var(--brand)]">{tabTitle}</p>
                      <p className="font-num text-micro text-[var(--text-muted)]">
                        www.quant-port.com{route.path === "/" ? "" : route.path}
                      </p>
                      <p className="mt-1 text-cell leading-relaxed text-[var(--text-primary)]">
                        {effectiveDescription.length > SNIPPET_CHARS
                          ? `${effectiveDescription.slice(0, SNIPPET_CHARS)}…`
                          : effectiveDescription}
                      </p>
                    </div>
                    {effectiveDescription.length > SNIPPET_CHARS && (
                      <span className="mt-1 block text-micro text-[var(--text-muted)]">
                        {effectiveDescription.length}자 — 뒤 {effectiveDescription.length -
                          SNIPPET_CHARS}
                        자는 검색 결과에서 잘립니다.
                      </span>
                    )}
                  </div>
                </div>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => void handleSave()}
                  disabled={busy}
                  className={cn(
                    buttonVariants({ variant: "default", size: "sm" }),
                    "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-50",
                  )}
                >
                  {busy ? (
                    <Loader2 className="mr-1.5 size-3.5 animate-spin" />
                  ) : (
                    <Save className="mr-1.5 size-3.5" />
                  )}
                  저장
                </button>
                <button
                  type="button"
                  onClick={() => void handleReset()}
                  disabled={busy || !overridden}
                  className={cn(
                    buttonVariants({ variant: "outline", size: "sm" }),
                    "disabled:opacity-40",
                  )}
                >
                  <RotateCcw className="mr-1.5 size-3.5" />
                  기본값으로
                </button>
              </div>
            </section>
          </div>
        </div>
      </PageScrollShell>
    </>
  );
}
