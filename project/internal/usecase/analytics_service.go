package usecase

import (
	"context"
	"strings"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type AnalyticsService struct {
	Items    ports.PlanItemRepo
	Goals    ports.GoalRepo
	Plans    ports.PlanRepo
	Docs     ports.DocumentRepo
	Analyzer ports.Analyzer
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

// Recommendations runs the analytics agent for an owned plan item and returns
// its free-text recommendations. The agent introspects/queries the database via
// the internal callback handlers.
func (s *AnalyticsService) Recommendations(ctx context.Context, ownerID, itemID int64, model, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", ErrValidation
	}
	if s.Analyzer == nil {
		return "", ErrValidation
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, itemID, ownerID); err != nil {
		return "", err
	}
	return s.Analyzer.Analyze(ctx, model, message)
}
