"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import { Loader2 } from "lucide-react";
import { useAuth } from "@/components/providers/auth-provider";
import { useConfirm } from "@/components/providers/confirm-provider";
import { changeDisplayName, changePassword, deleteAccount } from "@/lib/auth";
import { toUserMessage } from "@/lib/error-message";
import { DISPLAY_NAME_MAX, PASSWORD_MAX, PASSWORD_MIN } from "@/lib/auth-rules";
import { cn } from "@/lib/utils";
import { buttonVariants } from "@/components/ui/button";

/**
 * 계정 설정 (§A-6).
 *
 * 이 화면이 없어서 생기던 구멍: **비밀번호를 바꿀 수도, 탈퇴할 수도 없었다.** 설정에는
 * 색상 모드와 기본 통화뿐이었고 계정 관련 항목은 앱 어디에도 없었다. 비밀번호를 잊은
 * 사람은 계정으로 돌아올 방법이 없고, 그만두려는 사람은 데이터를 남긴 채 떠나야 했다.
 *
 * 세 가지를 한 화면에 둔다 — 표시 이름, 비밀번호, 탈퇴. 위험이 커지는 순서다.
 */

const inputClass =
  "h-9 w-full rounded-lg border border-[var(--border)] bg-[var(--background)] px-3 text-sm text-[var(--text-primary)] focus:border-[var(--brand)] focus:outline-none";
const labelClass = "mb-1 block text-cell text-[var(--text-secondary)]";
const sectionClass = "rounded-xl border border-[var(--border)] bg-[var(--surface)] p-4";

export function AccountSettings() {
  const { session, refresh, logout } = useAuth();
  const confirm = useConfirm();
  const router = useRouter();

  const [name, setName] = useState(session?.user.displayName ?? "");
  const [nameBusy, setNameBusy] = useState(false);
  const [nameMsg, setNameMsg] = useState<{ ok: boolean; text: string } | null>(null);

  const [current, setCurrent] = useState("");
  const [next, setNext] = useState("");
  const [confirmNext, setConfirmNext] = useState("");
  const [pwBusy, setPwBusy] = useState(false);
  const [pwMsg, setPwMsg] = useState<{ ok: boolean; text: string } | null>(null);

  const [deletePw, setDeletePw] = useState("");
  const [deleteBusy, setDeleteBusy] = useState(false);
  const [deleteMsg, setDeleteMsg] = useState<string | null>(null);

  if (!session) {
    return (
      <div className="rounded-xl border border-[var(--border)] bg-[var(--surface)] p-6 text-center">
        <p className="text-sm text-[var(--text-secondary)]">
          계정 설정은 로그인한 뒤에 볼 수 있습니다.
        </p>
        <Link
          href="/login?next=%2Fsettings"
          className={cn(buttonVariants({ variant: "default", size: "sm" }), "mt-3 inline-block bg-[var(--brand)] text-white hover:opacity-90")}
        >
          로그인
        </Link>
      </div>
    );
  }

  async function submitName() {
    setNameBusy(true);
    setNameMsg(null);
    try {
      await changeDisplayName(name.trim());
      await refresh();
      setNameMsg({ ok: true, text: "표시 이름을 바꿨습니다." });
    } catch (err) {
      setNameMsg({ ok: false, text: toUserMessage(err, "표시 이름을 바꾸지 못했습니다.") });
    } finally {
      setNameBusy(false);
    }
  }

  async function submitPassword() {
    // 서버도 검사하지만, 두 번 입력한 값이 서로 다른 것은 **여기서** 잡아야 한다 —
    // 서버는 확인란을 모르고, 왕복해 봐야 같은 답을 받는다.
    if (next !== confirmNext) {
      setPwMsg({ ok: false, text: "새 비밀번호가 서로 다릅니다." });
      return;
    }
    setPwBusy(true);
    setPwMsg(null);
    try {
      await changePassword(current, next);
      setCurrent("");
      setNext("");
      setConfirmNext("");
      /*
        서버가 모든 기기의 세션을 끊는다(비밀번호를 바꾸는 이유의 대부분이 「누가 내
        계정을 보고 있는 것 같다」이므로). 이 브라우저도 예외가 아니라서, 화면만
        로그인 상태로 남겨 두면 이후 요청이 전부 401이 되는 어정쩡한 상태가 된다.
      */
      setPwMsg({ ok: true, text: "비밀번호를 바꿨습니다. 보안을 위해 다시 로그인해 주세요." });
      window.setTimeout(() => {
        void logout();
      }, 1200);
    } catch (err) {
      setPwMsg({ ok: false, text: toUserMessage(err, "비밀번호를 바꾸지 못했습니다.") });
    } finally {
      setPwBusy(false);
    }
  }

  async function submitDelete() {
    const ok = await confirm({
      title: "정말 탈퇴할까요?",
      description: (
        <>
          <p>
            보유·관심종목·목표가·메모·스탬프가 <b>모두 삭제되고 되돌릴 수 없습니다.</b>
          </p>
          <p className="text-[var(--text-muted)]">
            이미 발행한 리포트와 댓글은 다른 사람의 대화가 걸려 있어 남지만, 작성자 이름은
            사라집니다.
          </p>
        </>
      ),
      confirmLabel: "탈퇴하기",
      tone: "danger",
    });
    if (!ok) return;

    setDeleteBusy(true);
    setDeleteMsg(null);
    try {
      await deleteAccount(deletePw);
      router.replace("/");
      // 세션 상태를 비운다 — 헤더가 계속 로그인 상태로 보이면 안 된다.
      await refresh().catch(() => {});
    } catch (err) {
      setDeleteMsg(toUserMessage(err, "탈퇴하지 못했습니다."));
      setDeleteBusy(false);
    }
  }

  return (
    <div className="space-y-4">
      <section className={sectionClass}>
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">계정</h3>
        <p className="mt-1 text-cell text-[var(--text-muted)]">아이디 {session.user.email}</p>

        <div className="mt-3 flex flex-wrap items-end gap-2">
          <div className="min-w-0 flex-1">
            <label className={labelClass} htmlFor="account-display-name">닉네임 (2~10자 · 다른 회원과 겹칠 수 없음)</label>
            <input
              id="account-display-name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={DISPLAY_NAME_MAX}
              className={inputClass}
            />
          </div>
          <button
            type="button"
            onClick={submitName}
            disabled={nameBusy || !name.trim() || name.trim() === session.user.displayName}
            className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-[var(--brand)] px-3.5 text-cell font-semibold text-white hover:opacity-90 disabled:opacity-40"
          >
            {nameBusy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            저장
          </button>
        </div>
        {nameMsg && (
          <p
            className={cn(
              "mt-1.5 text-cell",
              nameMsg.ok ? "text-[var(--success)]" : "text-[var(--danger)]",
            )}
          >
            {nameMsg.text}
          </p>
        )}
      </section>

      <section className={sectionClass}>
        <h3 className="text-sm font-semibold text-[var(--text-primary)]">비밀번호 변경</h3>
        <p className="mt-1 text-cell text-[var(--text-muted)]">
          바꾸고 나면 다른 기기의 로그인도 함께 해제됩니다.
        </p>
        <div className="mt-3 grid gap-2 sm:grid-cols-3">
          <div className="min-w-0">
            <label className={labelClass} htmlFor="account-current-password">현재 비밀번호</label>
            <input
              type="password"
              autoComplete="current-password"
              id="account-current-password"
              value={current}
              onChange={(e) => setCurrent(e.target.value)}
              className={inputClass}
            />
          </div>
          <div className="min-w-0">
            <label className={labelClass} htmlFor="account-new-password">새 비밀번호</label>
            <input
              type="password"
              autoComplete="new-password"
              minLength={PASSWORD_MIN}
              maxLength={PASSWORD_MAX}
              placeholder="8~15자"
              id="account-new-password"
              value={next}
              onChange={(e) => setNext(e.target.value)}
              className={inputClass}
            />
          </div>
          <div className="min-w-0">
            <label className={labelClass} htmlFor="account-confirm-password">새 비밀번호 확인</label>
            <input
              type="password"
              autoComplete="new-password"
              minLength={PASSWORD_MIN}
              maxLength={PASSWORD_MAX}
              id="account-confirm-password"
              value={confirmNext}
              onChange={(e) => setConfirmNext(e.target.value)}
              className={inputClass}
            />
          </div>
        </div>
        <button
          type="button"
          onClick={submitPassword}
          disabled={pwBusy || !current || next.length < 8 || !confirmNext}
          className="mt-3 inline-flex h-9 items-center gap-1.5 rounded-lg bg-[var(--brand)] px-3.5 text-cell font-semibold text-white hover:opacity-90 disabled:opacity-40"
        >
          {pwBusy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
          비밀번호 바꾸기
        </button>
        {pwMsg && (
          <p
            className={cn(
              "mt-1.5 text-cell",
              pwMsg.ok ? "text-[var(--success)]" : "text-[var(--danger)]",
            )}
          >
            {pwMsg.text}
          </p>
        )}
      </section>

      <section className={cn(sectionClass, "border-[var(--danger)]/40")}>
        <h3 className="text-sm font-semibold text-[var(--danger)]">회원 탈퇴</h3>
        <p className="mt-1 text-cell leading-relaxed text-[var(--text-muted)]">
          보유·관심종목·목표가·메모·스탬프가 모두 삭제되고 되돌릴 수 없습니다. 발행한
          리포트와 댓글은 다른 사람의 대화가 걸려 있어 남지만, 작성자 이름은 사라집니다.
        </p>
        <div className="mt-3 flex flex-wrap items-end gap-2">
          <div className="min-w-0 flex-1">
            <label className={labelClass} htmlFor="account-delete-password">확인을 위해 비밀번호를 입력해 주세요</label>
            <input
              type="password"
              autoComplete="current-password"
              id="account-delete-password"
              value={deletePw}
              onChange={(e) => setDeletePw(e.target.value)}
              className={inputClass}
            />
          </div>
          <button
            type="button"
            onClick={submitDelete}
            disabled={deleteBusy || !deletePw}
            className="inline-flex h-9 items-center gap-1.5 rounded-lg bg-[var(--danger)] px-3.5 text-cell font-semibold text-white hover:opacity-90 disabled:opacity-40"
          >
            {deleteBusy && <Loader2 className="h-3.5 w-3.5 animate-spin" />}
            탈퇴하기
          </button>
        </div>
        {deleteMsg && <p className="mt-1.5 text-cell text-[var(--danger)]">{deleteMsg}</p>}
      </section>
    </div>
  );
}
