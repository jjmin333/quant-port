import { plainMentions } from "@/lib/mention-text";

/**
 * A small Markdown parser for user-written 투자 리포트 bodies.
 *
 * It exists instead of a dependency for one reason: the output is rendered as
 * React elements, never as HTML. Reports are public, user-written documents, so
 * a renderer that could be made to emit raw HTML would be a stored-XSS hole on
 * the most-read screen in the product. Nothing in this file produces markup —
 * it produces a tree, and `components/ui/markdown.tsx` turns that into elements.
 *
 * Supported (the subset a blog post actually uses):
 *   # ~ #### 제목 · 문단 · **굵게** · *기울임* · ~~취소선~~ · `코드` · [링크](url)
 *   - / 1. 목록 (중첩 가능) · > 인용 · ``` 코드 블록 ``` · --- 구분선 · GFM 표
 *   ![대체텍스트](url) 이미지 (업로드는 lib/media-api.ts)
 *
 * Deliberately absent: raw HTML. HTML comments are dropped, which is what makes
 * any leftover `<!-- 안내 -->` invisible to readers instead of leaking into
 * their view.
 */

export type InlineNode =
  | { type: "text"; value: string }
  | { type: "strong"; children: InlineNode[] }
  | { type: "em"; children: InlineNode[] }
  | { type: "del"; children: InlineNode[] }
  | { type: "code"; value: string }
  | { type: "link"; href: string; children: InlineNode[] }
  | { type: "image"; src: string; alt: string }
  | { type: "break" };

export type TableAlign = "left" | "center" | "right";

export type BlockNode =
  | { type: "heading"; level: 1 | 2 | 3 | 4; children: InlineNode[] }
  | { type: "paragraph"; children: InlineNode[] }
  | { type: "list"; ordered: boolean; start: number; items: BlockNode[][] }
  | { type: "quote"; children: BlockNode[] }
  | { type: "code"; lang: string; value: string }
  | { type: "hr" }
  | { type: "table"; align: TableAlign[]; header: InlineNode[][]; rows: InlineNode[][][] };

const HEADING_RE = /^(#{1,4})\s+(.*)$/;
const HR_RE = /^\s{0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/;
const FENCE_RE = /^\s{0,3}```\s*([\w+-]*)\s*$/;
const BULLET_RE = /^(\s*)([-*+])\s+(.*)$/;
const ORDERED_RE = /^(\s*)(\d{1,9})[.)]\s+(.*)$/;
const QUOTE_RE = /^\s{0,3}>\s?(.*)$/;
const TABLE_DIVIDER_RE = /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/;

/** Parses a Markdown document into blocks. Never throws on malformed input. */
export function parseMarkdown(source: string): BlockNode[] {
  const text = stripComments(source ?? "").replace(/\r\n?/g, "\n");
  return parseBlocks(text.split("\n"));
}

/** HTML comments are dropped before parsing — see the file comment. */
function stripComments(src: string): string {
  return src.replace(/<!--[\s\S]*?-->/g, "");
}

function parseBlocks(lines: string[]): BlockNode[] {
  const out: BlockNode[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    if (line.trim() === "") {
      i += 1;
      continue;
    }

    const fence = FENCE_RE.exec(line);
    if (fence) {
      const body: string[] = [];
      i += 1;
      while (i < lines.length && !FENCE_RE.test(lines[i])) {
        body.push(lines[i]);
        i += 1;
      }
      i += 1; // closing fence (or EOF, which closes it too)
      out.push({ type: "code", lang: fence[1] ?? "", value: body.join("\n") });
      continue;
    }

    if (HR_RE.test(line)) {
      out.push({ type: "hr" });
      i += 1;
      continue;
    }

    const heading = HEADING_RE.exec(line);
    if (heading) {
      out.push({
        type: "heading",
        level: heading[1].length as 1 | 2 | 3 | 4,
        children: parseInline(heading[2].replace(/\s+#+\s*$/, "")),
      });
      i += 1;
      continue;
    }

    if (QUOTE_RE.test(line)) {
      const inner: string[] = [];
      while (i < lines.length && (QUOTE_RE.test(lines[i]) || lines[i].trim() !== "")) {
        const m = QUOTE_RE.exec(lines[i]);
        if (!m) {
          // A lazy continuation line belongs to the quote's paragraph.
          if (inner.length === 0) break;
          inner.push(lines[i]);
        } else {
          inner.push(m[1]);
        }
        i += 1;
      }
      out.push({ type: "quote", children: parseBlocks(inner) });
      continue;
    }

    if (isTableStart(lines, i)) {
      const table = parseTable(lines, i);
      out.push(table.node);
      i = table.next;
      continue;
    }

    if (BULLET_RE.test(line) || ORDERED_RE.test(line)) {
      const list = parseList(lines, i);
      out.push(list.node);
      i = list.next;
      continue;
    }

    // Paragraph: everything up to a blank line or the start of another block.
    const buf: string[] = [];
    while (i < lines.length && lines[i].trim() !== "" && !startsBlock(lines, i)) {
      buf.push(lines[i].trim());
      i += 1;
    }
    if (buf.length > 0) out.push({ type: "paragraph", children: parseInline(buf.join("\n")) });
  }

  return out;
}

function startsBlock(lines: string[], i: number): boolean {
  const line = lines[i];
  return (
    HEADING_RE.test(line) ||
    HR_RE.test(line) ||
    FENCE_RE.test(line) ||
    QUOTE_RE.test(line) ||
    BULLET_RE.test(line) ||
    ORDERED_RE.test(line) ||
    isTableStart(lines, i)
  );
}

// ------------------------------------------------------------------- lists

function parseList(lines: string[], start: number): { node: BlockNode; next: number } {
  const first = BULLET_RE.exec(lines[start]) ?? ORDERED_RE.exec(lines[start]);
  const ordered = !BULLET_RE.test(lines[start]);
  const baseIndent = (first?.[1] ?? "").length;
  const startNo = ordered ? Number(first?.[2] ?? "1") : 1;

  const items: BlockNode[][] = [];
  let i = start;
  let current: string[] | null = null;

  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === "") {
      // A blank line ends the list unless the next line continues it.
      const next = lines[i + 1];
      if (next === undefined || next.trim() === "") break;
      const indent = next.length - next.trimStart().length;
      if (indent <= baseIndent && !BULLET_RE.test(next) && !ORDERED_RE.test(next)) break;
      if (current) current.push("");
      i += 1;
      continue;
    }

    const m = BULLET_RE.exec(line) ?? ORDERED_RE.exec(line);
    const indent = line.length - line.trimStart().length;

    if (m && indent <= baseIndent) {
      const isOrdered = !BULLET_RE.test(line);
      if (isOrdered !== ordered) break; // a different list starts here
      if (current) items.push(parseBlocks(current));
      current = [m[3]];
      i += 1;
      continue;
    }

    if (indent > baseIndent && current) {
      // Nested content: dedent by the marker width so it parses on its own.
      current.push(line.slice(Math.min(indent, baseIndent + 2)));
      i += 1;
      continue;
    }

    break;
  }

  if (current) items.push(parseBlocks(current));
  return { node: { type: "list", ordered, start: startNo, items }, next: i };
}

// ------------------------------------------------------------------ tables

function isTableStart(lines: string[], i: number): boolean {
  const header = lines[i];
  const divider = lines[i + 1];
  return (
    header !== undefined &&
    divider !== undefined &&
    header.includes("|") &&
    TABLE_DIVIDER_RE.test(divider) &&
    divider.includes("-")
  );
}

function parseTable(lines: string[], start: number): { node: BlockNode; next: number } {
  const header = splitRow(lines[start]);
  const align = splitRow(lines[start + 1]).map<TableAlign>((cell) => {
    const left = cell.startsWith(":");
    const right = cell.endsWith(":");
    if (left && right) return "center";
    if (right) return "right";
    return "left";
  });

  const rows: InlineNode[][][] = [];
  let i = start + 2;
  while (i < lines.length && lines[i].includes("|") && lines[i].trim() !== "") {
    rows.push(splitRow(lines[i]).map(parseInline));
    i += 1;
  }

  return {
    node: { type: "table", align, header: header.map(parseInline), rows },
    next: i,
  };
}

function splitRow(line: string): string[] {
  return line
    .trim()
    .replace(/^\|/, "")
    .replace(/\|$/, "")
    .split("|")
    .map((cell) => cell.trim());
}

// ------------------------------------------------------------------ inline

/** Parses inline markup. Unclosed markers stay literal text. */
export function parseInline(source: string): InlineNode[] {
  const out: InlineNode[] = [];
  let buf = "";
  let i = 0;

  const flush = () => {
    if (buf !== "") {
      out.push({ type: "text", value: buf });
      buf = "";
    }
  };

  while (i < source.length) {
    const rest = source.slice(i);
    const ch = source[i];

    if (ch === "\\" && i + 1 < source.length) {
      buf += source[i + 1];
      i += 2;
      continue;
    }

    if (ch === "\n") {
      flush();
      out.push({ type: "break" });
      i += 1;
      continue;
    }

    if (ch === "`") {
      const code = /^`([^`\n]+)`/.exec(rest);
      if (code) {
        flush();
        out.push({ type: "code", value: code[1] });
        i += code[0].length;
        continue;
      }
    }

    if (ch === "!" && rest.startsWith("![")) {
      const image = /^!\[([^\]\n]*)\]\(\s*([^)\s]*)(?:\s+"[^"]*")?\s*\)/.exec(rest);
      if (image) {
        const src = safeHref(image[2]);
        flush();
        // An unusable source degrades to the alt text — a broken image icon
        // says less than the words the author wrote for it.
        if (src) out.push({ type: "image", src, alt: image[1] });
        else if (image[1]) out.push({ type: "text", value: image[1] });
        i += image[0].length;
        continue;
      }
    }

    if (ch === "[") {
      const link = /^\[([^\]\n]*)\]\(\s*([^)\s]*)(?:\s+"[^"]*")?\s*\)/.exec(rest);
      if (link) {
        const href = safeHref(link[2]);
        flush();
        const label = parseInline(link[1] || link[2]);
        // An unsafe scheme degrades to plain text rather than a dead link.
        if (href) out.push({ type: "link", href, children: label });
        else out.push(...label);
        i += link[0].length;
        continue;
      }
    }

    if (rest.startsWith("~~")) {
      const del = /^~~([\s\S]+?)~~/.exec(rest);
      if (del) {
        flush();
        out.push({ type: "del", children: parseInline(del[1]) });
        i += del[0].length;
        continue;
      }
    }

    if (ch === "*" || ch === "_") {
      const double = new RegExp(`^\\${ch}\\${ch}([\\s\\S]+?)\\${ch}\\${ch}`).exec(rest);
      if (double) {
        flush();
        out.push({ type: "strong", children: parseInline(double[1]) });
        i += double[0].length;
        continue;
      }
      const single = new RegExp(`^\\${ch}([^\\${ch}\\n]+?)\\${ch}`).exec(rest);
      if (single) {
        flush();
        out.push({ type: "em", children: parseInline(single[1]) });
        i += single[0].length;
        continue;
      }
    }

    buf += ch;
    i += 1;
  }

  flush();
  return out;
}

/**
 * Keeps only schemes a report body has a reason to link to. Anything else
 * (javascript:, data:, vbscript:, …) returns null and is rendered as text.
 */
function safeHref(raw: string): string | null {
  const href = raw.trim();
  if (href === "") return null;
  if (href.startsWith("/") || href.startsWith("#")) return href;
  if (/^https?:\/\//i.test(href) || /^mailto:/i.test(href)) return href;
  return null;
}

// ---------------------------------------------------------------- excerpts

/**
 * Markdown → plain text, for feed card previews. Syntax is removed rather than
 * escaped, so a card never shows `## 투자 논거` with its hashes.
 */
export function markdownToPlainText(source: string): string {
  return stripComments(source ?? "")
    .replace(/\r\n?/g, "\n")
    .replace(/```[\s\S]*?(?:```|$)/g, " ")
    .replace(/^\s{0,3}(?:-{3,}|\*{3,}|_{3,})\s*$/gm, " ")
    .replace(/^\s{0,3}#{1,6}\s+/gm, "")
    .replace(/^\s{0,3}>\s?/gm, "")
    .replace(/^\s*(?:[-*+]|\d{1,9}[.)])\s+/gm, "")
    .replace(/^\s*\|.*\|\s*$/gm, " ")
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
    .replace(/(\*\*|__|~~)(.*?)\1/g, "$2")
    .replace(/[*_`]/g, "")
    .replace(/\s+/g, " ")
    .trim();
}

/** First `limit` characters of the body, ellipsised on a real cut only. */
export function markdownExcerpt(source: string, limit = 140): string {
  const text = plainMentions(markdownToPlainText(source));
  if (text.length <= limit) return text;
  return `${text.slice(0, limit).trimEnd()}…`;
}

/** Rough reading time in minutes, floored at 1. Korean prose ≈ 500자/분. */
export function readingMinutes(source: string): number {
  const chars = markdownToPlainText(source).length;
  return Math.max(1, Math.round(chars / 500));
}

/**
 * First image URL in the body, for offering it as the report's cover.
 *
 * Runs the same `safeHref` gate the renderer uses — the returned string goes
 * straight into an `<img src>` and then into a public list card, so a
 * `javascript:`/`data:` source must not survive the trip.
 */
export function firstMarkdownImage(source: string): string | null {
  const match = /!\[[^\]\n]*\]\(\s*([^)\s]+)(?:\s+"[^"]*")?\s*\)/.exec(
    stripComments(source ?? ""),
  );
  return match ? safeHref(match[1]) : null;
}
