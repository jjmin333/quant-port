package llm

/*
투자 리포트 「검토 노트」의 모양 (2026-09-27 사용자 결정 C ④).

리포트가 **주장하는 것**(방향·목표가)과 **우리 데이터**(점수·가격)가 서로 맞는지 대조한 메모다. 판정하거나
채점하지 않는다. 2026-10-12부터 모델을 부르지 않고 규칙 노트만 만든다(service.RuleResearchReview) —
이 파일에는 그 노트와 예전 LLM 노트가 같이 쓰는 형식만 남았다.
*/

type ResearchReviewCheck struct {
	Claim   string `json:"claim"`
	Data    string `json:"data"`
	Verdict string `json:"verdict"`
}

type ResearchReviewLLMOutput struct {
	Checks  []ResearchReviewCheck `json:"checks"`
	Summary string                `json:"summary"`
}

// ResearchReviewVerdicts is the closed set the page colours.
var ResearchReviewVerdicts = map[string]bool{"일치": true, "근거 약함": true, "반대": true, "확인 불가": true}
