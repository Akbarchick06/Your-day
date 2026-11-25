package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID   int64
	Role string
}

type LoginData struct {
	Error string
}

type Faculty struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

var db *sql.DB
var sessionKey []byte

var loginAttemptsMu sync.Mutex
var loginAttempts = map[string]struct {
	Count   int
	ResetAt time.Time
}{}

func loginIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func loginAllowed(ip string) bool {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	now := time.Now()
	entry := loginAttempts[ip]
	if now.After(entry.ResetAt) {
		delete(loginAttempts, ip)
		return true
	}
	return entry.Count < 10
}

func recordLoginFailure(ip string) {
	loginAttemptsMu.Lock()
	defer loginAttemptsMu.Unlock()
	now := time.Now()
	entry := loginAttempts[ip]
	if now.After(entry.ResetAt) {
		entry = struct {
			Count   int
			ResetAt time.Time
		}{ResetAt: now.Add(15 * time.Minute)}
	}
	entry.Count++
	loginAttempts[ip] = entry
}

func clearLoginFailures(ip string) {
	loginAttemptsMu.Lock()
	delete(loginAttempts, ip)
	loginAttemptsMu.Unlock()
}

func main() {
	if location, err := time.LoadLocation("Asia/Bishkek"); err == nil {
		time.Local = location
	}
	dbPassword := os.Getenv("DB_PASSWORD")
	sessionSecret := os.Getenv("SESSION_KEY")

	if dbPassword == "" {
		log.Fatal("Не задан DB_PASSWORD")
	}

	if len(sessionSecret) < 32 {
		log.Fatal("SESSION_KEY должен содержать минимум 32 символа")
	}

	sessionKey = []byte(sessionSecret)

	connStr := (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword("postgres", dbPassword),
		Host:     "localhost:5432",
		Path:     "/my_edu",
		RawQuery: "sslmode=disable",
	}).String()

	var err error
	db, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal(err)
	}
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
	defer db.Close()

	if err = db.Ping(); err != nil {
		log.Fatal("Ошибка PostgreSQL:", err)
	}
	if err = ensureAdminSchema(db); err != nil {
		log.Fatal("Ошибка создания таблиц администратора:", err)
	}

	http.HandleFunc("/uploads/", secureUploadHandler)
	http.Handle("/static/", http.StripPrefix(
		"/static/",
		http.FileServer(http.Dir("web/static")),
	))

	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/portfolio/", publicPortfolioHandler)

	http.HandleFunc("/admin", roleHandler("admin", "web/templates/admin.html"))
	http.HandleFunc("/student", roleHandler("student", "web/templates/student.html"))
	http.HandleFunc("/teacher", roleHandler("teacher", "web/templates/teacher.html"))

	http.HandleFunc("/api/faculties", facultiesAPI)
	http.HandleFunc("/api/admin/", adminAPI)
	http.HandleFunc("/api/portal/", portalAPI)

	fmt.Println("YOUR DAY запущен!")
	fmt.Println("Открой: http://localhost:8080")

	server := &http.Server{
		Addr:              ":8080",
		Handler:           http.DefaultServeMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	log.Fatal(server.ListenAndServe())
}

// secureUploadHandler отдаёт учебные материалы только пользователям,
// которым соответствующий материал доступен по назначению.
func secureUploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	name := filepath.Base(strings.TrimPrefix(r.URL.Path, "/uploads/"))
	if name == "." || name == "" || name != strings.TrimPrefix(r.URL.Path, "/uploads/") {
		http.NotFound(w, r)
		return
	}
	var user User
	var ok bool
	for _, role := range []string{"admin", "teacher", "student"} {
		if user, ok = getSessionForRole(r, role); ok {
			break
		}
	}
	if !ok {
		http.Error(w, "Требуется авторизация", http.StatusUnauthorized)
		return
	}
	filePath := "/uploads/" + name
	var allowed bool
	switch user.Role {
	case "admin":
		allowed = true
	case "teacher":
		err := db.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM course_materials cm
			JOIN teacher_assignments a ON a.id = cm.assignment_id
			JOIN teachers t ON t.id = a.teacher_id
			WHERE cm.file_path = $1 AND t.user_id = $2
		)`, filePath, user.ID).Scan(&allowed)
		if err != nil {
			allowed = false
		}
	case "student":
		err := db.QueryRow(`SELECT EXISTS (
			SELECT 1 FROM course_materials cm
			JOIN teacher_assignments a ON a.id = cm.assignment_id
			JOIN students s ON s.group_id = a.group_id
			WHERE cm.file_path = $1 AND s.user_id = $2
		)`, filePath, user.ID).Scan(&allowed)
		if err != nil {
			allowed = false
		}
	}
	if !allowed {
		http.Error(w, "Нет доступа к этому материалу", http.StatusForbidden)
		return
	}
	fileServer := http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads")))
	fileServer.ServeHTTP(w, r)
}

func publicPortfolioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	id, err := strconv.ParseInt(strings.Trim(strings.TrimPrefix(r.URL.Path, "/portfolio/"), "/"), 10, 64)
	if err != nil || id < 1 {
		http.NotFound(w, r)
		return
	}
	var name string
	if err = db.QueryRow(`SELECT full_name FROM users WHERE id=$1 AND role='student' AND is_active=TRUE`, id).Scan(&name); err != nil {
		http.NotFound(w, r)
		return
	}
	rows, err := db.Query(`SELECT title,description,category,skills,resource_url FROM portfolio_items WHERE student_user_id=$1 AND is_public=TRUE ORDER BY created_at DESC`, id)
	if err != nil {
		http.Error(w, "Не удалось загрузить портфолио", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type item struct{ Title, Description, Category, Skills, URL string }
	data := struct {
		Name  string
		Items []item
	}{Name: name, Items: []item{}}
	for rows.Next() {
		var x item
		if err := rows.Scan(&x.Title, &x.Description, &x.Category, &x.Skills, &x.URL); err != nil {
			http.Error(w, "Ошибка чтения портфолио", 500)
			return
		}
		data.Items = append(data.Items, x)
	}
	if err := rows.Err(); err != nil {
		http.Error(w, "Ошибка чтения портфолио", 500)
		return
	}
	const page = `<!doctype html><html lang="ru"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Портфолио — {{.Name}}</title><style>body{margin:0;background:#f3f6fb;color:#18243a;font:16px Arial,sans-serif}.wrap{max-width:900px;margin:48px auto;padding:0 20px}.brand{color:#e87524;font-weight:800;letter-spacing:2px}.card{background:white;border:1px solid #e3e9f2;border-radius:16px;padding:22px;margin:16px 0}.muted{color:#64748b}a{color:#e87524}</style></head><body><main class="wrap"><div class="brand">YOUR DAY · STUDENT PORTFOLIO</div><h1>{{.Name}}</h1><p class="muted">Публичные проекты, сертификаты и достижения</p>{{if .Items}}{{range .Items}}<article class="card"><small class="muted">{{.Category}}</small><h2>{{.Title}}</h2><p>{{.Description}}</p><p class="muted">{{.Skills}}</p>{{if .URL}}<a href="{{.URL}}" target="_blank" rel="noopener">Открыть работу ↗</a>{{end}}</article>{{end}}{{else}}<article class="card">Студент пока не опубликовал работы.</article>{{end}}</main></body></html>`
	tmpl, err := template.New("public-portfolio").Parse(page)
	if err != nil {
		http.Error(w, "Ошибка страницы", 500)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Println("Ошибка публичного портфолио:", err)
	}
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	if r.Method != http.MethodGet {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}

	renderLogin(w, "")
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	// Открытие страницы авторизации
	if r.Method == http.MethodGet {
		renderLogin(w, "")
		return
	}

	// Обработка отправленной формы
	if r.Method != http.MethodPost {
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		renderLogin(w, "Некорректная форма входа")
		return
	}

	ip := loginIP(r)
	if !loginAllowed(ip) {
		renderLogin(w, "Слишком много попыток входа. Попробуйте через 15 минут.")
		return
	}
	email := strings.TrimSpace(r.FormValue("email"))
	password := r.FormValue("password")

	if email == "" || password == "" {
		renderLogin(w, "Введите email и пароль")
		return
	}

	var user User
	var passwordHash string
	var isActive bool

	err := db.QueryRow(`
		SELECT id, password_hash, role, is_active
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&passwordHash,
		&user.Role,
		&isActive,
	)

	if err != nil || !isActive {
		recordLoginFailure(ip)
		renderLogin(w, "Неверный email или пароль")
		return
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(passwordHash),
		[]byte(password),
	)

	if err != nil {
		recordLoginFailure(ip)
		renderLogin(w, "Неверный email или пароль")
		return
	}
	clearLoginFailures(ip)

	cookieValue := createSession(user)

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName(user.Role),
		Value:    cookieValue,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   8 * 60 * 60,
	})

	redirectByRole(w, r, user.Role)
}

func createSession(user User) string {
	expires := time.Now().Add(8 * time.Hour).Unix()

	payload := fmt.Sprintf(
		"%d|%s|%d",
		user.ID,
		user.Role,
		expires,
	)

	encoded := base64.RawURLEncoding.EncodeToString([]byte(payload))

	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(encoded))

	signature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return encoded + "." + signature
}

func sessionCookieName(role string) string {
	return "myedu_session_" + role
}

// getSessionForRole изолирует сессии разных ролей: вход администратора
// не перезаписывает сессию преподавателя или студента в другой вкладке.
func getSessionForRole(r *http.Request, expectedRole string) (User, bool) {
	if expectedRole != "admin" && expectedRole != "teacher" && expectedRole != "student" {
		return User{}, false
	}
	cookie, err := r.Cookie(sessionCookieName(expectedRole))
	if err != nil {
		return User{}, false
	}

	parts := strings.Split(cookie.Value, ".")
	if len(parts) != 2 {
		return User{}, false
	}

	mac := hmac.New(sha256.New, sessionKey)
	mac.Write([]byte(parts[0]))

	expectedSignature := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(parts[1]), []byte(expectedSignature)) {
		return User{}, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return User{}, false
	}

	values := strings.Split(string(payload), "|")
	if len(values) != 3 {
		return User{}, false
	}

	id, err := strconv.ParseInt(values[0], 10, 64)
	if err != nil {
		return User{}, false
	}

	expires, err := strconv.ParseInt(values[2], 10, 64)
	if err != nil || time.Now().Unix() > expires {
		return User{}, false
	}

	role := values[1]
	if role != expectedRole {
		return User{}, false
	}
	if db != nil {
		var active bool
		var currentRole string
		if err := db.QueryRow(`SELECT is_active, role FROM users WHERE id=$1`, id).Scan(&active, &currentRole); err != nil || !active || currentRole != role {
			return User{}, false
		}
	}

	return User{ID: id, Role: role}, true
}

func roleHandler(requiredRole string, templatePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
			return
		}

		user, ok := getSessionForRole(r, requiredRole)
		if !ok {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if user.Role != requiredRole {
			redirectByRole(w, r, user.Role)
			return
		}

		if _, err := os.Stat(templatePath); err != nil {
			http.Error(w, "Файл кабинета пока не установлен: "+templatePath, http.StatusServiceUnavailable)
			return
		}

		tmpl, err := template.ParseFiles(templatePath)
		if err != nil {
			log.Println("Ошибка шаблона:", err)
			http.Error(w, "Ошибка загрузки страницы", http.StatusInternalServerError)
			return
		}

		if err := tmpl.Execute(w, nil); err != nil {
			log.Println("Ошибка отображения:", err)
		}
	}
}

func redirectByRole(w http.ResponseWriter, r *http.Request, role string) {
	switch role {
	case "admin":
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
	case "teacher":
		http.Redirect(w, r, "/teacher", http.StatusSeeOther)
	case "student":
		http.Redirect(w, r, "/student", http.StatusSeeOther)
	default:
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	for _, name := range []string{"myedu_session_admin", "myedu_session_teacher", "myedu_session_student", "myedu_session"} {
		http.SetCookie(w, &http.Cookie{
			Name: name, Value: "", Path: "/", HttpOnly: true,
			SameSite: http.SameSiteLaxMode, MaxAge: -1,
		})
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func renderLogin(w http.ResponseWriter, message string) {
	tmpl, err := template.ParseFiles("web/templates/login.html")
	if err != nil {
		log.Println("Ошибка загрузки входа:", err)
		http.Error(w, "Не удалось загрузить страницу входа", http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, LoginData{Error: message})
	if err != nil {
		log.Println(err)
	}
}

func facultiesAPI(w http.ResponseWriter, r *http.Request) {
	_, ok := getSessionForRole(r, "admin")
	if !ok {
		http.Error(w, "Доступ запрещён", http.StatusForbidden)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	switch r.Method {

	case http.MethodGet:
		rows, err := db.Query(`
			SELECT id, name
			FROM faculties
			ORDER BY name
		`)
		if err != nil {
			http.Error(w, "Ошибка получения факультетов", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		faculties := []Faculty{}

		for rows.Next() {
			var faculty Faculty

			if err := rows.Scan(&faculty.ID, &faculty.Name); err != nil {
				http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
				return
			}

			faculties = append(faculties, faculty)
		}

		if err := rows.Err(); err != nil {
			http.Error(w, "Ошибка чтения данных", http.StatusInternalServerError)
			return
		}

		json.NewEncoder(w).Encode(faculties)

	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Некорректные данные", http.StatusBadRequest)
			return
		}

		name := strings.TrimSpace(r.FormValue("name"))

		if name == "" {
			http.Error(w, "Введите название факультета", http.StatusBadRequest)
			return
		}

		var faculty Faculty

		err := db.QueryRow(`
			INSERT INTO faculties (name)
			VALUES ($1)
			RETURNING id, name
		`, name).Scan(&faculty.ID, &faculty.Name)

		if err != nil {
			http.Error(w, "Не удалось создать факультет. Возможно, такое название уже существует.", http.StatusConflict)
			return
		}

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(faculty)

	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)

		if err != nil || id <= 0 {
			http.Error(w, "Некорректный ID факультета", http.StatusBadRequest)
			return
		}

		result, err := db.Exec(`
			DELETE FROM faculties
			WHERE id = $1
		`, id)

		if err != nil {
			http.Error(w, "Нельзя удалить факультет: возможно, к нему привязаны кафедры", http.StatusConflict)
			return
		}

		count, err := result.RowsAffected()
		if err != nil || count == 0 {
			http.Error(w, "Факультет не найден", http.StatusNotFound)
			return
		}

		json.NewEncoder(w).Encode(map[string]string{
			"message": "Факультет удалён",
		})

	default:
		http.Error(w, "Метод не поддерживается", http.StatusMethodNotAllowed)
	}
}
