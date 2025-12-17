package service

import (
	pb "awesome-defect-tracker/shared/protogen/user"
	"context"
	"crypto/rsa"
	"errors"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"

	"awesome-defect-tracker/user-service/internal/models"
	"awesome-defect-tracker/user-service/internal/repository"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserExists         = errors.New("user already exists")
	ErrUserNotFound       = errors.New("user not found")
	ErrInvalidRole        = errors.New("invalid role")
)

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresAt    int64  `json:"expires_at"`
}

type UserService interface {
	// Аутентификация
	Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error)
	Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error)
	RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.TokenResponse, error)
	ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error)

	// Управление пользователями
	GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error)
	ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error)
	UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.User, error)
	ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*emptypb.Empty, error)

	// Административные
	UpdateUserRole(ctx context.Context, req *pb.UpdateUserRoleRequest) (*pb.User, error)
	DeactivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error)
	ActivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error)
}

type userService struct {
	repo       repository.UserRepository
	privateKey *rsa.PrivateKey
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewUserService(repo repository.UserRepository, privateKey *rsa.PrivateKey) UserService {
	return &userService{
		repo:       repo,
		privateKey: privateKey,
		accessTTL:  15 * time.Minute,
		refreshTTL: 7 * 24 * time.Hour,
	}
}

func (s *userService) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	// 1. Проверяем, существует ли пользователь
	exists, err := s.repo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, status.Error(codes.Internal, "database error")
	}
	if exists {
		return nil, status.Error(codes.AlreadyExists, "user already exists")
	}

	// 2. Хэшируем пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	// 3. Создаём пользователя
	user := &models.User{
		Email:     req.Email,
		Password:  string(hashedPassword),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Role:      models.Role(req.Role),
		IsActive:  true,
	}

	if err := s.repo.Create(user); err != nil {
		return nil, status.Error(codes.Internal, "failed to create user")
	}

	// 4. Генерируем токены
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	return &pb.AuthResponse{
		User:   s.userToProto(user),
		Tokens: tokens,
	}, nil
}

func (s *userService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	// 1. Находим пользователя
	user, err := s.repo.FindByEmail(req.Email)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 2. Проверяем активность
	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "account is deactivated")
	}

	// 3. Проверяем пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	// 4. Генерируем токены
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	return &pb.AuthResponse{
		User:   s.userToProto(user),
		Tokens: tokens,
	}, nil
}

func (s *userService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.TokenResponse, error) {
	// 1. Парсим refresh token
	token, err := jwt.Parse(req.RefreshToken, func(token *jwt.Token) (interface{}, error) {
		return &s.privateKey.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, status.Error(codes.Unauthenticated, "invalid refresh token")
	}

	// 2. Извлекаем claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "invalid token claims")
	}

	// 3. Проверяем тип токена
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "refresh" {
		return nil, status.Error(codes.Unauthenticated, "invalid token type")
	}

	// 4. Получаем user_id
	userIDStr, ok := claims["user_id"].(string)
	if !ok || userIDStr == "" {
		return nil, status.Error(codes.Unauthenticated, "invalid user id in token")
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid user id format")
	}

	// 5. Находим пользователя
	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	if !user.IsActive {
		return nil, status.Error(codes.PermissionDenied, "account is deactivated")
	}

	// 6. Генерируем новую пару токенов
	tokens, err := s.generateTokens(user)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate tokens")
	}

	return &pb.TokenResponse{
		Tokens: tokens,
	}, nil
}

func (s *userService) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	// 1. Парсим токен
	token, err := jwt.Parse(req.Token, func(token *jwt.Token) (interface{}, error) {
		return &s.privateKey.PublicKey, nil
	})

	if err != nil || !token.Valid {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	// 2. Извлекаем claims
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	// 3. Проверяем тип токена
	tokenType, ok := claims["type"].(string)
	if !ok || tokenType != "access" {
		return &pb.ValidateTokenResponse{Valid: false}, nil
	}

	// 4. Извлекаем данные пользователя
	userID, _ := claims["user_id"].(string)
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)

	return &pb.ValidateTokenResponse{
		Valid:  true,
		UserId: userID,
		Email:  email,
		Role:   role,
	}, nil
}

func (s *userService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	return s.userToProto(user), nil
}

func (s *userService) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	offset := (int(req.Page) - 1) * int(req.Limit)

	var role models.Role
	if req.Role != "" {
		role = models.Role(req.Role)
	}

	users, err := s.repo.List(role, int(req.Limit), offset)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to list users")
	}

	protoUsers := make([]*pb.User, len(users))
	for i, user := range users {
		protoUsers[i] = s.userToProto(&user)
	}

	return &pb.ListUsersResponse{
		Users: protoUsers,
		Page:  req.Page,
		Limit: req.Limit,
	}, nil
}

func (s *userService) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.User, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// Обновляем поля, если они переданы
	if req.FirstName != "" {
		user.FirstName = req.FirstName
	}
	if req.LastName != "" {
		user.LastName = req.LastName
	}

	if err := s.repo.Update(user); err != nil {
		return nil, status.Error(codes.Internal, "failed to update user")
	}

	return s.userToProto(user), nil
}

func (s *userService) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*emptypb.Empty, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	// Проверяем старый пароль
	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.OldPassword)); err != nil {
		return nil, status.Error(codes.Unauthenticated, "invalid old password")
	}

	// Хэшируем новый пароль
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to hash password")
	}

	user.Password = string(hashedPassword)
	if err := s.repo.Update(user); err != nil {
		return nil, status.Error(codes.Internal, "failed to update password")
	}

	return &emptypb.Empty{}, nil
}

func (s *userService) UpdateUserRole(ctx context.Context, req *pb.UpdateUserRoleRequest) (*pb.User, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	role := models.Role(req.Role)
	if role != models.RoleEngineer && role != models.RoleManager && role != models.RoleSupervisor {
		return nil, status.Error(codes.InvalidArgument, "invalid role")
	}

	user, err := s.repo.FindByID(userID)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}

	user.Role = role
	if err := s.repo.Update(user); err != nil {
		return nil, status.Error(codes.Internal, "failed to update role")
	}

	return s.userToProto(user), nil
}

// ========== ВСПОМОГАТЕЛЬНЫЕ МЕТОДЫ ==========

func (s *userService) generateTokens(user *models.User) (*pb.TokenPair, error) {
	now := time.Now()
	accessExp := now.Add(s.accessTTL)
	refreshExp := now.Add(s.refreshTTL)

	// Access Token
	accessClaims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"email":   user.Email,
		"role":    string(user.Role),
		"exp":     accessExp.Unix(),
		"iat":     now.Unix(),
		"type":    "access",
	}

	accessToken := jwt.NewWithClaims(jwt.SigningMethodRS256, accessClaims)
	accessSigned, err := accessToken.SignedString(s.privateKey)
	if err != nil {
		return nil, err
	}

	// Refresh Token
	refreshClaims := jwt.MapClaims{
		"user_id": user.ID.String(),
		"exp":     refreshExp.Unix(),
		"iat":     now.Unix(),
		"type":    "refresh",
	}

	refreshToken := jwt.NewWithClaims(jwt.SigningMethodRS256, refreshClaims)
	refreshSigned, err := refreshToken.SignedString(s.privateKey)
	if err != nil {
		return nil, err
	}

	return &pb.TokenPair{
		AccessToken:  accessSigned,
		RefreshToken: refreshSigned,
		ExpiresAt:    accessExp.Unix(),
	}, nil
}

func (s *userService) userToProto(user *models.User) *pb.User {
	return &pb.User{
		Id:        user.ID.String(),
		Email:     user.Email,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Role:      string(user.Role),
		IsActive:  user.IsActive,
		CreatedAt: timestamppb.New(user.CreatedAt),
		UpdatedAt: timestamppb.New(user.UpdatedAt),
	}
}

func (s *userService) DeactivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	if err := s.repo.Deactivate(userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to deactivate user")
	}

	return &emptypb.Empty{}, nil
}

func (s *userService) ActivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error) {
	userID, err := uuid.Parse(req.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user id")
	}

	if err := s.repo.Activate(userID); err != nil {
		return nil, status.Error(codes.Internal, "failed to activate user")
	}

	return &emptypb.Empty{}, nil
}
