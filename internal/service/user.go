package service

import (
	"context"
	"errors"
	"strings"

	"pie/internal/auth"
	"pie/internal/data/model"
	"pie/internal/repo"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type UserService struct {
	userRepo   repo.UserRepo
	tokenRepo  repo.TokenRepo
	orgTagRepo repo.OrgTagRepo
	jwtManager *auth.JWTManager
	logger     *zap.Logger
}

type OrgTagDetail struct {
	TagID       string `json:"tagId"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type UserOrgTags struct {
	OrgTags       []string       `json:"orgTags"`
	PrimaryOrg    string         `json:"primaryOrg"`
	OrgTagDetails []OrgTagDetail `json:"orgTagDetails"`
}

func NewUserService(userRepo repo.UserRepo, tokenRepo repo.TokenRepo, orgTagRepo repo.OrgTagRepo, jwtManager *auth.JWTManager, logger *zap.Logger) *UserService {
	return &UserService{
		userRepo:   userRepo,
		tokenRepo:  tokenRepo,
		orgTagRepo: orgTagRepo,
		jwtManager: jwtManager,
		logger:     logger,
	}
}

func (s *UserService) Register(ctx context.Context, username, password string) (*model.User, error) {
	_, err := s.userRepo.FindByUsername(ctx, username)
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

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	privateOrgTag, err := s.createPrivateOrgTag(ctx, user.ID, username)
	if err != nil {
		return nil, err
	}

	user.OrgTags = &privateOrgTag
	user.PrimaryOrg = &privateOrgTag
	if err := s.userRepo.Update(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (string, string, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrInvalidCredentials
		}
		return "", "", err
	}

	if !auth.CheckPasswordHash(password, user.Password) {
		return "", "", ErrInvalidCredentials
	}

	token, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	return token, refreshToken, nil
}

func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	blacklisted, err := s.tokenRepo.IsBlacklisted(ctx, refreshToken)
	if err != nil {
		return "", "", err
	}
	if blacklisted {
		return "", "", ErrInvalidRefreshToken
	}

	claims, err := s.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		return "", "", ErrInvalidRefreshToken
	}

	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", "", ErrInvalidRefreshToken
		}
		return "", "", err
	}

	newToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		return "", "", err
	}

	return newToken, newRefreshToken, nil
}

func (s *UserService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := s.jwtManager.VerifyAccessToken(accessToken)
	if err != nil {
		return ErrInvalidCredentials
	}

	if err := s.tokenRepo.Blacklist(ctx, accessToken, auth.TokenTTL(claims)); err != nil {
		return err
	}

	if refreshToken == "" {
		return nil
	}

	refreshClaims, err := s.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		return ErrInvalidRefreshToken
	}

	return s.tokenRepo.Blacklist(ctx, refreshToken, auth.TokenTTL(refreshClaims))
}

func (s *UserService) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	return s.tokenRepo.IsBlacklisted(ctx, token)
}

func (s *UserService) Me(ctx context.Context, userID int64) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetUserOrgTags(ctx context.Context, userID int64) (*UserOrgTags, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}

	orgTags := splitOrgTags(user.OrgTags)
	tags, err := s.orgTagRepo.FindBatchByIDs(ctx, orgTags)
	if err != nil {
		return nil, err
	}

	details := make([]OrgTagDetail, 0, len(tags))
	for _, tag := range tags {
		details = append(details, OrgTagDetail{
			TagID:       tag.TagID,
			Name:        tag.Name,
			Description: stringValue(tag.Description),
		})
	}

	return &UserOrgTags{
		OrgTags:       orgTags,
		PrimaryOrg:    stringValue(user.PrimaryOrg),
		OrgTagDetails: details,
	}, nil
}

func (s *UserService) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	return s.userRepo.FindByUsername(ctx, username)
}

func (s *UserService) createPrivateOrgTag(ctx context.Context, userID int64, username string) (string, error) {
	privateOrgTag := "PRIVATE_" + username
	_, err := s.orgTagRepo.FindByID(ctx, privateOrgTag)
	if err == nil {
		return privateOrgTag, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return "", err
	}

	description := "Private organization tag for this user."
	orgTag := &model.OrganizationTag{
		TagID:       privateOrgTag,
		Name:        username + " private space",
		Description: &description,
		CreatedBy:   userID,
	}
	if err := s.orgTagRepo.Create(ctx, orgTag); err != nil {
		return "", err
	}

	return privateOrgTag, nil
}

func splitOrgTags(value *string) []string {
	if value == nil || strings.TrimSpace(*value) == "" {
		return []string{}
	}

	parts := strings.Split(*value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
