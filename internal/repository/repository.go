package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/xela07ax/luna-tasks-sbs/internal/domain"
)

type Repository struct {
	db    *sqlx.DB
	redis *redis.Client
}

func NewRepository(db *sqlx.DB, rdb *redis.Client) *Repository {
	return &Repository{db: db, redis: rdb}
}

// User methods
func (r *Repository) CreateUser(ctx context.Context, user *domain.User) error {
	query := `INSERT INTO users (email, username, password_hash) VALUES (?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, user.Email, user.Username, user.PasswordHash)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	user.ID = id
	return nil
}

func (r *Repository) GetUserByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user, `SELECT * FROM users WHERE username = ?`, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (r *Repository) GetUserByID(ctx context.Context, id int64) (*domain.User, error) {
	var user domain.User
	err := r.db.GetContext(ctx, &user, `SELECT * FROM users WHERE id = ?`, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

// Refresh Token methods (Redis)
func (r *Repository) StoreRefreshToken(ctx context.Context, token string, userID int64, ttl time.Duration) error {
	return r.redis.Set(ctx, "rt:"+token, userID, ttl).Err()
}

func (r *Repository) GetUserIDByRefreshToken(ctx context.Context, token string) (int64, error) {
	return r.redis.Get(ctx, "rt:"+token).Int64()
}

func (r *Repository) DeleteRefreshToken(ctx context.Context, token string) error {
	return r.redis.Del(ctx, "rt:"+token).Err()
}

// Team methods
func (r *Repository) CreateTeam(ctx context.Context, team *domain.Team) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `INSERT INTO teams (name, created_by) VALUES (?, ?)`, team.Name, team.CreatedBy)
	if err != nil {
		return err
	}
	teamID, _ := res.LastInsertId()
	team.ID = teamID

	_, err = tx.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, 'owner')`, teamID, team.CreatedBy)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (r *Repository) GetUserTeams(ctx context.Context, userID int64) ([]domain.Team, error) {
	var teams []domain.Team
	query := `
		SELECT t.* FROM teams t
		JOIN team_members tm ON t.id = tm.team_id
		WHERE tm.user_id = ?`
	err := r.db.SelectContext(ctx, &teams, query, userID)
	return teams, err
}

func (r *Repository) InviteToTeam(ctx context.Context, teamID, userID int64, role string) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO team_members (team_id, user_id, role) VALUES (?, ?, ?)`, teamID, userID, role)
	return err
}

func (r *Repository) GetTeamMemberRole(ctx context.Context, teamID, userID int64) (string, error) {
	var role string
	err := r.db.GetContext(ctx, &role, `SELECT role FROM team_members WHERE team_id = ? AND user_id = ?`, teamID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return role, err
}

// Task methods
func (r *Repository) CreateTask(ctx context.Context, task *domain.Task) error {
	query := `INSERT INTO tasks (team_id, title, description, status, assignee_id, created_by) VALUES (?, ?, ?, ?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, task.TeamID, task.Title, task.Description, task.Status, task.AssigneeID, task.CreatedBy)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	task.ID = id

	// Invalidate cache
	r.redis.Del(ctx, fmt.Sprintf("team_tasks:%d", task.TeamID))
	return nil
}

func (r *Repository) GetTasks(ctx context.Context, teamID int64, status string, assigneeID *int64, limit, offset int) ([]domain.Task, error) {
	// Try cache first if no filters
	cacheKey := fmt.Sprintf("team_tasks:%d", teamID)
	if status == "" && assigneeID == nil && offset == 0 {
		val, err := r.redis.Get(ctx, cacheKey).Result()
		if err == nil {
			var tasks []domain.Task
			if json.Unmarshal([]byte(val), &tasks) == nil {
				return tasks, nil
			}
		}
	}

	query := `SELECT * FROM tasks WHERE team_id = ?`
	args := []interface{}{teamID}

	if status != "" {
		query += ` AND status = ?`
		args = append(args, status)
	}
	if assigneeID != nil {
		query += ` AND assignee_id = ?`
		args = append(args, *assigneeID)
	}
	query += ` ORDER BY created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, offset)

	var tasks []domain.Task
	err := r.db.SelectContext(ctx, &tasks, query, args...)

	// Кешируем только первую страницу
	if err == nil && status == "" && assigneeID == nil && offset == 0 {
		data, _ := json.Marshal(tasks)
		r.redis.Set(ctx, cacheKey, data, 5*time.Minute)
	}

	return tasks, err
}

func (r *Repository) UpdateTask(ctx context.Context, taskID int64, newStatus string, changedBy int64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var oldStatus string
	var teamID int64
	err = tx.QueryRowContext(ctx, `SELECT status, team_id FROM tasks WHERE id = ? FOR UPDATE`, taskID).Scan(&oldStatus, &teamID)
	if err != nil {
		return err
	}

	if oldStatus == newStatus {
		return nil
	}

	_, err = tx.ExecContext(ctx, `UPDATE tasks SET status = ? WHERE id = ?`, newStatus, taskID)
	if err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx, `INSERT INTO task_history (task_id, changed_by, old_status, new_status) VALUES (?, ?, ?, ?)`, taskID, changedBy, oldStatus, newStatus)
	if err != nil {
		return err
	}

	err = tx.Commit()
	if err == nil {
		r.redis.Del(ctx, fmt.Sprintf("team_tasks:%d", teamID))
	}
	return err
}

func (r *Repository) GetTaskHistory(ctx context.Context, taskID int64) ([]domain.TaskHistory, error) {
	var history []domain.TaskHistory
	err := r.db.SelectContext(ctx, &history, `SELECT * FROM task_history WHERE task_id = ? ORDER BY changed_at DESC`, taskID)
	return history, err
}

func (r *Repository) GetTaskByID(ctx context.Context, taskID int64) (*domain.Task, error) {
	var task domain.Task
	err := r.db.GetContext(ctx, &task, `SELECT * FROM tasks WHERE id = ?`, taskID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &task, nil
}

// Comment methods
func (r *Repository) CreateComment(ctx context.Context, comment *domain.Comment) error {
	query := `INSERT INTO task_comments (task_id, user_id, content) VALUES (?, ?, ?)`
	res, err := r.db.ExecContext(ctx, query, comment.TaskID, comment.UserID, comment.Content)
	if err != nil {
		return err
	}
	id, _ := res.LastInsertId()
	comment.ID = id
	return nil
}

func (r *Repository) GetTaskComments(ctx context.Context, taskID int64) ([]domain.Comment, error) {
	var comments []domain.Comment
	err := r.db.SelectContext(ctx, &comments, `SELECT * FROM task_comments WHERE task_id = ? ORDER BY created_at ASC`, taskID)
	return comments, err
}

// Analytics (Complex SQL)
func (r *Repository) GetTeamStats(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		SELECT 
			t.name,
			COUNT(DISTINCT tm.user_id) as member_count,
			COUNT(DISTINCT CASE WHEN tk.status = 'done' AND tk.updated_at >= NOW() - INTERVAL 7 DAY THEN tk.id END) as done_tasks_count
		FROM teams t
		LEFT JOIN team_members tm ON t.id = tm.team_id
		LEFT JOIN tasks tk ON t.id = tk.team_id
		GROUP BY t.id, t.name
	`
	var results []map[string]interface{}
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		m := make(map[string]interface{})
		if err := rows.MapScan(m); err != nil {
			return nil, err
		}

		// Convert []byte to string for JSON serialization
		for k, v := range m {
			if b, ok := v.([]byte); ok {
				m[k] = string(b)
			}
		}
		results = append(results, m)
	}
	return results, nil
}

func (r *Repository) GetTopUsersPerTeam(ctx context.Context) ([]map[string]interface{}, error) {
	query := `
		WITH RankedUsers AS (
			SELECT 
				team_id,
				created_by as user_id,
				COUNT(id) as task_count,
				ROW_NUMBER() OVER(PARTITION BY team_id ORDER BY COUNT(id) DESC) as rnk
			FROM tasks
			WHERE created_at >= NOW() - INTERVAL 1 MONTH
			GROUP BY team_id, created_by
		)
		SELECT team_id, user_id, task_count
		FROM RankedUsers
		WHERE rnk <= 3
	`
	var results []map[string]interface{}
	rows, err := r.db.QueryxContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		m := make(map[string]interface{})
		if err := rows.MapScan(m); err != nil {
			return nil, err
		}
		for k, v := range m {
			if b, ok := v.([]byte); ok {
				m[k] = string(b)
			}
		}
		results = append(results, m)
	}
	return results, nil
}

func (r *Repository) GetTasksWithExternalAssignee(ctx context.Context) ([]domain.Task, error) {
	query := `
		SELECT t.*
		FROM tasks t
		LEFT JOIN team_members tm ON t.team_id = tm.team_id AND t.assignee_id = tm.user_id
		WHERE t.assignee_id IS NOT NULL AND tm.user_id IS NULL
	`
	var tasks []domain.Task
	err := r.db.SelectContext(ctx, &tasks, query)
	return tasks, err
}
