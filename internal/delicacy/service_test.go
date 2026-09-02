package delicacy

import (
	"errors"
	"github.com/google/uuid"
	apperrors "github.com/wizzyszn/cooked/pkg/errors"
	"testing"
)

func TestCleanNormalizesDishInput(t *testing.T) {
	c := uuid.New()
	r, e := clean(WriteRequest{Name: "  Jollof   Rice ", Description: " food ", CategoryID: &c, RegionIDs: []uuid.UUID{uuid.New()}, Aliases: []string{"jollof rice", "  Party   Jollof ", "party jollof"}, CountryCodes: []string{"ng"}})
	if e != nil {
		t.Fatal(e)
	}
	if r.Name != "Jollof Rice" || r.Description != "food" || len(r.Aliases) != 1 || r.Aliases[0] != "Party Jollof" || r.CountryCodes[0] != "NG" {
		t.Fatalf("unexpected normalized request: %#v", r)
	}
}
func TestCleanRequiresCuratedTaxonomy(t *testing.T) {
	_, e := clean(WriteRequest{Name: "Dish", Description: "Food"})
	var a *apperrors.AppError
	if !errors.As(e, &a) || a.Code != "DISH_TAXONOMY_REQUIRED" {
		t.Fatalf("error=%v", e)
	}
}
func TestCleanRejectsInvalidCountryCode(t *testing.T) {
	c := uuid.New()
	_, e := clean(WriteRequest{Name: "Dish", Description: "Food", CategoryID: &c, RegionIDs: []uuid.UUID{uuid.New()}, CountryCodes: []string{"NGA"}})
	if e == nil {
		t.Fatal("expected invalid country code")
	}
}
