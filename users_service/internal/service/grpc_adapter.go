package service

import (
	"context"

	pb "awesome-defect-tracker/shared/protogen/user"
	"google.golang.org/protobuf/types/known/emptypb"
)

type GrpcUserServiceAdapter struct {
	pb.UnimplementedUserServiceServer
	service UserService
}

func NewGrpcUserServiceAdapter(service UserService) *GrpcUserServiceAdapter {
	return &GrpcUserServiceAdapter{
		service: service,
	}
}

func (a *GrpcUserServiceAdapter) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.AuthResponse, error) {
	return a.service.Register(ctx, req)
}

func (a *GrpcUserServiceAdapter) Login(ctx context.Context, req *pb.LoginRequest) (*pb.AuthResponse, error) {
	return a.service.Login(ctx, req)
}

func (a *GrpcUserServiceAdapter) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.TokenResponse, error) {
	return a.service.RefreshToken(ctx, req)
}

func (a *GrpcUserServiceAdapter) ValidateToken(ctx context.Context, req *pb.ValidateTokenRequest) (*pb.ValidateTokenResponse, error) {
	return a.service.ValidateToken(ctx, req)
}

func (a *GrpcUserServiceAdapter) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.User, error) {
	return a.service.GetUser(ctx, req)
}

func (a *GrpcUserServiceAdapter) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	return a.service.ListUsers(ctx, req)
}

func (a *GrpcUserServiceAdapter) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.User, error) {
	return a.service.UpdateProfile(ctx, req)
}

func (a *GrpcUserServiceAdapter) ChangePassword(ctx context.Context, req *pb.ChangePasswordRequest) (*emptypb.Empty, error) {
	return a.service.ChangePassword(ctx, req)
}

func (a *GrpcUserServiceAdapter) UpdateUserRole(ctx context.Context, req *pb.UpdateUserRoleRequest) (*pb.User, error) {
	return a.service.UpdateUserRole(ctx, req)
}

func (a *GrpcUserServiceAdapter) DeactivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error) {
	return a.service.DeactivateUser(ctx, req)
}

func (a *GrpcUserServiceAdapter) ActivateUser(ctx context.Context, req *pb.UserRequest) (*emptypb.Empty, error) {
	return a.service.ActivateUser(ctx, req)
}
