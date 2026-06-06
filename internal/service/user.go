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
		s.logger.Warn("register user failed: user already exists", zap.String("username", username))
		return nil, ErrUserAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Error("register user failed: find user by username failed",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, err
	}

	hashedPassword, err := auth.HashPassword(password)
	if err != nil {
		s.logger.Error("register user failed: hash password failed",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, err
	}

	user := &model.User{
		Username: username,
		Password: hashedPassword,
		Role:     "USER",
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		s.logger.Error("register user failed: create user failed",
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, err
	}

	privateOrgTag, err := s.createPrivateOrgTag(ctx, user.ID, username)
	if err != nil {
		s.logger.Error("register user failed: create private org tag failed",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, err
	}

	user.OrgTags = &privateOrgTag
	user.PrimaryOrg = &privateOrgTag
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("register user failed: update user org tag failed",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
			zap.Error(err),
		)
		return nil, err
	}

	s.logger.Info("register user success",
		zap.Int64("user_id", user.ID),
		zap.String("username", username),
	)

	return user, nil
}

func (s *UserService) Login(ctx context.Context, username, password string) (string, string, error) {
	user, err := s.userRepo.FindByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("login user failed: user not found", zap.String("username", username))
			return "", "", ErrInvalidCredentials
		}
		s.logger.Error("login user failed: find user by username failed",
			zap.String("username", username),
			zap.Error(err),
		)
		return "", "", err
	}

	if !auth.CheckPasswordHash(password, user.Password) {
		s.logger.Warn("login user failed: invalid password",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
		)
		return "", "", ErrInvalidCredentials
	}

	token, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		s.logger.Error("login user failed: generate access token failed",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
			zap.Error(err),
		)
		return "", "", err
	}

	refreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		s.logger.Error("login user failed: generate refresh token failed",
			zap.Int64("user_id", user.ID),
			zap.String("username", username),
			zap.Error(err),
		)
		return "", "", err
	}

	s.logger.Info("login user success",
		zap.Int64("user_id", user.ID),
		zap.String("username", username),
	)

	return token, refreshToken, nil
}

func (s *UserService) RefreshToken(ctx context.Context, refreshToken string) (string, string, error) {
	blacklisted, err := s.tokenRepo.IsBlacklisted(ctx, refreshToken)
	if err != nil {
		s.logger.Error("refresh token failed: check token blacklist failed", zap.Error(err))
		return "", "", err
	}
	if blacklisted {
		s.logger.Warn("refresh token failed: token is blacklisted")
		return "", "", ErrInvalidRefreshToken
	}

	claims, err := s.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		s.logger.Warn("refresh token failed: invalid refresh token", zap.Error(err))
		return "", "", ErrInvalidRefreshToken
	}

	user, err := s.userRepo.FindByID(ctx, claims.UserID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("refresh token failed: user not found", zap.Int64("user_id", claims.UserID))
			return "", "", ErrInvalidRefreshToken
		}
		s.logger.Error("refresh token failed: find user by id failed",
			zap.Int64("user_id", claims.UserID),
			zap.Error(err),
		)
		return "", "", err
	}

	newToken, err := s.jwtManager.GenerateAccessToken(user.ID, user.Username, user.Role)
	if err != nil {
		s.logger.Error("refresh token failed: generate access token failed",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return "", "", err
	}

	newRefreshToken, err := s.jwtManager.GenerateRefreshToken(user.ID, user.Username, user.Role)
	if err != nil {
		s.logger.Error("refresh token failed: generate refresh token failed",
			zap.Int64("user_id", user.ID),
			zap.Error(err),
		)
		return "", "", err
	}

	s.logger.Info("refresh token success", zap.Int64("user_id", user.ID))

	return newToken, newRefreshToken, nil
}

func (s *UserService) Logout(ctx context.Context, accessToken, refreshToken string) error {
	claims, err := s.jwtManager.VerifyAccessToken(accessToken)
	if err != nil {
		s.logger.Warn("logout user failed: invalid access token", zap.Error(err))
		return ErrInvalidCredentials
	}

	if err := s.tokenRepo.Blacklist(ctx, accessToken, auth.TokenTTL(claims)); err != nil {
		s.logger.Error("logout user failed: blacklist access token failed",
			zap.Int64("user_id", claims.UserID),
			zap.Error(err),
		)
		return err
	}

	if refreshToken == "" {
		s.logger.Info("logout user success", zap.Int64("user_id", claims.UserID))
		return nil
	}

	refreshClaims, err := s.jwtManager.VerifyRefreshToken(refreshToken)
	if err != nil {
		s.logger.Warn("logout user failed: invalid refresh token",
			zap.Int64("user_id", claims.UserID),
			zap.Error(err),
		)
		return ErrInvalidRefreshToken
	}

	if err := s.tokenRepo.Blacklist(ctx, refreshToken, auth.TokenTTL(refreshClaims)); err != nil {
		s.logger.Error("logout user failed: blacklist refresh token failed",
			zap.Int64("user_id", claims.UserID),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("logout user success", zap.Int64("user_id", claims.UserID))

	return nil
}

func (s *UserService) IsTokenBlacklisted(ctx context.Context, token string) (bool, error) {
	blacklisted, err := s.tokenRepo.IsBlacklisted(ctx, token)
	if err != nil {
		s.logger.Error("check token blacklist failed", zap.Error(err))
		return false, err
	}
	return blacklisted, nil
}

func (s *UserService) Me(ctx context.Context, userID int64) (*model.User, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("get current user failed: user not found", zap.Int64("user_id", userID))
			return nil, ErrUserNotFound
		}
		s.logger.Error("get current user failed: find user by id failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetUserOrgTags(ctx context.Context, userID int64) (*UserOrgTags, error) {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("get user org tags failed: user not found", zap.Int64("user_id", userID))
			return nil, ErrUserNotFound
		}
		s.logger.Error("get user org tags failed: find user by id failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return nil, err
	}

	orgTags := splitOrgTags(user.OrgTags)
	tags, err := s.orgTagRepo.FindBatchByIDs(ctx, orgTags)
	if err != nil {
		s.logger.Error("get user org tags failed: find org tags failed",
			zap.Int64("user_id", userID),
			zap.Strings("org_tags", orgTags),
			zap.Error(err),
		)
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

func (s *UserService) SetPrimaryOrg(ctx context.Context, userID int64, primaryOrg string) error {
	user, err := s.userRepo.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			s.logger.Warn("set primary org failed: user not found", zap.Int64("user_id", userID))
			return ErrUserNotFound
		}
		s.logger.Error("set primary org failed: find user by id failed",
			zap.Int64("user_id", userID),
			zap.Error(err),
		)
		return err
	}

	orgTags := splitOrgTags(user.OrgTags)
	if !containsString(orgTags, primaryOrg) {
		s.logger.Warn("set primary org failed: org tag does not belong to user",
			zap.Int64("user_id", userID),
			zap.String("primary_org", primaryOrg),
		)
		return ErrOrgTagNotBelong
	}

	user.PrimaryOrg = &primaryOrg
	if err := s.userRepo.Update(ctx, user); err != nil {
		s.logger.Error("set primary org failed: update user failed",
			zap.Int64("user_id", userID),
			zap.String("primary_org", primaryOrg),
			zap.Error(err),
		)
		return err
	}

	s.logger.Info("set primary org success",
		zap.Int64("user_id", userID),
		zap.String("primary_org", primaryOrg),
	)

	return nil
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

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
