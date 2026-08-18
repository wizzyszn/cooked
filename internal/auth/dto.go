package auth

import "github.com/wizzyszn/cooked/internal/domain"

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=3,max=32"`
	UserName string `json:"user_name" binding:"required,alphanum,min=3,max=24"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterResponse struct {
	User    *domain.SanitizedUser `json:"user"`
	Message string                `json:"message"`
}

type VerifyEmailResponse struct {
	User    *domain.SanitizedUser `json:"user"`
	Message string                `json:"message"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	User   *domain.SanitizedUser `json:"user"`
	Tokens *TokenPair            `json:"tokens"`
}
