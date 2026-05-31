package pgcore

import (
	"strings"
	"testing"
)

func TestOwnerScopedSQL(t *testing.T) {
	const original = "SELECT * FROM documents WHERE status = 'confirmed'"
	result := ownerScopedSQL(original, 99)

	// Must contain the CTE prefix wrapping the original query.
	if !strings.Contains(result, "public.documents WHERE uploaded_by = 99") {
		t.Errorf("expected documents CTE scoped to owner 99;\ngot:\n%s", result)
	}
	if !strings.Contains(result, "public.plans WHERE created_by = 99") {
		t.Errorf("expected plans CTE scoped to owner 99;\ngot:\n%s", result)
	}
	if !strings.Contains(result, "public.folders WHERE created_by = 99") {
		t.Errorf("expected folders CTE scoped to owner 99;\ngot:\n%s", result)
	}
	if !strings.HasSuffix(strings.TrimSpace(result), original) {
		t.Errorf("original query must appear at end of wrapped SQL;\ngot:\n%s", result)
	}
}

func TestOwnerScopedSQLZeroOwner(t *testing.T) {
	// ownerID=0 means no scoping; the repo passes sql through unchanged.
	// ownerScopedSQL itself is only called when ownerID > 0.
	result := ownerScopedSQL("SELECT 1", 0)
	if !strings.Contains(result, "uploaded_by = 0") {
		t.Errorf("unexpected result for ownerID 0: %s", result)
	}
}
