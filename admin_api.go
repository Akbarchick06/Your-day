package main

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type DepartmentData struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	FacultyID int64  `json:"faculty_id"`
	Faculty   string `json:"faculty"`
}

type GroupData struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	DepartmentID  int64  `json:"department_id"`
	Department    string `json:"department"`
	AdmissionYear int    `json:"admission_year"`
	CourseNumber  int    `json:"course_number"`
}

type SubjectData struct {
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	SubjectCode  string `json:"subject_code"`
	DepartmentID int64  `json:"department_id"`
	Department   string `json:"department"`
}

type UserData struct {
	ID             int64  `json:"id"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Password       string `json:"password"`
	Role           string `json:"role"`
	IsActive       bool   `json:"is_active"`
	GroupID        int64  `json:"group_id"`
	DepartmentID   int64  `json:"department_id"`
	StudentNumber  string `json:"student_number"`
	EmployeeNumber string `json:"employee_number"`
	AdmissionYear  int    `json:"admission_year"`
	Position       string `json:"position"`
}

type AssignmentData struct {
	ID           int64  `json:"id"`
	TeacherID    int64  `json:"teacher_id"`
	SubjectID    int64  `json:"subject_id"`
	GroupID      int64  `json:"group_id"`
	AcademicYear string `json:"academic_year"`
	Semester     int    `json:"semester"`
	Teacher      string `json:"teacher"`
	Subject      string `json:"subject"`
	Group        string `json:"group"`
}

func adminAPI(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	_, ok := getSessionForRole(r, "admin")
	if !ok {
		writeAdminError(w, http.StatusForbidden, "Доступ запрещён")
		return
	}

	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/admin/"), "/")
	parts := strings.Split(path, "/")

	if path == "" {
		writeAdminError(w, http.StatusNotFound, "Раздел не найден")
		return
	}

	if parts[0] == "dashboard" {
		if r.Method != http.MethodGet {
			writeAdminError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
			return
		}
		adminDashboard(w)
		return
	}

	var id int64
	if len(parts) > 1 {
		var err error
		id, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || id <= 0 {
			writeAdminError(w, http.StatusBadRequest, "Некорректный ID")
			return
		}
	}

	switch parts[0] {
	case "faculties":
		adminFaculties(w, r, id)
	case "departments":
		adminDepartments(w, r, id)
	case "groups":
		adminGroups(w, r, id)
	case "curators":
		adminCurators(w, r)
	case "subjects":
		adminSubjects(w, r, id)
	case "users":
		adminUsers(w, r, id)
	case "assignments":
		adminAssignments(w, r, id)
	case "options":
		adminOptions(w, r)
	case "schedule":
		adminSchedule(w, r, id)

	case "periods":
		adminPeriods(w, r, id)
	case "settings":
		adminSettings(w, r)

	default:
		writeAdminError(w, http.StatusNotFound, "Раздел не найден")
	}
}

func writeAdminJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeAdminError(w http.ResponseWriter, status int, message string) {
	writeAdminJSON(w, status, map[string]string{
		"error": message,
	})
}

func readAdminJSON(r *http.Request, data interface{}) error {
	return json.NewDecoder(r.Body).Decode(data)
}

func adminDashboard(w http.ResponseWriter) {
	tables := []string{
		"users",
		"students",
		"teachers",
		"faculties",
		"departments",
		"study_groups",
		"subjects",
	}

	result := make(map[string]int64)

	for _, table := range tables {
		var count int64
		query := "SELECT COUNT(*) FROM " + table

		if err := db.QueryRow(query).Scan(&count); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Не удалось получить статистику")
			return
		}

		result[table] = count
	}

	writeAdminJSON(w, http.StatusOK, result)
}

func adminDepartments(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT d.id, d.name, d.faculty_id, f.name
			FROM departments d
			JOIN faculties f ON f.id = d.faculty_id
			ORDER BY d.name
		`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения кафедр")
			return
		}
		defer rows.Close()

		items := []DepartmentData{}

		for rows.Next() {
			var item DepartmentData
			if err := rows.Scan(&item.ID, &item.Name, &item.FacultyID, &item.Faculty); err != nil {
				writeAdminError(w, 500, "Ошибка чтения кафедр")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost:
		var item DepartmentData
		if readAdminJSON(r, &item) != nil || strings.TrimSpace(item.Name) == "" || item.FacultyID <= 0 {
			writeAdminError(w, 400, "Укажите название и факультет")
			return
		}

		err := db.QueryRow(`
			INSERT INTO departments(name, faculty_id)
			VALUES($1, $2)
			RETURNING id
		`, strings.TrimSpace(item.Name), item.FacultyID).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать кафедру. Проверьте факультет и название")
			return
		}

		writeAdminJSON(w, 201, item)

	case http.MethodPut:
		var item DepartmentData
		if id <= 0 || readAdminJSON(r, &item) != nil || strings.TrimSpace(item.Name) == "" || item.FacultyID <= 0 {
			writeAdminError(w, 400, "Некорректные данные кафедры")
			return
		}

		result, err := db.Exec(`
			UPDATE departments
			SET name=$1, faculty_id=$2
			WHERE id=$3
		`, strings.TrimSpace(item.Name), item.FacultyID, id)

		if err != nil {
			writeAdminError(w, 409, "Не удалось изменить кафедру")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Кафедра не найдена")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Кафедра обновлена"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID кафедры")
			return
		}

		result, err := db.Exec(`DELETE FROM departments WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 409, "Нельзя удалить кафедру: с ней связаны другие записи")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Кафедра не найдена")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Кафедра удалена"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func adminGroups(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT g.id, g.name, g.department_id, d.name,
			       g.admission_year, g.course_number
			FROM study_groups g
			JOIN departments d ON d.id = g.department_id
			ORDER BY g.name
		`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения групп")
			return
		}
		defer rows.Close()

		items := []GroupData{}

		for rows.Next() {
			var item GroupData
			if err := rows.Scan(
				&item.ID, &item.Name, &item.DepartmentID,
				&item.Department, &item.AdmissionYear, &item.CourseNumber,
			); err != nil {
				writeAdminError(w, 500, "Ошибка чтения групп")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost:
		var item GroupData
		if readAdminJSON(r, &item) != nil ||
			strings.TrimSpace(item.Name) == "" ||
			item.DepartmentID <= 0 ||
			item.AdmissionYear < 2000 ||
			item.CourseNumber < 1 || item.CourseNumber > 6 {
			writeAdminError(w, 400, "Проверьте название группы, кафедру, год и курс")
			return
		}

		err := db.QueryRow(`
			INSERT INTO study_groups(name, department_id, admission_year, course_number)
			VALUES($1, $2, $3, $4)
			RETURNING id
		`, strings.TrimSpace(item.Name), item.DepartmentID, item.AdmissionYear, item.CourseNumber).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать группу. Возможно, такое название уже существует")
			return
		}

		writeAdminJSON(w, 201, item)

	case http.MethodPut:
		var item GroupData
		if id <= 0 || readAdminJSON(r, &item) != nil ||
			strings.TrimSpace(item.Name) == "" ||
			item.DepartmentID <= 0 ||
			item.AdmissionYear < 2000 ||
			item.CourseNumber < 1 || item.CourseNumber > 6 {
			writeAdminError(w, 400, "Некорректные данные группы")
			return
		}

		result, err := db.Exec(`
			UPDATE study_groups
			SET name=$1, department_id=$2, admission_year=$3, course_number=$4
			WHERE id=$5
		`, strings.TrimSpace(item.Name), item.DepartmentID, item.AdmissionYear, item.CourseNumber, id)

		if err != nil {
			writeAdminError(w, 409, "Не удалось изменить группу")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Группа не найдена")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Группа обновлена"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID группы")
			return
		}

		result, err := db.Exec(`DELETE FROM study_groups WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 409, "Нельзя удалить группу: к ней привязаны студенты или назначения")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Группа не найдена")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Группа удалена"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func adminSubjects(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT s.id, s.name, COALESCE(s.subject_code, ''),
			       s.department_id, d.name
			FROM subjects s
			JOIN departments d ON d.id = s.department_id
			ORDER BY s.name
		`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения предметов")
			return
		}
		defer rows.Close()

		items := []SubjectData{}

		for rows.Next() {
			var item SubjectData
			if err := rows.Scan(
				&item.ID, &item.Name, &item.SubjectCode,
				&item.DepartmentID, &item.Department,
			); err != nil {
				writeAdminError(w, 500, "Ошибка чтения предметов")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost:
		var item SubjectData
		if readAdminJSON(r, &item) != nil ||
			strings.TrimSpace(item.Name) == "" || item.DepartmentID <= 0 {
			writeAdminError(w, 400, "Укажите название предмета и кафедру")
			return
		}

		err := db.QueryRow(`
			INSERT INTO subjects(name, subject_code, department_id)
			VALUES($1, NULLIF($2, ''), $3)
			RETURNING id
		`, strings.TrimSpace(item.Name), strings.TrimSpace(item.SubjectCode), item.DepartmentID).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать предмет. Возможно, код уже используется")
			return
		}

		writeAdminJSON(w, 201, item)

	case http.MethodPut:
		var item SubjectData
		if id <= 0 || readAdminJSON(r, &item) != nil ||
			strings.TrimSpace(item.Name) == "" || item.DepartmentID <= 0 {
			writeAdminError(w, 400, "Некорректные данные предмета")
			return
		}

		result, err := db.Exec(`
			UPDATE subjects
			SET name=$1, subject_code=NULLIF($2, ''), department_id=$3
			WHERE id=$4
		`, strings.TrimSpace(item.Name), strings.TrimSpace(item.SubjectCode), item.DepartmentID, id)

		if err != nil {
			writeAdminError(w, 409, "Не удалось изменить предмет")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Предмет не найден")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Предмет обновлён"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID предмета")
			return
		}

		result, err := db.Exec(`DELETE FROM subjects WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 409, "Нельзя удалить предмет: он используется в назначениях")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Предмет не найден")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Предмет удалён"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func adminUsers(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT id, full_name, email, role, is_active
			FROM users
			ORDER BY created_at DESC
		`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения пользователей")
			return
		}
		defer rows.Close()

		items := []UserData{}

		for rows.Next() {
			var item UserData
			if err := rows.Scan(
				&item.ID, &item.FullName, &item.Email,
				&item.Role, &item.IsActive,
			); err != nil {
				writeAdminError(w, 500, "Ошибка чтения пользователей")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost:
		var item UserData
		if readAdminJSON(r, &item) != nil {
			writeAdminError(w, 400, "Некорректные данные")
			return
		}

		item.FullName = strings.TrimSpace(item.FullName)
		item.Email = strings.TrimSpace(item.Email)

		if item.FullName == "" || item.Email == "" || len(item.Password) < 8 {
			writeAdminError(w, 400, "Укажите ФИО, email и пароль минимум из 8 символов")
			return
		}

		if item.Role != "student" && item.Role != "teacher" && item.Role != "admin" {
			writeAdminError(w, 400, "Неизвестная роль пользователя")
			return
		}

		if item.Role == "student" && (item.GroupID <= 0 || item.StudentNumber == "" || item.AdmissionYear < 2000) {
			writeAdminError(w, 400, "Для студента укажите группу, номер и год поступления")
			return
		}

		if item.Role == "teacher" && (item.DepartmentID <= 0 || item.EmployeeNumber == "") {
			writeAdminError(w, 400, "Для преподавателя укажите кафедру и табельный номер")
			return
		}

		passwordHash, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
		if err != nil {
			writeAdminError(w, 500, "Не удалось обработать пароль")
			return
		}

		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать операцию")
			return
		}
		defer tx.Rollback()

		err = tx.QueryRow(`
			INSERT INTO users(full_name, email, password_hash, role, is_active)
			VALUES($1, $2, $3, $4, TRUE)
			RETURNING id
		`, item.FullName, item.Email, string(passwordHash), item.Role).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать пользователя. Проверьте email")
			return
		}

		if item.Role == "student" {
			_, err = tx.Exec(`
				INSERT INTO students(user_id, group_id, student_number, admission_year)
				VALUES($1, $2, $3, $4)
			`, item.ID, item.GroupID, item.StudentNumber, item.AdmissionYear)
		}

		if item.Role == "teacher" {
			_, err = tx.Exec(`
				INSERT INTO teachers(user_id, department_id, employee_number, position)
				VALUES($1, $2, $3, $4)
			`, item.ID, item.DepartmentID, item.EmployeeNumber, item.Position)
		}

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать профиль. Проверьте группу, кафедру или номер")
			return
		}

		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить пользователя")
			return
		}

		item.Password = ""
		writeAdminJSON(w, 201, item)

	case http.MethodPut:
		var item UserData
		if id <= 0 || readAdminJSON(r, &item) != nil {
			writeAdminError(w, 400, "Некорректные данные")
			return
		}

		item.FullName = strings.TrimSpace(item.FullName)
		item.Email = strings.TrimSpace(item.Email)

		if item.FullName == "" || item.Email == "" {
			writeAdminError(w, 400, "Укажите ФИО и email")
			return
		}
		if item.Role != "student" && item.Role != "teacher" && item.Role != "admin" {
			writeAdminError(w, 400, "Неизвестная роль пользователя")
			return
		}

		current, ok := getSessionForRole(r, "admin")
		if !ok {
			writeAdminError(w, http.StatusUnauthorized, "Сессия администратора истекла")
			return
		}
		if current.ID == id && !item.IsActive {
			writeAdminError(w, http.StatusConflict, "Нельзя отключить собственную активную учётную запись")
			return
		}
		if item.Password != "" && len(item.Password) < 8 {
			writeAdminError(w, http.StatusBadRequest, "Новый пароль должен содержать минимум 8 символов")
			return
		}
		if item.Role == "student" && (item.GroupID <= 0 || strings.TrimSpace(item.StudentNumber) == "" || item.AdmissionYear < 2000) {
			writeAdminError(w, http.StatusBadRequest, "Проверьте группу, номер студента и год поступления")
			return
		}
		if item.Role == "teacher" && (item.DepartmentID <= 0 || strings.TrimSpace(item.EmployeeNumber) == "") {
			writeAdminError(w, http.StatusBadRequest, "Проверьте кафедру и табельный номер преподавателя")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать обновление пользователя")
			return
		}
		defer tx.Rollback()
		var targetRole string
		if err = tx.QueryRow(`SELECT role FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&targetRole); err == sql.ErrNoRows {
			writeAdminError(w, 404, "Пользователь не найден")
			return
		} else if err != nil {
			writeAdminError(w, 500, "Не удалось проверить пользователя")
			return
		}
		if targetRole == "admin" && !item.IsActive {
			var lockResult interface{}
			if err = tx.QueryRow(`SELECT pg_advisory_xact_lock(847201)`).Scan(&lockResult); err != nil {
				writeAdminError(w, 500, "Не удалось проверить блокировку администратора")
				return
			}
			var admins int
			if err = tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND is_active=TRUE`).Scan(&admins); err != nil {
				writeAdminError(w, 500, "Не удалось проверить администраторов")
				return
			}
			if admins <= 1 {
				writeAdminError(w, 409, "Нельзя отключить последнего активного администратора")
				return
			}
		}
		if item.Password != "" {
			hash, hashErr := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
			if hashErr != nil {
				writeAdminError(w, 500, "Не удалось обработать новый пароль")
				return
			}
			_, err = tx.Exec(`UPDATE users SET full_name=$1,email=$2,is_active=$3,password_hash=$4 WHERE id=$5`, item.FullName, item.Email, item.IsActive, string(hash), id)
		} else {
			_, err = tx.Exec(`UPDATE users SET full_name=$1,email=$2,is_active=$3 WHERE id=$4`, item.FullName, item.Email, item.IsActive, id)
		}
		if err != nil {
			writeAdminError(w, 409, "Не удалось обновить пользователя. Проверьте email")
			return
		}
		if targetRole == "student" {
			_, err = tx.Exec(`UPDATE students SET group_id=$1,student_number=$2,admission_year=$3 WHERE user_id=$4`, item.GroupID, strings.TrimSpace(item.StudentNumber), item.AdmissionYear, id)
		} else if targetRole == "teacher" {
			_, err = tx.Exec(`UPDATE teachers SET department_id=$1,employee_number=$2,position=$3 WHERE user_id=$4`, item.DepartmentID, strings.TrimSpace(item.EmployeeNumber), strings.TrimSpace(item.Position), id)
		}
		if err != nil {
			writeAdminError(w, 409, "Не удалось обновить учебный профиль. Проверьте группу, кафедру и уникальность номера")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить изменения пользователя")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Пользователь обновлён"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID пользователя")
			return
		}
		current, ok := getSessionForRole(r, "admin")
		if !ok {
			writeAdminError(w, http.StatusUnauthorized, "Сессия администратора истекла")
			return
		}
		if current.ID == id {
			writeAdminError(w, http.StatusConflict, "Нельзя удалить аккаунт, под которым выполнен вход")
			return
		}

		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать удаление")
			return
		}
		defer tx.Rollback()

		var targetRole string
		err = tx.QueryRow(`SELECT role FROM users WHERE id=$1 FOR UPDATE`, id).Scan(&targetRole)
		if err == sql.ErrNoRows {
			writeAdminError(w, 404, "Пользователь не найден")
			return
		}
		if err != nil {
			writeAdminError(w, 500, "Не удалось проверить пользователя")
			return
		}
		if targetRole == "admin" {
			var lockResult interface{}
			if err = tx.QueryRow(`SELECT pg_advisory_xact_lock(847201)`).Scan(&lockResult); err != nil {
				writeAdminError(w, 500, "Не удалось проверить блокировку администратора")
				return
			}
			var admins int
			if err := tx.QueryRow(`SELECT COUNT(*) FROM users WHERE role='admin' AND is_active=TRUE`).Scan(&admins); err != nil {
				writeAdminError(w, 500, "Не удалось проверить администраторов")
				return
			}
			if admins <= 1 {
				writeAdminError(w, 409, "Нельзя удалить последнего администратора")
				return
			}
		}
		// Не удаляем учебный профиль физически: оценки, посещаемость,
		// сообщения и история назначений должны сохраниться.
		_, err = tx.Exec(`UPDATE users SET is_active=FALSE WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 500, "Не удалось деактивировать аккаунт")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось завершить удаление")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Аккаунт деактивирован. Учебная история сохранена."})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func adminAssignments(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT a.id, a.teacher_id, a.subject_id, a.group_id,
			       a.academic_year, a.semester,
			       u.full_name, s.name, g.name
			FROM teacher_assignments a
			JOIN teachers t ON t.id = a.teacher_id
			JOIN users u ON u.id = t.user_id
			JOIN subjects s ON s.id = a.subject_id
			JOIN study_groups g ON g.id = a.group_id
			ORDER BY a.academic_year DESC, a.semester
		`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения назначений")
			return
		}
		defer rows.Close()

		items := []AssignmentData{}

		for rows.Next() {
			var item AssignmentData
			if err := rows.Scan(
				&item.ID, &item.TeacherID, &item.SubjectID,
				&item.GroupID, &item.AcademicYear, &item.Semester,
				&item.Teacher, &item.Subject, &item.Group,
			); err != nil {
				writeAdminError(w, 500, "Ошибка чтения назначений")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost:
		var item AssignmentData
		if readAdminJSON(r, &item) != nil ||
			item.TeacherID <= 0 || item.SubjectID <= 0 ||
			item.GroupID <= 0 || strings.TrimSpace(item.AcademicYear) == "" ||
			(item.Semester != 1 && item.Semester != 2) {
			writeAdminError(w, 400, "Проверьте преподавателя, предмет, группу, учебный год и семестр")
			return
		}

		err := db.QueryRow(`
			INSERT INTO teacher_assignments(
				teacher_id, subject_id, group_id, academic_year, semester
			)
			VALUES($1, $2, $3, $4, $5)
			RETURNING id
		`, item.TeacherID, item.SubjectID, item.GroupID,
			strings.TrimSpace(item.AcademicYear), item.Semester).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, 409, "Не удалось создать назначение. Возможно, оно уже существует")
			return
		}
		notifyAssignmentMembers(item.ID, "assignment", "Назначен новый предмет", "Вам назначен новый предмет. Проверьте расписание и материалы.", "/student")
		writeAdminJSON(w, 201, item)

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID назначения")
			return
		}

		result, err := db.Exec(`DELETE FROM teacher_assignments WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 500, "Не удалось удалить назначение")
			return
		}

		if affected, _ := result.RowsAffected(); affected == 0 {
			writeAdminError(w, 404, "Назначение не найдено")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Назначение удалено"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

// ФАКУЛЬТЕТЫ
func adminFaculties(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id, name FROM faculties ORDER BY name`)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения факультетов")
			return
		}
		defer rows.Close()

		items := []Faculty{}
		for rows.Next() {
			var item Faculty
			if err := rows.Scan(&item.ID, &item.Name); err != nil {
				writeAdminError(w, 500, "Ошибка чтения факультетов")
				return
			}
			items = append(items, item)
		}
		writeAdminJSON(w, 200, items)

	case http.MethodPost, http.MethodPut:
		var item Faculty
		if readAdminJSON(r, &item) != nil || strings.TrimSpace(item.Name) == "" {
			writeAdminError(w, 400, "Введите название факультета")
			return
		}

		item.Name = strings.TrimSpace(item.Name)

		if r.Method == http.MethodPost {
			err := db.QueryRow(`
                INSERT INTO faculties(name)
                VALUES($1) RETURNING id
            `, item.Name).Scan(&item.ID)

			if err != nil {
				writeAdminError(w, 409, "Не удалось создать факультет. Возможно, название уже существует")
				return
			}

			writeAdminJSON(w, 201, item)
			return
		}

		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID факультета")
			return
		}

		result, err := db.Exec(
			`UPDATE faculties SET name=$1 WHERE id=$2`,
			item.Name, id,
		)
		if err != nil {
			writeAdminError(w, 409, "Не удалось изменить факультет")
			return
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, 404, "Факультет не найден")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Факультет обновлён"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID факультета")
			return
		}

		result, err := db.Exec(`DELETE FROM faculties WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 409, "Нельзя удалить факультет: к нему привязаны кафедры")
			return
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, 404, "Факультет не найден")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Факультет удалён"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

// ДАННЫЕ ДЛЯ СПИСКОВ В ФОРМАХ
func adminOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeAdminError(w, 405, "Метод не поддерживается")
		return
	}

	result := map[string]interface{}{}

	// Преподаватели
	teacherRows, err := db.Query(`
        SELECT t.id, u.full_name
        FROM teachers t
        JOIN users u ON u.id = t.user_id
        WHERE u.is_active = TRUE
        ORDER BY u.full_name
    `)
	if err != nil {
		writeAdminError(w, 500, "Ошибка получения преподавателей")
		return
	}

	teachers := []map[string]interface{}{}
	for teacherRows.Next() {
		var id int64
		var name string
		if err := teacherRows.Scan(&id, &name); err != nil {
			teacherRows.Close()
			writeAdminError(w, 500, "Ошибка чтения преподавателей")
			return
		}
		teachers = append(teachers, map[string]interface{}{
			"id": id, "name": name,
		})
	}
	teacherRows.Close()
	result["teachers"] = teachers

	// Предметы
	subjectRows, err := db.Query(`
        SELECT id, name FROM subjects ORDER BY name
    `)
	if err != nil {
		writeAdminError(w, 500, "Ошибка получения предметов")
		return
	}

	subjects := []map[string]interface{}{}
	for subjectRows.Next() {
		var id int64
		var name string
		if err := subjectRows.Scan(&id, &name); err != nil {
			subjectRows.Close()
			writeAdminError(w, 500, "Ошибка чтения предметов")
			return
		}
		subjects = append(subjects, map[string]interface{}{
			"id": id, "name": name,
		})
	}
	subjectRows.Close()
	result["subjects"] = subjects

	// Группы
	groupRows, err := db.Query(`
        SELECT id, name FROM study_groups ORDER BY name
    `)
	if err != nil {
		writeAdminError(w, 500, "Ошибка получения групп")
		return
	}

	groups := []map[string]interface{}{}
	for groupRows.Next() {
		var id int64
		var name string
		if err := groupRows.Scan(&id, &name); err != nil {
			groupRows.Close()
			writeAdminError(w, 500, "Ошибка чтения групп")
			return
		}
		groups = append(groups, map[string]interface{}{
			"id": id, "name": name,
		})
	}
	groupRows.Close()
	result["groups"] = groups

	writeAdminJSON(w, 200, result)
}

// РАСПИСАНИЕ
type ScheduleData struct {
	ID           int64  `json:"id"`
	AssignmentID int64  `json:"assignment_id"`
	DayOfWeek    int    `json:"day_of_week"`
	StartTime    string `json:"start_time"`
	EndTime      string `json:"end_time"`
	Room         string `json:"room"`
	LessonType   string `json:"lesson_type"`
	Teacher      string `json:"teacher"`
	Subject      string `json:"subject"`
	Group        string `json:"group"`
}

func adminSchedule(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
            SELECT se.id, se.assignment_id, se.day_of_week,
                   to_char(se.start_time, 'HH24:MI'),
                   to_char(se.end_time, 'HH24:MI'),
                   se.room, se.lesson_type,
                   u.full_name, s.name, g.name
            FROM schedule_entries se
            JOIN teacher_assignments a ON a.id = se.assignment_id
            JOIN teachers t ON t.id = a.teacher_id
            JOIN users u ON u.id = t.user_id
            JOIN subjects s ON s.id = a.subject_id
            JOIN study_groups g ON g.id = a.group_id
            ORDER BY se.day_of_week, se.start_time
        `)
		if err != nil {
			writeAdminError(w, 500, "Ошибка получения расписания")
			return
		}
		defer rows.Close()

		items := []ScheduleData{}
		for rows.Next() {
			var item ScheduleData
			if err := rows.Scan(
				&item.ID, &item.AssignmentID, &item.DayOfWeek,
				&item.StartTime, &item.EndTime, &item.Room,
				&item.LessonType, &item.Teacher, &item.Subject, &item.Group,
			); err != nil {
				writeAdminError(w, 500, "Ошибка чтения расписания")
				return
			}
			items = append(items, item)
		}

		writeAdminJSON(w, 200, items)

	case http.MethodPost, http.MethodPut:
		var item ScheduleData
		if readAdminJSON(r, &item) != nil ||
			item.AssignmentID <= 0 ||
			item.DayOfWeek < 1 || item.DayOfWeek > 7 ||
			item.StartTime == "" || item.EndTime == "" ||
			strings.TrimSpace(item.Room) == "" {
			writeAdminError(w, 400, "Заполните все поля расписания")
			return
		}

		if item.LessonType == "" {
			item.LessonType = "Лекция"
		}
		startTime, startErr := time.Parse("15:04", item.StartTime)
		endTime, endErr := time.Parse("15:04", item.EndTime)
		if startErr != nil || endErr != nil || !endTime.After(startTime) {
			writeAdminError(w, 400, "Время окончания должно быть позже времени начала")
			return
		}
		excludeID := int64(0)
		if r.Method == http.MethodPut {
			excludeID = id
		}
		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать проверку расписания")
			return
		}
		defer tx.Rollback()
		var lockResult interface{}
		if err = tx.QueryRow(`SELECT pg_advisory_xact_lock(847202)`).Scan(&lockResult); err != nil {
			writeAdminError(w, 500, "Не удалось заблокировать проверку расписания")
			return
		}
		var conflict bool
		err = tx.QueryRow(`
			SELECT EXISTS (
				SELECT 1 FROM schedule_entries se
				JOIN teacher_assignments existing ON existing.id=se.assignment_id
				JOIN teacher_assignments incoming ON incoming.id=$1
				WHERE se.day_of_week=$2
				  AND se.start_time < $4::time AND se.end_time > $3::time
				  AND (existing.teacher_id=incoming.teacher_id
				       OR existing.group_id=incoming.group_id
				       OR lower(trim(se.room))=lower(trim($5)))
				  AND ($6=0 OR se.id<>$6)
			)
		`, item.AssignmentID, item.DayOfWeek, item.StartTime, item.EndTime, strings.TrimSpace(item.Room), excludeID).Scan(&conflict)
		if err != nil {
			writeAdminError(w, 500, "Не удалось проверить конфликты расписания")
			return
		}
		if conflict {
			writeAdminError(w, 409, "Конфликт расписания: преподаватель, группа или аудитория уже заняты в это время")
			return
		}

		if r.Method == http.MethodPost {
			err = tx.QueryRow(`
				INSERT INTO schedule_entries
				    (assignment_id, day_of_week, start_time, end_time, room, lesson_type)
				VALUES ($1, $2, $3, $4, $5, $6)
				RETURNING id
			`, item.AssignmentID, item.DayOfWeek, item.StartTime, item.EndTime, strings.TrimSpace(item.Room), item.LessonType).Scan(&item.ID)
			if err != nil {
				writeAdminError(w, 409, "Не удалось добавить занятие. Проверьте время и назначение")
				return
			}
			if err = tx.Commit(); err != nil {
				writeAdminError(w, 500, "Не удалось сохранить занятие")
				return
			}
			notifyAssignmentMembers(item.AssignmentID, "schedule", "Изменено расписание", "Добавлено новое занятие в расписание", "/student")
			writeAdminJSON(w, 201, item)
			return
		}

		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID занятия")
			return
		}
		result, err := tx.Exec(`
			UPDATE schedule_entries
			SET assignment_id=$1, day_of_week=$2, start_time=$3,
			    end_time=$4, room=$5, lesson_type=$6
			WHERE id=$7
		`, item.AssignmentID, item.DayOfWeek, item.StartTime, item.EndTime, strings.TrimSpace(item.Room), item.LessonType, id)
		if err != nil {
			writeAdminError(w, 409, "Не удалось изменить занятие")
			return
		}
		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, 404, "Занятие не найдено")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить расписание")
			return
		}
		notifyAssignmentMembers(item.AssignmentID, "schedule", "Изменено расписание", "Изменено занятие в расписании", "/student")
		writeAdminJSON(w, 200, map[string]string{"message": "Расписание обновлено"})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, 400, "Не указан ID занятия")
			return
		}

		result, err := db.Exec(`DELETE FROM schedule_entries WHERE id=$1`, id)
		if err != nil {
			writeAdminError(w, 500, "Не удалось удалить занятие")
			return
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, 404, "Занятие не найдено")
			return
		}

		writeAdminJSON(w, 200, map[string]string{"message": "Занятие удалено"})

	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

var _ = sql.ErrNoRows

func adminCurators(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT g.id,g.name,COALESCE(t.id,0),COALESCE(u.full_name,'') FROM study_groups g LEFT JOIN group_curators gc ON gc.group_id=g.id LEFT JOIN teachers t ON t.id=gc.teacher_id LEFT JOIN users u ON u.id=t.user_id ORDER BY g.name`)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить кураторов")
			return
		}
		defer rows.Close()
		type row struct {
			GroupID   int64  `json:"group_id"`
			Group     string `json:"group"`
			TeacherID int64  `json:"teacher_id"`
			Teacher   string `json:"teacher"`
		}
		out := []row{}
		for rows.Next() {
			var x row
			if err = rows.Scan(&x.GroupID, &x.Group, &x.TeacherID, &x.Teacher); err != nil {
				writeAdminError(w, 500, "Ошибка чтения кураторов")
				return
			}
			out = append(out, x)
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost:
		var x struct {
			GroupID   int64 `json:"group_id"`
			TeacherID int64 `json:"teacher_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&x); err != nil || x.GroupID < 1 || x.TeacherID < 1 {
			writeAdminError(w, 400, "Выберите группу и преподавателя")
			return
		}
		res, err := db.Exec(`INSERT INTO group_curators(group_id,teacher_id) VALUES($1,$2) ON CONFLICT(group_id) DO UPDATE SET teacher_id=EXCLUDED.teacher_id,assigned_at=CURRENT_TIMESTAMP`, x.GroupID, x.TeacherID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось назначить куратора")
			return
		}
		_ = res
		writeAdminJSON(w, 200, map[string]string{"message": "Куратор назначен"})
	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func adminSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		var value sql.NullFloat64
		if err := db.QueryRow(`SELECT max_absence_percent FROM academic_settings WHERE id=1`).Scan(&value); err != nil {
			writeAdminError(w, 500, "Не удалось загрузить настройки")
			return
		}
		var percent interface{}
		if value.Valid {
			percent = value.Float64
		}
		writeAdminJSON(w, 200, map[string]interface{}{"max_absence_percent": percent})
	case http.MethodPut:
		var req struct {
			MaxAbsencePercent *float64 `json:"max_absence_percent"`
		}
		if err := readAdminJSON(r, &req); err != nil {
			writeAdminError(w, 400, "Некорректные настройки")
			return
		}
		if req.MaxAbsencePercent != nil && (*req.MaxAbsencePercent < 0 || *req.MaxAbsencePercent > 100) {
			writeAdminError(w, 400, "Процент допустимых пропусков должен быть от 0 до 100")
			return
		}
		if _, err := db.Exec(`UPDATE academic_settings SET max_absence_percent=$1,updated_at=CURRENT_TIMESTAMP WHERE id=1`, req.MaxAbsencePercent); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить настройки")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Настройки сохранены"})
	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}
