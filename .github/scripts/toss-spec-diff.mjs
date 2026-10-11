// Toss Open API 스펙 두 판을 비교해 바뀐 곳을 마크다운으로 낸다. 같으면 아무것도 출력하지 않는다.
// 사용: node .github/scripts/toss-spec-diff.mjs <저장소 기준본.json> <새 판.json>
// OpenAPI·AsyncAPI 둘 다 쓴다 — 구조를 몰라도 되게 JSON 경로 단위로 비교한다.
import { readFileSync } from 'node:fs';

const [oldDoc, newDoc] = process.argv.slice(2, 4).map((f) => JSON.parse(readFileSync(f, 'utf8')));

// tags·parameters처럼 name이 있는 배열은 이름으로 짚는다 — 하나 끼워 넣으면 뒤가 전부 밀려 보이지 않게.
const named = (v) => Array.isArray(v) && v.length > 0 && v.every((e) => typeof e?.name === 'string');
const flat = (v, path = '', out = new Map()) => {
  if (named(v)) for (const c of v) flat(c, `${path}/${c.name}`, out);
  else if (v && typeof v === 'object') for (const [k, c] of Object.entries(v)) flat(c, `${path}/${k}`, out);
  else out.set(path, v);
  return out;
};
const a = flat(oldDoc);
const b = flat(newDoc);
const clip = (v) => JSON.stringify(v).slice(0, 160);

// 예시 값(examples)은 바뀌어도 계약이 아니다 — 세지만 줄로 늘어놓지 않는다.
const lines = [];
let exampleOnly = 0;
for (const p of new Set([...a.keys(), ...b.keys()])) {
  if (a.get(p) === b.get(p)) continue;
  if (/\/examples?\//.test(p)) { exampleOnly++; continue; }
  if (!a.has(p)) lines.push(`+ \`${p}\` = ${clip(b.get(p))}`);
  else if (!b.has(p)) lines.push(`- \`${p}\``);
  else lines.push(`~ \`${p}\`: ${clip(a.get(p))} → ${clip(b.get(p))}`);
}
if (lines.length === 0 && exampleOnly === 0) process.exit(0);

const MAX = 200;
console.log(`**${oldDoc.info?.version} → ${newDoc.info?.version}** · 바뀐 경로 ${lines.length}개 (예시만 바뀐 곳 ${exampleOnly}개 제외)\n`);
console.log('```diff');
console.log(lines.slice(0, MAX).join('\n'));
if (lines.length > MAX) console.log(`… ${lines.length - MAX}개 더`);
console.log('```');
