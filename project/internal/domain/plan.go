package domain

import "time"

type Plan struct {
	ID          int64
	Name        string
	Description *string
	CreatedBy   int64
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Goal struct {
	ID          int64
	PlanID      int64
	Name        string
	Description *string
	SortOrder   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type PlanItem struct {
	ID           int64
	GoalID       int64
	GoalName     string
	Name         string
	Description  *string
	ItemType     *string
	Status       *string
	TargetValue  *float64
	CurrentValue *float64
	Unit         *string
	SortOrder    int
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// ProgressPercent returns the computed completion ratio (0..100) when both a
// positive target and a current value are present.
func (i PlanItem) ProgressPercent() *float64 {
	if i.TargetValue == nil || i.CurrentValue == nil || *i.TargetValue <= 0 {
		return nil
	}
	p := (*i.CurrentValue / *i.TargetValue) * 100
	if p < 0 {
		p = 0
	}
	if p > 100 {
		p = 100
	}
	return &p
}
