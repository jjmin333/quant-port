/**
 * JSON-LD 한 덩이를 HTML에 심는다.
 *
 * `next/script`가 아니라 평범한 `<script>`인 이유는 루트 레이아웃의 같은 주석과
 * 같다 — 크롤러는 자바스크립트 실행 전의 HTML을 읽는 일이 많고, 특히 네이버
 * Yeti는 사실상 그것만 본다.
 */
export function JsonLd({ data }: { data: object }) {
  return (
    <script
      type="application/ld+json"
      dangerouslySetInnerHTML={{ __html: JSON.stringify(data) }}
    />
  );
}
