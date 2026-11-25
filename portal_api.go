package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type PortalProfile struct {
	ID             int64  `json:"id"`
	FullName       string `json:"full_name"`
	Email          string `json:"email"`
	Role           string `json:"role"`
	GroupID        int64  `json:"group_id,omitempty"`
	GroupName      string `json:"group_name,omitempty"`
	StudentNumber  string `json:"student_number,omitempty"`
	AdmissionYear  int    `json:"admission_year,omitempty"`
	Department     string `json:"department,omitempty"`
	Faculty        string `json:"faculty,omitempty"`
	Position       string `json:"position,omitempty"`
	EmployeeNumber string `json:"employee_number,omitempty"`
}

type PortalSubject struct {
	AssignmentID int64  `json:"assignment_id"`
	SubjectID    int64  `json:"subject_id"`
	Subject      string `json:"subject"`
	SubjectCode  string `json:"subject_code,omitempty"`
	Teacher      string `json:"teacher"`
	EmployeeNo   string `json:"teacher_employee_number"`
	GroupID      int64  `json:"group_id"`
	Group        string `json:"group"`
	AcademicYear string `json:"academic_year"`
	Semester     int    `json:"semester"`
}

type PortalSchedule struct {
	ID         int64  `json:"id"`
	DayOfWeek  int    `json:"day_of_week"`
	StartTime  string `json:"start_time"`
	EndTime    string `json:"end_time"`
	Room       string `json:"room"`
	LessonType string `json:"lesson_type"`
	Subject    string `json:"subject"`
	Teacher    string `json:"teacher"`
	EmployeeNo string `json:"teacher_employee_number"`
	Group      string `json:"group"`
}

type PortalStudent struct {
	ID            int64  `json:"id"`
	FullName      string `json:"full_name"`
	StudentNumber string `json:"student_number"`
}

func portalAPI(w http.ResponseWriter, r *http.Request) {
	path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/portal/"), "/")
	parts := strings.Split(path, "/")
	if len(parts) != 2 || (parts[0] != "student" && parts[0] != "teacher") {
		writeAdminError(w, http.StatusNotFound, "Раздел API не найден")
		return
	}
	user, ok := getSessionForRole(r, parts[0])
	if !ok {
		writeAdminError(w, http.StatusUnauthorized, "Требуется авторизация")
		return
	}
	if parts[0] != user.Role {
		writeAdminError(w, http.StatusForbidden, "Нет доступа к этому разделу")
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		writeAdminError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
		return
	}

	switch user.Role + "/" + parts[1] {
	case "student/profile":
		portalStudentProfile(w, user.ID)
	case "student/subjects":
		portalStudentSubjects(w, user.ID)
	case "student/schedule":
		portalStudentSchedule(w, user.ID)
	case "student/group":
		portalStudentGroup(w, user.ID)
	case "student/materials":
		portalStudentMaterials(w, user.ID)
	case "student/grades":
		portalStudentGrades(w, user.ID)
	case "student/attendance":
		portalStudentAttendance(w, user.ID)
	case "student/tasks":
		portalStudentTasks(w, r, user.ID)
	case "student/study-plan":
		portalStudentStudyPlan(w, r, user.ID)
	case "student/portfolio":
		portalStudentPortfolio(w, r, user.ID)
	case "student/topics":
		portalStudentTopics(w, user.ID)
	case "student/curator":
		portalStudentCurator(w, user.ID)
	case "student/messages", "teacher/messages":
		portalMessaging(w, r, user)
	case "student/ai":
		portalStudentAI(w, r, user.ID)
	case "student/notifications", "teacher/notifications":
		portalNotifications(w, r, user.ID)
	case "teacher/profile":
		portalTeacherProfile(w, r, user.ID)
	case "teacher/assignments":
		portalTeacherAssignments(w, user.ID)
	case "teacher/schedule":
		portalTeacherSchedule(w, user.ID)
	case "teacher/journal":
		portalTeacherJournal(w, r, user.ID)
	case "teacher/module":
		portalTeacherModule(w, r, user.ID)
	case "teacher/materials":
		portalTeacherMaterials(w, r, user.ID)
	case "teacher/topics":
		portalTeacherTopics(w, r, user.ID)
	case "teacher/curators":
		writeAdminError(w, http.StatusForbidden, "Только администратор может назначать кураторов")
	default:
		writeAdminError(w, http.StatusNotFound, "Раздел API не найден")
	}
}

func portalStudentProfile(w http.ResponseWriter, userID int64) {
	var p PortalProfile
	p.Role = "student"
	err := db.QueryRow(`
		SELECT u.id, u.full_name, u.email, s.group_id, g.name,
		       s.student_number, s.admission_year, d.name, f.name
		FROM users u
		JOIN students s ON s.user_id = u.id
		JOIN study_groups g ON g.id = s.group_id
		JOIN departments d ON d.id = g.department_id
		JOIN faculties f ON f.id = d.faculty_id
		WHERE u.id = $1 AND u.is_active = TRUE
	`, userID).Scan(&p.ID, &p.FullName, &p.Email, &p.GroupID, &p.GroupName,
		&p.StudentNumber, &p.AdmissionYear, &p.Department, &p.Faculty)
	if err != nil {
		writePortalDBError(w, err, "Не удалось загрузить профиль студента")
		return
	}
	writeAdminJSON(w, http.StatusOK, p)
}

func portalStudentSubjects(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`
		SELECT a.id, s.id, s.name, COALESCE(s.subject_code, ''),
		       u.full_name, t.employee_number, g.id, g.name,
		       a.academic_year, a.semester
		FROM students st
		JOIN teacher_assignments a ON a.group_id = st.group_id
		JOIN subjects s ON s.id = a.subject_id
		JOIN teachers t ON t.id = a.teacher_id
		JOIN users u ON u.id = t.user_id AND u.is_active = TRUE
		JOIN study_groups g ON g.id = a.group_id
		WHERE st.user_id = $1
		ORDER BY a.academic_year DESC, a.semester, s.name
	`, userID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Не удалось загрузить предметы")
		return
	}
	defer rows.Close()
	items := []PortalSubject{}
	for rows.Next() {
		var x PortalSubject
		if err := rows.Scan(&x.AssignmentID, &x.SubjectID, &x.Subject, &x.SubjectCode,
			&x.Teacher, &x.EmployeeNo, &x.GroupID, &x.Group, &x.AcademicYear, &x.Semester); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения предметов")
			return
		}
		items = append(items, x)
	}
	if err := rows.Err(); err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения предметов")
		return
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func portalStudentSchedule(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`
		SELECT se.id, se.day_of_week, se.start_time::text, se.end_time::text,
		       se.room, se.lesson_type, s.name, u.full_name, t.employee_number, g.name
		FROM students st
		JOIN teacher_assignments a ON a.group_id = st.group_id
		JOIN schedule_entries se ON se.assignment_id = a.id
		JOIN subjects s ON s.id = a.subject_id
		JOIN teachers t ON t.id = a.teacher_id
		JOIN users u ON u.id = t.user_id AND u.is_active = TRUE
		JOIN study_groups g ON g.id = a.group_id
		WHERE st.user_id = $1
		ORDER BY se.day_of_week, se.start_time
	`, userID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Не удалось загрузить расписание")
		return
	}
	defer rows.Close()
	items := []PortalSchedule{}
	for rows.Next() {
		var x PortalSchedule
		if err := rows.Scan(&x.ID, &x.DayOfWeek, &x.StartTime, &x.EndTime, &x.Room,
			&x.LessonType, &x.Subject, &x.Teacher, &x.EmployeeNo, &x.Group); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения расписания")
			return
		}
		items = append(items, x)
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func portalStudentGroup(w http.ResponseWriter, userID int64) {
	var groupID int64
	err := db.QueryRow(`SELECT group_id FROM students WHERE user_id=$1`, userID).Scan(&groupID)
	if err != nil {
		writePortalDBError(w, err, "Студент не привязан к учебной группе")
		return
	}
	rows, err := db.Query(`
		SELECT st.user_id, u.full_name, st.student_number
		FROM students st JOIN users u ON u.id=st.user_id
		WHERE st.group_id=$1 AND u.is_active=TRUE
		ORDER BY u.full_name
	`, groupID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Не удалось загрузить список группы")
		return
	}
	defer rows.Close()
	items := []PortalStudent{}
	for rows.Next() {
		var x PortalStudent
		if err := rows.Scan(&x.ID, &x.FullName, &x.StudentNumber); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения группы")
			return
		}
		items = append(items, x)
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func portalTeacherProfile(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method == http.MethodPut {
		var req struct {
			FullName string `json:"full_name"`
			Email    string `json:"email"`
			Position string `json:"position"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректные данные профиля")
			return
		}
		req.FullName = strings.TrimSpace(req.FullName)
		req.Email = strings.ToLower(strings.TrimSpace(req.Email))
		req.Position = strings.TrimSpace(req.Position)
		if req.FullName == "" || len(req.FullName) > 150 || !strings.Contains(req.Email, "@") || len(req.Email) > 150 || len(req.Position) > 100 {
			writeAdminError(w, 400, "Проверьте ФИО, email и должность")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать обновление профиля")
			return
		}
		defer tx.Rollback()
		if _, err = tx.Exec(`UPDATE users SET full_name=$1,email=$2 WHERE id=$3 AND role='teacher'`, req.FullName, req.Email, userID); err != nil {
			writeAdminError(w, 409, "Не удалось обновить ФИО или email. Проверьте уникальность email")
			return
		}
		if _, err = tx.Exec(`UPDATE teachers SET position=$1 WHERE user_id=$2`, req.Position, userID); err != nil {
			writeAdminError(w, 500, "Не удалось обновить должность")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить профиль")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Профиль обновлён"})
		return
	}
	if r.Method != http.MethodGet {
		writeAdminError(w, 405, "Метод не поддерживается")
		return
	}
	var p PortalProfile
	p.Role = "teacher"
	err := db.QueryRow(`
		SELECT u.id, u.full_name, u.email, t.employee_number,
		       COALESCE(t.position,''), d.name, f.name
		FROM users u
		JOIN teachers t ON t.user_id=u.id
		JOIN departments d ON d.id=t.department_id
		JOIN faculties f ON f.id=d.faculty_id
		WHERE u.id=$1 AND u.is_active=TRUE
	`, userID).Scan(&p.ID, &p.FullName, &p.Email, &p.EmployeeNumber,
		&p.Position, &p.Department, &p.Faculty)
	if err != nil {
		writePortalDBError(w, err, "Не удалось загрузить профиль преподавателя")
		return
	}
	writeAdminJSON(w, http.StatusOK, p)
}

func portalTeacherAssignments(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`
		SELECT a.id, s.id, s.name, COALESCE(s.subject_code,''),
		       u.full_name, t.employee_number, g.id, g.name,
		       a.academic_year, a.semester
		FROM teachers t
		JOIN teacher_assignments a ON a.teacher_id=t.id
		JOIN subjects s ON s.id=a.subject_id
		JOIN users u ON u.id=t.user_id
		JOIN study_groups g ON g.id=a.group_id
		WHERE t.user_id=$1
		ORDER BY a.academic_year DESC, a.semester, s.name, g.name
	`, userID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Не удалось загрузить назначения")
		return
	}
	defer rows.Close()
	items := []PortalSubject{}
	for rows.Next() {
		var x PortalSubject
		if err := rows.Scan(&x.AssignmentID, &x.SubjectID, &x.Subject, &x.SubjectCode,
			&x.Teacher, &x.EmployeeNo, &x.GroupID, &x.Group, &x.AcademicYear, &x.Semester); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения назначений")
			return
		}
		items = append(items, x)
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func portalTeacherSchedule(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`
		SELECT se.id, se.day_of_week, se.start_time::text, se.end_time::text,
		       se.room, se.lesson_type, s.name, u.full_name, t.employee_number, g.name
		FROM teachers t
		JOIN teacher_assignments a ON a.teacher_id=t.id
		JOIN schedule_entries se ON se.assignment_id=a.id
		JOIN subjects s ON s.id=a.subject_id
		JOIN users u ON u.id=t.user_id
		JOIN study_groups g ON g.id=a.group_id
		WHERE t.user_id=$1
		ORDER BY se.day_of_week, se.start_time
	`, userID)
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, "Не удалось загрузить расписание")
		return
	}
	defer rows.Close()
	items := []PortalSchedule{}
	for rows.Next() {
		var x PortalSchedule
		if err := rows.Scan(&x.ID, &x.DayOfWeek, &x.StartTime, &x.EndTime, &x.Room,
			&x.LessonType, &x.Subject, &x.Teacher, &x.EmployeeNo, &x.Group); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения расписания")
			return
		}
		items = append(items, x)
	}
	writeAdminJSON(w, http.StatusOK, items)
}

func writePortalDBError(w http.ResponseWriter, err error, message string) {
	if err == sql.ErrNoRows {
		writeAdminError(w, http.StatusNotFound, message)
		return
	}
	writeAdminError(w, http.StatusInternalServerError, message)
}

// Journal API: GET loads actual students and saved marks; POST persists them.
type journalRow struct {
	StudentID     int64   `json:"student_id"`
	FullName      string  `json:"full_name"`
	StudentNumber string  `json:"student_number"`
	Attendance    string  `json:"attendance"`
	Points        float64 `json:"points"`
	Comment       string  `json:"comment"`
	Lectures      float64 `json:"lectures"`
	Labs          float64 `json:"labs"`
	OtherWork     float64 `json:"other_work"`
	ExtraPoints   float64 `json:"extra_points"`
}

func portalTeacherJournal(w http.ResponseWriter, r *http.Request, userID int64) {
	aid, _ := strconv.ParseInt(r.URL.Query().Get("assignment_id"), 10, 64)
	if aid < 1 {
		writeAdminError(w, 400, "Не выбрано назначение")
		return
	}
	var owns bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teacher_assignments a JOIN teachers t ON t.id=a.teacher_id WHERE a.id=$1 AND t.user_id=$2)`, aid, userID).Scan(&owns); err != nil || !owns {
		writeAdminError(w, 403, "Нет доступа к этому назначению")
		return
	}
	if r.Method == http.MethodPost {
		var req struct {
			Date       string       `json:"date"`
			LessonType string       `json:"lesson_type"`
			Rows       []journalRow `json:"rows"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректные данные журнала")
			return
		}
		if _, err := time.Parse("2006-01-02", req.Date); err != nil {
			writeAdminError(w, 400, "Укажите корректную дату занятия")
			return
		}
		allowedLessonTypes := map[string]bool{"Лекция": true, "Лабораторная": true, "Практика": true}
		if !allowedLessonTypes[req.LessonType] || len(req.Rows) > 1000 {
			writeAdminError(w, 400, "Проверьте тип занятия и количество студентов")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось начать сохранение")
			return
		}
		defer tx.Rollback()
		for _, x := range req.Rows {
			if x.StudentID < 1 || x.Points < 0 || x.Points > 25 {
				writeAdminError(w, 400, "Проверьте студента и баллы (0–25)")
				return
			}
			var inGroup bool
			if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM students st JOIN teacher_assignments a ON a.group_id=st.group_id WHERE st.user_id=$1 AND a.id=$2)`, x.StudentID, aid).Scan(&inGroup); err != nil || !inGroup {
				writeAdminError(w, 403, "Студент не относится к назначенной группе")
				return
			}
			if x.Attendance == "" {
				x.Attendance = "present"
			}
			if x.Attendance != "present" && x.Attendance != "absent" && x.Attendance != "excused" && x.Attendance != "sick" {
				writeAdminError(w, 400, "Неверная отметка посещаемости")
				return
			}
			if x.Attendance != "present" {
				x.Points = 0
			}
			_, err = tx.Exec(`INSERT INTO lesson_records(assignment_id,student_id,lesson_date,lesson_type,attendance,points,comment) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(assignment_id,student_id,lesson_date) DO UPDATE SET lesson_type=EXCLUDED.lesson_type,attendance=EXCLUDED.attendance,points=EXCLUDED.points,comment=EXCLUDED.comment,updated_at=CURRENT_TIMESTAMP`, aid, x.StudentID, req.Date, req.LessonType, x.Attendance, x.Points, x.Comment)
			if err != nil {
				writeAdminError(w, 500, "Не удалось сохранить занятие")
				return
			}
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось завершить сохранение")
			return
		}
		notifyAssignmentMembers(aid, "journal", "Обновлён электронный журнал", "Преподаватель внёс изменения в посещаемость или баллы", "/student")
		writeAdminJSON(w, 200, map[string]string{"message": "Занятие сохранено"})
		return
	}
	date := r.URL.Query().Get("date")
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	rows, err := db.Query(`SELECT st.user_id,u.full_name,st.student_number,COALESCE(l.attendance,'present'),COALESCE(l.points,0),COALESCE(l.comment,''),COALESCE(m.lectures,0),COALESCE(m.labs,0),COALESCE(m.other_work,0),COALESCE(m.extra_points,0) FROM teacher_assignments a JOIN students st ON st.group_id=a.group_id JOIN users u ON u.id=st.user_id AND u.is_active=TRUE LEFT JOIN lesson_records l ON l.assignment_id=a.id AND l.student_id=st.user_id AND l.lesson_date=$2 LEFT JOIN module_scores m ON m.assignment_id=a.id AND m.student_id=st.user_id AND m.module_number=$3 WHERE a.id=$1 ORDER BY u.full_name`, aid, date, moduleNumber(r.URL.Query().Get("module")))
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить журнал")
		return
	}
	defer rows.Close()
	out := []journalRow{}
	for rows.Next() {
		var x journalRow
		if err = rows.Scan(&x.StudentID, &x.FullName, &x.StudentNumber, &x.Attendance, &x.Points, &x.Comment, &x.Lectures, &x.Labs, &x.OtherWork, &x.ExtraPoints); err != nil {
			writeAdminError(w, 500, "Ошибка чтения журнала")
			return
		}
		out = append(out, x)
	}
	writeAdminJSON(w, 200, out)
}
func moduleNumber(v string) int {
	n, _ := strconv.Atoi(v)
	if n != 2 {
		return 1
	}
	return 2
}

func portalTeacherMaterials(w http.ResponseWriter, r *http.Request, userID int64) {
	aid, _ := strconv.ParseInt(r.URL.Query().Get("assignment_id"), 10, 64)
	if aid < 1 {
		writeAdminError(w, 400, "Не выбрано назначение")
		return
	}
	var owns bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teacher_assignments a JOIN teachers t ON t.id=a.teacher_id WHERE a.id=$1 AND t.user_id=$2)`, aid, userID).Scan(&owns); err != nil || !owns {
		writeAdminError(w, 403, "Нет доступа к этому предмету")
		return
	}
	if r.Method == http.MethodPost {
		r.Body = http.MaxBytesReader(w, r.Body, 25<<20)
		if err := r.ParseMultipartForm(25 << 20); err != nil {
			writeAdminError(w, 400, "Файл слишком большой или форма повреждена")
			return
		}
		f, h, err := r.FormFile("file")
		if err != nil {
			writeAdminError(w, 400, "Выберите файл")
			return
		}
		defer f.Close()
		ext := strings.ToLower(filepath.Ext(h.Filename))
		allowed := map[string]bool{".pdf": true, ".doc": true, ".docx": true, ".ppt": true, ".pptx": true, ".xls": true, ".xlsx": true, ".txt": true, ".zip": true}
		if !allowed[ext] {
			writeAdminError(w, 400, "Формат файла не поддерживается")
			return
		}
		if err = os.MkdirAll("uploads", 0755); err != nil {
			writeAdminError(w, 500, "Не удалось создать папку загрузок")
			return
		}
		name := fmt.Sprintf("%d_%d%s", aid, time.Now().UnixNano(), ext)
		dst := filepath.Join("uploads", name)
		out, err := os.Create(dst)
		if err != nil {
			writeAdminError(w, 500, "Не удалось сохранить файл")
			return
		}
		_, copyErr := io.Copy(out, f)
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			os.Remove(dst)
			writeAdminError(w, 500, "Ошибка записи файла")
			return
		}
		title := strings.TrimSpace(r.FormValue("title"))
		if title == "" {
			title = h.Filename
		}
		typ := r.FormValue("type")
		if typ == "" {
			typ = "lecture"
		}
		_, err = db.Exec(`INSERT INTO course_materials(assignment_id,title,material_type,file_name,file_path) VALUES($1,$2,$3,$4,$5)`, aid, title, typ, h.Filename, "/uploads/"+name)
		if err != nil {
			os.Remove(dst)
			writeAdminError(w, 500, "Не удалось сохранить сведения о файле")
			return
		}
		notifyAssignmentMembers(aid, "material", "Новый учебный материал", title, "/student")
		writeAdminJSON(w, 201, map[string]string{"message": "Файл загружен"})
		return
	}
	rows, err := db.Query(`SELECT id,title,material_type,file_name,file_path FROM course_materials WHERE assignment_id=$1 ORDER BY uploaded_at DESC`, aid)
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить материалы")
		return
	}
	defer rows.Close()
	type item struct {
		ID       int64  `json:"id"`
		Title    string `json:"title"`
		Type     string `json:"type"`
		FileName string `json:"file_name"`
		URL      string `json:"url"`
	}
	list := []item{}
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.ID, &x.Title, &x.Type, &x.FileName, &x.URL); err != nil {
			writeAdminError(w, 500, "Ошибка чтения материалов")
			return
		}
		list = append(list, x)
	}
	writeAdminJSON(w, 200, list)
}

// portalStudentGrades возвращает реальные баллы из модульного журнала.
// Итоговая оценка здесь намеренно не вычисляется: формула университета
// должна быть задана отдельно администратором.

func portalStudentStudyPlan(w http.ResponseWriter, r *http.Request, userID int64) {
	if r.Method != http.MethodPost {
		writeAdminError(w, 405, "Метод не поддерживается")
		return
	}
	var req struct {
		Subject  string   `json:"subject"`
		ExamDate string   `json:"exam_date"`
		Topics   []string `json:"topics"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeAdminError(w, 400, "Некорректный план подготовки")
		return
	}
	req.Subject = strings.TrimSpace(req.Subject)
	if req.Subject == "" || len(req.Subject) > 150 || len(req.Topics) == 0 || len(req.Topics) > 50 {
		writeAdminError(w, 400, "Укажите предмет и от 1 до 50 тем")
		return
	}
	examDate, err := time.ParseInLocation("2006-01-02", req.ExamDate, time.Local)
	now := time.Now()
	todayDate := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
	if err != nil || !examDate.After(todayDate) {
		writeAdminError(w, 400, "Дата экзамена должна быть в будущем")
		return
	}
	topics := []string{}
	for _, topic := range req.Topics {
		topic = strings.TrimSpace(topic)
		if topic != "" {
			if len(topic) > 300 {
				writeAdminError(w, 400, "Название темы слишком длинное")
				return
			}
			topics = append(topics, topic)
		}
	}
	if len(topics) == 0 {
		writeAdminError(w, 400, "Введите хотя бы одну тему")
		return
	}
	preparationWindow := examDate.Sub(now)
	tx, err := db.Begin()
	if err != nil {
		writeAdminError(w, 500, "Не удалось создать план")
		return
	}
	defer tx.Rollback()
	for i, topic := range topics {
		fraction := float64(i+1) / float64(len(topics)+1)
		due := now.Add(time.Duration(float64(preparationWindow) * fraction))
		title := "Подготовка: " + req.Subject + " — " + topic
		description := "Тема из персонального плана подготовки к экзамену " + req.ExamDate
		if _, err = tx.Exec(`INSERT INTO personal_tasks(user_id,title,description,due_at,priority) VALUES($1,$2,$3,$4,2)`, userID, title, description, due); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить задачи плана")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAdminError(w, 500, "Не удалось сохранить план подготовки")
		return
	}
	writeAdminJSON(w, 201, map[string]int{"created_tasks": len(topics)})
}

func portalStudentTasks(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id,title,description,due_at,priority,is_completed,created_at FROM personal_tasks WHERE user_id=$1 ORDER BY is_completed,due_at NULLS LAST,priority,id DESC`, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить задачи")
			return
		}
		defer rows.Close()
		type task struct {
			ID          int64      `json:"id"`
			Title       string     `json:"title"`
			Description string     `json:"description"`
			DueAt       *time.Time `json:"due_at"`
			Priority    int        `json:"priority"`
			IsCompleted bool       `json:"is_completed"`
			CreatedAt   time.Time  `json:"created_at"`
		}
		out := []task{}
		for rows.Next() {
			var x task
			if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.DueAt, &x.Priority, &x.IsCompleted, &x.CreatedAt); err != nil {
				writeAdminError(w, 500, "Ошибка чтения задач")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeAdminError(w, 500, "Ошибка чтения задач")
			return
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost, http.MethodPut:
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			DueAt       string `json:"due_at"`
			Priority    int    `json:"priority"`
			IsCompleted bool   `json:"is_completed"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректные данные задачи")
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Description = strings.TrimSpace(req.Description)
		if req.Title == "" || len(req.Title) > 200 || req.Priority < 1 || req.Priority > 3 {
			writeAdminError(w, 400, "Укажите название задачи и приоритет от 1 до 3")
			return
		}
		var due any
		if req.DueAt != "" {
			parsed, err := time.Parse(time.RFC3339, req.DueAt)
			if err != nil {
				writeAdminError(w, 400, "Некорректный срок выполнения")
				return
			}
			due = parsed
		}
		if r.Method == http.MethodPost {
			var id int64
			err := db.QueryRow(`INSERT INTO personal_tasks(user_id,title,description,due_at,priority) VALUES($1,$2,$3,$4,$5) RETURNING id`, userID, req.Title, req.Description, due, req.Priority).Scan(&id)
			if err != nil {
				writeAdminError(w, 500, "Не удалось создать задачу")
				return
			}
			writeAdminJSON(w, 201, map[string]int64{"id": id})
			return
		}
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID задачи")
			return
		}
		res, err := db.Exec(`UPDATE personal_tasks SET title=$1,description=$2,due_at=$3,priority=$4,is_completed=$5,updated_at=CURRENT_TIMESTAMP WHERE id=$6 AND user_id=$7`, req.Title, req.Description, due, req.Priority, req.IsCompleted, id, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось обновить задачу")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Задача не найдена")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Задача обновлена"})
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID задачи")
			return
		}
		res, err := db.Exec(`DELETE FROM personal_tasks WHERE id=$1 AND user_id=$2`, id, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось удалить задачу")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Задача не найдена")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Задача удалена"})
	}
}

func portalStudentPortfolio(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id,title,description,category,skills,resource_url,is_public,created_at FROM portfolio_items WHERE student_user_id=$1 ORDER BY created_at DESC`, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить портфолио")
			return
		}
		defer rows.Close()
		type item struct {
			ID          int64     `json:"id"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			Category    string    `json:"category"`
			Skills      string    `json:"skills"`
			ResourceURL string    `json:"resource_url"`
			IsPublic    bool      `json:"is_public"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := []item{}
		for rows.Next() {
			var x item
			if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.Category, &x.Skills, &x.ResourceURL, &x.IsPublic, &x.CreatedAt); err != nil {
				writeAdminError(w, 500, "Ошибка чтения портфолио")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeAdminError(w, 500, "Ошибка чтения портфолио")
			return
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost, http.MethodPut:
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
			Category    string `json:"category"`
			Skills      string `json:"skills"`
			ResourceURL string `json:"resource_url"`
			IsPublic    bool   `json:"is_public"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректные данные портфолио")
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Description = strings.TrimSpace(req.Description)
		req.Skills = strings.TrimSpace(req.Skills)
		req.ResourceURL = strings.TrimSpace(req.ResourceURL)
		validCategory := map[string]bool{"project": true, "certificate": true, "lab": true, "achievement": true, "skill": true, "other": true}
		if req.Title == "" || len(req.Title) > 200 || !validCategory[req.Category] {
			writeAdminError(w, 400, "Проверьте название и категорию работы")
			return
		}
		if req.ResourceURL != "" {
			u, err := url.ParseRequestURI(req.ResourceURL)
			if err != nil || (u.Scheme != "https" && u.Scheme != "http") {
				writeAdminError(w, 400, "Ссылка должна начинаться с http:// или https://")
				return
			}
		}
		if r.Method == http.MethodPost {
			var id int64
			err := db.QueryRow(`INSERT INTO portfolio_items(student_user_id,title,description,category,skills,resource_url,is_public) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id`, userID, req.Title, req.Description, req.Category, req.Skills, req.ResourceURL, req.IsPublic).Scan(&id)
			if err != nil {
				writeAdminError(w, 500, "Не удалось добавить работу")
				return
			}
			writeAdminJSON(w, 201, map[string]int64{"id": id})
			return
		}
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID записи")
			return
		}
		res, err := db.Exec(`UPDATE portfolio_items SET title=$1,description=$2,category=$3,skills=$4,resource_url=$5,is_public=$6,updated_at=CURRENT_TIMESTAMP WHERE id=$7 AND student_user_id=$8`, req.Title, req.Description, req.Category, req.Skills, req.ResourceURL, req.IsPublic, id, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось обновить работу")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Запись портфолио не найдена")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Портфолио обновлено"})
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID записи")
			return
		}
		res, err := db.Exec(`DELETE FROM portfolio_items WHERE id=$1 AND student_user_id=$2`, id, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось удалить работу")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Запись портфолио не найдена")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Запись удалена"})
	}
}

func portalStudentGrades(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`
		SELECT a.id, s.name, a.academic_year, a.semester,
			COALESCE(m1.lectures,0), COALESCE(m1.labs,0), COALESCE(m1.other_work,0), COALESCE(m1.extra_points,0),
			COALESCE(m2.lectures,0), COALESCE(m2.labs,0), COALESCE(m2.other_work,0), COALESCE(m2.extra_points,0),
			(m1.id IS NOT NULL), (m2.id IS NOT NULL)
		FROM students st
		JOIN teacher_assignments a ON a.group_id=st.group_id
		JOIN subjects s ON s.id=a.subject_id
		LEFT JOIN module_scores m1 ON m1.assignment_id=a.id AND m1.student_id=st.user_id AND m1.module_number=1
		LEFT JOIN module_scores m2 ON m2.assignment_id=a.id AND m2.student_id=st.user_id AND m2.module_number=2
		WHERE st.user_id=$1
		ORDER BY a.academic_year DESC, a.semester, s.name`, userID)
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить оценки")
		return
	}
	defer rows.Close()
	type grade struct {
		AssignmentID int64   `json:"assignment_id"`
		Subject      string  `json:"subject"`
		AcademicYear string  `json:"academic_year"`
		Semester     int     `json:"semester"`
		Module1      float64 `json:"module1"`
		Module2      float64 `json:"module2"`
		Total        float64 `json:"total"`
		HasModule1   bool    `json:"has_module1"`
		HasModule2   bool    `json:"has_module2"`
	}
	out := []grade{}
	for rows.Next() {
		var x grade
		var l1, b1, o1, e1, l2, b2, o2, e2 float64
		if err := rows.Scan(&x.AssignmentID, &x.Subject, &x.AcademicYear, &x.Semester, &l1, &b1, &o1, &e1, &l2, &b2, &o2, &e2, &x.HasModule1, &x.HasModule2); err != nil {
			writeAdminError(w, 500, "Ошибка чтения оценок")
			return
		}
		x.Module1 = minFloat(25, minFloat(21, l1+b1+o1)+e1)
		x.Module2 = minFloat(25, minFloat(21, l2+b2+o2)+e2)
		x.Total = x.Module1 + x.Module2
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		writeAdminError(w, 500, "Ошибка чтения оценок")
		return
	}
	writeAdminJSON(w, 200, out)
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func portalStudentAttendance(w http.ResponseWriter, userID int64) {
	var maxAbsence sql.NullFloat64
	if err := db.QueryRow(`SELECT max_absence_percent FROM academic_settings WHERE id=1`).Scan(&maxAbsence); err != nil && err != sql.ErrNoRows {
		writeAdminError(w, 500, "Не удалось загрузить правило посещаемости")
		return
	}
	rows, err := db.Query(`
		SELECT s.name, COUNT(l.id),
			COUNT(l.id) FILTER (WHERE l.attendance='present'),
			COUNT(l.id) FILTER (WHERE l.attendance='absent'),
			COUNT(l.id) FILTER (WHERE l.attendance='excused'),
			COUNT(l.id) FILTER (WHERE l.attendance='sick')
		FROM students st
		JOIN teacher_assignments a ON a.group_id=st.group_id
		JOIN subjects s ON s.id=a.subject_id
		LEFT JOIN lesson_records l ON l.assignment_id=a.id AND l.student_id=st.user_id
		WHERE st.user_id=$1
		GROUP BY a.id,s.name
		ORDER BY s.name`, userID)
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить посещаемость")
		return
	}
	defer rows.Close()
	type attendance struct {
		Subject           string   `json:"subject"`
		Lessons           int      `json:"lessons"`
		Present           int      `json:"present"`
		Absent            int      `json:"absent"`
		Excused           int      `json:"excused"`
		Sick              int      `json:"sick"`
		Percent           float64  `json:"percent"`
		AbsencePercent    float64  `json:"absence_percent"`
		MaxAbsencePercent *float64 `json:"max_absence_percent"`
		Risk              bool     `json:"risk"`
	}
	out := []attendance{}
	for rows.Next() {
		var x attendance
		if err := rows.Scan(&x.Subject, &x.Lessons, &x.Present, &x.Absent, &x.Excused, &x.Sick); err != nil {
			writeAdminError(w, 500, "Ошибка чтения посещаемости")
			return
		}
		if x.Lessons > 0 {
			x.Percent = float64(x.Present) / float64(x.Lessons) * 100
			x.AbsencePercent = float64(x.Absent) / float64(x.Lessons) * 100
		}
		if maxAbsence.Valid {
			limit := maxAbsence.Float64
			x.MaxAbsencePercent = &limit
			x.Risk = x.Absent > 0 && x.Lessons > 0 && x.AbsencePercent >= limit
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		writeAdminError(w, 500, "Ошибка чтения посещаемости")
		return
	}
	writeAdminJSON(w, 200, out)
}

func portalStudentMaterials(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`SELECT cm.id,cm.title,cm.material_type,cm.file_name,cm.file_path,s.name,u.full_name,t.employee_number FROM students st JOIN teacher_assignments a ON a.group_id=st.group_id JOIN course_materials cm ON cm.assignment_id=a.id JOIN subjects s ON s.id=a.subject_id JOIN teachers t ON t.id=a.teacher_id JOIN users u ON u.id=t.user_id WHERE st.user_id=$1 ORDER BY s.name,cm.uploaded_at DESC`, userID)
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить материалы")
		return
	}
	defer rows.Close()
	type item struct {
		ID            int64  `json:"id"`
		Title         string `json:"title"`
		Type          string `json:"type"`
		FileName      string `json:"file_name"`
		URL           string `json:"url"`
		Subject       string `json:"subject"`
		Teacher       string `json:"teacher"`
		TeacherNumber string `json:"teacher_number"`
	}
	out := []item{}
	for rows.Next() {
		var x item
		if err = rows.Scan(&x.ID, &x.Title, &x.Type, &x.FileName, &x.URL, &x.Subject, &x.Teacher, &x.TeacherNumber); err != nil {
			writeAdminError(w, 500, "Ошибка чтения материалов")
			return
		}
		out = append(out, x)
	}
	writeAdminJSON(w, 200, out)
}

func notifyAssignmentMembers(assignmentID int64, notificationType, title, body, link string) {
	_, _ = db.Exec(`INSERT INTO notifications(user_id,notification_type,title,body,link) SELECT st.user_id,$2,$3,$4,$5 FROM teacher_assignments a JOIN students st ON st.group_id=a.group_id WHERE a.id=$1`, assignmentID, notificationType, title, body, link)
	_, _ = db.Exec(`INSERT INTO notifications(user_id,notification_type,title,body,link) SELECT t.user_id,$2,$3,$4,$5 FROM teacher_assignments a JOIN teachers t ON t.id=a.teacher_id WHERE a.id=$1`, assignmentID, notificationType, title, body, link)
}

func portalNotifications(w http.ResponseWriter, r *http.Request, userID int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id,notification_type,title,body,link,read_at,created_at FROM notifications WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить уведомления")
			return
		}
		defer rows.Close()
		type notification struct {
			ID        int64      `json:"id"`
			Type      string     `json:"type"`
			Title     string     `json:"title"`
			Body      string     `json:"body"`
			Link      string     `json:"link"`
			ReadAt    *time.Time `json:"read_at"`
			CreatedAt time.Time  `json:"created_at"`
		}
		out := []notification{}
		for rows.Next() {
			var x notification
			if err := rows.Scan(&x.ID, &x.Type, &x.Title, &x.Body, &x.Link, &x.ReadAt, &x.CreatedAt); err != nil {
				writeAdminError(w, 500, "Ошибка чтения уведомлений")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeAdminError(w, 500, "Ошибка чтения уведомлений")
			return
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID уведомления")
			return
		}
		res, err := db.Exec(`UPDATE notifications SET read_at=COALESCE(read_at,CURRENT_TIMESTAMP) WHERE id=$1 AND user_id=$2`, id, userID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось отметить уведомление")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Уведомление не найдено")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Уведомление прочитано"})
	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func portalMessaging(w http.ResponseWriter, r *http.Request, user User) {
	switch r.Method {
	case http.MethodGet:
		if r.URL.Query().Get("contacts") == "1" {
			var query string
			if user.Role == "student" {
				query = `SELECT DISTINCT u.id,u.full_name,u.role FROM users u WHERE u.is_active=TRUE AND u.id<>$1 AND ((u.role='student' AND EXISTS(SELECT 1 FROM students me JOIN students other ON other.group_id=me.group_id WHERE me.user_id=$1 AND other.user_id=u.id)) OR (u.role='teacher' AND (EXISTS(SELECT 1 FROM students me JOIN teacher_assignments a ON a.group_id=me.group_id JOIN teachers t ON t.id=a.teacher_id WHERE me.user_id=$1 AND t.user_id=u.id) OR EXISTS(SELECT 1 FROM students me JOIN group_curators gc ON gc.group_id=me.group_id JOIN teachers t ON t.id=gc.teacher_id WHERE me.user_id=$1 AND t.user_id=u.id)))) ORDER BY u.role,u.full_name`
			} else {
				query = `SELECT DISTINCT u.id,u.full_name,u.role FROM users u WHERE u.is_active=TRUE AND u.id<>$1 AND u.role='student' AND (EXISTS(SELECT 1 FROM teachers me JOIN teacher_assignments a ON a.teacher_id=me.id JOIN students st ON st.group_id=a.group_id WHERE me.user_id=$1 AND st.user_id=u.id) OR EXISTS(SELECT 1 FROM teachers me JOIN group_curators gc ON gc.teacher_id=me.id JOIN students st ON st.group_id=gc.group_id WHERE me.user_id=$1 AND st.user_id=u.id)) ORDER BY u.full_name`
			}
			rows, err := db.Query(query, user.ID)
			if err != nil {
				writeAdminError(w, 500, "Не удалось загрузить список контактов")
				return
			}
			defer rows.Close()
			type contact struct {
				ID   int64  `json:"id"`
				Name string `json:"name"`
				Role string `json:"role"`
			}
			out := []contact{}
			for rows.Next() {
				var x contact
				if err := rows.Scan(&x.ID, &x.Name, &x.Role); err != nil {
					writeAdminError(w, 500, "Ошибка чтения контактов")
					return
				}
				out = append(out, x)
			}
			if err := rows.Err(); err != nil {
				writeAdminError(w, 500, "Ошибка чтения контактов")
				return
			}
			writeAdminJSON(w, 200, out)
			return
		}
		if rawID := r.URL.Query().Get("conversation_id"); rawID != "" {
			id, err := strconv.ParseInt(rawID, 10, 64)
			if err != nil || id < 1 {
				writeAdminError(w, 400, "Некорректный чат")
				return
			}
			var member bool
			if err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, id, user.ID).Scan(&member); err != nil || !member {
				writeAdminError(w, 403, "Нет доступа к этому чату")
				return
			}
			_, _ = db.Exec(`UPDATE conversation_members SET last_read_at=CURRENT_TIMESTAMP WHERE conversation_id=$1 AND user_id=$2`, id, user.ID)
			rows, err := db.Query(`SELECT m.id,m.sender_id,u.full_name,m.body,m.created_at FROM messages m JOIN users u ON u.id=m.sender_id WHERE m.conversation_id=$1 AND m.deleted_at IS NULL ORDER BY m.created_at,m.id LIMIT 300`, id)
			if err != nil {
				writeAdminError(w, 500, "Не удалось загрузить сообщения")
				return
			}
			defer rows.Close()
			type msg struct {
				ID         int64     `json:"id"`
				SenderID   int64     `json:"sender_id"`
				SenderName string    `json:"sender_name"`
				Body       string    `json:"body"`
				CreatedAt  time.Time `json:"created_at"`
			}
			out := []msg{}
			for rows.Next() {
				var x msg
				if err := rows.Scan(&x.ID, &x.SenderID, &x.SenderName, &x.Body, &x.CreatedAt); err != nil {
					writeAdminError(w, 500, "Ошибка чтения сообщений")
					return
				}
				out = append(out, x)
			}
			if err := rows.Err(); err != nil {
				writeAdminError(w, 500, "Ошибка чтения сообщений")
				return
			}
			writeAdminJSON(w, 200, out)
			return
		}
		rows, err := db.Query(`SELECT c.id,COALESCE(MAX(other.full_name),c.title),COALESCE((SELECT body FROM messages m WHERE m.conversation_id=c.id AND m.deleted_at IS NULL ORDER BY m.created_at DESC LIMIT 1),''),COALESCE((SELECT created_at FROM messages m WHERE m.conversation_id=c.id AND m.deleted_at IS NULL ORDER BY m.created_at DESC LIMIT 1),c.created_at),COUNT(m.id) FILTER (WHERE m.sender_id<>$1 AND (cm.last_read_at IS NULL OR m.created_at>cm.last_read_at)) FROM conversations c JOIN conversation_members cm ON cm.conversation_id=c.id AND cm.user_id=$1 JOIN conversation_members otherm ON otherm.conversation_id=c.id AND otherm.user_id<>$1 JOIN users other ON other.id=otherm.user_id LEFT JOIN messages m ON m.conversation_id=c.id AND m.deleted_at IS NULL GROUP BY c.id,cm.last_read_at ORDER BY MAX(m.created_at) DESC NULLS LAST,c.created_at DESC`, user.ID)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить список чатов")
			return
		}
		defer rows.Close()
		type chat struct {
			ID          int64     `json:"id"`
			Name        string    `json:"name"`
			LastMessage string    `json:"last_message"`
			LastAt      time.Time `json:"last_at"`
			Unread      int       `json:"unread"`
		}
		out := []chat{}
		for rows.Next() {
			var x chat
			if err := rows.Scan(&x.ID, &x.Name, &x.LastMessage, &x.LastAt, &x.Unread); err != nil {
				writeAdminError(w, 500, "Ошибка чтения списка чатов")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeAdminError(w, 500, "Ошибка чтения списка чатов")
			return
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost:
		var req struct {
			ConversationID int64  `json:"conversation_id"`
			RecipientID    int64  `json:"recipient_id"`
			Body           string `json:"body"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректное сообщение")
			return
		}
		req.Body = strings.TrimSpace(req.Body)
		if req.Body == "" || len(req.Body) > 5000 {
			writeAdminError(w, 400, "Сообщение должно содержать от 1 до 5000 символов")
			return
		}
		if req.ConversationID > 0 {
			var member bool
			if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM conversation_members WHERE conversation_id=$1 AND user_id=$2)`, req.ConversationID, user.ID).Scan(&member); err != nil || !member {
				writeAdminError(w, 403, "Нет доступа к этому чату")
				return
			}
			var id int64
			if err := db.QueryRow(`INSERT INTO messages(conversation_id,sender_id,body) VALUES($1,$2,$3) RETURNING id`, req.ConversationID, user.ID, req.Body).Scan(&id); err != nil {
				writeAdminError(w, 500, "Не удалось отправить сообщение")
				return
			}
			_, _ = db.Exec(`INSERT INTO notifications(user_id,notification_type,title,body,link) SELECT user_id,'message','Новое сообщение',$2,CASE WHEN role='teacher' THEN '/teacher' ELSE '/student' END FROM conversation_members cm JOIN users u ON u.id=cm.user_id WHERE cm.conversation_id=$1 AND cm.user_id<>$3`, req.ConversationID, "Вам пришло новое сообщение", user.ID)
			writeAdminJSON(w, 201, map[string]int64{"id": id, "conversation_id": req.ConversationID})
			return
		}
		if req.RecipientID < 1 || req.RecipientID == user.ID {
			writeAdminError(w, 400, "Выберите собеседника")
			return
		}
		var allowed bool
		var err error
		if user.Role == "student" {
			err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM students me JOIN students other ON other.group_id=me.group_id WHERE me.user_id=$1 AND other.user_id=$2) OR EXISTS(SELECT 1 FROM students me JOIN teacher_assignments a ON a.group_id=me.group_id JOIN teachers t ON t.id=a.teacher_id WHERE me.user_id=$1 AND t.user_id=$2) OR EXISTS(SELECT 1 FROM students me JOIN group_curators gc ON gc.group_id=me.group_id JOIN teachers t ON t.id=gc.teacher_id WHERE me.user_id=$1 AND t.user_id=$2)`, user.ID, req.RecipientID).Scan(&allowed)
		} else {
			err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teachers me JOIN teacher_assignments a ON a.teacher_id=me.id JOIN students st ON st.group_id=a.group_id WHERE me.user_id=$1 AND st.user_id=$2) OR EXISTS(SELECT 1 FROM teachers me JOIN group_curators gc ON gc.teacher_id=me.id JOIN students st ON st.group_id=gc.group_id WHERE me.user_id=$1 AND st.user_id=$2)`, user.ID, req.RecipientID).Scan(&allowed)
		}
		if err != nil || !allowed {
			writeAdminError(w, 403, "Можно переписываться только с участниками учебного процесса, связанными с вами")
			return
		}
		tx, err := db.Begin()
		if err != nil {
			writeAdminError(w, 500, "Не удалось создать чат")
			return
		}
		defer tx.Rollback()
		var convID int64
		if err = tx.QueryRow(`INSERT INTO conversations(conversation_type,created_by) VALUES('direct',$1) RETURNING id`, user.ID).Scan(&convID); err != nil {
			writeAdminError(w, 500, "Не удалось создать чат")
			return
		}
		if _, err = tx.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2),($1,$3)`, convID, user.ID, req.RecipientID); err != nil {
			writeAdminError(w, 500, "Не удалось добавить участников чата")
			return
		}
		var msgID int64
		if err = tx.QueryRow(`INSERT INTO messages(conversation_id,sender_id,body) VALUES($1,$2,$3) RETURNING id`, convID, user.ID, req.Body).Scan(&msgID); err != nil {
			writeAdminError(w, 500, "Не удалось отправить сообщение")
			return
		}
		if err = tx.Commit(); err != nil {
			writeAdminError(w, 500, "Не удалось сохранить чат")
			return
		}
		_, _ = db.Exec(`INSERT INTO notifications(user_id,notification_type,title,body,link) VALUES($1,'message','Новое сообщение',$2,CASE WHEN (SELECT role FROM users WHERE id=$1)='teacher' THEN '/teacher' ELSE '/student' END)`, req.RecipientID, "Вам пришло новое сообщение")
		writeAdminJSON(w, 201, map[string]int64{"id": msgID, "conversation_id": convID})
	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func portalTeacherTopics(w http.ResponseWriter, r *http.Request, userID int64) {
	aid, err := strconv.ParseInt(r.URL.Query().Get("assignment_id"), 10, 64)
	if err != nil || aid < 1 {
		writeAdminError(w, 400, "Не выбрана дисциплина")
		return
	}
	var owns bool
	if err = db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teacher_assignments a JOIN teachers t ON t.id=a.teacher_id WHERE a.id=$1 AND t.user_id=$2)`, aid, userID).Scan(&owns); err != nil || !owns {
		writeAdminError(w, 403, "Нет доступа к этой дисциплине")
		return
	}
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`SELECT id,title,description,created_at FROM course_topics WHERE assignment_id=$1 ORDER BY created_at,id`, aid)
		if err != nil {
			writeAdminError(w, 500, "Не удалось загрузить темы")
			return
		}
		defer rows.Close()
		type topic struct {
			ID          int64     `json:"id"`
			Title       string    `json:"title"`
			Description string    `json:"description"`
			CreatedAt   time.Time `json:"created_at"`
		}
		out := []topic{}
		for rows.Next() {
			var x topic
			if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.CreatedAt); err != nil {
				writeAdminError(w, 500, "Ошибка чтения тем")
				return
			}
			out = append(out, x)
		}
		if err := rows.Err(); err != nil {
			writeAdminError(w, 500, "Ошибка чтения тем")
			return
		}
		writeAdminJSON(w, 200, out)
	case http.MethodPost:
		var req struct {
			Title       string `json:"title"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
			writeAdminError(w, 400, "Некорректные данные темы")
			return
		}
		req.Title = strings.TrimSpace(req.Title)
		req.Description = strings.TrimSpace(req.Description)
		if req.Title == "" || len(req.Title) > 200 {
			writeAdminError(w, 400, "Введите название темы до 200 символов")
			return
		}
		var id int64
		err = db.QueryRow(`INSERT INTO course_topics(assignment_id,title,description) VALUES($1,$2,$3) RETURNING id`, aid, req.Title, req.Description).Scan(&id)
		if err != nil {
			writeAdminError(w, 409, "Не удалось создать тему. Возможно, такое название уже есть")
			return
		}
		notifyAssignmentMembers(aid, "course", "Добавлена новая тема", req.Title, "/student")
		writeAdminJSON(w, 201, map[string]int64{"id": id})
	case http.MethodDelete:
		id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
		if err != nil || id < 1 {
			writeAdminError(w, 400, "Некорректный ID темы")
			return
		}
		res, err := db.Exec(`DELETE FROM course_topics WHERE id=$1 AND assignment_id=$2`, id, aid)
		if err != nil {
			writeAdminError(w, 500, "Не удалось удалить тему")
			return
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			writeAdminError(w, 404, "Тема не найдена")
			return
		}
		writeAdminJSON(w, 200, map[string]string{"message": "Тема удалена"})
	default:
		writeAdminError(w, 405, "Метод не поддерживается")
	}
}

func portalStudentCurator(w http.ResponseWriter, userID int64) {
	var item struct {
		FullName       string `json:"full_name"`
		Email          string `json:"email"`
		Position       string `json:"position"`
		EmployeeNumber string `json:"employee_number"`
	}
	err := db.QueryRow(`SELECT u.full_name,u.email,COALESCE(t.position,''),t.employee_number FROM students st JOIN group_curators gc ON gc.group_id=st.group_id JOIN teachers t ON t.id=gc.teacher_id JOIN users u ON u.id=t.user_id AND u.is_active=TRUE WHERE st.user_id=$1`, userID).Scan(&item.FullName, &item.Email, &item.Position, &item.EmployeeNumber)
	if err == sql.ErrNoRows {
		writeAdminJSON(w, 200, map[string]string{})
		return
	}
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить данные куратора")
		return
	}
	writeAdminJSON(w, 200, item)
}

func portalStudentTopics(w http.ResponseWriter, userID int64) {
	rows, err := db.Query(`SELECT ct.id,ct.title,ct.description,s.name,a.id FROM students st JOIN teacher_assignments a ON a.group_id=st.group_id JOIN course_topics ct ON ct.assignment_id=a.id JOIN subjects s ON s.id=a.subject_id WHERE st.user_id=$1 ORDER BY s.name,ct.created_at`, userID)
	if err != nil {
		writeAdminError(w, 500, "Не удалось загрузить темы предметов")
		return
	}
	defer rows.Close()
	type topic struct {
		ID           int64  `json:"id"`
		Title        string `json:"title"`
		Description  string `json:"description"`
		Subject      string `json:"subject"`
		AssignmentID int64  `json:"assignment_id"`
	}
	out := []topic{}
	for rows.Next() {
		var x topic
		if err := rows.Scan(&x.ID, &x.Title, &x.Description, &x.Subject, &x.AssignmentID); err != nil {
			writeAdminError(w, 500, "Ошибка чтения тем предметов")
			return
		}
		out = append(out, x)
	}
	if err := rows.Err(); err != nil {
		writeAdminError(w, 500, "Ошибка чтения тем предметов")
		return
	}
	writeAdminJSON(w, 200, out)
}

func portalTeacherModule(w http.ResponseWriter, r *http.Request, userID int64) {
	aid, _ := strconv.ParseInt(r.URL.Query().Get("assignment_id"), 10, 64)
	if aid < 1 {
		writeAdminError(w, 400, "Не выбрано назначение")
		return
	}
	var owns bool
	if err := db.QueryRow(`SELECT EXISTS(SELECT 1 FROM teacher_assignments a JOIN teachers t ON t.id=a.teacher_id WHERE a.id=$1 AND t.user_id=$2)`, aid, userID).Scan(&owns); err != nil || !owns {
		writeAdminError(w, 403, "Нет доступа")
		return
	}
	if r.Method != http.MethodPost {
		writeAdminError(w, 405, "Метод не поддерживается")
		return
	}
	var req struct {
		Module int          `json:"module"`
		Rows   []journalRow `json:"rows"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeAdminError(w, 400, "Некорректные данные")
		return
	}
	if req.Module != 1 && req.Module != 2 {
		writeAdminError(w, 400, "Неверный номер модуля")
		return
	}
	if len(req.Rows) > 1000 {
		writeAdminError(w, 400, "Слишком много записей в одном запросе")
		return
	}
	tx, err := db.Begin()
	if err != nil {
		writeAdminError(w, 500, "Ошибка базы данных")
		return
	}
	defer tx.Rollback()
	for _, x := range req.Rows {
		if x.StudentID < 1 || x.Lectures < 0 || x.Lectures > 21 || x.Labs < 0 || x.Labs > 21 || x.OtherWork < 0 || x.OtherWork > 21 || x.ExtraPoints < 0 || x.ExtraPoints > 4 {
			writeAdminError(w, 400, "Баллы должны быть в допустимых пределах")
			return
		}
		var inGroup bool
		if err = tx.QueryRow(`SELECT EXISTS(SELECT 1 FROM teacher_assignments a JOIN students st ON st.group_id=a.group_id WHERE a.id=$1 AND st.user_id=$2)`, aid, x.StudentID).Scan(&inGroup); err != nil || !inGroup {
			writeAdminError(w, 403, "Студент не относится к назначенной группе")
			return
		}
		_, err = tx.Exec(`INSERT INTO module_scores(assignment_id,student_id,module_number,lectures,labs,other_work,extra_points) SELECT $1,$2,$3,$4,$5,$6,$7 WHERE EXISTS(SELECT 1 FROM teacher_assignments a JOIN students st ON st.group_id=a.group_id WHERE a.id=$1 AND st.user_id=$2) ON CONFLICT(assignment_id,student_id,module_number) DO UPDATE SET lectures=EXCLUDED.lectures,labs=EXCLUDED.labs,other_work=EXCLUDED.other_work,extra_points=EXCLUDED.extra_points,updated_at=CURRENT_TIMESTAMP`, aid, x.StudentID, req.Module, x.Lectures, x.Labs, x.OtherWork, x.ExtraPoints)
		if err != nil {
			writeAdminError(w, 500, "Не удалось сохранить модульные баллы")
			return
		}
	}
	if err = tx.Commit(); err != nil {
		writeAdminError(w, 500, "Не удалось сохранить изменения")
		return
	}
	notifyAssignmentMembers(aid, "grade", "Обновлены баллы модуля", "Преподаватель обновил модульные баллы", "/student")
	writeAdminJSON(w, 200, map[string]string{"message": "Модульные баллы сохранены"})
}
