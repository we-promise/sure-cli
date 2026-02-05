package rules

import (
	"testing"
	"time"

	"github.com/we-promise/sure-cli/internal/models"
)

func TestProposeRules_ConsistentCategory(t *testing.T) {
	now := time.Now().UTC()

	txs := []models.Transaction{
		// Same merchant, mostly same category
		{ID: "1", Name: "Starbucks", Classification: "expense", CategoryName: "Coffee", Date: now.AddDate(0, 0, -1)},
		{ID: "2", Name: "Starbucks", Classification: "expense", CategoryName: "Coffee", Date: now.AddDate(0, 0, -2)},
		{ID: "3", Name: "Starbucks", Classification: "expense", CategoryName: "Coffee", Date: now.AddDate(0, 0, -3)},
		{ID: "4", Name: "Starbucks", Classification: "expense", CategoryName: "", Date: now.AddDate(0, 0, -4)}, // uncategorized
		// Different merchant
		{ID: "5", Name: "Grocery", Classification: "expense", CategoryName: "Food", Date: now.AddDate(0, 0, -1)},
	}

	result := ProposeRules(txs)

	if result.TotalTx != 5 {
		t.Errorf("expected TotalTx=5, got %d", result.TotalTx)
	}

	// Should propose a rule for Starbucks
	found := false
	for _, p := range result.Proposals {
		if p.Pattern == "Starbucks" && p.Value == "Coffee" {
			found = true
			if p.AffectedCount != 1 {
				t.Errorf("expected AffectedCount=1, got %d", p.AffectedCount)
			}
			if p.Confidence < 0.7 {
				t.Errorf("expected Confidence >= 0.7, got %f", p.Confidence)
			}
		}
	}
	if !found {
		t.Error("expected a proposal for Starbucks -> Coffee")
	}
}

func TestProposeRules_NotEnoughOccurrences(t *testing.T) {
	now := time.Now().UTC()

	txs := []models.Transaction{
		// Only 1 occurrence - should not propose
		{ID: "1", Name: "OneTime Shop", Classification: "expense", CategoryName: "Shopping", Date: now},
	}

	result := ProposeRules(txs)

	for _, p := range result.Proposals {
		if p.Pattern == "OneTime Shop" {
			t.Error("should not propose rules for single-occurrence merchants")
		}
	}
}

func TestProposeRules_InconsistentCategory(t *testing.T) {
	now := time.Now().UTC()

	txs := []models.Transaction{
		// Same merchant, mixed categories (not consistent enough)
		{ID: "1", Name: "Amazon", Classification: "expense", CategoryName: "Shopping", Date: now.AddDate(0, 0, -1)},
		{ID: "2", Name: "Amazon", Classification: "expense", CategoryName: "Electronics", Date: now.AddDate(0, 0, -2)},
		{ID: "3", Name: "Amazon", Classification: "expense", CategoryName: "Books", Date: now.AddDate(0, 0, -3)},
	}

	result := ProposeRules(txs)

	// Should not propose because categories are inconsistent (33% each)
	for _, p := range result.Proposals {
		if p.Pattern == "Amazon" {
			t.Error("should not propose rules for inconsistent categories")
		}
	}
}
