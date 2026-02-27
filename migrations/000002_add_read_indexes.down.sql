DROP INDEX idx_tasks_created_at ON tasks;
DROP INDEX idx_tasks_status_updated_at ON tasks;
DROP INDEX idx_tasks_team_created_at ON tasks;
DROP INDEX idx_tasks_assignee_id ON tasks;
DROP INDEX idx_tasks_team_status ON tasks;
DROP INDEX idx_team_members_user_id ON team_members;
DROP INDEX idx_task_history_task_id_changed ON task_history;
DROP INDEX idx_task_comments_task_id_created ON task_comments;