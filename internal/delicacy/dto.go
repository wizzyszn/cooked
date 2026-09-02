package delicacy

import "github.com/google/uuid"

type WriteRequest struct {
	Name           string      `json:"name" binding:"required,min=3,max=255"`
	Description    string      `json:"description" binding:"required,min=3"`
	CategoryID     *uuid.UUID  `json:"category_id"`
	RegionIDs      []uuid.UUID `json:"region_ids"`
	Aliases        []string    `json:"aliases"`
	CountryCodes   []string    `json:"country_codes"`
	OriginNotes    string      `json:"origin_notes"`
	CoverMediaID   *uuid.UUID  `json:"cover_media_id"`
	ConfirmSimilar bool        `json:"confirm_similar"`
}
type ModerateRequest struct {
	Reason string `json:"reason" binding:"required,min=3"`
}
type MergeRequest struct {
	TargetID uuid.UUID `json:"target_id" binding:"required"`
	Reason   string    `json:"reason" binding:"required,min=3"`
}
type TaxonomyRequest struct {
	Name   string `json:"name" binding:"required,min=2,max=100"`
	Slug   string `json:"slug"`
	Symbol string `json:"symbol"`
}
