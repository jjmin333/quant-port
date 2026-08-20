import { apiRequest } from "@/lib/api-client";

/**
 * User uploads.
 *
 * Deliberately not `research-*`: the first caller is the 투자 리포트 editor, but
 * nothing here knows about reports. A journal attachment or a profile picture
 * calls the same function and gets the same kind of URL back — the server side
 * is one `media.Store` behind one endpoint (`be/internal/media`).
 */

export type MediaUpload = {
  key: string;
  /** What to embed. Relative to this origin unless a CDN base is configured. */
  url: string;
  contentType: string;
  size: number;
  /** 0 for formats whose header the server does not parse (webp). */
  width?: number;
  height?: number;
};

export const MEDIA_IMAGE_ACCEPT = "image/jpeg,image/png,image/gif,image/webp";

/** Types the server will accept — checked here only to fail fast, not to trust. */
const ACCEPTED = new Set(["image/jpeg", "image/png", "image/gif", "image/webp"]);

export function isUploadableImage(file: File): boolean {
  // A file dragged from a phone can arrive with an empty type; let the server
  // sniff those rather than refusing something it would have accepted.
  return file.type === "" || ACCEPTED.has(file.type);
}

export async function uploadImageClient(file: File): Promise<MediaUpload> {
  const form = new FormData();
  form.append("file", file, file.name || "image");
  // No Content-Type header on purpose — the browser must set the multipart
  // boundary itself, and adding one here silently breaks the upload.
  return apiRequest<MediaUpload>(
    "/api/v1/media/images",
    { method: "POST", body: form },
    "이미지 업로드에 실패했습니다.",
  );
}
