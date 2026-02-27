package repository

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	testmysql "github.com/testcontainers/testcontainers-go/modules/mysql"
	testredis "github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/xela07ax/luna-tasks-sbs/internal/domain"
)

// =========================================================================
// ХЕЛПЕРЫ ДЛЯ ЛОКАЛЬНОЙ РАЗРАБОТКИ (Docker Compose)
// =========================================================================

func getEnvOrDefault(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func getTestDSN() string {
	cfg := mysql.Config{
		User:                 getEnvOrDefault("TEST_DB_USER", "test_user"),
		Passwd:               getEnvOrDefault("TEST_DB_PASS", "test_pass"),
		Net:                  "tcp",
		Addr:                 getEnvOrDefault("TEST_DB_HOST", "localhost") + ":" + getEnvOrDefault("TEST_DB_PORT", "3306"),
		DBName:               getEnvOrDefault("TEST_DB_NAME", "luna_tasks_test"),
		ParseTime:            true,
		MultiStatements:      true,
		AllowNativePasswords: true,
	}
	return cfg.FormatDSN()
}

// setupLocalTestDB подключается к статичному Docker Compose (Быстро, удобно для дебага)
func setupLocalTestDB(t *testing.T) (*sqlx.DB, *redis.Client, func()) {
	t.Log("🚀 Использование локального окружения (Docker Compose)")
	dsn := getTestDSN()

	db, err := sqlx.Connect("mysql", dsn)
	require.NoError(t, err, "Не удалось подключиться к локальной MySQL. Запущен ли docker-compose.test.yml?")

	redisHost := getEnvOrDefault("TEST_REDIS_HOST", "localhost") + ":" + getEnvOrDefault("TEST_REDIS_PORT", "6379")
	rdb := redis.NewClient(&redis.Options{Addr: redisHost})

	err = rdb.Ping(context.Background()).Err()
	require.NoError(t, err, "Не удалось подключиться к локальному Redis")

	runMigrations(t, db)
	cleanDB(t, db)
	rdb.FlushAll(context.Background())

	cleanup := func() {
		db.Close()
		rdb.Close()
	}
	return db, rdb, cleanup
}

func runMigrations(t *testing.T, db *sqlx.DB) {
	files := []string{
		filepath.Join("..", "..", "migrations", "000001_init.up.sql"),
		filepath.Join("..", "..", "migrations", "000002_add_read_indexes.up.sql"),
	}

	for _, file := range files {
		content, err := os.ReadFile(file)
		require.NoError(t, err, "Не удалось прочитать файл миграции: "+file)

		_, err = db.Exec(string(content))
		if err != nil && !strings.Contains(err.Error(), "already exists") && !strings.Contains(err.Error(), "Duplicate key name") {
			t.Fatalf("Ошибка выполнения миграции %s: %v", file, err)
		}
	}
}

func cleanDB(t *testing.T, db *sqlx.DB) {
	queries := []string{
		"SET FOREIGN_KEY_CHECKS = 0;",
		"TRUNCATE TABLE task_comments;",
		"TRUNCATE TABLE task_history;",
		"TRUNCATE TABLE tasks;",
		"TRUNCATE TABLE team_members;",
		"TRUNCATE TABLE teams;",
		"TRUNCATE TABLE users;",
		"SET FOREIGN_KEY_CHECKS = 1;",
	}
	for _, q := range queries {
		_, err := db.Exec(q)
		require.NoError(t, err)
	}
}

// =========================================================================
// ХЕЛПЕРЫ ДЛЯ CI/CD (Testcontainers - Требование из ТЗ)
// =========================================================================

// setupTestcontainersDB поднимает эфемерные контейнеры (Изолированно, надежно для CI)
func setupTestcontainersDB(t *testing.T) (*sqlx.DB, *redis.Client, func()) {
	t.Log("🐳 Использование Testcontainers (CI Environment)")
	ctx := context.Background()

	mysqlContainer, err := testmysql.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		testmysql.WithDatabase("luna_tasks"),
		testmysql.WithUsername("test_user"),
		testmysql.WithPassword("test_pass"),
		testmysql.WithScripts(
			filepath.Join("..", "..", "migrations", "000001_init.up.sql"),
			filepath.Join("..", "..", "migrations", "000002_add_read_indexes.up.sql"),
		),
	)
	require.NoError(t, err)

	dsn, err := mysqlContainer.ConnectionString(ctx, "parseTime=true&multiStatements=true")
	require.NoError(t, err)

	db, err := sqlx.Connect("mysql", dsn)
	require.NoError(t, err)

	redisContainer, err := testredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
	)
	require.NoError(t, err)

	redisURI, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := redis.ParseURL(redisURI)
	require.NoError(t, err, "Не удалось распарсить Redis URI от Testcontainers")

	rdb := redis.NewClient(opts)

	cleanup := func() {
		db.Close()
		rdb.Close()
		mysqlContainer.Terminate(ctx)
		redisContainer.Terminate(ctx)
	}
	return db, rdb, cleanup
}

func TestRepository_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration tests in short mode")
	}

	var db *sqlx.DB
	var rdb *redis.Client
	var cleanup func()

	// 💡 МАГИЯ ЗДЕСЬ: Переключаем инфраструктуру в зависимости от окружения
	if os.Getenv("CI") != "" {
		db, rdb, cleanup = setupTestcontainersDB(t)
	} else {
		db, rdb, cleanup = setupLocalTestDB(t)
	}
	defer cleanup()

	repo := NewRepository(db, rdb)
	ctx := context.Background()

	// =========================================================================
	// 1. Тест: Создание пользователя
	// =========================================================================
	user := &domain.User{
		Email:        "test@example.com",
		Username:     "testuser",
		PasswordHash: "hash",
	}
	err := repo.CreateUser(ctx, user)
	require.NoError(t, err)
	assert.NotZero(t, user.ID)

	// =========================================================================
	// 2. Тест: Получение пользователя
	// =========================================================================
	fetchedUser, err := repo.GetUserByUsername(ctx, "testuser")
	require.NoError(t, err)
	assert.Equal(t, user.ID, fetchedUser.ID)

	// =========================================================================
	// 3. Тест: Создание команды
	// =========================================================================
	team := &domain.Team{
		Name:      "Test Team",
		CreatedBy: user.ID,
	}
	err = repo.CreateTeam(ctx, team)
	require.NoError(t, err)
	assert.NotZero(t, team.ID)

	// Проверяем, что создатель стал owner'ом
	role, err := repo.GetTeamMemberRole(ctx, team.ID, user.ID)
	require.NoError(t, err)
	assert.Equal(t, "owner", role)

	// =========================================================================
	// 4. Тест: Создание задачи и проверка кеширования (Redis)
	// =========================================================================
	task := &domain.Task{
		TeamID:      team.ID,
		Title:       "Integration Task",
		Description: "Test",
		Status:      "todo",
		CreatedBy:   user.ID,
	}
	err = repo.CreateTask(ctx, task)
	require.NoError(t, err)

	// Запрашиваем задачи (должен быть промах кеша -> запрос в БД -> сохранение в Redis)
	tasks, err := repo.GetTasks(ctx, team.ID, "", nil, 10, 0)
	require.NoError(t, err)
	assert.Len(t, tasks, 1)

	// Проверяем, что данные появились в Redis
	time.Sleep(100 * time.Millisecond) // Даем горутине/Redis время на запись
	cacheKey := "team_tasks:1"         // team.ID = 1
	val, err := rdb.Get(ctx, cacheKey).Result()
	require.NoError(t, err)
	assert.Contains(t, val, "Integration Task")

	// =========================================================================
	// 5. Тест: Обновление задачи и запись в историю (Audit)
	// =========================================================================
	err = repo.UpdateTask(ctx, task.ID, "done", user.ID)
	require.NoError(t, err)

	history, err := repo.GetTaskHistory(ctx, task.ID)
	require.NoError(t, err)
	assert.Len(t, history, 1)
	assert.Equal(t, "todo", history[0].OldStatus)
	assert.Equal(t, "done", history[0].NewStatus)

	// =========================================================================
	// 6. Тест: Сложный SQL (GetTeamStats)
	// =========================================================================
	stats, err := repo.GetTeamStats(ctx)
	require.NoError(t, err)
	assert.Len(t, stats, 1)
	assert.Equal(t, "Test Team", stats[0]["name"])

	// В зависимости от драйвера MySQL, COUNT() может возвращать string или []byte.
	// В репозитории мы принудительно кастим []byte к string для JSON.
	assert.Equal(t, int64(1), stats[0]["member_count"])
	assert.Equal(t, int64(1), stats[0]["done_tasks_count"])
}
