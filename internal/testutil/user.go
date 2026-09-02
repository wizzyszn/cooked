package testutil

import (
	"github.com/google/uuid"
	"github.com/wizzyszn/cooked/internal/domain"
)

type UserBuilder struct {
	user domain.User
}

func NewUser() *UserBuilder {
	return &UserBuilder{user: domain.User{
		BaseModel:  domain.BaseModel{ID: uuid.New()},
		Email:      "cook@example.com",
		Name:       "Test Cook",
		UserName:   "testcook",
		IsVerified: true,
		Roles:      []domain.UserRole{{Role: domain.RoleUser}},
	}}
}

func (b *UserBuilder) Unverified() *UserBuilder {
	b.user.IsVerified = false
	return b
}

func (b *UserBuilder) WithRole(role domain.Role) *UserBuilder {
	b.user.Roles = append(b.user.Roles, domain.UserRole{Role: role})
	return b
}

func (b *UserBuilder) Build() *domain.User {
	copy := b.user
	copy.Roles = append([]domain.UserRole(nil), b.user.Roles...)
	return &copy
}
