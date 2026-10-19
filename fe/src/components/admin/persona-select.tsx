"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import {
  createPersona,
  fetchPersonaBadges,
  fetchPersonas,
  type BotPersona,
  type PersonaBadgeOption,
} from "@/lib/bot-admin";
import { toUserMessage } from "@/lib/error-message";
import { cn } from "@/lib/utils";

const SELECT_CLASS =
  "rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1.5 text-cell text-[var(--text-primary)] outline-none focus:border-[var(--brand)]";

/** 서비스 뱃지 목록 — 화면마다 다시 받지 않게 한 번만. */
let badgeCache: Promise<PersonaBadgeOption[]> | null = null;
export function usePersonaBadges(): PersonaBadgeOption[] {
  const [list, setList] = useState<PersonaBadgeOption[]>([]);
  useEffect(() => {
    let alive = true;
    badgeCache ??= fetchPersonaBadges().catch(() => {
      badgeCache = null;
      return [];
    });
    void badgeCache.then((l) => {
      if (alive) setList(l);
    });
    return () => {
      alive = false;
    };
  }, []);
  return list;
}

/** 뱃지 고르기 — 서비스에 있는 뱃지를 여러 개 켠다(2026-10-19 사용자 요청). 실제 화면과 같은 모양으로 보인다. */
export function BadgePicker({ value, onChange }: { value: string[]; onChange: (keys: string[]) => void }) {
  const options = usePersonaBadges();
  return (
    <div className="flex flex-wrap gap-1.5" role="group" aria-label="뱃지">
      {options.map((b) => {
        const on = value.includes(b.key);
        return (
          <button
            key={b.key}
            type="button"
            aria-pressed={on}
            onClick={() => onChange(on ? value.filter((k) => k !== b.key) : [...value, b.key])}
            className={cn(
              "inline-flex items-center gap-0.5 rounded-md border px-1.5 py-px text-micro font-semibold",
              on
                ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
                : "border-[var(--border)] text-[var(--text-muted)] hover:border-[var(--brand)]",
            )}
          >
            <span aria-hidden>{b.emoji}</span>
            {b.label}
          </button>
        );
      })}
    </div>
  );
}

/**
 * 게시 계정 고르기 — 관리자 전용(봇 초안 게시 · 답글). 목록에 없으면 이름·뱃지를 골라 그 자리에서 만든다.
 * 이름은 가입 닉네임과 같은 규칙(2~20자, 중복 불가). 말투는 「봇 초안」 화면의 게시 계정 목록에서 적는다.
 */
export function PersonaSelect({
  value,
  onChange,
  className,
}: {
  value: number | null;
  onChange: (id: number | null) => void;
  className?: string;
}) {
  const [personas, setPersonas] = useState<BotPersona[]>([]);
  const [adding, setAdding] = useState(false);
  const [name, setName] = useState("");
  const [badges, setBadges] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const options = usePersonaBadges();
  const badgeText = (keys: string[]) =>
    keys
      .map((k) => options.find((o) => o.key === k))
      .filter(Boolean)
      .map((o) => `${o!.emoji}${o!.label}`)
      .join(" ");

  const load = useCallback(async () => {
    try {
      setPersonas(await fetchPersonas());
    } catch (e) {
      toast.error(toUserMessage(e, "게시 계정을 불러오지 못했습니다."));
    }
  }, []);

  useEffect(() => {
    let alive = true;
    Promise.resolve().then(() => {
      if (alive) void load();
    });
    return () => {
      alive = false;
    };
  }, [load]);

  async function add() {
    setSaving(true);
    try {
      const { id } = await createPersona({ displayName: name.trim(), badges, style: "" });
      await load();
      onChange(id);
      setAdding(false);
      setName("");
      setBadges([]);
    } catch (e) {
      toast.error(toUserMessage(e, "게시 계정을 만들지 못했습니다."));
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className={className}>
      <div className="flex flex-wrap items-center gap-2">
        <span className="text-cell font-semibold text-[var(--text-secondary)]">게시 계정</span>
        <select
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value ? Number(e.target.value) : null)}
          className={SELECT_CLASS}
          aria-label="게시 계정"
        >
          <option value="">고르세요</option>
          {personas.map((p) => (
            <option key={p.id} value={p.id}>
              {p.displayName}
              {p.badges.length > 0 ? ` · ${badgeText(p.badges)}` : ""}
            </option>
          ))}
        </select>
        <Button type="button" size="sm" variant="outline" onClick={() => setAdding((v) => !v)}>
          {adding ? "닫기" : "새 계정"}
        </Button>
      </div>
      {adding && (
        <div className="mt-2 space-y-2">
          <div className="flex flex-wrap items-center gap-2">
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="이름 (2~20자)"
              maxLength={20}
              className="h-8 w-40"
            />
            <Button type="button" size="sm" onClick={() => void add()} disabled={saving || name.trim().length < 2}>
              만들기
            </Button>
          </div>
          <BadgePicker value={badges} onChange={setBadges} />
        </div>
      )}
    </div>
  );
}
