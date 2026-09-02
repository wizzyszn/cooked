package db

import (
	"context"
	"fmt"

	"gorm.io/gorm"
)

// WithinTransaction executes fn atomically and rolls back on any returned
// error or panic. Services should use the provided transaction handle for all
// repository calls that participate in the invariant.
func WithinTransaction(ctx context.Context, database *gorm.DB, fn func(tx *gorm.DB) error) error {
	if database == nil {
		return fmt.Errorf("database is required")
	}
	if fn == nil {
		return fmt.Errorf("transaction function is required")
	}
	return database.WithContext(ctx).Transaction(fn)
}
