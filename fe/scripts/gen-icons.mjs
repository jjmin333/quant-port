// `favicon.ico` · `apple-icon.png` 를 `src/app/icon.svg` 와 같은 도형으로 다시 굽는다.
//   node scripts/gen-icons.mjs src/app
// 정식 로고가 나오면 이 스크립트째로 지우고 디자이너가 준 파일을 넣으면 된다.

import zlib from "node:zlib";
import fs from "node:fs";

// 브랜드 마크 기하 — src/app/icon.svg 와 같은 좌표계(32x32)를 쓴다.
// 여기 숫자를 고치면 icon.svg 도 같이 고쳐야 한다.
const BRAND = [0x25, 0x63, 0xeb];           // #2563EB
const BARS = [                               // x, y, w, h (32 단위)
  [5.0, 18.0, 5.0, 9.0],
  [13.5, 13.0, 5.0, 14.0],
  [22.0, 7.0, 5.0, 20.0],
];
const BAR_R = 1.5;
const PANEL_R = 7;

function roundRectCovers(x, y, rx0, ry0, w, h, r) {
  if (x < rx0 || y < ry0 || x > rx0 + w || y > ry0 + h) return false;
  const cx = Math.min(Math.max(x, rx0 + r), rx0 + w - r);
  const cy = Math.min(Math.max(y, ry0 + r), ry0 + h - r);
  return (x - cx) ** 2 + (y - cy) ** 2 <= r * r;
}

/** size x size RGBA. rounded=false 면 모서리를 깎지 않는다(애플 아이콘은 iOS가 깎는다). */
function raster(size, rounded) {
  const S = 4; // 4x 슈퍼샘플링
  const out = Buffer.alloc(size * size * 4);
  const k = 32 / size;
  for (let py = 0; py < size; py++) {
    for (let px = 0; px < size; px++) {
      let panel = 0, bar = 0;
      for (let sy = 0; sy < S; sy++) {
        for (let sx = 0; sx < S; sx++) {
          const x = (px + (sx + 0.5) / S) * k;
          const y = (py + (sy + 0.5) / S) * k;
          if (rounded ? roundRectCovers(x, y, 0, 0, 32, 32, PANEL_R) : true) panel++;
          for (const [bx, by, bw, bh] of BARS) {
            if (roundRectCovers(x, y, bx, by, bw, bh, BAR_R)) { bar++; break; }
          }
        }
      }
      const n = S * S;
      const a = panel / n;
      const b = Math.min(bar / n, a);
      const i = (py * size + px) * 4;
      // 흰 막대를 브랜드 파랑 위에 합성한 뒤, 패널 알파를 곱한다.
      for (let c = 0; c < 3; c++) {
        const covered = a > 0 ? b / a : 0;
        out[i + c] = Math.round(BRAND[c] * (1 - covered) + 255 * covered);
      }
      out[i + 3] = Math.round(a * 255);
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
console.log("wrote favicon.ico, apple-icon.png");
