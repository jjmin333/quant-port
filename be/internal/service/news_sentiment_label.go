package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
	"github.com/enact/quant-port/be/internal/sentiment"
)

/*
NewsSentimentLabelService — 종목이 붙은 기사 제목에 감성 점수와 사건 종류를 매겨 둔다.

# 어디서 도나

**점수 배치(00:25)의 첫 단계**다. 점수가 이 라벨로 심리 축을 매기므로 순서가 곧
정확도다 — 라벨을 뒤에 붙이면 그날 기사가 하루 늦게 점수에 들어간다. 관리자
「뉴스 감성 라벨」 수동 실행은 창을 넓혀 과거를 채우는 용도다(limit = 거슬러 볼 일수).

# LLM을 부르지 않는다

로컬 분류기(be/sentiment, CPU)만 부른다. 평일 약 4,500건이 몇 분 안에 끝나고 LLM 예산을
한 푼도 쓰지 않는다(2026-09-23 사용자 결정 — 로컬 모델).

# 분류기가 없으면

한 번 `/health`로 확인하고 없으면 그 회차를 건너뛴다. 라벨이 없는 종목의 심리 축은
「없음」으로 남는다 — 0이 아니다(sentiment_axis.go 머리글).
*/
type NewsSentimentLabelService struct {
	repo   *repository.NewsSentimentRepo
	client *sentiment.Client
}

func NewNewsSentimentLabelService(repo *repository.NewsSentimentRepo, client *sentiment.Client) *NewsSentimentLabelService {
	return &NewsSentimentLabelService{repo: repo, client: client}
}

const (
	// newsSentimentDefaultDays — 예약 회차가 거슬러 보는 창. 하루치에 여유를 둔다
	// (배치가 하루 빠져도 다음 회차가 메운다).
	newsSentimentDefaultDays = 3
	newsSentimentMaxDays     = 30
	newsSentimentBatch       = 64
	// newsSentimentMaxPerRun — 한 회차 상한. 평일 하루 약 4,500건의 여러 배다.
	newsSentimentMaxPerRun = 40000
)

// Run labels unlabelled tagged articles published within the last `days` days.
func (s *NewsSentimentLabelService) Run(ctx context.Context, days int) (map[string]any, string, error) {
	if s == nil || s.repo == nil {
		return nil, "", fmt.Errorf("news sentiment not configured")
	}
	if days <= 0 {
		days = newsSentimentDefaultDays
	}
	days = min(days, newsSentimentMaxDays)
	if err := s.client.Healthy(ctx); err != nil {
		// 잡의 실패가 아니라 재료의 부재다 — 점수 배치를 막지 않는다.
		msg := fmt.Sprintf("분류기 없음 — 라벨 0건 (%v)", err)
		return map[string]any{"labeled": 0, "skipped": "classifier unavailable"}, msg, nil
	}

	since := time.Now().AddDate(0, 0, -days)
	articles, err := s.repo.Unlabeled(ctx, since, newsSentimentMaxPerRun)
	if err != nil {
		return nil, "", err
	}

	start := time.Now()
	labeled, events, failed := 0, 0, 0
	for i := 0; i < len(articles); i += newsSentimentBatch {
		if ctx.Err() != nil {
			break
		}
		part := articles[i:min(i+newsSentimentBatch, len(articles))]
		items := make([]sentiment.Item, 0, len(part))
		byID := make(map[string]repository.UnlabeledArticle, len(part))
		for _, a := range part {
			lang := "en"
			if a.Market == "domestic" {
				lang = "ko"
			}
			items = append(items, sentiment.Item{ID: a.ID, Text: a.Headline, Lang: lang})
			byID[a.ID] = a
		}
		results, err := s.client.Classify(ctx, items)
		if err != nil {
			failed += len(part)
			log.Printf("news sentiment: batch at %d: %v", i, err)
			continue
		}
		labels := make([]repository.NewsSentimentLabel, 0, len(results))
		for _, r := range results {
			a, ok := byID[r.ID]
			if !ok {
				continue
			}
			ev := model.ClassifyNewsEvent(a.Headline)
			if ev != model.NewsEventNone {
				events++
			}
			labels = append(labels, repository.NewsSentimentLabel{
				ArticleID: r.ID, Score: r.Score, Label: r.Label, EventType: string(ev), Model: r.Model,
			})
		}
		if err := s.repo.UpsertLabels(ctx, labels); err != nil {
			return nil, "", err
		}
		labeled += len(labels)
	}

	result := map[string]any{
		"days": days, "candidates": len(articles), "labeled": labeled,
		"withEvent": events, "failed": failed,
		"seconds": int(time.Since(start).Seconds()),
	}
	msg := fmt.Sprintf("최근 %d일 기사 %d건 중 %d건 라벨 · 사건 %d건 · 실패 %d (%s)",
		days, len(articles), labeled, events, failed, time.Since(start).Round(time.Second))
	if len(articles) > 0 && labeled == 0 {
		return result, msg, fmt.Errorf("news sentiment: all %d articles failed", len(articles))
	}
	return result, msg, nil
}
