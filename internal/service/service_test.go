package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xela07ax/luna-tasks-sbs/internal/domain"
	"golang.org/x/crypto/bcrypt"
)

// MockRepository - мок для слоя БД
type MockRepository struct {
	mock.Mock
}

func (m *MockRepository) CreateUser(ctx context.Context, user *domain.User) error {
	args := m.Called(ctx, user)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	user.ID = 1 // Имитируем присвоение ID базой данных
	return nil
}

func (m *MockRepository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockRepository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.User), args.Error(1)
}

func (m *MockRepository) StoreRefreshToken(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	args := m.Called(ctx, token, userID, ttl)
	return args.Error(0)
}

func (m *MockRepository) GetUserIDByRefreshToken(ctx context.Context, token string) (int64, error) {
	args := m.Called(ctx, token)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockRepository) DeleteRefreshToken(ctx context.Context, token string) error {
	args := m.Called(ctx, token)
	return args.Error(0)
}

func (m *MockRepository) CreateTeam(ctx context.Context, team *domain.Team) error {
	args := m.Called(ctx, team)
	return args.Error(0)
}

func (m *MockRepository) GetUserTeams(ctx context.Context, userID int64) ([]domain.Team, error) {
	args := m.Called(ctx, userID)
	return args.Get(0).([]domain.Team), args.Error(1)
}

func (m *MockRepository) InviteToTeam(ctx context.Context, teamID, userID int64, role string) error {
	args := m.Called(ctx, teamID, userID, role)
	return args.Error(0)
}

func (m *MockRepository) GetTeamMemberRole(ctx context.Context, teamID, userID int64) (string, error) {
	args := m.Called(ctx, teamID, userID)
	return args.String(0), args.Error(1)
}

func (m *MockRepository) CreateTask(ctx context.Context, task *domain.Task) error {
	args := m.Called(ctx, task)
	return args.Error(0)
}

func (m *MockRepository) GetTasks(ctx context.Context, teamID int64, status string, assigneeID *int64, limit, offset int) ([]domain.Task, error) {
	args := m.Called(ctx, teamID, status, assigneeID, limit, offset)
	return args.Get(0).([]domain.Task), args.Error(1)
}

func (m *MockRepository) UpdateTask(ctx context.Context, taskID int64, newStatus string, changedBy int64) error {
	args := m.Called(ctx, taskID, newStatus, changedBy)
	return args.Error(0)
}

func (m *MockRepository) GetTaskHistory(ctx context.Context, taskID int64) ([]domain.TaskHistory, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).([]domain.TaskHistory), args.Error(1)
}

func (m *MockRepository) GetTaskByID(ctx context.Context, taskID int64) (*domain.Task, error) {
	args := m.Called(ctx, taskID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domain.Task), args.Error(1)
}

func (m *MockRepository) CreateComment(ctx context.Context, comment *domain.Comment) error {
	args := m.Called(ctx, comment)
	if args.Get(0) != nil {
		return args.Error(0)
	}
	comment.ID = 1
	return nil
}

func (m *MockRepository) GetTaskComments(ctx context.Context, taskID int64) ([]domain.Comment, error) {
	args := m.Called(ctx, taskID)
	return args.Get(0).([]domain.Comment), args.Error(1)
}

func (m *MockRepository) GetTeamStats(ctx context.Context) ([]map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockRepository) GetTopUsersPerTeam(ctx context.Context) ([]map[string]interface{}, error) {
	args := m.Called(ctx)
	return args.Get(0).([]map[string]interface{}), args.Error(1)
}

func (m *MockRepository) GetTasksWithExternalAssignee(ctx context.Context) ([]domain.Task, error) {
	args := m.Called(ctx)
	return args.Get(0).([]domain.Task), args.Error(1)
}

// MockTokenGenerator - мок для JWT
type MockTokenGenerator struct {
	mock.Mock
}

func (m *MockTokenGenerator) GenerateToken(userID int64, username string) (string, error) {
	args := m.Called(userID, username)
	return args.String(0), args.Error(1)
}

// =========================================================================
// UNIT TESTS (BUSINESS LOGIC)
// =========================================================================

func TestService_Register(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

	mockRepo.On("CreateUser", mock.Anything, mock.AnythingOfType("*domain.User")).Return(nil)

	err := svc.Register(context.Background(), "test@test.com", "testuser", "password123")

	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
}

func TestService_Login_Success(t *testing.T) {
	mockRepo := new(MockRepository)
	mockJWT := new(MockTokenGenerator)
	svc := NewService(mockRepo, mockJWT, time.Minute*15, time.Hour*24)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &domain.User{ID: 1, Username: "testuser", PasswordHash: string(hash)}

	mockRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, nil)
	mockJWT.On("GenerateToken", int64(1), "testuser").Return("mock.jwt.token", nil)
	mockRepo.On("StoreRefreshToken", mock.Anything, mock.AnythingOfType("string"), int64(1), time.Hour*24).Return(nil)

	tokens, err := svc.Login(context.Background(), "testuser", "password123")

	assert.NoError(t, err)
	assert.NotNil(t, tokens)
	assert.Equal(t, "mock.jwt.token", tokens.AccessToken)
	assert.NotEmpty(t, tokens.RefreshToken)
	mockRepo.AssertExpectations(t)
	mockJWT.AssertExpectations(t)
}

func TestService_Login_InvalidPassword(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

	hash, _ := bcrypt.GenerateFromPassword([]byte("password123"), bcrypt.DefaultCost)
	user := &domain.User{ID: 1, Username: "testuser", PasswordHash: string(hash)}

	mockRepo.On("GetUserByUsername", mock.Anything, "testuser").Return(user, nil)

	tokens, err := svc.Login(context.Background(), "testuser", "wrongpassword")

	assert.Error(t, err)
	assert.Equal(t, "invalid credentials", err.Error())
	assert.Nil(t, tokens)
}

// -------------------------------------------------------------------------
// БИЗНЕС-ПРАВИЛА: Управление командами (Ролевая модель)
// -------------------------------------------------------------------------

func TestService_InviteToTeam_RoleMatrix(t *testing.T) {
	tests := []struct {
		name        string
		inviterRole string
		roleErr     error
		expectedErr string
	}{
		{"Owner can invite", "owner", nil, ""},
		{"Admin can invite", "admin", nil, ""},
		{"Member cannot invite", "member", nil, "insufficient permissions"},
		{"Non-member cannot invite", "", nil, "insufficient permissions"},
		{"DB Error on role check", "", errors.New("db error"), "db error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

			teamID, inviterID, inviteeID := int64(1), int64(100), int64(200)

			// Мокаем проверку роли
			mockRepo.On("GetTeamMemberRole", mock.Anything, teamID, inviterID).Return(tt.inviterRole, tt.roleErr)

			// Если ожидается успех, мокаем само приглашение
			if tt.expectedErr == "" {
				mockRepo.On("InviteToTeam", mock.Anything, teamID, inviteeID, "member").Return(nil)
			}

			err := svc.InviteToTeam(context.Background(), teamID, inviterID, inviteeID)

			if tt.expectedErr == "" {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}

// -------------------------------------------------------------------------
// БИЗНЕС-ПРАВИЛА: Управление задачами
// -------------------------------------------------------------------------

func TestService_CreateTask_Permissions(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

	task := &domain.Task{TeamID: 1, Title: "New Task"}

	// Успешный кейс: пользователь состоит в команде
	mockRepo.On("GetTeamMemberRole", mock.Anything, int64(1), int64(100)).Return("member", nil).Once()
	mockRepo.On("CreateTask", mock.Anything, task).Return(nil).Once()

	err := svc.CreateTask(context.Background(), task, 100)
	assert.NoError(t, err)
	assert.Equal(t, int64(100), task.CreatedBy)

	// Негативный кейс: пользователь НЕ состоит в команде
	mockRepo.On("GetTeamMemberRole", mock.Anything, int64(1), int64(200)).Return("", nil).Once()

	err = svc.CreateTask(context.Background(), task, 200)
	assert.Error(t, err)
	assert.Equal(t, "user is not a member of the team", err.Error())

	mockRepo.AssertExpectations(t)
}

func TestService_UpdateTask_Permissions(t *testing.T) {
	mockRepo := new(MockRepository)
	svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

	teamID, taskID, userID := int64(1), int64(10), int64(100)

	// Успешный кейс: пользователь в команде
	mockRepo.On("GetTeamMemberRole", mock.Anything, teamID, userID).Return("admin", nil).Once()
	mockRepo.On("UpdateTask", mock.Anything, taskID, "in_progress", userID).Return(nil).Once()

	err := svc.UpdateTask(context.Background(), taskID, teamID, "in_progress", userID)
	assert.NoError(t, err)

	// Негативный кейс: пользователь не в команде
	mockRepo.On("GetTeamMemberRole", mock.Anything, teamID, int64(999)).Return("", nil).Once()

	err = svc.UpdateTask(context.Background(), taskID, teamID, "done", int64(999))
	assert.Error(t, err)
	assert.Equal(t, "user is not a member of the team", err.Error())

	mockRepo.AssertExpectations(t)
}

// -------------------------------------------------------------------------
// БИЗНЕС-ПРАВИЛА: Комментарии к задачам
// -------------------------------------------------------------------------

func TestService_AddComment_BusinessRules(t *testing.T) {
	tests := []struct {
		name        string
		taskExists  bool
		userRole    string
		expectedErr string
	}{
		{"Success: Member can comment", true, "member", ""},
		{"Success: Owner can comment", true, "owner", ""},
		{"Fail: Non-member cannot comment", true, "", "user is not a member of the team"},
		{"Fail: Task does not exist", false, "", "task not found"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := new(MockRepository)
			svc := NewService(mockRepo, nil, time.Minute*15, time.Hour*24)

			taskID, teamID, userID := int64(10), int64(1), int64(100)
			content := "Looks good to me!"

			// 1. Мокаем получение задачи
			if tt.taskExists {
				mockRepo.On("GetTaskByID", mock.Anything, taskID).Return(&domain.Task{ID: taskID, TeamID: teamID}, nil)

				// 2. Мокаем проверку роли (только если задача существует)
				mockRepo.On("GetTeamMemberRole", mock.Anything, teamID, userID).Return(tt.userRole, nil)

				// 3. Мокаем создание коммента (только если роль позволяет)
				if tt.expectedErr == "" {
					mockRepo.On("CreateComment", mock.Anything, mock.AnythingOfType("*domain.Comment")).Return(nil)
				}
			} else {
				mockRepo.On("GetTaskByID", mock.Anything, taskID).Return(nil, nil)
			}

			comment, err := svc.AddComment(context.Background(), taskID, userID, content)

			if tt.expectedErr == "" {
				assert.NoError(t, err)
				assert.NotNil(t, comment)
				assert.Equal(t, content, comment.Content)
				assert.Equal(t, taskID, comment.TaskID)
				assert.Equal(t, userID, comment.UserID)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, comment)
			}
			mockRepo.AssertExpectations(t)
		})
	}
}
