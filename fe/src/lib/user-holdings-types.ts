export type ImportCropRect = {
  x: number;
  y: number;
  w: number;
  h: number;
};

export type ImportImageMeta = {
  crop?: ImportCropRect;
  brokerHint?: string;
};

export type ImportPreviewRow = {
  id: string;
  rawName: string;
  symbol?: string;
  name?: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  confidence: number;
  selected: boolean;
  needsReview: boolean;
  brokerHint?: string;
  resolveError?: string;
  sourceImageIdx: number;
};

export type ImportJobStatus = "pending" | "processing" | "preview" | "failed" | "committed";

export type ImportImageResult = {
  index: number;
  success: boolean;
  error?: string;
};

export type ImportJobSnapshot = {
  jobId: string;
  status: ImportJobStatus;
  progress: { done: number; total: number };
  preview?: ImportPreviewRow[];
  imageResults?: ImportImageResult[];
  warning?: string;
  error?: string;
};

export type UserHolding = {
  id: number;
  symbol: string;
  name?: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  brokerHint?: string;
  source: string;
  updatedAt: string;
  /** 공통 11섹터 키. 모르면 없다 — 구성 도넛은 「미분류」로 그린다. */
  sector?: string;
  /** 서버가 주는 한국어 섹터 이름. */
  sectorLabel?: string;
  /** 살 때 본 축(최대 셋) — 보유 이유 점검이 추적한다. */
  thesisAxes?: string[];
};

export type ImportCommitRow = {
  id: string;
  symbol: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  selected: boolean;
};
