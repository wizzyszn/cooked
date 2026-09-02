package platform

import "github.com/google/uuid"

type IDGenerator interface {
	New() uuid.UUID
}

type UUIDGenerator struct{}

func (UUIDGenerator) New() uuid.UUID { return uuid.New() }
