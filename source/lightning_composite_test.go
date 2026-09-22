package main

import "testing"

func TestCompositeNormalizedRequirements_Legacy(t *testing.T) {
	r := CompositeRedAlertRule{
		RequireFeedIDs:   []string{"failover_1", "failover_2"},
		RequireCondition: "Warning",
	}
	reqs := r.normalizedRequirements()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 legacy reqs, got %d", len(reqs))
	}
	if reqs[0].FeedID != "failover_1" || reqs[0].Condition != "Warning" {
		t.Fatalf("unexpected req0: %+v", reqs[0])
	}
	if reqs[1].FeedID != "failover_2" || reqs[1].Condition != "Warning" {
		t.Fatalf("unexpected req1: %+v", reqs[1])
	}
}

func TestCompositeNormalizedRequirements_PerFeed(t *testing.T) {
	r := CompositeRedAlertRule{
		Requirements: []CompositeFeedRequirement{
			{FeedID: "failover_1", Condition: "Warning"},
			{FeedID: "failover_2", Condition: "RedAlert"},
		},
		// Legacy fields present should be ignored when Requirements has ≥2 entries
		RequireFeedIDs:   []string{"primary"},
		RequireCondition: "Caution",
	}
	reqs := r.normalizedRequirements()
	if len(reqs) != 2 {
		t.Fatalf("expected 2 per-feed reqs, got %d", len(reqs))
	}
	if reqs[0].Condition != "Warning" || reqs[1].Condition != "RedAlert" {
		t.Fatalf("per-feed conditions not preserved: %+v", reqs)
	}
}

func TestCompositeNormalizedRequirements_TooFew(t *testing.T) {
	r := CompositeRedAlertRule{
		Requirements: []CompositeFeedRequirement{
			{FeedID: "failover_1", Condition: "Warning"},
		},
	}
	if len(r.normalizedRequirements()) >= 2 {
		t.Fatal("single requirement must not normalize to a matchable rule")
	}
}
