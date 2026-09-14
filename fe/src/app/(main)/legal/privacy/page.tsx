import { LegalDocument, LegalSection } from "@/components/legal/legal-document";
import { breadcrumbJsonLd, pageMetadata } from "@/lib/seo";
import { JsonLd } from "@/components/seo/json-ld";

export const metadata = pageMetadata({
  title: "개인정보처리방침",
  description:
    "Quant Port가 수집하는 개인정보 항목과 이용 목적, 보관 기간, 파기 절차, 이용자의 권리를 안내합니다.",
  path: "/legal/privacy",
});

/*
 * ⚠️ **사업자 정보는 사람이 채워야 한다.**
 *
 * 상호·대표자·사업자등록번호·주소·개인정보 보호책임자·연락처는 운영자만 아는
 * 사실이라 지어낼 수 없다(지어내면 그 자체가 허위 표시다). 아래 `TODO(운영자)`
 * 자리를 채우기 전까지는 이 문서가 법적 요건을 충족하지 못한다.
 *
 * 이 초안은 **현재 코드가 실제로 하는 일**을 근거로 적었다 — 쿠키 인증(`qp_access`),
 * 잔고 화면 OCR, 리포트 공개 발행. 기능이 바뀌면 이 글도 같이 바뀌어야 한다.
 */
export default function PrivacyPage() {
  return (
    <>
      <JsonLd data={breadcrumbJsonLd([{ name: "개인정보처리방침", path: "/legal/privacy" }])} />
      <LegalDocument title="개인정보처리방침" updatedAt="2026-09-08">
        <LegalSection title="1. 수집하는 개인정보 항목">
          <ul className="list-disc space-y-1 pl-5">
            <li>회원가입 시: 이메일 주소, 비밀번호(복원 불가능한 형태로 저장), 표시 이름</li>
            <li>서비스 이용 시: 보유·관심 종목, 목표가·손절가, 투자 메모, 작성한 리포트와 댓글</li>
            <li>
              잔고 화면 이미지를 올린 경우: 이미지에서 읽어 낸 종목·수량·평균단가
            </li>
            <li>자동 생성: 접속 일시, 요청 경로, IP 주소(장애 대응·비정상 접근 차단 목적)</li>
          </ul>
        </LegalSection>

        <LegalSection title="2. 이용 목적">
          <p>
            계정 인증과 유지, 이용자가 입력한 투자 기록의 저장·계산·표시, 공개 리포트의 게시,
            서비스 오류 대응과 비정상 접근 차단에 사용합니다. 위 목적 외로 이용하지 않으며,
            이용자의 투자 기록을 광고에 활용하거나 제3자에게 판매하지 않습니다.
          </p>
        </LegalSection>

        <LegalSection title="3. 보관 기간과 파기">
          <p>
            계정이 유지되는 동안 보관하고, 계정 삭제 시 지체 없이 파기합니다. 다만 이용자가
            <strong> 공개</strong>로 발행한 리포트와 댓글은 다른 이용자의 열람 기록과 얽혀 있어,
            계정 삭제 시 작성자 표시를 지운 형태로 남을 수 있습니다. 법령이 보관을 요구하는
            기록은 해당 기간 동안 보관합니다.
          </p>
        </LegalSection>

        <LegalSection title="4. 제3자 제공과 처리 위탁">
          <p>
            이용자의 개인정보를 제3자에게 제공하지 않습니다. 서비스 운영을 위해 클라우드 인프라와
            콘텐츠 전송 네트워크를 이용하며, 이 과정에서 접속 기록이 해당 사업자의 설비를 거칩니다.
          </p>
          <p className="text-[var(--text-muted)]">
            TODO(운영자): 실제로 사용하는 처리 위탁 업체(호스팅·CDN·메일 발송 등)의 상호와 위탁
            업무를 표로 적어야 합니다.
          </p>
        </LegalSection>

        <LegalSection title="5. 쿠키">
          <p>
            로그인 상태를 유지하기 위해 인증 쿠키를 사용합니다. 화면 설정(테마 등)은 브라우저
            저장소에 보관되며 서버로 전송되지 않습니다. 브라우저 설정에서 쿠키를 거부할 수 있으나
            그 경우 로그인이 필요한 기능을 사용할 수 없습니다.
          </p>
        </LegalSection>

        <LegalSection title="6. 이용자의 권리">
          <p>
            이용자는 언제든 자신의 개인정보를 조회·수정할 수 있고, 계정 삭제를 통해 처리 정지를
            요청할 수 있습니다. 문의는 아래 연락처로 받습니다.
          </p>
        </LegalSection>

        <LegalSection title="7. 운영자 정보 및 문의처">
          <p className="text-[var(--text-muted)]">
            TODO(운영자): 상호, 대표자, 사업자등록번호, 주소, 개인정보 보호책임자 이름과 연락처를
            채워야 합니다. 이 정보는 저희가 알 수 없어 비워 두었습니다 — 채우기 전까지 이 문서는
            법적 요건을 충족하지 않습니다.
          </p>
        </LegalSection>
      </LegalDocument>
    </>
  );
}
