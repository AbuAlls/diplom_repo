package usecase

import (
	"context"
	"regexp"
	"strings"

	"diplom.com/m/internal/ports"
)

// InternalAnalyticsService backs the AI agent's read-only callbacks. It guards
// ad-hoc SQL so only a single SELECT/WITH may run, then delegates to the repo
// (which additionally executes inside a read-only transaction).
type InternalAnalyticsService struct {
	Query ports.AnalyticsQueryRepo
}

func (s *InternalAnalyticsService) Schema(ctx context.Context) (map[string][]string, error) {
	return s.Query.Schema(ctx)
}

func (s *InternalAnalyticsService) RunQuery(ctx context.Context, sql string) (columns []string, rows [][]any, rowCount int, err error) {
	if err := validateReadOnlySQL(sql); err != nil {
		return nil, nil, 0, err
	}
	cols, data, err := s.Query.RunReadOnlyQuery(ctx, sql)
	if err != nil {
		return nil, nil, 0, err
	}
	return cols, data, len(data), nil
}

// forbiddenSQL matches data-modifying / DDL keywords as whole words.
var forbiddenSQL = regexp.MustCompile(`(?i)\b(insert|update|delete|drop|alter|create|truncate|grant|revoke|copy|merge|call|do|vacuum|analyze|reindex|comment|lock|set)\b`)

// validateReadOnlySQL enforces a single read-only statement: it must start with
// SELECT or WITH, must not chain statements via ';', and must not contain any
// data-modifying or DDL keyword.
func validateReadOnlySQL(sql string) error {
	trimmed := strings.TrimSpace(sql)
	trimmed = strings.TrimSuffix(trimmed, ";")
	if trimmed == "" {
		return ErrValidation
	}
	// No statement chaining once the single trailing ';' is removed.
	if strings.Contains(trimmed, ";") {
		return ErrValidation
	}
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return ErrValidation
	}
	if forbiddenSQL.MatchString(trimmed) {
		return ErrValidation
	}
	return nil
}
