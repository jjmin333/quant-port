"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { ScreenshotCropCanvas } from "@/components/holdings/screenshot-crop-canvas";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { SectionCard } from "@/components/ui/section-card";
import {
  commitImportJobClient,
  pollImportJob,
  uploadHoldingsScreenshotsClient,
} from "@/lib/holdings-import-api";
import {
  filterAllowedImportImages,
  formatRejectedImportNames,
  isAllowedImportImage,
} from "@/lib/import-image-format";
import type { ImportImageMeta, ImportPreviewRow } from "@/lib/user-holdings-types";
import { cn } from "@/lib/utils";
import { toUserMessage } from "@/lib/error-message";
import { buttonVariants } from "@/components/ui/button";

const PREVIEW_GRID =
  "grid grid-cols-[minmax(7rem,1.2fr)_minmax(5rem,0.75fr)_minmax(4rem,0.45fr)_minmax(5.5rem,0.75fr)_5.5rem] gap-2";

const NO_SPINNER =
  "[appearance:textfield] [&::-webkit-outer-spin-button]:appearance-none [&::-webkit-inner-spin-button]:appearance-none";

const MAX_IMAGES = 10;

type WizardStep = "upload" | "processing" | "preview" | "done";

type ScreenshotImportWizardProps = {
  onCommitted: () => void;
  /**
   * 접힌 상태로 시작한다. 보유 등록은 가끔 하는 일인데 이 카드가 늘 펼쳐져 있으면
   * 매일 보는 목록을 매번 아래로 밀어낸다(관심종목 화면에서 같은 이유로 "종목 추가"를
   * 레일로 옮긴 적이 있다). 작업이 시작되면 자동으로 펼쳐진다 — 진행 중인 일을
   * 접어두면 사용자가 뭐가 돌고 있는지 알 수 없다.
   */
  collapsible?: boolean;
  /**
   * `rail` — 우측 레일에 종목 직접 추가와 같은 크기의 진입 카드를 두고, 본문은 Dialog로
   * 연다. 본문 하단에 두면 스크롤 끝까지 내려야 찾을 수 있어서 발견성이 떨어진다.
   */
  placement?: "inline" | "rail";
};

function rowCostCurrency(row: ImportPreviewRow): "KRW" | "USD" {
  if (row.costCurrency === "USD" || row.costCurrency === "KRW") {
    return row.costCurrency;
  }
  const sym = row.symbol ?? "";
  if (/^\d{6}$/.test(sym)) {
    return "KRW";
  }
  return "USD";
}

function normalizeAvgPrice(
  value: number | undefined,
  currency: "KRW" | "USD",
): number | undefined {
  if (value == null || value <= 0 || !Number.isFinite(value)) {
    return undefined;
  }
  if (currency === "USD") {
    return Math.round(value * 100) / 100;
  }
  return Math.round(value);
}

function normalizePreviewRow(row: ImportPreviewRow): ImportPreviewRow {
  const currency = rowCostCurrency(row);
  return {
    ...row,
    avgPrice: normalizeAvgPrice(row.avgPrice, currency),
  };
}

function newManualRow(id: string): ImportPreviewRow {
  return {
    id,
    rawName: "",
    quantity: 0,
    confidence: 0,
    selected: true,
    needsReview: true,
    sourceImageIdx: -1,
  };
}

export function ScreenshotImportWizard({
  onCommitted,
  collapsible = false,
  placement = "inline",
}: ScreenshotImportWizardProps) {
  const [expanded, setExpanded] = useState(
    placement === "rail" ? false : !collapsible,
  );
  const [step, setStep] = useState<WizardStep>("upload");
  const [files, setFiles] = useState<File[]>([]);
  const [activeIdx, setActiveIdx] = useState(0);
  const [meta, setMeta] = useState<ImportImageMeta[]>([]);
  const [jobId, setJobId] = useState<string | null>(null);
  const [progress, setProgress] = useState({ done: 0, total: 0 });
  const [rows, setRows] = useState<ImportPreviewRow[]>([]);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [resultWarning, setResultWarning] = useState<string | null>(null);
  const [resultError, setResultError] = useState<string | null>(null);
  const [addToWatchlist, setAddToWatchlist] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [symbolSearch, setSymbolSearch] = useState<Record<string, string>>({});
  const [editingRows, setEditingRows] = useState<Set<string>>(() => new Set());

  const activeFile = files[activeIdx];
  const [dragOver, setDragOver] = useState(false);
  const dragDepthRef = useRef(0);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const uploadZoneRef = useRef<HTMLDivElement>(null);

  const addFiles = useCallback((incoming: File[]) => {
    if (incoming.length === 0) return;
    const { allowed, rejected } = filterAllowedImportImages(incoming);
    if (rejected.length > 0) {
      setUploadError(
        `사진 파일(jpg, png)만 올릴 수 있습니다. 제외: ${formatRejectedImportNames(rejected)}`,
      );
    } else {
      setUploadError(null);
    }
    if (allowed.length === 0) return;

    setFiles((prev) => {
      const merged = [...prev, ...allowed].slice(0, MAX_IMAGES);
      return merged;
    });
    setMeta((prev) => {
      const merged = [...prev, ...allowed.map(() => ({}))].slice(0, MAX_IMAGES);
      return merged;
    });
    setActiveIdx((prev) => (prev === 0 && files.length === 0 ? 0 : prev));
    setResultWarning(null);
    setResultError(null);
  }, [files.length]);

  const removeFile = useCallback((idx: number) => {
    setFiles((prev) => {
      const next = prev.filter((_, i) => i !== idx);
      setActiveIdx((active) => {
        if (next.length === 0) return 0;
        if (active > idx) return active - 1;
        if (active >= next.length) return next.length - 1;
        return active;
      });
      return next;
    });
    setMeta((prev) => prev.filter((_, i) => i !== idx));
    setUploadError(null);
  }, []);

  const onFilesSelected = useCallback(
    (list: FileList | null) => {
      if (!list) return;
      addFiles(Array.from(list));
    },
    [addFiles],
  );

  const onPaste = useCallback(
    (event: ClipboardEvent) => {
      if (step !== "upload") return;
      const target = event.target;
      if (
        target instanceof HTMLElement &&
        target.closest("input, textarea, [contenteditable=true]")
      ) {
        return;
      }

      const items = event.clipboardData?.items;
      if (!items) return;

      const pasted: File[] = [];
      for (const item of items) {
        if (item.kind !== "file") continue;
        const file = item.getAsFile();
        if (file && isAllowedImportImage(file)) {
          pasted.push(file);
        }
      }
      if (pasted.length === 0) return;

      event.preventDefault();
      addFiles(pasted);
    },
    [addFiles, step],
  );

  useEffect(() => {
    if (step !== "upload" || !expanded) return;
    document.addEventListener("paste", onPaste);
    return () => document.removeEventListener("paste", onPaste);
  }, [onPaste, step, expanded]);

  const onDragEnter = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    dragDepthRef.current += 1;
    setDragOver(true);
  }, []);

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = "copy";
    setDragOver(true);
  }, []);

  const onDragLeave = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.stopPropagation();
    dragDepthRef.current = Math.max(0, dragDepthRef.current - 1);
    if (dragDepthRef.current === 0) {
      setDragOver(false);
    }
  }, []);

  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();
      event.stopPropagation();
      dragDepthRef.current = 0;
      setDragOver(false);
      if (step !== "upload") return;
      const dropped = Array.from(event.dataTransfer.files ?? []);
      if (dropped.length > 0) addFiles(dropped);
    },
    [addFiles, step],
  );

  const startImport = useCallback(async () => {
    if (files.length === 0) {
      setUploadError("jpg 또는 png 사진을 1장 이상 선택해 주세요.");
      return;
    }
    const invalid = files.filter((f) => !isAllowedImportImage(f));
    if (invalid.length > 0) {
      setUploadError(
        `사진 파일(jpg, png)만 올릴 수 있습니다. 제외: ${formatRejectedImportNames(invalid)}`,
      );
      return;
    }
    setUploading(true);
    setUploadError(null);
    setResultWarning(null);
    setResultError(null);
    try {
      const { jobId: id } = await uploadHoldingsScreenshotsClient(files, meta);
      setJobId(id);
      setStep("processing");
    } catch (err) {
      setUploadError(toUserMessage(err, "업로드 실패"));
    } finally {
      setUploading(false);
    }
  }, [files, meta]);

  useEffect(() => {
    if (step !== "processing" || !jobId) return;
    const stop = pollImportJob(
      jobId,
      (snap) => {
        setProgress(snap.progress);
        if (snap.status === "failed") {
          setRows([]);
          setResultWarning(null);
          setResultError(snap.error ?? "보유 종목을 불러오지 못했습니다.");
          setUploadError(snap.error ?? null);
          setStep("upload");
          return;
        }
        if (snap.status === "preview") {
          const preview = (snap.preview ?? []).map(normalizePreviewRow);
          setRows(preview);
          setEditingRows(
            new Set(preview.filter((r) => r.needsReview || !r.symbol).map((r) => r.id)),
          );
          setResultWarning(snap.warning ?? null);
          setResultError(snap.error ?? null);
          setStep("preview");
        }
      },
      (err) => setUploadError(err.message),
    );
    return stop;
  }, [step, jobId]);

  const saveCount = useMemo(
    () => rows.filter((r) => r.symbol && r.quantity > 0).length,
    [rows],
  );

  const canManualSave = saveCount > 0;

  const commit = useCallback(async () => {
    if (!jobId) return;
    setCommitting(true);
    setUploadError(null);
    try {
      await commitImportJobClient(jobId, {
        rows: rows.map((r) => ({
          id: r.id,
          symbol: r.symbol || "",
          quantity: r.quantity,
          avgPrice: r.avgPrice,
          costCurrency: r.costCurrency,
          selected: Boolean(r.symbol) && r.quantity > 0,
        })),
        addToWatchlist,
        replaceExisting: true,
      });
      setStep("done");
      onCommitted();
    } catch (err) {
      setUploadError(toUserMessage(err, "저장 실패"));
    } finally {
      setCommitting(false);
    }
  }, [addToWatchlist, jobId, onCommitted, rows]);

  const updateRow = (id: string, patch: Partial<ImportPreviewRow>) => {
    setRows((prev) =>
      prev.map((r) => {
        if (r.id !== id) return r;
        return normalizePreviewRow({ ...r, ...patch });
      }),
    );
  };

  const addManualRow = () => {
    const id = `manual-${Date.now()}`;
    setRows((prev) => [...prev, newManualRow(id)]);
    setEditingRows((prev) => new Set(prev).add(id));
  };

  const removeRow = (id: string) => {
    setRows((prev) => prev.filter((r) => r.id !== id));
    setEditingRows((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };

  const startEditRow = (id: string) => {
    setEditingRows((prev) => new Set(prev).add(id));
  };

  const finishEditRow = (id: string) => {
    setEditingRows((prev) => {
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  };

  const resetWizard = () => {
    setStep("upload");
    setFiles([]);
    setMeta([]);
    setActiveIdx(0);
    setJobId(null);
    setRows([]);
    setEditingRows(new Set());
    setUploadError(null);
    setResultWarning(null);
    setResultError(null);
  };

  const closeWizard = () => {
    resetWizard();
    setExpanded(false);
  };

  // 진행 중인 작업은 접힌 채로 두지 않는다 — 무엇이 돌고 있는지 안 보이면
  // 사용자는 멈춘 것으로 읽는다.
  const open = expanded || step !== "upload";
  const isRail = placement === "rail";

  const handleDialogOpenChange = (next: boolean) => {
    if (next) {
      setExpanded(true);
      return;
    }
    if (step === "processing") return;
    setExpanded(false);
    if (step !== "upload") resetWizard();
  };

  const wizardHeader = isRail ? (
    <DialogHeader className="text-left">
      <DialogTitle className="text-base">스크린샷으로 가져오기</DialogTitle>
      <DialogDescription className="text-cell">
        증권앱 보유종목 화면을 최대 {MAX_IMAGES}장까지 올리면 종목과 수량을 불러옵니다. 저장 전에 꼭 확인해
        주세요.
      </DialogDescription>
    </DialogHeader>
  ) : (
    <div className="flex items-start justify-between gap-3">
      <div className="min-w-0">
        <h2 className="text-lg font-semibold text-[var(--text-primary)]">스크린샷으로 가져오기</h2>
        <p className="text-sm text-[var(--text-muted)]">
          증권앱 보유종목 화면을 최대 {MAX_IMAGES}장까지 올리면 종목과 수량을 불러옵니다. 저장 전에 꼭 확인해
          주세요.
        </p>
      </div>
      {collapsible && step === "upload" && (
        <button
          type="button"
          onClick={() => setExpanded(false)}
          className={cn(buttonVariants({ variant: "outline", size: "xs" }), "hover:bg-accent")}
        >
          접기
        </button>
      )}
    </div>
  );

  const wizardBody = (
    <>
      {wizardHeader}

      {uploadError && (
        <div className="rounded-lg border border-[var(--loss)]/40 bg-[var(--loss)]/10 px-3 py-2 text-sm text-[var(--text-primary)]">
          {uploadError}
        </div>
      )}

      {step === "upload" && (
        <div className="space-y-4">
          <div
            ref={uploadZoneRef}
            tabIndex={0}
            role="region"
            aria-label="스크린샷 업로드"
            className={cn(
              "flex cursor-default flex-col items-center justify-center rounded-xl border border-dashed px-4 py-8 text-center outline-none transition-colors focus-visible:ring-2 focus-visible:ring-[var(--brand)]",
              dragOver
                ? "border-[var(--brand)] bg-[var(--brand-muted)]/30"
                : "border-[var(--border)] bg-[var(--surface-elevated)]",
            )}
            onClick={() => uploadZoneRef.current?.focus()}
            onKeyDown={(event) => {
              if (event.key === "Enter" || event.key === " ") {
                event.preventDefault();
                fileInputRef.current?.click();
              }
            }}
            onDragEnter={onDragEnter}
            onDragOver={onDragOver}
            onDragLeave={onDragLeave}
            onDrop={onDrop}
          >
            <span className="text-sm font-medium text-[var(--text-primary)]">
              보유 화면 스크린샷을 붙여넣거나 끌어다 놓아 주세요
            </span>
            <span className="mt-1 text-cell text-[var(--text-muted)]">
              Ctrl+V · jpg, png · 최대 {MAX_IMAGES}장
            </span>
            <button
              type="button"
              className={cn(buttonVariants({ variant: "outline", size: "sm" }), "mt-4 border-[var(--border)] bg-[var(--surface)] text-[var(--text-primary)] hover:border-[var(--brand)] hover:bg-[var(--brand-muted)]/20")}
              onClick={(event) => {
                event.stopPropagation();
                fileInputRef.current?.click();
              }}
            >
              사진 선택
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/jpeg,image/png,.jpg,.jpeg,.png"
              multiple
              className="hidden"
              onChange={(e) => onFilesSelected(e.target.files)}
            />
          </div>

          {files.length > 0 && (
            <>
              <div className="flex flex-wrap gap-2">
                {files.map((f, idx) => (
                  <div
                    key={`${f.name}-${idx}`}
                    className={cn(
                      "flex items-center gap-1 rounded-md border px-2 py-1 text-cell",
                      idx === activeIdx
                        ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--text-primary)]"
                        : "border-[var(--border)] text-[var(--text-secondary)]",
                    )}
                  >
                    <button type="button" className="max-w-[8rem] truncate" onClick={() => setActiveIdx(idx)}>
                      {idx + 1}. {f.name.slice(0, 18)}
                    </button>
                    <button
                      type="button"
                      className="text-[var(--text-muted)] hover:text-[var(--loss)]"
                      aria-label={`${idx + 1}번 이미지 제거`}
                      onClick={() => removeFile(idx)}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
              {activeFile && (
                <ScreenshotCropCanvas
                  file={activeFile}
                  crop={meta[activeIdx]?.crop}
                  onCropChange={(crop) => {
                    setMeta((prev) => {
                      const next = [...prev];
                      next[activeIdx] = { ...next[activeIdx], crop };
                      return next;
                    });
                  }}
                />
              )}
            </>
          )}

          <div className="flex justify-end">
            <button
              type="button"
              disabled={uploading || files.length === 0}
              className={cn(buttonVariants({ variant: "default", size: "default" }), "bg-[var(--brand)] text-[var(--brand-foreground)] disabled:opacity-50")}
              onClick={() => void startImport()}
            >
              {uploading ? "업로드 중…" : "불러오기"}
            </button>
          </div>
        </div>
      )}

      {step === "processing" && (
        <div className="py-10 text-center">
          <p className="text-base font-medium text-[var(--text-primary)]">보유 종목을 읽는 중…</p>
          <p className="mt-2 text-sm text-[var(--text-muted)]">
            {progress.total > 0 ? `${progress.done} / ${progress.total} 장 완료` : "준비 중"}
          </p>
          <div className="mx-auto mt-4 h-1.5 w-48 overflow-hidden rounded-full bg-[var(--surface-elevated)]">
            <div
              className="h-full bg-[var(--brand)] transition-all"
              style={{
                width:
                  progress.total > 0 ? `${Math.round((progress.done / progress.total) * 100)}%` : "20%",
              }}
            />
          </div>
        </div>
      )}

      {step === "preview" && (
        <div className="space-y-3">
          {resultWarning && (
            <div className="rounded-lg border border-amber-500/40 bg-amber-500/10 px-3 py-2 text-sm text-[var(--text-primary)]">
              {resultWarning}
            </div>
          )}
          {resultError && (
            <div className="rounded-lg border border-[var(--loss)]/40 bg-[var(--loss)]/10 px-3 py-2 text-sm text-[var(--text-primary)]">
              {resultError}
            </div>
          )}

          <p className="text-sm text-[var(--text-secondary)]">
            저장 전 종목·수량·평단가를 확인하고 수정하세요. 불러오지 못한 경우 아래에서 직접 추가할 수 있습니다.
          </p>

          <div className="overflow-x-auto rounded-lg border border-[var(--border)]">
            <div className="min-w-[36rem] p-2">
              <div className={cn(PREVIEW_GRID, "px-1 pb-2 text-cell font-medium text-[var(--text-muted)]")}>
                <span>종목명</span>
                <span>종목코드</span>
                <span>수량</span>
                <span>평단가</span>
                <span />
              </div>

              {rows.length === 0 ? (
                <p className="px-2 py-6 text-center text-sm text-[var(--text-muted)]">
                  추출된 종목이 없습니다. 직접 추가하거나 스크린샷을 다시 업로드해 주세요.
                </p>
              ) : (
                rows.map((row) => {
                  const editing = editingRows.has(row.id);
                  const displayName = row.rawName || row.name || "—";

                  return (
                    <div
                      key={row.id}
                      className={cn(
                        PREVIEW_GRID,
                        "items-start border-t border-[var(--border)] px-1 py-2 first:border-t-0",
                      )}
                    >
                      <div className="min-w-0">
                        <p className="truncate py-1.5 text-sm font-medium text-[var(--text-primary)]">
                          {displayName}
                        </p>
                        {row.needsReview && (
                          <p className="text-cell text-[var(--brand)]">확인 필요</p>
                        )}
                      </div>

                      <div className="min-w-0">
                        {editing ? (
                          <input
                            className="w-full min-w-0 rounded border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 font-num text-cell"
                            value={row.symbol ?? symbolSearch[row.id] ?? ""}
                            placeholder="검색/코드"
                            onChange={(e) => {
                              const v = e.target.value;
                              setSymbolSearch((prev) => ({ ...prev, [row.id]: v }));
                              updateRow(row.id, { symbol: v.toUpperCase(), needsReview: true });
                            }}
                          />
                        ) : (
                          <p className="truncate py-1.5 font-num text-sm text-[var(--text-primary)]">
                            {row.symbol || "—"}
                          </p>
                        )}
                        {row.name && row.name !== displayName && (
                          <p className="mt-0.5 truncate text-cell text-[var(--text-muted)]">{row.name}</p>
                        )}
                      </div>

                      <div>
                        {editing ? (
                          <div className="flex items-center gap-1">
                            <input
                              type="number"
                              min={0}
                              step={1}
                              className={cn(
                                "w-full min-w-0 rounded border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 font-num text-cell",
                                NO_SPINNER,
                              )}
                              value={row.quantity || ""}
                              placeholder="0"
                              onChange={(e) =>
                                updateRow(row.id, {
                                  quantity: Number.parseFloat(e.target.value) || 0,
                                })
                              }
                            />
                            <span className="shrink-0 text-cell text-[var(--text-muted)]">주</span>
                          </div>
                        ) : (
                          <p className="py-1.5 font-num text-sm text-[var(--text-primary)]">
                            {row.quantity > 0 ? `${row.quantity}주` : "—"}
                          </p>
                        )}
                      </div>

                      <div className="min-w-0">
                        <input
                          type="number"
                          min={0}
                          step={1}
                          inputMode="numeric"
                          className={cn(
                            "w-full min-w-0 rounded border border-[var(--border)] bg-[var(--surface)] px-2 py-1.5 font-num text-cell",
                            NO_SPINNER,
                          )}
                          value={row.avgPrice ?? ""}
                          placeholder="—"
                          onChange={(e) => {
                            const raw = e.target.value.trim();
                            updateRow(row.id, {
                              avgPrice:
                                raw === "" ? undefined : Number.parseFloat(raw) || undefined,
                            });
                          }}
                        />
                      </div>

                      <div className="flex gap-1 pt-0.5">
                        <button
                          type="button"
                          className={cn(
                            "flex h-9 w-9 shrink-0 items-center justify-center rounded-md border text-cell font-medium leading-none transition-colors",
                            editing
                              ? "border-[var(--brand)] bg-[var(--brand-muted)] text-[var(--brand)]"
                              : "border-[var(--border)] bg-[var(--surface)] text-[var(--text-secondary)] hover:border-[var(--brand)] hover:text-[var(--brand)]",
                          )}
                          onClick={() =>
                            editing ? void finishEditRow(row.id) : startEditRow(row.id)
                          }
                        >
                          {editing ? "완료" : "수정"}
                        </button>
                        <button
                          type="button"
                          className="flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-[var(--border)] bg-[var(--surface)] text-cell font-medium leading-none text-[var(--text-secondary)] transition-colors hover:border-[var(--loss)] hover:text-[var(--loss)]"
                          onClick={() => removeRow(row.id)}
                        >
                          삭제
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>

          <button
            type="button"
            className={cn(buttonVariants({ variant: "outline", size: "default" }), "border-dashed border-[var(--border)] text-[var(--text-secondary)] hover:border-[var(--brand)]")}
            onClick={addManualRow}
          >
            + 종목 수동 추가
          </button>

          <label className="flex items-center gap-2 text-sm text-[var(--text-secondary)]">
            <input
              type="checkbox"
              checked={addToWatchlist}
              onChange={(e) => setAddToWatchlist(e.target.checked)}
            />
            관심종목에도 추가
          </label>

          <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
            <button
              type="button"
              className="w-full rounded-lg border border-[var(--border)] px-4 py-2.5 text-sm sm:w-auto"
              onClick={resetWizard}
            >
              다시 업로드
            </button>
            <button
              type="button"
              disabled={committing || !canManualSave}
              className="w-full rounded-lg bg-[var(--brand)] px-4 py-2.5 text-sm font-medium text-[var(--brand-foreground)] disabled:opacity-50 sm:w-auto"
              onClick={() => void commit()}
            >
              {committing ? "저장 중…" : `${saveCount}개 저장`}
            </button>
          </div>
        </div>
      )}

      {step === "done" && (
        <div className="py-8 text-center">
          <p className="text-lg font-semibold text-[var(--text-primary)]">보유종목 저장 완료</p>
          <p className="mt-2 text-sm text-[var(--text-muted)]">시세는 아래 목록에서 확인할 수 있습니다.</p>
          <button
            type="button"
            className={cn(buttonVariants({ variant: "default", size: "default" }), "mt-4 bg-[var(--brand)] text-[var(--brand-foreground)]")}
            onClick={closeWizard}
          >
            확인
          </button>
        </div>
      )}
    </>
  );

  if (isRail) {
    return (
      <>
        <SectionCard title="스크린샷으로 가져오기">
          <p className="text-micro leading-relaxed text-muted-foreground">
            증권앱 보유종목 화면을 올리면 종목과 수량을 한 번에 불러옵니다
          </p>
          <Button
            size="sm"
            className="mt-2 w-full"
            disabled={open}
            onClick={() => setExpanded(true)}
          >
            {open ? "가져오기 진행 중…" : "스크린샷 가져오기"}
          </Button>
        </SectionCard>
        <Dialog open={open} onOpenChange={handleDialogOpenChange}>
          <DialogContent
            className="max-h-[90dvh] gap-4 overflow-y-auto p-4 sm:max-w-3xl sm:p-5"
            onInteractOutside={(e) => {
              if (step === "processing") e.preventDefault();
            }}
            onEscapeKeyDown={(e) => {
              if (step === "processing") e.preventDefault();
            }}
          >
            <div className="space-y-4">{wizardBody}</div>
          </DialogContent>
        </Dialog>
      </>
    );
  }

  if (!open) {
    return (
      <Card className="flex flex-wrap items-center justify-between gap-2 border-[var(--brand)]/30 px-3 py-2">
        <div className="min-w-0">
          <p className="text-cell font-semibold">스크린샷으로 가져오기</p>
          <p className="truncate text-micro text-muted-foreground">
            증권앱 보유종목 화면을 올리면 종목과 수량을 한 번에 불러옵니다
          </p>
        </div>
        <button
          type="button"
          onClick={() => setExpanded(true)}
          className={cn(buttonVariants({ variant: "outline", size: "xs" }), "hover:bg-accent")}
        >
          펼치기
        </button>
      </Card>
    );
  }

  return (
    <Card className="space-y-4 border-[var(--brand)]/30">
      {wizardBody}
    </Card>
  );
}
