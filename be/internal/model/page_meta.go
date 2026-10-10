package model

/*
화면별 제목·설명 덮어쓰기 (2026-09-15 사용자 요청 2).

브라우저 탭 이름과 검색 결과에 나가는 글을 **배포 없이** 고치기 위한 자료다. 표에
무엇을 담고 왜 「덮어쓰기」인지는 `migrations/074_page_meta.sql` 머리글에 적어 두었다.

# 빈 칸은 「안 정했다」이다

문자열 필드가 포인터가 아닌 이유는 빈 문자열 자체가 그 뜻이기 때문이다 — 제목을
「없음」으로 만들고 싶은 사람은 없다. 반면 `NoIndex`는 포인터다. **「아직 안 정했다」와
「색인해도 된다고 정했다」가 다른 상태**라서, 셋을 구분하지 않으면 관리자가 한 번
켰다 끈 화면을 코드 기본값으로 되돌릴 방법이 없어진다.
*/
type PageMeta struct {
	/** 앱 경로 그대로 — `/`, `/market`, `/news/domestic`. */
	Path        string `json:"path"`
	Title       string `json:"title"`
	Description string `json:"description"`
	/** 비어 있으면 위의 title/description 이 SNS 카드에도 그대로 쓰인다. */
	OGTitle       string `json:"ogTitle"`
	OGDescription string `json:"ogDescription"`
	OGImage       string `json:"ogImage"`
	/** nil = 안 정했다(코드 기본값을 따른다). */
	NoIndex   *bool  `json:"noindex,omitempty"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// IsEmpty reports whether this override would change nothing. Saving such a
// row is the same as deleting it — the write path does exactly that, so a row
// never sits in the table claiming to override while overriding nothing.
func (p PageMeta) IsEmpty() bool {
	return p.Title == "" &&
		p.Description == "" &&
		p.OGTitle == "" &&
		p.OGDescription == "" &&
		p.OGImage == "" &&
		p.NoIndex == nil
}
