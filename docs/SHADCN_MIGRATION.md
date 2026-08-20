# shadcn/ui 전환 설계

FE 전체 UI를 shadcn/ui 기반으로 교체한다. 최종 목표는 **shadcn 기본 스타일(neutral 팔레트 · 기본 여백/반경)로의 리디자인**(사용자 결정, 08-10).

기존 화면 규모: 페이지 37개, `.tsx` 163개. 사용처는 `<Card>` 137 · `<button>` 182 · `<input>` 48 · `<Badge>` 36 · `<table>` 11.
한 번에 갈아엎지 않고 4단계로 끊는다. 각 단계는 `tsc` + `next build` + 브라우저 실측(1280 / 375)으로 닫는다.

---

## 왜 단계를 나누는가

`--radius-*` 하나만 켜도 앱 전체 341곳의 모서리가 동시에 움직인다(1단계 실측). 토큰 교체와 컴포넌트 교체를 같은 커밋에 섞으면 회귀가 생겼을 때 **어느 쪽이 원인인지 되짚을 수 없다**. 그래서 1단계는 "정의만 하고 화면은 그대로"를 강제한다.

---

## 스택 적합성 (1단계에서 실측 확인)

| 항목 | 값 | 비고 |
|---|---|---|
| Next / React | 16.2.10 / 19.2.4 App Router | shadcn CLI 4.16.2가 프레임워크 감지 성공 |
| Tailwind | v4 (CSS 기반) | `@theme inline` / `@custom-variant` 사용 |
| 별칭 | `@/*` → `src/*` | `components.json` 그대로 사용 |
| `cn()` | `lib/utils.ts` (clsx + tailwind-merge) | 이미 있어 재생성 불필요 |
| 아이콘 | lucide-react | shadcn 기본과 동일 |
| 추가된 의존성 | `radix-ui`, `class-variance-authority`, `tw-animate-css` | |

CLI는 `globals.css`를 건드리지 않았다(`shadcn add` 전후 diff 0). 토큰은 전부 수작업으로 넣었다 — CLI init에 맡기면 자기 `:root` 블록을 뒤에 붙이면서 기존 `[data-theme=...]` 팔레트를 특정도 동률·순서 우위로 덮는다.

---

## 토큰 이름 충돌 (핵심)

이 앱은 `<html data-theme="...">`로 **테마 4종**(light / dark / midnight / general)을 돌리고, shadcn은 `.dark` 클래스 + 자기 이름 세트를 전제한다.

| 이름 | 이 앱의 의미 | shadcn의 의미 | 처리 |
|---|---|---|---|
| `--accent` | **브랜드 블루** (링크·활성) | **은은한 hover 표면** | 앱 쪽을 `--brand`로 이름 변경(273곳 일괄). `--accent`는 shadcn에 넘김 |
| `--accent-muted` | 브랜드 반투명 | (없음) | `--brand-muted` |
| `--accent-foreground` | (정의된 적 없음 · 사용 3곳) | 있음 | `--brand-foreground`로 이름만 옮김 — **여전히 미정의라 그 3곳은 글자색이 상속으로 떨어진다**(기존부터 있던 버그, 2단계에서 shadcn Button으로 교체하며 해소) |
| `--background` | 앱 배경 (#F1F5F9 등) | 앱 배경 (흰색) | 의미 같고 값 다름 → **1단계에서 손대지 않음**, 2단계에 넘김 |
| `--border` | 구분선 | 구분선 | 위와 동일 |
| `--sidebar-width` | 레이아웃 220px | shadcn sidebar 블록 16rem | 블록이 자기 wrapper에 인라인 선언 → 지금은 무해. **3단계에서 정리 필요** |
| `--profit` / `--loss` | 상승 빨강 / 하락 파랑(한국식) | 대응물 없음 | 그대로 유지. shadcn 색으로 흡수하지 않는다 |

`dark:` 변형은 `@custom-variant`로 이 앱의 테마 스위치에 연결했다:

```css
@custom-variant dark (&:is([data-theme="dark"], [data-theme="midnight"], [data-theme="general"],
                           [data-theme="dark"] *, [data-theme="midnight"] *, [data-theme="general"] *));
```

`[data-theme=x]` 자신을 넣는 이유는 속성이 `<html>`에 붙어 있어서다. 컴파일된 CSS에서 `.dark\:bg-destructive\/60:is([data-theme="dark"], ...)`로 나오는 것까지 확인했다.

---

## 단계

### 1단계 — 기반 (완료, 08-10)

- `components.json`(new-york / neutral / cssVariables)
- shadcn 토큰 전체를 `globals.css`에 정의. 값은 레지스트리 `neutral` 원본 그대로(`https://ui.shadcn.com/r/colors/neutral.json`)
- `@custom-variant dark` + `@theme inline` 매핑(색 + 앱 고유 토큰 `--brand`/`--surface`/`--profit`/`--loss`)
- `--accent` → `--brand` 이름 변경
- 스모크 테스트로 `ui/button.tsx` 1개만 설치

**일부러 넣지 않은 것**(전부 2단계):
- `--radius-*` 스케일 — Tailwind 기본 반경을 덮어써서 `rounded-lg` 8px→10px 등 341곳이 움직인다(실측)
- shadcn base 레이어(`* { @apply border-border }`, `body { @apply bg-background text-foreground }`) — 색 미지정 테두리 전체와 body 글자색을 즉시 바꾼다

### 2단계 — 토큰 전환 + 프리미티브 (완료, 08-10)

**토큰**: `--background`/`--border`/`--foreground`를 shadcn 값으로 넘기고, `--radius-*` 스케일과 base 레이어(`* { border-color }`)를 켰다. 테마는 라이트/다크 2종으로 축소(`midnight` 제거 + 저장값 마이그레이션).

**핵심 구현 판단 — 이 앱 고유 토큰을 shadcn 토큰의 별칭으로 만들었다**:

```css
--surface: var(--card);
--surface-elevated: var(--accent);
--text-primary: var(--foreground);
--text-secondary: var(--muted-foreground);
```

`var(--surface)`를 쓰는 화면 137곳을 건드리지 않고 색만 shadcn으로 넘어간다. 별칭을 걷어내는 건 호출부 교체가 끝난 뒤의 일이다.

**컴포넌트**: `dropdown-menu`·`tooltip`·`sheet`·`dialog`·`alert-dialog`·`tabs`·`separator`·`sonner`·`command`·`input`·`label` 설치. `button`/`input`은 촘촘 밀도(h-8 / text-xs)로 조정.

**`Card`/`Badge`는 레지스트리 버전으로 갈아끼우지 않고 토큰만 갈아입혔다.** shadcn `CardHeader`는 `grid` + `px-6` 기반이라 이 앱의 `flex items-center justify-between`와 레이아웃 규칙이 달라, 호출부 137곳의 헤더 정렬이 한 번에 어긋난다. 둘 다 동작이 없는 표현 전용 컴포넌트라 토큰을 바꾸면 결과가 같다.

### 2.5단계 — 셸 (완료, 08-10)

헤더 56px + 계정 드롭다운 통합(알림·설정 아이콘 제거), 사이드바 48px 레일 + 접힘 상태 저장 + `--sidebar` 토큰 + 중립 활성 표시, `PageHeader` 우측 동작 영역, 종목 탭 바 중립 활성 + 우측 페이드.

### 3단계 — 전역 크롬

사이드바(`layout/sidebar.tsx`) → shadcn sidebar 블록, 헤더 검색 → `Command`(⌘K), 토스트 → `Sonner`, 드롭다운/다이얼로그/툴팁. `--sidebar-width` 충돌을 여기서 정리한다.

### 4단계 — 페이지별

`Table`·`Tabs`·`Select`·`Popover`로 화면 단위 교체. 관리자 화면처럼 폼이 많은 곳이 이득이 크다.

---

## 교체하지 않는 것

- `lightweight-charts` 터미널(`stock-chart-terminal.tsx`, `candlestick-chart.tsx`), `recharts` 차트 — shadcn에 대응물이 없다. 껍데기(Card/Tabs)만 교체
- 자체 마크다운 파서/렌더러/에디터(`lib/markdown.ts`, `ui/markdown.tsx`, `ui/markdown-editor.tsx`) — 공개 사용자 작성물의 XSS 경계를 직접 통제하려고 의존성 없이 만든 것이다
- 손익 색(`--profit`/`--loss`) — 한국식 상승 빨강/하락 파랑. shadcn 팔레트에 흡수하면 뒤집힌다

---

## 실측 방법 메모

테마를 바꾸며 계산값을 읽을 때 **`getComputedStyle`을 그냥 쓰면 안 된다**. shadcn 컴포넌트에는 `transition-all`이 붙어 있어 속성을 바꾼 직후 읽으면 전이 시작값(= 이전 테마 색)이 나오고, 기다렸다 읽으면 `PreferencesProvider`가 저장된 설정으로 `data-theme`를 되돌려 놓는다. 1단계에서 이 둘 때문에 "dark 테마에서 토큰이 안 먹는다"는 잘못된 결론을 한 번 냈다.

```js
// 읽기 전에 전이를 죽이고 같은 태스크 안에서 동기적으로 읽는다
const kill = document.createElement("style");
kill.textContent = "*{transition:none!important;animation:none!important}";
document.head.appendChild(kill);
```
