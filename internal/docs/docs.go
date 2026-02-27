package docs

type Field struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
}

type Route struct {
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Description string  `json:"description"`
	Auth        bool    `json:"auth_required"`
	Request     []Field `json:"request_payload,omitempty"`
	Response    string  `json:"response_description"`
}

type APIDoc struct {
	Version string  `json:"version"`
	Title   string  `json:"title"`
	Routes  []Route `json:"routes"`
}

func GetAPIDocs() APIDoc {
	return APIDoc{
		Version: "1.0.0",
		Title:   "Luna Tasks API",
		Routes: []Route{
			{
				Method:      "POST",
				Path:        "/api/v1/register",
				Description: "Регистрация нового пользователя",
				Auth:        false,
				Request: []Field{
					{Name: "email", Type: "string", Description: "Email пользователя", Required: true},
					{Name: "username", Type: "string", Description: "Уникальное имя пользователя", Required: true},
					{Name: "password", Type: "string", Description: "Пароль", Required: true},
				},
				Response: "201 Created",
			},
			{
				Method:      "POST",
				Path:        "/api/v1/login",
				Description: "Аутентификация пользователя",
				Auth:        false,
				Request: []Field{
					{Name: "username", Type: "string", Description: "Имя пользователя", Required: true},
					{Name: "password", Type: "string", Description: "Пароль", Required: true},
				},
				Response: `{"access_token": "jwt_string", "refresh_token": "opaque_string", "expires_in": 900}`,
			},
			{
				Method:      "POST",
				Path:        "/api/v1/refresh",
				Description: "Обновление Access Token с помощью Refresh Token",
				Auth:        false,
				Request: []Field{
					{Name: "refresh_token", Type: "string", Description: "Действующий Refresh Token", Required: true},
				},
				Response: `{"access_token": "new_jwt", "refresh_token": "new_opaque_string", "expires_in": 900}`,
			},
			{
				Method:      "POST",
				Path:        "/api/v1/logout",
				Description: "Отзыв Refresh Token (Logout)",
				Auth:        false,
				Request: []Field{
					{Name: "refresh_token", Type: "string", Description: "Refresh Token для отзыва", Required: true},
				},
				Response: "204 No Content",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/docs",
				Description: "Получить документацию API в формате JSON",
				Auth:        false,
				Response:    "JSON с описанием всех роутов",
			},
			{
				Method:      "POST",
				Path:        "/api/v1/teams",
				Description: "Создать новую команду",
				Auth:        true,
				Request: []Field{
					{Name: "name", Type: "string", Description: "Название команды", Required: true},
				},
				Response: "Объект созданной команды",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/teams",
				Description: "Получить список команд пользователя",
				Auth:        true,
				Response:    "Массив команд",
			},
			{
				Method:      "POST",
				Path:        "/api/v1/teams/{id}/invite",
				Description: "Пригласить пользователя в команду (только owner/admin)",
				Auth:        true,
				Request: []Field{
					{Name: "user_id", Type: "int64", Description: "ID приглашаемого пользователя", Required: true},
				},
				Response: "200 OK",
			},
			{
				Method:      "POST",
				Path:        "/api/v1/tasks",
				Description: "Создать задачу в команде",
				Auth:        true,
				Request: []Field{
					{Name: "team_id", Type: "int64", Description: "ID команды", Required: true},
					{Name: "title", Type: "string", Description: "Название задачи", Required: true},
					{Name: "description", Type: "string", Description: "Описание", Required: false},
					{Name: "status", Type: "string", Description: "Статус (todo, in_progress, done)", Required: true},
					{Name: "assignee_id", Type: "int64", Description: "ID исполнителя", Required: false},
				},
				Response: "Объект созданной задачи",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/tasks",
				Description: "Получить список задач (с фильтрацией)",
				Auth:        true,
				Request: []Field{
					{Name: "team_id", Type: "query", Description: "Фильтр по команде", Required: true},
					{Name: "status", Type: "query", Description: "Фильтр по статусу", Required: false},
					{Name: "assignee_id", Type: "query", Description: "Фильтр по исполнителю", Required: false},
					{Name: "limit", Type: "query", Description: "Пагинация: лимит", Required: false},
					{Name: "offset", Type: "query", Description: "Пагинация: отступ", Required: false},
				},
				Response: "Массив задач",
			},
			{
				Method:      "PUT",
				Path:        "/api/v1/tasks/{id}",
				Description: "Обновить статус задачи",
				Auth:        true,
				Request: []Field{
					{Name: "team_id", Type: "int64", Description: "ID команды", Required: true},
					{Name: "status", Type: "string", Description: "Новый статус", Required: true},
				},
				Response: "200 OK",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/tasks/{id}/history",
				Description: "Получить историю изменений задачи",
				Auth:        true,
				Response:    "Массив записей истории",
			},
			{
				Method:      "POST",
				Path:        "/api/v1/tasks/{id}/comments",
				Description: "Добавить комментарий к задаче",
				Auth:        true,
				Request: []Field{
					{Name: "content", Type: "string", Description: "Текст комментария", Required: true},
				},
				Response: "Объект созданного комментария",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/tasks/{id}/comments",
				Description: "Получить комментарии к задаче",
				Auth:        true,
				Response:    "Массив комментариев",
			},
			{
				Method:      "GET",
				Path:        "/api/v1/analytics",
				Description: "Получить аналитику (статистика команд, топ пользователей, невалидные задачи)",
				Auth:        true,
				Response:    "Сложный JSON объект со статистикой",
			},
		},
	}
}
