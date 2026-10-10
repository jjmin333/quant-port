import { Fragment, type ReactNode } from "react";

/**
 * 검색 유입 보고서 Markdown을 그린다 — worker-growth가 쓰는 문법만.
 *
 * 라이브러리를 들이지 않은 이유: 보고서는 우리가 만드는 글이라 문법이 정해져 있다(제목 ·
 * 표 · 목록 · 인용 · 구분선 · **굵게** · _기울임_ · `코드`). 그 정도를 위해 파서 의존성을
 * 번들에 싣는 것보다 이 파일이 싸다. **innerHTML을 쓰지 않는다** — 보고서에는 검색어(외부
 * 사람이 친 글자)가 그대로 들어가므로, 전부 React 텍스트 노드로만 그린다.
 *
 * 표는 가로로 넘칠 수 있어 칸을 감싸 가로 스크롤을 준다(모바일 폭).
 */
export function GrowthMarkdown({ source }: { source: string }) {
  return <div className="space-y-3 text-cell leading-relaxed text-[var(--text-primary)]">{renderBlocks(source)}</div>;
}

function renderBlocks(source: string): ReactNode[] {
  const lines = source.replace(/\r\n/g, "\n").split("\n");
  const out: ReactNode[] = [];
  let i = 0;
  while (i < lines.length) {
    const line = lines[i]!;
    const key = `b${i}`;
    if (line.trim() === "") {
      i++;
      continue;
    }
    const heading = /^(#{1,3})\s+(.*)$/.exec(line);
    if (heading) {
      const level = heading[1]!.length;
      const cls =
        level === 1
          ? "text-lg font-semibold"
          : level === 2
            ? "mt-4 border-b border-[var(--border)] pb-1 text-section font-semibold"
            : "text-cell font-semibold";
      out.push(
        <p key={key} className={cls}>
          {inline(heading[2]!)}
        </p>,
      );
      i++;
      continue;
    }
    if (/^---+\s*$/.test(line)) {
      out.push(<hr key={key} className="border-[var(--border)]" />);
      i++;
      continue;
    }
    if (line.startsWith("|")) {
      const rows: string[] = [];
      while (i < lines.length && lines[i]!.startsWith("|")) rows.push(lines[i++]!);
      out.push(<Table key={key} rows={rows} />);
      continue;
    }
    if (/^>\s?/.test(line)) {
      const quote: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i]!)) quote.push(lines[i++]!.replace(/^>\s?/, ""));
      out.push(
        <p key={key} className="border-l-2 border-[var(--border)] pl-3 text-[var(--text-muted)]">
          {inline(quote.join(" "))}
        </p>,
      );
      continue;
    }
    if (/^(- |\d+\. )/.test(line)) {
      const ordered = /^\d+\. /.test(line);
      const marker = ordered ? /^\d+\. / : /^- /;
      const items: string[] = [];
      while (i < lines.length && marker.test(lines[i]!)) items.push(lines[i++]!.replace(marker, ""));
      const List = ordered ? "ol" : "ul";
      out.push(
        <List key={key} className={`${ordered ? "list-decimal" : "list-disc"} space-y-1 pl-5`}>
          {items.map((item, n) => (
            <li key={n}>{inline(item)}</li>
          ))}
        </List>,
      );
      continue;
    }
    // 문단 — 다음 빈 줄이나 다른 블록까지.
    const para: string[] = [];
    while (i < lines.length && lines[i]!.trim() !== "" && !/^(#|\||>|- |\d+\. |---)/.test(lines[i]!)) para.push(lines[i++]!);
    if (para.length === 0) para.push(lines[i++]!);
    const text = para.join(" ");
    // 줄 전체가 _…_ 인 문단(보고서의 각주). 안에 growth_* 같은 밑줄이 있어 인라인 규칙으로는 못 가른다.
    if (text.length > 2 && text.startsWith("_") && text.endsWith("_")) {
      out.push(
        <p key={key} className="text-micro text-[var(--text-muted)]">
          {inline(text.slice(1, -1))}
        </p>,
      );
      continue;
    }
    out.push(<p key={key}>{inline(text)}</p>);
  }
  return out;
}

function splitRow(row: string): string[] {
  // 「\|」는 칸 안의 글자다(보고서가 그렇게 escape한다).
  const cells: string[] = [];
  let cur = "";
  const body = row.trim().replace(/^\|/, "").replace(/\|$/, "");
  for (let k = 0; k < body.length; k++) {
    const ch = body[k]!;
    if (ch === "\\" && body[k + 1] === "|") {
      cur += "|";
      k++;
    } else if (ch === "|") {
      cells.push(cur.trim());
      cur = "";
    } else {
      cur += ch;
    }
  }
  cells.push(cur.trim());
  return cells;
}

function Table({ rows }: { rows: string[] }) {
  const parsed = rows.map(splitRow).filter((cells) => !cells.every((c) => /^:?-{3,}:?$/.test(c)));
  const [head, ...body] = parsed;
  if (!head) return null;
  return (
    <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
      <table className="w-full min-w-max border-collapse text-cell">
        <thead className="bg-[var(--surface-elevated)] text-left text-micro text-[var(--text-muted)]">
          <tr>
            {head.map((c, n) => (
              <th key={n} className="px-2 py-1.5 font-medium">
                {inline(c)}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {body.map((cells, r) => (
            <tr key={r} className="border-t border-[var(--border)] align-top">
              {cells.map((c, n) => (
                <td key={n} className="max-w-[28rem] px-2 py-1.5">
                  {inline(c)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

/** **굵게** · _기울임_ · `코드` — 겹치지 않는다고 본다(보고서가 겹쳐 쓰지 않는다). */
function inline(text: string): ReactNode {
  const parts = text.split(/(\*\*[^*]+\*\*|`[^`]+`|(?<![\w/])_[^_]+_(?![\w/]))/g);
  return parts.map((part, n) => {
    if (part.startsWith("**") && part.endsWith("**") && part.length > 4) {
      return <strong key={n}>{part.slice(2, -2)}</strong>;
    }
    if (part.startsWith("`") && part.endsWith("`") && part.length > 2) {
      return (
        <code key={n} className="rounded bg-[var(--surface-elevated)] px-1 font-num text-micro">
          {part.slice(1, -1)}
        </code>
      );
    }
    if (part.startsWith("_") && part.endsWith("_") && part.length > 2) {
      return (
        <em key={n} className="text-[var(--text-muted)]">
          {part.slice(1, -1)}
        </em>
      );
    }
    return <Fragment key={n}>{part}</Fragment>;
  });
}
