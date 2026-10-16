"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { Loader2 } from "lucide-react";
import { toast } from "sonner";
import { PageHeader } from "@/components/layout/header";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { useAuth } from "@/components/providers/auth-provider";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
import { isAdminSession } from "@/lib/admin-features";
import {
  discardBotDraft,
  fetchBotDrafts,
  fetchPersonaActivity,
  fetchPersonas,
  updatePersona,
  type BotActivity,
  type BotDraft,
  type BotDraftKind,
  type BotDraftStatus,
  type BotPersona,
} from "@/lib/bot-admin";
import { addCommunityComment } from "@/lib/community-api";
import { toUserMessage } from "@/lib/error-message";
import { formatDateTimeKST } from "@/lib/format";
import { addResearchCommentClient } from "@/lib/research-api";
import { cn } from "@/lib/utils";

const KIND_LABEL: Record<BotDraftKind, string> = { community: "커뮤니티", research: "투자 리포트" };
const MARKET_LABEL: Record<string, string> = { domestic: "국내", us: "해외" };

/**
 * 봇 초안 (2026-10-16 사용자 요청) — 3시간마다(하루 8번) 커뮤니티 5개 · 리포트 2개가 쌓인다.
 * 「편집해서 게시」는 일반 글쓰기 화면을 초안이 채워진 채로 연다(`?draft=`). 거기서 게시 계정을 골라 올린다.
 * 아래 「게시 계정 글의 새 댓글」에서 그 계정으로 답글을 단다.
 */
export function AdminBotsPanel() {
  const router = useRouter();
  const { session, loading } = useAuth();
  const [kind, setKind] = useState<BotDraftKind>("community");
  const [status, setStatus] = useState<BotDraftStatus>("draft");
  const [drafts, setDrafts] = useState<BotDraft[] | null>(null);
  const [personas, setPersonas] = useState<BotPersona[]>([]);
  const [activity, setActivity] = useState<BotActivity[]>([]);

  const loadDrafts = useCallback(async () => {
    try {
      setDrafts(await fetchBotDrafts(kind, status));
    } catch (e) {
      toast.error(toUserMessage(e, "초안을 불러오지 못했습니다."));
      setDrafts([]);
    }
  }, [kind, status]);

  const loadSide = useCallback(async () => {
    try {
      const [p, a] = await Promise.all([fetchPersonas(), fetchPersonaActivity()]);
      setPersonas(p);
      setActivity(a);
    } catch (e) {
      toast.error(toUserMessage(e, "게시 계정을 불러오지 못했습니다."));
    }
  }, []);

  const admin = !loading && isAdminSession(session);
  useEffect(() => {
    if (loading) return;
    if (!isAdminSession(session)) {
      router.replace("/");
      return;
    }
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void loadDrafts();
    });
    return () => {
      alive = false;
    };
  }, [loading, session, router, loadDrafts]);

  useEffect(() => {
    if (!admin) return;
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void loadSide();
    });
    return () => {
      alive = false;
    };
  }, [admin, loadSide]);

  async function discard(id: number) {
    try {
      await discardBotDraft(id);
      setDrafts((prev) => prev?.filter((d) => d.id !== id) ?? null);
    } catch (e) {
      toast.error(toUserMessage(e, "버리지 못했습니다."));
    }
  }

  if (!admin) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-[var(--text-muted)]">
        <Loader2 className="mr-2 h-4 w-4 animate-spin" />
        확인 중…
      </div>
    );
  }

  return (
    <>
      <PageHeader
        title="봇 초안"
        subtitle="3시간마다 커뮤니티 5개 · 리포트 2개 — 고쳐서 게시 계정으로 올립니다. 게시하지 않은 초안은 24시간 뒤 사라집니다"
      />
      <PageScrollShell bare>
        <div className="space-y-4">
          <div className="flex flex-wrap items-center gap-2">
            {(["community", "research"] as const).map((k) => (
              <Pill key={k} active={kind === k} onClick={() => setKind(k)}>
                {KIND_LABEL[k]}
              </Pill>
            ))}
            <span className="mx-1 h-4 w-px bg-[var(--border)]" aria-hidden />
            {(["draft", "published"] as const).map((s) => (
              <Pill key={s} active={status === s} onClick={() => setStatus(s)}>
                {s === "draft" ? "초안" : "게시함"}
              </Pill>
            ))}
          </div>

          {drafts === null ? (
            <Card className="p-6 text-center text-sm text-[var(--text-muted)]">불러오는 중…</Card>
          ) : drafts.length === 0 ? (
            <Card className="p-6 text-center text-sm text-[var(--text-muted)]">
              {status === "draft" ? "남은 초안이 없습니다. 다음 회차(3시간마다)에 채워집니다." : "게시한 초안이 없습니다."}
            </Card>
          ) : (
            <ul className="space-y-3">
              {drafts.map((d) => (
                <DraftCard key={d.id} draft={d} personas={personas} onDiscard={() => void discard(d.id)} />
              ))}
            </ul>
          )}

          <PersonaList personas={personas} onSaved={() => void loadSide()} />
          <ActivityList items={activity} onReplied={() => void loadSide()} />
        </div>
      </PageScrollShell>
    </>
  );
}

function Pill({ active, onClick, children }: { active: boolean; onClick: () => void; children: React.ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={cn(
        "rounded-full border px-3 py-1 text-cell font-semibold",
        active
          ? "border-[var(--brand)] bg-[var(--brand)] text-white"
          : "border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]",
      )}
    >
      {children}
    </button>
  );
}

function DraftCard({ draft, personas, onDiscard }: { draft: BotDraft; personas: BotPersona[]; onDiscard: () => void }) {
  const [open, setOpen] = useState(false);
  const href = draft.kind === "community" ? `/community/new?draft=${draft.id}` : `/reports/new?draft=${draft.id}`;
  const by = personas.find((p) => p.id === draft.publishedBy);
  return (
    <li>
      <Card className="space-y-2 p-4">
        <div className="flex flex-wrap items-center gap-2 text-micro text-[var(--text-muted)]">
          <span>{formatDateTimeKST(draft.slotAt)} 회차</span>
          <span className="rounded bg-[var(--surface-elevated)] px-1.5 font-semibold">
            {MARKET_LABEL[draft.market] ?? draft.market} · {draft.symbolName || draft.symbol}
          </span>
          <span
            className={cn(
              "rounded px-1.5 font-semibold",
              draft.length === "long" ? "bg-sky-500/10 text-sky-700 dark:text-sky-400" : "bg-[var(--surface-elevated)]",
            )}
          >
            {draft.length === "long" ? "긴 글" : "짧은 글"}
          </span>
          {draft.reason && <span className="min-w-0 truncate">고른 이유: {draft.reason}</span>}
        </div>
        <h3 className="text-note font-semibold text-[var(--text-primary)]">{draft.title}</h3>
        <p className={cn("whitespace-pre-wrap text-cell text-[var(--text-secondary)]", !open && "line-clamp-4")}>
          {draft.body}
        </p>
        {draft.articles.length > 0 && (
          <p className="text-micro text-[var(--text-muted)]">붙일 기사: {draft.articles.map((a) => a.title).join(" · ")}</p>
        )}
        <div className="flex flex-wrap items-center gap-2 pt-1">
          <Button type="button" size="sm" variant="ghost" onClick={() => setOpen((v) => !v)}>
            {open ? "접기" : "전체 보기"}
          </Button>
          {draft.status === "draft" ? (
            <>
              <Button asChild size="sm">
                <Link href={href}>편집해서 게시</Link>
              </Button>
              <Button type="button" size="sm" variant="outline" onClick={onDiscard}>
                버리기
              </Button>
            </>
          ) : (
            draft.publishedRef && (
              <Link
                href={draft.kind === "community" ? `/community/${draft.publishedRef}` : `/reports/${draft.publishedRef}`}
                className="text-cell font-semibold text-[var(--brand)] hover:underline"
              >
                올린 글 보기{by ? ` · ${by.displayName}` : ""}
              </Link>
            )
          )}
        </div>
      </Card>
    </li>
  );
}

function PersonaList({ personas, onSaved }: { personas: BotPersona[]; onSaved: () => void }) {
  return (
    <Card className="p-0">
      <h2 className="border-b border-[var(--border)] px-4 py-2.5 text-note font-semibold">
        게시 계정 {personas.length}
        <span className="ml-2 text-micro font-normal text-[var(--text-muted)]">새 계정은 글쓰기 화면의 「새 계정」에서 만듭니다</span>
      </h2>
      {personas.length === 0 ? (
        <p className="px-4 py-4 text-cell text-[var(--text-muted)]">아직 게시 계정이 없습니다.</p>
      ) : (
        <ul className="divide-y divide-[var(--border)]">
          {personas.map((p) => (
            <PersonaRow key={p.id} persona={p} onSaved={onSaved} />
          ))}
        </ul>
      )}
    </Card>
  );
}

function PersonaRow({ persona, onSaved }: { persona: BotPersona; onSaved: () => void }) {
  const [name, setName] = useState(persona.displayName);
  const [badge, setBadge] = useState(persona.badge);
  const dirty = name !== persona.displayName || badge !== persona.badge;
  async function save() {
    try {
      await updatePersona(persona.id, name.trim(), badge.trim());
      toast.success("저장했습니다.");
      onSaved();
    } catch (e) {
      toast.error(toUserMessage(e, "저장하지 못했습니다."));
    }
  }
  return (
    <li className="flex flex-wrap items-center gap-2 px-4 py-2.5 text-cell">
      <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={20} className="h-8 w-40" aria-label="이름" />
      <Input
        value={badge}
        onChange={(e) => setBadge(e.target.value)}
        maxLength={20}
        placeholder="뱃지 없음"
        className="h-8 w-44"
        aria-label="뱃지"
      />
      <span className="text-micro text-[var(--text-muted)]">
        글 {persona.posts} · 리포트 {persona.calls}
      </span>
      {dirty && (
        <Button type="button" size="sm" onClick={() => void save()}>
          저장
        </Button>
      )}
    </li>
  );
}

function ActivityList({ items, onReplied }: { items: BotActivity[]; onReplied: () => void }) {
  return (
    <Card className="p-0">
      <h2 className="border-b border-[var(--border)] px-4 py-2.5 text-note font-semibold">
        게시 계정 글의 새 댓글 (7일) {items.length}
      </h2>
      {items.length === 0 ? (
        <p className="px-4 py-4 text-cell text-[var(--text-muted)]">새 댓글이 없습니다.</p>
      ) : (
        <ul className="divide-y divide-[var(--border)]">
          {items.map((a) => (
            <ActivityRow key={`${a.kind}-${a.commentId}`} item={a} onReplied={onReplied} />
          ))}
        </ul>
      )}
    </Card>
  );
}

function ActivityRow({ item, onReplied }: { item: BotActivity; onReplied: () => void }) {
  const [reply, setReply] = useState("");
  const [sending, setSending] = useState(false);
  const href = item.kind === "community" ? `/community/${item.targetId}` : `/reports/${item.targetId}`;
  async function send() {
    setSending(true);
    try {
      if (item.kind === "community") {
        await addCommunityComment(item.targetId, reply.trim(), item.commentId, item.personaId);
      } else {
        await addResearchCommentClient(item.targetId, reply.trim(), item.commentId, item.personaId);
      }
      setReply("");
      toast.success(`${item.personaName}(으)로 답글을 달았습니다.`);
      onReplied();
    } catch (e) {
      toast.error(toUserMessage(e, "답글을 달지 못했습니다."));
    } finally {
      setSending(false);
    }
  }
  return (
    <li className="space-y-1.5 px-4 py-3 text-cell">
      <div className="flex flex-wrap items-center gap-2 text-micro text-[var(--text-muted)]">
        <span className="rounded bg-[var(--surface-elevated)] px-1.5 font-semibold">{KIND_LABEL[item.kind]}</span>
        <Link href={href} className="min-w-0 truncate font-semibold text-[var(--text-secondary)] hover:text-[var(--brand)]">
          {item.targetTitle}
        </Link>
        <span>· {item.personaName}의 글</span>
        <span>{formatDateTimeKST(item.createdAt)}</span>
      </div>
      <p className="whitespace-pre-wrap text-[var(--text-primary)]">
        <span className="font-semibold">{item.authorName}</span> {item.body}
      </p>
      <div className="flex items-center gap-2">
        <Input
          value={reply}
          onChange={(e) => setReply(e.target.value)}
          placeholder={`${item.personaName}(으)로 답글`}
          className="h-8 min-w-0 flex-1"
          maxLength={1000}
        />
        <Button type="button" size="sm" onClick={() => void send()} disabled={sending || reply.trim() === ""}>
          답글
        </Button>
      </div>
    </li>
  );
}
