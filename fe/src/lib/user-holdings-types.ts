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
};

export type ImportCommitRow = {
  id: string;
  symbol: string;
  quantity: number;
  avgPrice?: number;
  costCurrency?: string;
  selected: boolean;
};
