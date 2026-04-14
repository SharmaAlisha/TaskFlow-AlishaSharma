package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/taskflow/backend/internal/apperror"
	"github.com/taskflow/backend/internal/config"
	"github.com/taskflow/backend/internal/dto"
	"github.com/taskflow/backend/internal/model"
	"github.com/taskflow/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	userRepo    *repository.UserRepository
	refreshRepo *repository.RefreshTokenRepository
	cfg         *config.Config
	logger      *slog.Logger
}

func NewAuthService(
	userRepo *repository.UserRepository,
	refreshRepo *repository.RefreshTokenRepository,
	cfg *config.Config,
	logger *slog.Logger,
) *AuthService {
	return &AuthService{
		userRepo:    userRepo,
		refreshRepo: refreshRepo,
		cfg:         cfg,
		logger:      logger,
	}
}

func (s *AuthService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, string, error) {
	existing, err := s.userRepo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}
	if existing != nil {
		return nil, "", apperror.NewValidation(map[string]string{
			"email": "already registered",
		})
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	user := &model.User{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(req.Name),
		Email:     strings.ToLower(strings.TrimSpace(req.Email)),
		Password:  string(hashed),
		CreatedAt: time.Now().UTC(),
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return nil, "", apperror.NewValidation(map[string]string{
				"email": "already registered",
			})
		}
		return nil, "", apperror.NewInternal(err)
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	refreshToken, err := s.createRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	return &dto.AuthResponse{
		AccessToken: accessToken,
		User: dto.UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		},
	}, refreshToken, nil
}

func (s *AuthService) Login(ctx context.Context, req dto.LoginRequest) (*dto.AuthResponse, string, error) {
	user, err := s.userRepo.FindByEmail(ctx, strings.ToLower(strings.TrimSpace(req.Email)))
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}
	if user == nil {
		return nil, "", apperror.NewUnauthorized("invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, "", apperror.NewUnauthorized("invalid credentials")
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	refreshToken, err := s.createRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	return &dto.AuthResponse{
		AccessToken: accessToken,
		User: dto.UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		},
	}, refreshToken, nil
}

func (s *AuthService) Refresh(ctx context.Context, rawToken string) (*dto.RefreshResponse, string, error) {
	tokenHash := hashToken(rawToken)

	existing, err := s.refreshRepo.FindByHash(ctx, tokenHash)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}
	if existing == nil {
		return nil, "", apperror.NewUnauthorized("invalid or expired refresh token")
	}

	if err := s.refreshRepo.DeleteByHash(ctx, tokenHash); err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	user, err := s.userRepo.FindByID(ctx, existing.UserID)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}
	if user == nil {
		return nil, "", apperror.NewUnauthorized("user not found")
	}

	accessToken, err := s.generateAccessToken(user)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	newRefreshToken, err := s.createRefreshToken(ctx, user.ID)
	if err != nil {
		return nil, "", apperror.NewInternal(err)
	}

	return &dto.RefreshResponse{AccessToken: accessToken}, newRefreshToken, nil
}

func (s *AuthService) Logout(ctx context.Context, rawToken string) error {
	tokenHash := hashToken(rawToken)
	return s.refreshRepo.DeleteByHash(ctx, tokenHash)
}

func (s *AuthService) generateAccessToken(user *model.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"email":   user.Email,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(s.cfg.JWTAccessExpiry).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

func (s *AuthService) createRefreshToken(ctx context.Context, userID string) (string, error) {
	rawBytes := make([]byte, 32)
	if _, err := rand.Read(rawBytes); err != nil {
		return "", err
	}
	rawToken := base64.URLEncoding.EncodeToString(rawBytes)

	rt := &repository.RefreshToken{
		ID:        uuid.NewString(),
		UserID:    userID,
		TokenHash: hashToken(rawToken),
		ExpiresAt: time.Now().UTC().Add(s.cfg.JWTRefreshExpiry),
		CreatedAt: time.Now().UTC(),
	}
	if err := s.refreshRepo.Create(ctx, rt); err != nil {
		return "", err
	}
	return rawToken, nil
}

func hashToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}
