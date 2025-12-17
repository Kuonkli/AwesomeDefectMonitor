package clients

import (
	pb "awesome-defect-tracker/shared/protogen/user"
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"strings"
)

type UserClient struct {
	client pb.UserServiceClient
	conn   *grpc.ClientConn
}

func NewUserClient(addr string) (*UserClient, error) {
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

func (c *UserClient) GetUser(ctx context.Context, userID string) (*pb.User, error) {
	req := &pb.GetUserRequest{UserId: userID}
	return c.client.GetUser(ctx, req)
}

func (c *UserClient) GetUsers(ctx context.Context, userIDs []string) (map[string]*pb.User, error) {
	// Для простоты запрашиваем по одному (можно оптимизировать)
	users := make(map[string]*pb.User)

	for _, userID := range userIDs {
		user, err := c.GetUser(ctx, userID)
		if err != nil {
			// Создаем заглушку если пользователь не найден
			users[userID] = &pb.User{
				Id:        userID,
				Email:     "unknown@example.com",
				FirstName: "Unknown",
				LastName:  "User",
			}
			continue
		}
		users[userID] = user
	}

	return users, nil
}

// Вспомогательная функция для получения роли пользователя
func (c *UserClient) GetUserRole(ctx context.Context, userID string) (string, error) {
	user, err := c.GetUser(ctx, userID)
	if err != nil {
		return "", err
	}
	return user.Role, nil
}

func (c *UserClient) IsManager(ctx context.Context, userID string) (bool, error) {
	role, err := c.GetUserRole(ctx, userID)
	if err != nil {
		return false, err
	}
	return strings.ToLower(role) == "manager" || strings.ToLower(role) == "supervisor", nil
}

func (c *UserClient) IsEngineer(ctx context.Context, userID string) (bool, error) {
	role, err := c.GetUserRole(ctx, userID)
	if err != nil {
		return false, err
	}
	return strings.ToLower(role) == "engineer", nil
}
