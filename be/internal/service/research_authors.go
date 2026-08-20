package service

import (
	"context"

	"github.com/enact/quant-port/be/internal/authctx"
	"github.com/enact/quant-port/be/internal/model"
	"github.com/enact/quant-port/be/internal/repository"
)

// The author scorecard.
//
// There is deliberately no ranking table on top of this. Ranking authors turns
// a report into a contest entry, and a contest needs rewards to stay alive —
// which is exactly what this feature must not have (docs/RESEARCH_CALL_DESIGN.md
// §15-1). A per-author scorecard still answers the only question that matters
// when reading someone's report: what happened to the ones they wrote before.

// AuthorProfile is one author's public scorecard plus their public reports.
func (s *ResearchService) AuthorProfile(ctx context.Context, userID int64) (model.ResearchAuthorProfile, error) {
	viewerID, _ := authctx.UserID(ctx)
	// Own profile shows everything; other people see the public reports only.
	filter := repository.ResearchListFilter{UserID: userID, Limit: 100}
	if viewerID != userID {
		filter.PublicOnly = true
	} else {
		filter.IncludeHidden = true
	}
	calls, err := s.repo.List(ctx, filter)
	if err != nil {
		return model.ResearchAuthorProfile{}, err
	}

	// The summary counts every report the author owns, public or not — a
	// scorecard that only counted the visible half would be the same
	// cherry-picking the retract rate exists to prevent.
	all, err := s.repo.ListAllForUser(ctx, userID)
	if err != nil {
		return model.ResearchAuthorProfile{}, err
	}

	out := model.ResearchAuthorProfile{UserID: userID, Summary: summarizeResearch(all)}
	for _, c := range all {
		if c.AuthorName != "" {
			out.AuthorName = c.AuthorName
			break
		}
	}
	views, err := s.decorate(ctx, viewerID, calls)
	if err != nil {
		return model.ResearchAuthorProfile{}, err
	}
	out.Calls = views
	return out, nil
}
