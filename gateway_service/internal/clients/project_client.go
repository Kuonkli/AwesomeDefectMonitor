// internal/clients/project_client.go
package clients

import (
	pb "awesome-defect-tracker/shared/protogen/project"
	"context"
	"errors"
	"fmt"
	"google.golang.org/protobuf/types/known/timestamppb"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ProjectClient struct {
	client pb.ProjectServiceClient
	conn   *grpc.ClientConn
}

func NewProjectClient(addr string) (*ProjectClient, error) {
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to project service: %w", err)
	}

	client := pb.NewProjectServiceClient(conn)
	return &ProjectClient{
		client: client,
		conn:   conn,
	}, nil
}

func (c *ProjectClient) Close() error {
	return c.conn.Close()
}

// Проекты
func (c *ProjectClient) CreateProject(ctx context.Context, name, description, managerID string, engineerIDs []string, startDate, endDate *time.Time) (*pb.Project, error) {
	req := &pb.CreateProjectRequest{
		Name:        name,
		Description: description,
		ManagerId:   managerID,
		EngineerIds: engineerIDs,
	}

	if startDate != nil {
		req.StartDate = timestamppb.New(*startDate)
	}
	if endDate != nil {
		req.EndDate = timestamppb.New(*endDate)
	}

	return c.client.CreateProject(ctx, req)
}

func (c *ProjectClient) GetProject(ctx context.Context, projectID string) (*pb.Project, error) {
	return c.client.GetProject(ctx, &pb.GetProjectRequest{
		ProjectId: projectID,
	})
}

func (c *ProjectClient) UpdateProject(ctx context.Context, req *pb.UpdateProjectRequest) (*pb.Project, error) {
	return c.client.UpdateProject(ctx, req)
}

func (c *ProjectClient) DeleteProject(ctx context.Context, projectID string) error {
	_, err := c.client.DeleteProject(ctx, &pb.GetProjectRequest{
		ProjectId: projectID,
	})
	return err
}

func (c *ProjectClient) ListProjects(ctx context.Context, managerID, engineerID, search string, status pb.ProjectStatus, page, limit int32) (*pb.ListProjectsResponse, error) {
	return c.client.ListProjects(ctx, &pb.ListProjectsRequest{
		ManagerId:  managerID,
		EngineerId: engineerID,
		Search:     search,
		Status:     status,
		Page:       page,
		Limit:      limit,
	})
}

func (c *ProjectClient) AddEngineers(ctx context.Context, projectID string, engineerIDs []string) (*pb.Project, error) {
	return c.client.AddEngineers(ctx, &pb.AddEngineersRequest{
		ProjectId:   projectID,
		EngineerIds: engineerIDs,
	})
}

func (c *ProjectClient) RemoveEngineer(ctx context.Context, projectID, engineerID string) (*pb.Project, error) {
	return c.client.RemoveEngineer(ctx, &pb.RemoveEngineerRequest{
		ProjectId:  projectID,
		EngineerId: engineerID,
	})
}

func (c *ProjectClient) ValidateAccess(ctx context.Context, projectID, userID string) (*pb.ValidateAccessResponse, error) {
	return c.client.ValidateAccess(ctx, &pb.ValidateAccessRequest{
		ProjectId: projectID,
		UserId:    userID,
	})
}

// Дефекты
func (c *ProjectClient) CreateDefect(ctx context.Context, req *pb.CreateDefectRequest) (*pb.Defect, error) {
	return c.client.CreateDefect(ctx, req)
}

func (c *ProjectClient) GetDefect(ctx context.Context, defectID string) (*pb.Defect, error) {
	return c.client.GetDefect(ctx, &pb.GetDefectRequest{
		DefectId: defectID,
	})
}

func (c *ProjectClient) UpdateDefect(ctx context.Context, req *pb.UpdateDefectRequest) (*pb.Defect, error) {
	return c.client.UpdateDefect(ctx, req)
}

func (c *ProjectClient) DeleteDefect(ctx context.Context, defectID string) error {
	_, err := c.client.DeleteDefect(ctx, &pb.GetDefectRequest{
		DefectId: defectID,
	})
	return err
}

func (c *ProjectClient) ListDefects(ctx context.Context, filter *pb.DefectFilter, page, limit int32) (*pb.ListDefectsResponse, error) {
	return c.client.ListDefects(ctx, &pb.ListDefectsRequest{
		Filter: filter,
		Pagination: &pb.PaginationRequest{
			Page:  page,
			Limit: limit,
		},
	})
}

func (c *ProjectClient) UpdateDefectStatus(ctx context.Context, defectID string, status pb.DefectStatus) (*pb.Defect, error) {
	return c.client.UpdateDefectStatus(ctx, &pb.UpdateDefectStatusRequest{
		DefectId: defectID,
		Status:   status,
	})
}

func (c *ProjectClient) UpdateDefectPriority(ctx context.Context, defectID string, priority pb.DefectPriority) (*pb.Defect, error) {
	return c.client.UpdateDefectPriority(ctx, &pb.UpdateDefectPriorityRequest{
		DefectId: defectID,
		Priority: priority,
	})
}

func (c *ProjectClient) AssignEngineer(ctx context.Context, defectID, engineerID string) (*pb.Defect, error) {
	return c.client.AssignEngineer(ctx, &pb.AssignEngineerRequest{
		DefectId:   defectID,
		EngineerId: engineerID,
	})
}

// Комментарии
func (c *ProjectClient) AddComment(ctx context.Context, defectID, text, userID string) (*pb.Comment, error) {
	return c.client.AddComment(ctx, &pb.AddCommentRequest{
		DefectId: defectID,
		Text:     text,
		UserId:   userID,
	})
}

func (c *ProjectClient) UpdateComment(ctx context.Context, commentID, text string) (*pb.Comment, error) {
	return c.client.UpdateComment(ctx, &pb.UpdateCommentRequest{
		CommentId: commentID,
		Text:      text,
	})
}

func (c *ProjectClient) DeleteComment(ctx context.Context, commentID string) error {
	_, err := c.client.DeleteComment(ctx, &pb.DeleteCommentRequest{
		CommentId: commentID,
	})
	return err
}

func (c *ProjectClient) GetComments(ctx context.Context, defectID string, page, limit int32) (*pb.GetCommentsResponse, error) {
	return c.client.GetComments(ctx, &pb.GetCommentsRequest{
		DefectId: defectID,
		Page:     page,
		Limit:    limit,
	})
}

// Вспомогательные методы
func (c *ProjectClient) HealthCheck(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	_, err := c.client.ListProjects(ctx, &pb.ListProjectsRequest{
		Limit: 1,
		Page:  1,
	})

	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("project service timeout")
		}
	}

	return nil
}

// Вспомогательные функции для создания Optional полей
func NewOptionalString(value string) *pb.OptionalString {
	return &pb.OptionalString{Value: value}
}

func NewOptionalDefectStatus(value pb.DefectStatus) *pb.OptionalDefectStatus {
	return &pb.OptionalDefectStatus{Value: value}
}

func NewOptionalDefectPriority(value pb.DefectPriority) *pb.OptionalDefectPriority {
	return &pb.OptionalDefectPriority{Value: value}
}
