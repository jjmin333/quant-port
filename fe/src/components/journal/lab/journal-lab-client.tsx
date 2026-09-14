"use client";

import { useState } from "react";
import { PageColumns } from "@/components/layout/page-columns";
import { PageScrollShell } from "@/components/layout/page-content-shell";
import { JournalNoteModal } from "@/components/journal/journal-note-modal";
import type { JournalNote } from "@/lib/journal-types";
import { JournalLabProvider, useLab } from "./jl-shared";
import { JournalDeskConcept, JournalDeskRail } from "./jl-desk";
import { JournalFeedConcept, JournalFeedRail } from "./jl-feed";
import { JournalReviewConcept, JournalReviewRail } from "./jl-review";

/**
 * 메모장 재설계 시안의 몸통 (2026-09-07 사용자 요청).
 *
 * # 지금 화면의 진단
 *
 * 1. **가장 넓은 칸이 가장 오래 비어 있다.** 1280px에서 목록 290px · 본문 나머지인데,
 *    들어오면 그 본문이 「왼쪽에서 메모를 고르면 여기에 펼쳐집니다」다. 첫인상이 빈 상자다.
 * 2. **쓰는 일이 가장 찾기 어렵다.** 「새 메모 작성」은 목록 카드 안쪽의 점선 버튼이고,
 *    좁은 화면에서는 목록 헤더 아래에 묻힌다.
 * 3. **우리 데이터가 메모에 닿지 않는다.** 저장 순간 박제되는 작성 시점 가격
 *    (`journal_notes.price_at_write`)은 이 앱의 메모가 다른 메모장과 다른 유일한 지점인데,
 *    화면에서는 행 끝의 작은 % 하나와 레일 맨 아래 두 줄이 전부다. 종목을 붙여도 시세·
 *    차트·점수·퀵룩 중 어디로도 이어지지 않는다.
 *
 * # 세 시안이 다투는 것
 *
 * 셋 다 (1)(2)는 같은 방식으로 고친다 — 본문 한 컬럼, 맨 위에 작성 바, 상세는 모달.
 * 갈리는 것은 **(3)을 무엇으로 갚느냐**다.
 *
 * - `feed` — 시간. 연대기 + 「답이 오고 있는 메모」 띠.
 * - `desk` — 종목. 메모를 종목 아래에 묶고 시세·흐름·성적을 머리에 세운다.
 * - `review` — 성과. 성적표를 첫 화면으로 끌어올린다.
 *
 * 그리는 조각은 셋 다 실서비스의 것이다(`journal-note-modal`·`journal-note-form`·
 * `journal-shared`·`stock-avatar`·`market-radar-sparkline`·`quick-look-provider`).
 * 데이터만 시안용 픽스처다 — 이유는 `./fixture` 머리말.
 */

export type JournalLabVariant = "feed" | "desk" | "review";

export function JournalLabClient({ variant }: { variant: JournalLabVariant }) {
  return (
    <JournalLabProvider>
      <JournalLabBody variant={variant} />
    </JournalLabProvider>
  );
}

function JournalLabBody({ variant }: { variant: JournalLabVariant }) {
  const { quoteFor } = useLab();
  const [opened, setOpened] = useState<JournalNote | null>(null);

  const rail =
    variant === "desk" ? (
      <JournalDeskRail />
    ) : variant === "review" ? (
      <JournalReviewRail />
    ) : (
      <JournalFeedRail />
    );

  return (
    <PageScrollShell>
      <PageColumns rail={rail}>
        {variant === "desk" ? (
          <JournalDeskConcept onOpen={setOpened} />
        ) : variant === "review" ? (
          <JournalReviewConcept onOpen={setOpened} />
        ) : (
          <JournalFeedConcept onOpen={setOpened} />
        )}
      </PageColumns>

      {/*
        상세는 **실서비스 모달 그대로**다 — 작성 시점 기준선이 얹힌 차트, 목표가 줄,
        수정 폼까지 같은 것이 뜬다. 시안이 따로 그린 상세를 보여 주면 「모달까지 새로
        만들어야 하나」라는 질문이 끼어든다.

        저장·삭제는 아무 일도 하지 않는다. 시안이 진짜 메모를 건드리면 곤란하다.
      */}
      <JournalNoteModal
        note={opened}
        quote={opened ? quoteFor(opened) : undefined}
        open={opened !== null}
        onClose={() => setOpened(null)}
        onSave={async () => {}}
        onTogglePin={() => {}}
        onDelete={() => setOpened(null)}
      />
    </PageScrollShell>
  );
}
