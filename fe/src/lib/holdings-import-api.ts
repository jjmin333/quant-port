import { apiRequest } from "@/lib/api-client";
import type {
  ImportCommitRow,
  ImportImageMeta,
  ImportJobSnapshot,
  UserHolding,
} from "@/lib/user-holdings-types";

function apiGet<T>(path: string): Promise<T> {
  return apiRequest<T>(path);
}

function apiPost<T>(path: string, payload?: unknown): Promise<T> {
  return apiRequest<T>(path, {
    method: "POST",
    headers: payload ? { "Content-Type": "application/json" } : undefined,
    body: payload ? JSON.stringify(payload) : undefined,
  });
}

export async function fetchUserHoldingsClient(): Promise<UserHolding[]> {
  const data = await apiGet<UserHolding[] | null>("/api/v1/user/holdings");
  return data ?? [];
}

export async function deleteUserHoldingClient(symbol: string): Promise<void> {
  await apiRequest<unknown>(`/api/v1/user/holdings/${encodeURIComponent(symbol)}`, {
    method: "DELETE",
  });
}

export function uploadHoldingsScreenshotsClient(
  files: File[],
  meta: ImportImageMeta[],
): Promise<{ jobId: string }> {
  const form = new FormData();
  for (const file of files) {
    form.append("images", file);
  }
  form.append("meta", JSON.stringify(meta));

  // No Content-Type header: the browser sets the multipart boundary.
  return apiRequest<{ jobId: string }>(
    "/api/v1/import/holdings/screenshots",
    { method: "POST", body: form },
    "업로드 실패",
  );
}

export async function fetchImportJobClient(jobId: string): Promise<ImportJobSnapshot> {
  return apiGet<ImportJobSnapshot>(`/api/v1/import/holdings/jobs/${encodeURIComponent(jobId)}`);
}

export async function commitImportJobClient(
  jobId: string,
  payload: {
    rows: ImportCommitRow[];
    addToWatchlist: boolean;
    replaceExisting: boolean;
  },
): Promise<{ holdingsCount: number; watchlistAdded: number }> {
  return apiPost(`/api/v1/import/holdings/jobs/${encodeURIComponent(jobId)}/commit`, payload);
}

export function pollImportJob(
  jobId: string,
  onUpdate: (snap: ImportJobSnapshot) => void,
  onError: (err: Error) => void,
): () => void {
  let stopped = false;
  let timer: ReturnType<typeof setTimeout> | null = null;

  const tick = async () => {
    if (stopped) return;
    try {
      const snap = await fetchImportJobClient(jobId);
      onUpdate(snap);
      if (snap.status === "processing" || snap.status === "pending") {
        timer = setTimeout(tick, 800);
      }
    } catch (err) {
      onError(err instanceof Error ? err : new Error("import poll failed"));
    }
  };

  void tick();

  return () => {
    stopped = true;
    if (timer) clearTimeout(timer);
  };
}
