package usecase

import (
	"context"
	"fmt"
	"strings"
	"time"

	"diplom.com/m/internal/domain"
	"diplom.com/m/internal/ports"
)

type AnalyticsService struct {
	Items    ports.PlanItemRepo
	Goals    ports.GoalRepo
	Plans    ports.PlanRepo
	Groups   ports.GroupRepo
	Docs     ports.DocumentRepo
	Analyzer ports.Analyzer
	// Sessions maps per-analyze-call nonces to ownerIDs so the AI agent's
	// backend callbacks can be scoped to the correct user's rows.
	Sessions *SessionStore
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
	item, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, itemID, ownerID)
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
// sessionTTL is the lifetime of a per-analyze session nonce. It must be long
// enough to cover the agent's full run (up to 3 LLM iterations + I/O).
const sessionTTL = 15 * time.Minute

func (s *AnalyticsService) Recommendations(ctx context.Context, ownerID, itemID int64, model, message string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return "", ErrValidation
	}
	if s.Analyzer == nil {
		return "", ErrValidation
	}
	if _, err := requireItemByID(ctx, s.Items, s.Goals, s.Plans, s.Groups, itemID, ownerID); err != nil {
		return "", err
	}

	// Create a short-lived session so the agent's backend callbacks can be
	// scoped to this user's data. We inject the nonce into the message so the
	// LLM includes it as X-Internal-Token on every /api/* call it makes.
	var scopedMessage = message
	if s.Sessions != nil {
		nonce := s.Sessions.Create(ownerID, sessionTTL)
		defer s.Sessions.Delete(nonce)
		scopedMessage = fmt.Sprintf(
			"[SYSTEM: You MUST send the HTTP header \"X-Internal-Token: %s\" "+
				"on EVERY request to the backend API (/api/schema and /api/analytics/query). "+
				"This is mandatory for data security — queries without it will be rejected.]\n\n%s",
			nonce, message,
		)
	}
	return s.Analyzer.Analyze(ctx, model, scopedMessage)
}
