package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"time"

	"github.com/xela07ax/luna-tasks-sbs/internal/domain"
	"github.com/xela07ax/luna-tasks-sbs/internal/infra"
	"golang.org/x/crypto/bcrypt"
)

// Repository описывает контракт для работы с базой данных (позволяет мокать в тестах)
type Repository interface {
	CreateUser(ctx context.Context, user *domain.User) error
	GetUserByUsername(ctx context.Context, username string) (*domain.User, error)
	GetUserByID(ctx context.Context, id int64) (*domain.User, error)
	StoreRefreshToken(ctx context.Context, token string, userID int64, ttl time.Duration) error
	GetUserIDByRefreshToken(ctx context.Context, token string) (int64, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	CreateTeam(ctx context.Context, team *domain.Team) error
	GetUserTeams(ctx context.Context, userID int64) ([]domain.Team, error)
	InviteToTeam(ctx context.Context, teamID, userID int64, role string) error
	GetTeamMemberRole(ctx context.Context, teamID, userID int64) (string, error)
	CreateTask(ctx context.Context, task *domain.Task) error
	GetTasks(ctx context.Context, teamID int64, status string, assigneeID *int64, limit, offset int) ([]domain.Task, error)
	UpdateTask(ctx context.Context, taskID int64, newStatus string, changedBy int64) error
	GetTaskHistory(ctx context.Context, taskID int64) ([]domain.TaskHistory, error)
	GetTaskByID(ctx context.Context, taskID int64) (*domain.Task, error)
	CreateComment(ctx context.Context, comment *domain.Comment) error
	GetTaskComments(ctx context.Context, taskID int64) ([]domain.Comment, error)
	GetTeamStats(ctx context.Context) ([]map[string]interface{}, error)
	GetTopUsersPerTeam(ctx context.Context) ([]map[string]interface{}, error)
	GetTasksWithExternalAssignee(ctx context.Context) ([]domain.Task, error)
}

// TokenGenerator описывает контракт для генерации JWT (позволяет мокать в тестах)
type TokenGenerator interface {
	GenerateToken(userID int64, username string) (string, error)
}

type Service struct {
	repo       Repository
	jwt        TokenGenerator
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewService(repo Repository, jwt TokenGenerator, accessTTL, refreshTTL time.Duration) *Service {
	return &Service{
		repo:       repo,
		jwt:        jwt,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func generateSecureToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

func (s *Service) issueTokens(ctx context.Context, userID int64, username string) (*domain.TokenPair, error) {
	accessToken, err := s.jwt.GenerateToken(userID, username)
	if err != nil {
		return nil, err
	}

	refreshToken := generateSecureToken()
	err = s.repo.StoreRefreshToken(ctx, refreshToken, userID, s.refreshTTL)
	if err != nil {
		return nil, err
	}

	return &domain.TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.accessTTL.Seconds()),
	}, nil
}

func (s *Service) Register(ctx context.Context, email, username, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	user := &domain.User{
		Email:        email,
		Username:     username,
		PasswordHash: string(hash),
	}
	return s.repo.CreateUser(ctx, user)
}

func (s *Service) Login(ctx context.Context, username, password string) (*domain.TokenPair, error) {
	user, err := s.repo.GetUserByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, errors.New("invalid credentials")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, errors.New("invalid credentials")
	}
	return s.issueTokens(ctx, user.ID, user.Username)
}

func (s *Service) RefreshTokens(ctx context.Context, refreshToken string) (*domain.TokenPair, error) {
	userID, err := s.repo.GetUserIDByRefreshToken(ctx, refreshToken)
	if err != nil {
		return nil, errors.New("invalid or expired refresh token")
	}

	user, err := s.repo.GetUserByID(ctx, userID)
	if err != nil || user == nil {
		return nil, errors.New("user not found")
	}

	// Token Rotation: удаляем старый рефреш токен
	_ = s.repo.DeleteRefreshToken(ctx, refreshToken)

	return s.issueTokens(ctx, user.ID, user.Username)
}

func (s *Service) Logout(ctx context.Context, refreshToken string) error {
	return s.repo.DeleteRefreshToken(ctx, refreshToken)
}

func (s *Service) CreateTeam(ctx context.Context, name string, userID int64) (*domain.Team, error) {
	team := &domain.Team{
		Name:      name,
		CreatedBy: userID,
	}
	err := s.repo.CreateTeam(ctx, team)
	return team, err
}

func (s *Service) GetUserTeams(ctx context.Context, userID int64) ([]domain.Team, error) {
	return s.repo.GetUserTeams(ctx, userID)
}

func (s *Service) InviteToTeam(ctx context.Context, teamID, inviterID, inviteeID int64) error {
	role, err := s.repo.GetTeamMemberRole(ctx, teamID, inviterID)
	if err != nil {
		return err
	}
	if role != "owner" && role != "admin" {
		return errors.New("insufficient permissions")
	}
	// Использование Circuit Breaker для вызова внешнего сервиса (Mock Email)
	err = infra.EmailCircuitBreaker.Execute(func() error {
		// Mock sending email
		return nil
	})
	if err != nil {
		return errors.New("failed to send invite email")
	}

	return s.repo.InviteToTeam(ctx, teamID, inviteeID, "member")
}

func (s *Service) CreateTask(ctx context.Context, task *domain.Task, userID int64) error {
	role, err := s.repo.GetTeamMemberRole(ctx, task.TeamID, userID)
	if err != nil || role == "" {
		return errors.New("user is not a member of the team")
	}
	task.CreatedBy = userID
	return s.repo.CreateTask(ctx, task)
}

func (s *Service) GetTasks(ctx context.Context, teamID int64, status string, assigneeID *int64, limit, offset int) ([]domain.Task, error) {
	return s.repo.GetTasks(ctx, teamID, status, assigneeID, limit, offset)
}

func (s *Service) UpdateTask(ctx context.Context, taskID int64, teamID int64, newStatus string, userID int64) error {
	role, err := s.repo.GetTeamMemberRole(ctx, teamID, userID)
	if err != nil || role == "" {
		return errors.New("user is not a member of the team")
	}
	return s.repo.UpdateTask(ctx, taskID, newStatus, userID)
}

func (s *Service) GetTaskHistory(ctx context.Context, taskID int64) ([]domain.TaskHistory, error) {
	return s.repo.GetTaskHistory(ctx, taskID)
}

func (s *Service) AddComment(ctx context.Context, taskID, userID int64, content string) (*domain.Comment, error) {
	// 1. Проверяем, существует ли задача
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("task not found")
	}

	// 2. Проверяем, состоит ли пользователь в команде этой задачи
	role, err := s.repo.GetTeamMemberRole(ctx, task.TeamID, userID)
	if err != nil || role == "" {
		return nil, errors.New("user is not a member of the team")
	}

	// 3. Создаем комментарий
	comment := &domain.Comment{
		TaskID:  taskID,
		UserID:  userID,
		Content: content,
	}
	err = s.repo.CreateComment(ctx, comment)
	return comment, err
}

func (s *Service) GetTaskComments(ctx context.Context, taskID, userID int64) ([]domain.Comment, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("task not found")
	}

	role, err := s.repo.GetTeamMemberRole(ctx, task.TeamID, userID)
	if err != nil || role == "" {
		return nil, errors.New("user is not a member of the team")
	}

	return s.repo.GetTaskComments(ctx, taskID)
}

func (s *Service) GetAnalytics(ctx context.Context) (map[string]interface{}, error) {
	teamStats, err := s.repo.GetTeamStats(ctx)
	if err != nil {
		return nil, err
	}
	topUsers, err := s.repo.GetTopUsersPerTeam(ctx)
	if err != nil {
		return nil, err
	}
	invalidTasks, err := s.repo.GetTasksWithExternalAssignee(ctx)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"team_stats":    teamStats,
		"top_users":     topUsers,
		"invalid_tasks": invalidTasks,
	}, nil
}
