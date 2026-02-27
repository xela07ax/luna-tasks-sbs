package domain

import "time"

type User struct {
	ID           int64     `db:"id" json:"id"`
	Email        string    `db:"email" json:"email"`
	Username     string    `db:"username" json:"username"`
	PasswordHash string    `db:"password_hash" json:"-"`
	CreatedAt    time.Time `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time `db:"updated_at" json:"updated_at"`
}

type Team struct {
	ID        int64     `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	CreatedBy int64     `db:"created_by" json:"created_by"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

type Task struct {
	ID          int64      `db:"id" json:"id"`
	TeamID      int64      `db:"team_id" json:"team_id"`
	Title       string     `db:"title" json:"title"`
	Description string     `db:"description" json:"description"`
	Status      string     `db:"status" json:"status"`
	AssigneeID  *int64     `db:"assignee_id" json:"assignee_id"`
	CreatedBy   int64      `db:"created_by" json:"created_by"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time  `db:"updated_at" json:"updated_at"`
}

type TaskHistory struct {
	ID        int64     `db:"id" json:"id"`
	TaskID    int64     `db:"task_id" json:"task_id"`
	ChangedBy int64     `db:"changed_by" json:"changed_by"`
	OldStatus string    `db:"old_status" json:"old_status"`
	NewStatus string    `db:"new_status" json:"new_status"`
	ChangedAt time.Time `db:"changed_at" json:"changed_at"`
}

type Comment struct {
	ID        int64     `db:"id" json:"id"`
	TaskID    int64     `db:"task_id" json:"task_id"`
	UserID    int64     `db:"user_id" json:"user_id"`
	Content   string    `db:"content" json:"content"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}
