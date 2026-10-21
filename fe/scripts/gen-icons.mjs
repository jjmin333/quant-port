// `favicon.ico` · `apple-icon.png` · PWA 아이콘(public/icon-*.png)을 `src/app/icon.svg` 와 같은 도형으로 다시 굽는다.
//   node scripts/gen-icons.mjs src/app
// 의존성 없이 직접 래스터화한다(sharp는 Next의 선택 의존성이라 믿지 않는다).

import zlib from "node:zlib";
import fs from "node:fs";

// 브랜드 마크 기하 — src/app/icon.svg 와 같은 좌표계(32x32)를 쓴다.
// 여기 숫자를 고치면 icon.svg 도 같이 고쳐야 한다.
const PANEL_TOP = [0x1c, 0x25, 0x41];       // #1C2541 → 아래로
const PANEL_BOT = [0x0a, 0x0f, 0x1e];       // #0A0F1E 그라데이션
const WHITE = [255, 255, 255];
const ACCENT = [0x6e, 0x9b, 0xff];          // #6E9BFF
const RING = { cx: 15, cy: 15, r: 8, w: 3.4 };
const TAIL = { x1: 20, y1: 20, x2: 25.5, y2: 25.5, w: 3.4 }; // 둥근 끝 선분
const PANEL_R = 7.5;

function ringCovers(x, y) {
  return Math.abs(Math.hypot(x - RING.cx, y - RING.cy) - RING.r) <= RING.w / 2;
}

function tailCovers(x, y) {
  const dx = TAIL.x2 - TAIL.x1, dy = TAIL.y2 - TAIL.y1;
  const t = Math.min(1, Math.max(0, ((x - TAIL.x1) * dx + (y - TAIL.y1) * dy) / (dx * dx + dy * dy)));
  return Math.hypot(x - (TAIL.x1 + t * dx), y - (TAIL.y1 + t * dy)) <= TAIL.w / 2;
}

function roundRectCovers(x, y, rx0, ry0, w, h, r) {
  if (x < rx0 || y < ry0 || x > rx0 + w || y > ry0 + h) return false;
  const cx = Math.min(Math.max(x, rx0 + r), rx0 + w - r);
  const cy = Math.min(Math.max(y, ry0 + r), ry0 + h - r);
  return (x - cx) ** 2 + (y - cy) ** 2 <= r * r;
}

/**
 * size x size RGBA. rounded=false 면 모서리를 깎지 않는다(애플 아이콘은 iOS가 깎는다).
 * inset < 1 이면 도형을 가운데로 줄인다 — maskable 아이콘은 런처가 원·물방울로
 * 잘라도 남는 안전 영역(지름 80%) 안에 도형이 있어야 한다.
 */
function raster(size, rounded, inset = 1) {
  const S = 4; // 4x 슈퍼샘플링
  const out = Buffer.alloc(size * size * 4);
  const k = 32 / size;
  for (let py = 0; py < size; py++) {
    for (let px = 0; px < size; px++) {
      let panel = 0;
      const rgb = [0, 0, 0];
      for (let sy = 0; sy < S; sy++) {
        for (let sx = 0; sx < S; sx++) {
          const x = (px + (sx + 0.5) / S) * k;
          const y = (py + (sy + 0.5) / S) * k;
          if (rounded && !roundRectCovers(x, y, 0, 0, 32, 32, PANEL_R)) continue;
          panel++;
          const ix = 16 + (x - 16) / inset, iy = 16 + (y - 16) / inset;
          // 꼬리가 고리 위에 온다(icon.svg 의 그리는 순서).
          const t = y / 32;
          const color = tailCovers(ix, iy) ? ACCENT
            : ringCovers(ix, iy) ? WHITE
            : PANEL_TOP.map((c, j) => c + (PANEL_BOT[j] - c) * t);
          for (let c = 0; c < 3; c++) rgb[c] += color[c];
        }
      }
      const i = (py * size + px) * 4;
      for (let c = 0; c < 3; c++) out[i + c] = panel ? Math.round(rgb[c] / panel) : 0;
      out[i + 3] = Math.round((panel / (S * S)) * 255);
    }
  }
  return out;
}

// ---- PNG ----------------------------------------------------------------
function crc32(buf) {
  let c, t = [];
  for (let n = 0; n < 256; n++) {
    c = n;
    for (let j = 0; j < 8; j++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
    t[n] = c >>> 0;
  }
  let crc = 0xffffffff;
  for (const b of buf) crc = t[(crc ^ b) & 0xff] ^ (crc >>> 8);
  return (crc ^ 0xffffffff) >>> 0;
}
function chunk(type, data) {
  const len = Buffer.alloc(4); len.writeUInt32BE(data.length);
  const td = Buffer.concat([Buffer.from(type, "ascii"), data]);
  const crc = Buffer.alloc(4); crc.writeUInt32BE(crc32(td));
  return Buffer.concat([len, td, crc]);
}
function png(size, rgba) {
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(size, 0); ihdr.writeUInt32BE(size, 4);
  ihdr[8] = 8; ihdr[9] = 6; // 8bit RGBA
  const raw = Buffer.alloc(size * (size * 4 + 1));
  for (let y = 0; y < size; y++) {
    raw[y * (size * 4 + 1)] = 0; // filter: none
    rgba.copy(raw, y * (size * 4 + 1) + 1, y * size * 4, (y + 1) * size * 4);
  }
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk("IHDR", ihdr),
    chunk("IDAT", zlib.deflateSync(raw, { level: 9 })),
    chunk("IEND", Buffer.alloc(0)),
  ]);
}

// ---- ICO (BMP/DIB 엔트리 — 구형 파서까지 안전) --------------------------
function icoEntry(size, rgba) {
  const rowMask = Math.ceil(size / 32) * 4; // AND 마스크는 4바이트 정렬
  const hdr = Buffer.alloc(40);
  hdr.writeUInt32LE(40, 0);
  hdr.writeInt32LE(size, 4);
  hdr.writeInt32LE(size * 2, 8); // XOR + AND 를 합친 높이
  hdr.writeUInt16LE(1, 12); hdr.writeUInt16LE(32, 14);
  const xor = Buffer.alloc(size * size * 4);
  const and = Buffer.alloc(rowMask * size);
  for (let y = 0; y < size; y++) {
    const src = size - 1 - y; // BMP 는 아래에서 위로
    for (let x = 0; x < size; x++) {
      const s = (src * size + x) * 4, d = (y * size + x) * 4;
      xor[d] = rgba[s + 2]; xor[d + 1] = rgba[s + 1]; xor[d + 2] = rgba[s]; xor[d + 3] = rgba[s + 3];
      if (rgba[s + 3] < 128) and[y * rowMask + (x >> 3)] |= 0x80 >> (x & 7);
    }
  }
  return Buffer.concat([hdr, xor, and]);
}
function ico(images) {
  const head = Buffer.alloc(6 + 16 * images.length);
  head.writeUInt16LE(0, 0); head.writeUInt16LE(1, 2); head.writeUInt16LE(images.length, 4);
  let offset = head.length;
  const bodies = [];
  images.forEach(({ size, data }, i) => {
    const body = icoEntry(size, data);
    const p = 6 + 16 * i;
    head[p] = size === 256 ? 0 : size; head[p + 1] = size === 256 ? 0 : size;
    head.writeUInt16LE(1, p + 4); head.writeUInt16LE(32, p + 6);
    head.writeUInt32LE(body.length, p + 8); head.writeUInt32LE(offset, p + 12);
    offset += body.length; bodies.push(body);
  });
  return Buffer.concat([head, ...bodies]);
}

const app = process.argv[2];
fs.writeFileSync(`${app}/favicon.ico`, ico([
  { size: 16, data: raster(16, true) },
  { size: 32, data: raster(32, true) },
  { size: 48, data: raster(48, true) },
]));
fs.writeFileSync(`${app}/apple-icon.png`, png(180, raster(180, false)));
// PWA 매니페스트(src/app/manifest.ts)가 가리키는 파일. 실행 위치는 fe/.
fs.writeFileSync("public/icon-192.png", png(192, raster(192, true)));
fs.writeFileSync("public/icon-512.png", png(512, raster(512, true)));
fs.writeFileSync("public/icon-maskable-512.png", png(512, raster(512, false, 0.72)));
console.log("wrote favicon.ico, apple-icon.png, public/icon-{192,512,maskable-512}.png");
