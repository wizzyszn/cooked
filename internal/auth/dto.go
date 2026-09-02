package auth

import "github.com/wizzyszn/cooked/internal/user"

type RegisterRequest struct {
	Name     string `json:"name" binding:"required,min=3,max=32"`
	UserName string `json:"user_name" binding:"required,alphanum,min=3,max=24"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type RegisterResponse struct {
	User    *user.PrivateProfile `json:"user"`
	Message string               `json:"message"`
}

type VerifyEmailResponse struct {
	User    *user.PrivateProfile `json:"user"`
	Message string               `json:"message"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

type LoginResponse struct {
	User   *user.PrivateProfile `json:"user"`
	Tokens *TokenPair           `json:"tokens"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,min=32"`
}

type RefreshResponse struct {
	RefreshToken string `json:"refresh_token"`
	AccessToken  string `json:"access_token"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required,min=32"`
}

type ForgotPasswordRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type ResetPasswordRequest struct {
	Otp      string `json:"otp" binding:"required,len=6"`
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}
