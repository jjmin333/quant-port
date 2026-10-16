"use client";

import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { createPersona, fetchPersonas, type BotPersona } from "@/lib/bot-admin";
import { toUserMessage } from "@/lib/error-message";

const SELECT_CLASS =
  "rounded-md border border-[var(--border)] bg-[var(--background)] px-2 py-1.5 text-cell text-[var(--text-primary)] outline-none focus:border-[var(--brand)]";

/**
 * 게시 계정 고르기 — 관리자 전용(봇 초안 게시 · 답글). 목록에 없으면 이름·뱃지를 적어 그 자리에서 만든다.
 * 이름은 가입 닉네임과 같은 규칙(2~20자, 중복 불가), 뱃지는 비워도 되고 20자까지(서버가 거부한다).
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
  const [badge, setBadge] = useState("");
  const [saving, setSaving] = useState(false);

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
      const { id } = await createPersona(name.trim(), badge.trim());
      await load();
      onChange(id);
      setAdding(false);
      setName("");
      setBadge("");
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
              {p.badge ? ` · ${p.badge}` : ""}
            </option>
          ))}
        </select>
        <Button type="button" size="sm" variant="outline" onClick={() => setAdding((v) => !v)}>
          {adding ? "닫기" : "새 계정"}
        </Button>
      </div>
      {adding && (
        <div className="mt-2 flex flex-wrap items-center gap-2">
          <Input
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="이름 (2~20자)"
            maxLength={20}
            className="h-8 w-40"
          />
          <Input
            value={badge}
            onChange={(e) => setBadge(e.target.value)}
            placeholder="뱃지 (선택, 20자)"
            maxLength={20}
            className="h-8 w-44"
          />
          <Button type="button" size="sm" onClick={() => void add()} disabled={saving || name.trim().length < 2}>
            만들기
          </Button>
        </div>
      )}
    </div>
  );
}
