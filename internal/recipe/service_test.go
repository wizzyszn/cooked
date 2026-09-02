package recipe

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
	"testing"
)

func TestValidatePublishableSnapshot(t *testing.T) {
	servings := 4
	q := 2.0
	i := uuid.New()
	s := Snapshot{Title: "Rice", BaseServings: &servings, Ingredients: []IngredientInput{{ID: i, Name: "Rice", Quantity: &q, Position: 0}}, Steps: []StepInput{{Instruction: "Boil", Action: "boil", Position: 0, IngredientEntryIDs: []uuid.UUID{i}}}}
	if e := validate(s, true); e != nil {
		t.Fatal(e)
	}
	s.Steps[0].Action = "microwave"
	if e := validate(s, true); e == nil {
		t.Fatal("invalid action accepted")
	}
}
func TestIngredientScalingProjection(t *testing.T) {
	base, requested := 4, 6
	q := 2.0
	v := &domain.RecipeVersion{BaseServings: &base, Ingredients: []domain.RecipeVersionIngredient{{Quantity: &q}, {DisplayAmount: "to taste"}}}
	scale(v, &requested)
	if v.Ingredients[0].ScaledQuantity == nil || *v.Ingredients[0].ScaledQuantity != 3 || v.Ingredients[1].Scalable {
		t.Fatalf("bad scaling: %#v", v.Ingredients)
	}
}
