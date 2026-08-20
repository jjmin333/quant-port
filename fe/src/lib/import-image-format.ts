const ALLOWED_MIME = new Set(["image/jpeg", "image/png"]);
const ALLOWED_EXT = /\.(jpe?g|png)$/i;

export function isAllowedImportImage(file: File): boolean {
  const mime = file.type.trim().toLowerCase();
  if (mime && ALLOWED_MIME.has(mime)) return true;
  return ALLOWED_EXT.test(file.name);
}

export function filterAllowedImportImages(files: File[]): {
  allowed: File[];
  rejected: File[];
} {
  const allowed: File[] = [];
  const rejected: File[] = [];
  for (const file of files) {
    if (isAllowedImportImage(file)) {
      allowed.push(file);
    } else {
      rejected.push(file);
    }
  }
  return { allowed, rejected };
}

export function formatRejectedImportNames(files: File[]): string {
  return files.map((f) => f.name).join(", ");
}
