package usecase

import (
	"context"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type AnalyticsService struct {
	Items ports.PlanItemRepo
	Goals ports.GoalRepo
	Plans ports.PlanRepo
	Docs  ports.DocumentRepo
}

// ItemAnalytics is the v0 single-item projection: the plan item plus
// document-derived counters and value/progress passthrough.
type ItemAnalytics struct {
	Item                 domain.PlanItem
	LatestDocumentID     *int64
	SourceDocumentsCount int
	Notes                []string
}

func (s *AnalyticsService) ItemAnalytics(ctx context.Context, ownerID, itemID int64) (ItemAnalytics, error) {
	item, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, itemID, ownerID)
	if err != nil {
		return ItemAnalytics{}, err
	}
	count, err := s.Docs.CountByPlanItem(ctx, itemID)
	if err != nil {
		return ItemAnalytics{}, err
	}
	latest, err := s.Docs.LatestDocIDByPlanItem(ctx, itemID)
	if err != nil {
		return ItemAnalytics{}, err
	}
	return ItemAnalytics{
		Item:                 item,
		LatestDocumentID:     latest,
		SourceDocumentsCount: count,
	}, nil
}
