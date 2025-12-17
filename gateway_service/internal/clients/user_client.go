package clients

import (
	pb "awesome-defect-tracker/shared/protogen/user"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

type UserClient struct {
	client pb.UserServiceClient
	conn   *grpc.ClientConn
}

func NewUserClient(addr string) (*UserClient, error) {
	// Подключаемся к User Service
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}

	client := pb.NewUserServiceClient(conn)

	return &UserClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *UserClient) Close() error {
	return c.conn.Close()
}

// Аутентификация
func (c *UserClient) Register(ctx context.Context, email, password, firstName, lastName, role string) (*pb.AuthResponse, error) {
	req := &pb.RegisterRequest{
		Email:     email,
		Password:  password,
		FirstName: firstName,
		LastName:  lastName,
		Role:      role,
	}

	return c.client.Register(ctx, req)
}

func (c *UserClient) Login(ctx context.Context, email, password string) (*pb.AuthResponse, error) {
	req := &pb.LoginRequest{
		Email:    email,
		Password: password,
	}

	return c.client.Login(ctx, req)
}

func (c *UserClient) RefreshToken(ctx context.Context, refreshToken string) (*pb.TokenResponse, error) {
	req := &pb.RefreshTokenRequest{
		RefreshToken: refreshToken,
	}

	return c.client.RefreshToken(ctx, req)
}

func (c *UserClient) ValidateToken(ctx context.Context, token string) (*pb.ValidateTokenResponse, error) {
	req := &pb.ValidateTokenRequest{
		Token: token,
	}

	return c.client.ValidateToken(ctx, req)
}

// Управление пользователями
func (c *UserClient) GetProfile(ctx context.Context, userID string) (*pb.User, error) {
	req := &pb.GetUserRequest{
		UserId: userID,
	}

	return c.client.GetUser(ctx, req)
}

func (c *UserClient) UpdateProfile(ctx context.Context, userID, firstName, lastName string) (*pb.User, error) {
	req := &pb.UpdateProfileRequest{
		UserId:    userID,
		FirstName: firstName,
		LastName:  lastName,
	}

	return c.client.UpdateProfile(ctx, req)
}

func (c *UserClient) ListUsers(ctx context.Context, role string, page, limit int32) (*pb.ListUsersResponse, error) {
	req := &pb.ListUsersRequest{
		Role:  role,
		Page:  page,
		Limit: limit,
	}

	return c.client.ListUsers(ctx, req)
}

func (c *UserClient) UpdateUserRole(ctx context.Context, userID, role string) (*pb.User, error) {
	return c.client.UpdateUserRole(ctx, &pb.UpdateUserRoleRequest{
		UserId: userID,
		Role:   role,
	})
}

func (c *UserClient) DeactivateUser(ctx context.Context, userID string) (*emptypb.Empty, error) {
	return c.client.DeactivateUser(ctx, &pb.UserRequest{UserId: userID})
}

func (c *UserClient) ActivateUser(ctx context.Context, userID string) (*emptypb.Empty, error) {
	return c.client.ActivateUser(ctx, &pb.UserRequest{UserId: userID})
}
