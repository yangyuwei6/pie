package biz

import (
	"context"
	"errors"
	"time"

	"pie/internal/auth"
	"pie/internal/data/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserRepo interface {
	Create(ctx context.Context, user *model.User) error
	FindByUsername(ctx context.Context, username string) (*model.User, error)
	FindByID(ctx context.Context, userID int64) (*model.User, error)
	Update(ctx context.Context, user *model.User) error
	FindAll(ctx context.Context) ([]*model.User, error)
	FindWithPagination(ctx context.Context, offset, limit int) ([]*model.User, int64, error)
}

type TokenRepo interface {
	Blacklist(ctx context.Context, token string, ttl time.Duration) error
	IsBlacklisted(ctx context.Context, token string) (bool, error)
}

type UserUsecase struct {
	userRepo   UserRepo
	tokenRepo  TokenRepo
	orgTagBiz  *OrgTagUsecase
	jwtManager *auth.JWTManager
	logger     *zap.Logger
}

func NewUserUsecase(userRepo UserRepo, tokenRepo TokenRepo, orgTagBiz *OrgTagUsecase, jwtManager *auth.JWTManager, logger *zap.Logger) *UserUsecase {
	return &UserUsecase{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		orgTagBiz:  orgTagBiz,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

func (u *UserUsecase) Register(ctx context.Context, username, password string) (*model.User, error) {
	_, err := u.userRepo.FindByUsername(ctx, username)
	if err == nil {
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		return nil, err
	}

	user := &model.User{
		Username: username,
		Password: hashedPassword,
		Role:     "USER",
	}

	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	privateOrgTag, err := u.orgTagBiz.CreatePrivateOrgTag(ctx, user.ID, username)
	if err != nil {
		return nil, err
	}

	user.OrgTags = &privateOrgTag
	user.PrimaryOrg = &privateOrgTag
	if err := u.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (u *UserUsecase) Login(ctx context.Context, username, password string) (string, string, error) {
	user, err := u.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}

	if !auth.CheckPasswordHash(password, user.Password) {
		return "", "", ErrInvalidCredentials
	}

	token, err := u.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := u.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	return token, refreshToken, nil
}

func (u *UserUsecase) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	blacklisted, err := u.tokenRepo.IsBlacklisted(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}
	if blacklisted {
		return "", "", ErrInvalidRefreshToken
	}

	claims, err := u.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		return "", "", ErrInvalidRefreshToken
	}

	user, err := u.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrInvalidRefreshToken
		}
		return "", "", err
	}

	newToken, err := u.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := u.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	return newToken, newRefreshToken, nil
}

func (u *UserUsecase) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := u.jwtManager.VerifyAccessToken(accessToken)
	if err != nil {
		return ErrInvalidCredentials
	}

	if err := u.tokenRepo.Blacklist(ctx, accessToken, auth.TokenTTL(claims)); err != nil {
		return err
	}

	if refreshToken == "" {
		return nil
	}

	refreshClaims, err := u.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	return u.tokenRepo.Blacklist(ctx, refreshToken, auth.TokenTTL(refreshClaims))
}

func (u *UserUsecase) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	return u.tokenRepo.IsBlacklisted(ctx, token)
}

func (u *UserUsecase) Me(ctx context.Context, userID int64) (*model.User, error) {
	user, err := u.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (u *UserUsecase) CreateUser(ctx context.Context, user *model.User) error {
	return u.userRepo.Create(ctx, user)
}

func (u *UserUsecase) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return u.userRepo.FindByUsername(ctx, username)
}

func (u *UserUsecase) GetUserByID(ctx context.Context, userID int64) (*model.User, error) {
	return u.userRepo.FindByID(ctx, userID)
}

func (u *UserUsecase) UpdateUser(ctx context.Context, user *model.User) error {
	return u.userRepo.Update(ctx, user)
}

func (u *UserUsecase) GetAllUsers(ctx context.Context) ([]*model.User, error) {
	return u.userRepo.FindAll(ctx)
}

func (u *UserUsecase) GetUsersWithPagination(ctx context.Context, offset, limit int) ([]*model.User, int64, error) {
	return u.userRepo.FindWithPagination(ctx, offset, limit)
}
