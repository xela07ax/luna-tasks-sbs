-- 1. Оптимизация получения списка команд пользователя (метод GetUserTeams)
-- В таблице team_members составной PK (team_id, user_id). Поиск по user_id без этого индекса приведет к Full Table Scan.
CREATE INDEX idx_team_members_user_id ON team_members(user_id);

-- 2. Оптимизация метода GetTasks (фильтрация по статусу и команде)
-- Покрывает запрос: WHERE team_id = ? AND status = ?
CREATE INDEX idx_tasks_team_status ON tasks(team_id, status);

-- 3. Оптимизация метода GetTasks (фильтрация по исполнителю)
-- Покрывает запрос: WHERE assignee_id = ?
CREATE INDEX idx_tasks_assignee_id ON tasks(assignee_id);

-- 4. Оптимизация пагинации списка задач (метод GetTasks)
-- Покрывает запрос: WHERE team_id = ? ORDER BY created_at DESC
CREATE INDEX idx_tasks_team_created_at ON tasks(team_id, created_at DESC);

-- 5. Оптимизация аналитики GetTeamStats
-- Покрывает условие: status = 'done' AND updated_at >= ...
CREATE INDEX idx_tasks_status_updated_at ON tasks(status, updated_at);

-- 6. Оптимизация оконной функции GetTopUsersPerTeam
-- Покрывает отсев старых задач до применения оконной функции: WHERE created_at >= NOW() - INTERVAL 1 MONTH
CREATE INDEX idx_tasks_created_at ON tasks(created_at);

-- 7. Оптимизация получения истории задачи (избегаем filesort при ORDER BY changed_at DESC)
-- Индекс по task_id уже создан неявно из-за FOREIGN KEY, но композитный индекс ускорит сортировку
CREATE INDEX idx_task_history_task_id_changed ON task_history(task_id, changed_at DESC);

-- 8. Оптимизация получения комментариев (избегаем filesort при ORDER BY created_at ASC)
CREATE INDEX idx_task_comments_task_id_created ON task_comments(task_id, created_at ASC);