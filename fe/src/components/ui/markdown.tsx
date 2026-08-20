import { Fragment, type ReactNode } from "react";
import {
  parseMarkdown,
  type BlockNode,
  type InlineNode,
  type TableAlign,
} from "@/lib/markdown";
import { cn } from "@/lib/utils";

/**
 * Renders Markdown as React elements.
 *
 * Every node becomes a real element — there is no `dangerouslySetInnerHTML`
 * anywhere in this file, which is what keeps a public, user-written report body
 * from being an injection surface. See `lib/markdown.ts` for the parser.
 *
 * The prose styles live here rather than in a plugin: the project has no
 * typography plugin, and reports are read on phones as often as on desktops, so
 * the scale is set once and reused by the editor preview and the detail page.
 */
export function Markdown({ source, className }: { source: string; className?: string }) {
  const blocks = parseMarkdown(source);
  if (blocks.length === 0) return null;
  return (
    <div className={cn("markdown-body space-y-4 text-[15px] leading-[1.85]", className)}>
      {blocks.map((block, i) => (
        <Block key={i} node={block} />
      ))}
    </div>
  );
}

function Block({ node }: { node: BlockNode }): ReactNode {
  switch (node.type) {
    case "heading": {
      const content = <Inlines nodes={node.children} />;
      if (node.level === 1) {
        return (
          <h2 className="mt-8 border-b border-[var(--border)] pb-2 text-xl font-semibold leading-snug text-[var(--text-primary)] first:mt-0">
            {content}
          </h2>
        );
      }
      if (node.level === 2) {
        return (
          <h3 className="mt-7 text-lg font-semibold leading-snug text-[var(--text-primary)] first:mt-0">
            {content}
          </h3>
        );
      }
      if (node.level === 3) {
        return (
          <h4 className="mt-6 text-base font-semibold leading-snug text-[var(--text-primary)] first:mt-0">
            {content}
          </h4>
        );
      }
      return (
        <h5 className="mt-5 text-sm font-semibold uppercase tracking-wide text-[var(--text-secondary)] first:mt-0">
          {content}
        </h5>
      );
    }

    case "paragraph":
      return (
        <p className="text-[var(--text-secondary)]">
          <Inlines nodes={node.children} />
        </p>
      );

    case "list": {
      const items = node.items.map((item, i) => (
        <li key={i} className="pl-1 marker:text-[var(--text-muted)]">
          <div className="space-y-2">
            {item.map((child, j) => (
              <Block key={j} node={child} />
            ))}
          </div>
        </li>
      ));
      return node.ordered ? (
        <ol
          start={node.start}
          className="list-outside list-decimal space-y-1.5 pl-5 text-[var(--text-secondary)]"
        >
          {items}
        </ol>
      ) : (
        <ul className="list-outside list-disc space-y-1.5 pl-5 text-[var(--text-secondary)]">
          {items}
        </ul>
      );
    }

    case "quote":
      return (
        <blockquote className="space-y-2 border-l-2 border-[var(--brand)] bg-[var(--surface-elevated)] px-4 py-3 text-[var(--text-secondary)]">
          {node.children.map((child, i) => (
            <Block key={i} node={child} />
          ))}
        </blockquote>
      );

    case "code":
      return (
        <pre className="overflow-x-auto rounded-xl border border-[var(--border)] bg-[var(--surface-elevated)] p-3 text-xs leading-relaxed">
          <code className="font-mono text-[var(--text-secondary)]">{node.value}</code>
        </pre>
      );

    case "hr":
      return <hr className="border-[var(--border)]" />;

    case "table":
      return (
        // Wide tables scroll inside their own box — the page body must never
        // scroll sideways on a phone.
        <div className="-mx-1 overflow-x-auto px-1">
          <table className="w-full min-w-[20rem] border-collapse text-sm">
            <thead>
              <tr className="border-b border-[var(--border)]">
                {node.header.map((cell, i) => (
                  <th
                    key={i}
                    className={cn(
                      "px-2.5 py-2 font-medium text-[var(--text-primary)]",
                      alignClass(node.align[i]),
                    )}
                  >
                    <Inlines nodes={cell} />
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {node.rows.map((row, i) => (
                <tr key={i} className="border-b border-[var(--border)] last:border-0">
                  {row.map((cell, j) => (
                    <td
                      key={j}
                      className={cn(
                        "px-2.5 py-2 text-[var(--text-secondary)]",
                        alignClass(node.align[j]),
                      )}
                    >
                      <Inlines nodes={cell} />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      );
  }
}

function alignClass(align: TableAlign | undefined): string {
  if (align === "center") return "text-center";
  if (align === "right") return "text-right tabular-nums";
  return "text-left";
}

function Inlines({ nodes }: { nodes: InlineNode[] }) {
  return (
    <>
      {nodes.map((node, i) => (
        <Fragment key={i}>
          <InlineNodeView node={node} />
        </Fragment>
      ))}
    </>
  );
}

function InlineNodeView({ node }: { node: InlineNode }): ReactNode {
  switch (node.type) {
    case "text":
      return node.value;
    case "break":
      return <br />;
    case "strong":
      return (
        <strong className="font-semibold text-[var(--text-primary)]">
          <Inlines nodes={node.children} />
        </strong>
      );
    case "em":
      return (
        <em className="italic">
          <Inlines nodes={node.children} />
        </em>
      );
    case "del":
      return (
        <del className="text-[var(--text-muted)] line-through">
          <Inlines nodes={node.children} />
        </del>
      );
    case "code":
      return (
        <code className="rounded bg-[var(--surface-elevated)] px-1.5 py-0.5 font-mono text-[0.85em] text-[var(--text-primary)]">
          {node.value}
        </code>
      );
    case "image":
      return (
        // Plain <img>, not next/image: the source is a user-written Markdown
        // URL, so there is no build-time size to optimise against, and the
        // loader would need every future host allow-listed.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={node.src}
          alt={node.alt}
          loading="lazy"
          decoding="async"
          className="my-2 block h-auto max-w-full rounded-xl border border-[var(--border)]"
        />
      );
    case "link":
      return (
        <a
          href={node.href}
          target={node.href.startsWith("/") || node.href.startsWith("#") ? undefined : "_blank"}
          rel="noopener noreferrer"
          className="break-words text-[var(--brand)] underline decoration-[var(--brand)]/40 underline-offset-2 hover:decoration-[var(--brand)]"
        >
          <Inlines nodes={node.children} />
        </a>
      );
  }
}
