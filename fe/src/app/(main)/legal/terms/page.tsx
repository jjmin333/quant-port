import { LegalDocument, LegalSection } from "@/components/legal/legal-document";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";

export const metadata = pageMetadata({
  title: "이용약관",
  description:
    "Quant Port 서비스 이용약관입니다. 서비스가 제공하는 것과 제공하지 않는 것, 투자 판단의 책임 소재, 이용자가 올린 글의 취급을 정합니다.",
  path: "/legal/terms",
});

export default function TermsPage() {
  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "이용약관", path: "/legal/terms" }])} />
      <LegalDocument title="이용약관" updatedAt="2026-09-08">
        <LegalSection title="제1조 (목적)">
          <p>
            이 약관은 Quant Port(이하 「서비스」)가 제공하는 투자 기록 관리 기능의 이용 조건과
            절차, 이용자와 서비스의 권리·의무를 정하는 것을 목적으로 합니다.
          </p>
        </LegalSection>

        <LegalSection title="제2조 (서비스의 성격 — 투자 자문이 아닙니다)">
          <p>
            서비스는 이용자가 직접 입력하거나 잔고 화면 이미지에서 읽어 온 보유·관심 종목,
            목표가, 메모를 정리해 보여 주고, 공개된 시세·뉴스·실적 데이터를 함께 배치합니다.
          </p>
          <p>
            <strong>
              서비스가 표시하는 점수·순위·AI 요약·목표가 채점은 정보 제공을 위한 것이며 특정
              종목의 매수·매도를 권유하지 않습니다.
            </strong>{" "}
            서비스는 자본시장과 금융투자업에 관한 법률에 따른 투자자문업자가 아니고, 투자자문을
            제공하지 않습니다. 모든 투자 판단과 그 결과는 이용자에게 귀속됩니다.
          </p>
        </LegalSection>

        <LegalSection title="제3조 (데이터의 정확성)">
          <p>
            시세·재무·실적·뉴스 데이터는 외부 제공자로부터 수집하며, 지연되거나 누락되거나 정정될
            수 있습니다. 서비스는 데이터의 정확성·완전성·적시성을 보증하지 않습니다. 이미지에서
            읽어 온(OCR) 보유 내역은 인식 오류가 있을 수 있으므로 이용자가 확인해야 합니다.
          </p>
        </LegalSection>

        <LegalSection title="제4조 (계정)">
          <p>
            이용자는 계정 정보를 스스로 관리할 책임이 있으며, 타인에게 계정을 양도하거나 대여할 수
            없습니다. 계정 삭제를 요청하면 관련 데이터는 제5조에 따라 처리됩니다.
          </p>
        </LegalSection>

        <LegalSection title="제5조 (이용자가 작성한 글)">
          <p>
            이용자가 작성한 리포트·댓글의 저작권은 작성자에게 있습니다. 서비스는 이를 서비스 내에
            표시·배포하기 위한 범위에서만 이용합니다. <strong>공개</strong>로 발행한 리포트는
            로그인하지 않은 사람도 볼 수 있고 검색엔진에 수집될 수 있습니다.{" "}
            <strong>링크 공개</strong>로 설정한 글은 검색엔진 수집 대상에서 제외합니다.
          </p>
          <p>
            서비스는 시세 조종·허위 사실·타인의 권리 침해에 해당하는 글을 사전 통지 없이 숨기거나
            삭제할 수 있습니다.
          </p>
        </LegalSection>

        <LegalSection title="제6조 (책임의 제한)">
          <p>
            서비스는 천재지변, 외부 데이터 제공자의 장애, 이용자의 귀책 사유로 인한 손해에 대해
            책임을 지지 않습니다. 서비스 이용으로 발생한 투자 손실에 대해서도 책임을 지지 않습니다.
          </p>
        </LegalSection>

        <LegalSection title="제7조 (약관의 변경)">
          <p>
            약관이 변경되면 서비스 화면에 공지합니다. 이용자에게 불리한 변경은 시행일 30일 전부터
            공지합니다.
          </p>
        </LegalSection>
      </LegalDocument>
    </>
  );
}
