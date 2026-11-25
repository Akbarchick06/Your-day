package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

const demoPassword = "Demo12345!"

var firstNames = []string{"Айбек", "Нурбек", "Эрмек", "Бекзат", "Азамат", "Данияр", "Руслан", "Тимур", "Алина", "Айдана", "Арууке", "Мээрим", "Нуриза", "Жылдыз", "Элина", "Каныкей", "Самат", "Адилет", "Ильяс", "Мирлан", "Бакыт", "Элдияр", "Сезим", "Назгуль", "Диана"}
var lastNames = []string{"Абдылдаев", "Осмонова", "Ибраимов", "Токтосунова", "Жумабеков", "Садыкова", "Калмурзаев", "Асанова", "Беков", "Мамбетова", "Кадыров", "Исакова", "Турсунов", "Алиева", "Эргешов", "Дуйшенова", "Мусаев", "Шарипова", "Ниязов", "Кожоева", "Султанов", "Боронбаева", "Абдраимов", "Рахманова", "Орозалиева"}

var teacherNames = []string{
	"Айтматова Гульмира Талантбековна", "Садыков Бакыт Эркинович", "Осмонова Нурзат Кубанычбековна", "Ибраимов Азамат Русланович",
	"Токтосунова Айжан Маратовна", "Абдылдаев Эрмек Нурланович", "Кадырова Элина Саматовна", "Мамбетова Жылдыз Болотбековна",
}
var subjectNames = []string{"Программирование", "Базы данных", "Веб-разработка", "Компьютерные сети", "Математика", "Английский язык", "Информационная безопасность", "Алгоритмы и структуры данных"}
var groupNames = []string{"ИТ-1-24", "ИТ-2-24", "ПИ-1-24"}
var lessonTypes = []string{"Лекция", "Практика", "Лабораторная"}

func main() {
	password := os.Getenv("DB_PASSWORD")
	if password == "" {
		log.Fatal("Не задана переменная DB_PASSWORD")
	}
	connURL := url.URL{Scheme: "postgres", User: url.UserPassword("postgres", password), Host: "localhost:5432", Path: "my_edu"}
	query := connURL.Query()
	query.Set("sslmode", "disable")
	connURL.RawQuery = query.Encode()
	conn := connURL.String()
	db, err := sql.Open("postgres", conn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatal("Не удалось подключиться к PostgreSQL: ", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(demoPassword), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()

	var facultyID, departmentID int64
	err = tx.QueryRow(`INSERT INTO faculties(name) VALUES ('Факультет информационных технологий') ON CONFLICT(name) DO UPDATE SET name=EXCLUDED.name RETURNING id`).Scan(&facultyID)
	if err != nil {
		log.Fatal("Факультет: ", err)
	}
	err = tx.QueryRow(`INSERT INTO departments(name, faculty_id) VALUES ('Кафедра информационных систем', $1) ON CONFLICT(name, faculty_id) DO UPDATE SET name=EXCLUDED.name RETURNING id`, facultyID).Scan(&departmentID)
	if err != nil {
		log.Fatal("Кафедра: ", err)
	}

	teacherIDs := make([]int64, len(teacherNames))
	for i, name := range teacherNames {
		email := fmt.Sprintf("teacher%02d@yourday.local", i+1)
		userID, err := upsertUser(tx, name, email, string(hash), "teacher")
		if err != nil {
			log.Fatal(err)
		}
		_, err = tx.Exec(`INSERT INTO teachers(user_id, department_id, employee_number, position) VALUES ($1,$2,$3,$4) ON CONFLICT(user_id) DO UPDATE SET department_id=EXCLUDED.department_id, employee_number=EXCLUDED.employee_number, position=EXCLUDED.position`, userID, departmentID, fmt.Sprintf("YD-T%03d", i+1), "Преподаватель")
		if err != nil {
			log.Fatal("Преподаватель ", name, ": ", err)
		}
		err = tx.QueryRow(`SELECT id FROM teachers WHERE user_id=$1`, userID).Scan(&teacherIDs[i])
		if err != nil {
			log.Fatal(err)
		}
	}

	groupIDs := make([]int64, len(groupNames))
	studentUserIDs := make([]int64, 0, len(groupNames)*25)
	for i, name := range groupNames {
		err := tx.QueryRow(`INSERT INTO study_groups(name, department_id, admission_year, course_number) VALUES ($1,$2,2024,2) ON CONFLICT(name) DO UPDATE SET department_id=EXCLUDED.department_id RETURNING id`, name, departmentID).Scan(&groupIDs[i])
		if err != nil {
			log.Fatal("Группа ", name, ": ", err)
		}
	}

	for g, groupID := range groupIDs {
		for n := 0; n < 25; n++ {
			idx := g*25 + n
			name := lastNames[idx%len(lastNames)] + " " + firstNames[idx%len(firstNames)] + " " + []string{"Бекович", "Кызы", "Нурланович", "Маратовна", "Эркинович"}[idx%5]
			email := fmt.Sprintf("student%03d@yourday.local", idx+1)
			userID, err := upsertUser(tx, name, email, string(hash), "student")
			if err != nil {
				log.Fatal(err)
			}
			studentUserIDs = append(studentUserIDs, userID)
			_, err = tx.Exec(`INSERT INTO students(user_id, group_id, student_number, admission_year) VALUES ($1,$2,$3,2024) ON CONFLICT(user_id) DO UPDATE SET group_id=EXCLUDED.group_id, student_number=EXCLUDED.student_number`, userID, groupID, fmt.Sprintf("YD-S%03d", idx+1))
			if err != nil {
				log.Fatal("Студент ", email, ": ", err)
			}
		}
	}

	subjectIDs := make([]int64, len(subjectNames))
	assignmentIDs := make([]int64, 0)
	for i, subject := range subjectNames {
		var subjectID int64
		err := tx.QueryRow(`INSERT INTO subjects(name, subject_code, department_id) VALUES ($1,$2,$3) ON CONFLICT(subject_code) DO UPDATE SET name=EXCLUDED.name RETURNING id`, subject, fmt.Sprintf("YD-%02d", i+1), departmentID).Scan(&subjectID)
		if err != nil {
			log.Fatal("Предмет ", subject, ": ", err)
		}
		subjectIDs[i] = subjectID
		teacherID := teacherIDs[i%len(teacherIDs)]
		for _, groupID := range groupIDs {
			_, err = tx.Exec(`INSERT INTO teacher_assignments(teacher_id, subject_id, group_id, academic_year, semester) VALUES ($1,$2,$3,'2026-2027',1) ON CONFLICT(teacher_id,subject_id,group_id,academic_year,semester) DO NOTHING`, teacherID, subjectID, groupID)
			if err != nil {
				log.Fatal("Назначение предмета: ", err)
			}
			var aid int64
			if err := tx.QueryRow(`SELECT id FROM teacher_assignments WHERE teacher_id=$1 AND subject_id=$2 AND group_id=$3 AND academic_year='2026-2027' AND semester=1`, teacherID, subjectID, groupID).Scan(&aid); err != nil {
				log.Fatal(err)
			}
			assignmentIDs = append(assignmentIDs, aid)
		}
	}
	if err := seedExtraDemo(tx, studentUserIDs, teacherIDs, groupIDs, subjectIDs, assignmentIDs); err != nil {
		log.Fatal("Дополнительные демо-данные: ", err)
	}
	// Regenerate demo schedule every run so it is complete and not duplicated.
	if err := seedSchedule(tx, assignmentIDs, groupIDs); err != nil {
		log.Fatal("Демо-расписание: ", err)
	}

	if err := tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("Демо-данные YOUR DAY добавлены.")
	fmt.Println("Групп: 3; студентов: 75; преподавателей: 8; предметов: 8; расписание, оценки, посещаемость, задания, материалы, портфолио, чаты и уведомления заполнены.")
	fmt.Println("Пароль всех демо-аккаунтов: " + demoPassword)
	fmt.Println("Логины: teacher01@yourday.local ... teacher08@yourday.local; student001@yourday.local ... student075@yourday.local")
	fmt.Println("Существующие аккаунты с другими email не удаляются.")
}

func seedExtraDemo(tx *sql.Tx, students, teachers, groups, subjects, assignments []int64) error {
	// Protect reruns from multiplying demo records. Seed the supplemental tables once.
	var already int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM personal_tasks pt JOIN users u ON u.id=pt.user_id WHERE u.email LIKE 'student%@yourday.local'`).Scan(&already); err != nil {
		return err
	}
	if already > 0 {
		return nil
	}
	// Two academic periods plus exam period.
	_, err := tx.Exec(`INSERT INTO academic_periods(academic_year,semester,period_type,title,start_date,end_date,is_open) VALUES
	('2026-2027',1,'module1','Осенний модуль 1','2026-09-01','2026-10-15',FALSE),
	('2026-2027',1,'module2','Осенний модуль 2','2026-10-16','2026-12-20',TRUE),
	('2026-2027',1,'exam','Зимняя сессия','2026-12-21','2027-01-20',FALSE)
	ON CONFLICT(academic_year,semester,period_type) DO NOTHING`)
	if err != nil {
		return err
	}
	// Assign curators and populate weekly schedules for all assignments.
	for i, groupID := range groups {
		_, err = tx.Exec(`INSERT INTO group_curators(group_id,teacher_id) VALUES($1,$2) ON CONFLICT(group_id) DO NOTHING`, groupID, teachers[i%len(teachers)])
		if err != nil {
			return err
		}
	}
	// Schedule is generated separately on every seed run by seedSchedule().
	// Course topic and material records.
	topicExamples := []string{"Введение и основные понятия", "Практическая работа", "Закрепление темы", "Подготовка к модулю"}
	for i, aid := range assignments {
		_, err := tx.Exec(`INSERT INTO course_topics(assignment_id,title,description) VALUES($1,$2,$3) ON CONFLICT(assignment_id,title) DO NOTHING`, aid, topicExamples[i%len(topicExamples)], "Демо-тема курса: изучить материал и выполнить практические задания.")
		if err != nil {
			return err
		}
		_, err = tx.Exec(`INSERT INTO course_materials(assignment_id,title,material_type,file_name,file_path) VALUES($1,$2,'link','demo_material.txt','demo://yourday/material')`, aid, "Конспект и материалы занятия")
		if err != nil {
			return err
		}
	}
	// Module points, attendance and reusable student task planner.
	taskTitles := []string{"Лабораторная работа №1", "Домашнее задание по теме", "Подготовка к модулю", "Самостоятельная работа", "Повторить лекционный материал"}
	for i, userID := range students {
		for j := 0; j < 3; j++ {
			title := taskTitles[(i+j)%len(taskTitles)]
			_, err := tx.Exec(`INSERT INTO personal_tasks(user_id,title,description,due_at,priority,is_completed) VALUES($1,$2,$3,NOW()+($4||' days')::interval,$5,$6)`, userID, title, "Демонстрационная задача YOUR DAY. Выполните и отметьте завершение в личном планировщике.", fmt.Sprintf("%d", 2+j*3), 1+(j%3), (i+j)%4 == 0)
			if err != nil {
				return err
			}
		}
		for j := 0; j < len(subjects); j++ {
			aid := assignments[j*len(groups)+(i/25)]
			// Only populate this student with assignments for their own group.
			for module := 1; module <= 2; module++ {
				_, err := tx.Exec(`INSERT INTO module_scores(assignment_id,student_id,module_number,lectures,labs,other_work,extra_points) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(assignment_id,student_id,module_number) DO NOTHING`, aid, userID, module, 12+(i+j+module)%9, 10+(i+j)%11, 11+(i*2+j+module)%10, (i+j)%4)
				if err != nil {
					return err
				}
			}
			// Four sample attendance dates per student/assignment, spread across recent months.
			for k := 0; k < 2; k++ {
				day := 3 + ((i + j + k*7) % 24)
				date := fmt.Sprintf("2026-10-%02d", day)
				status := []string{"present", "present", "present", "absent", "excused", "sick"}[(i+j+k)%6]
				points := 0
				if status == "present" {
					points = 15 + (i+j+k)%11
				}
				_, err := tx.Exec(`INSERT INTO lesson_records(assignment_id,student_id,lesson_date,lesson_type,attendance,points,comment) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(assignment_id,student_id,lesson_date) DO NOTHING`, aid, userID, date, lessonTypes[(i+j)%len(lessonTypes)], status, points, "Демо-запись журнала")
				if err != nil {
					return err
				}
			}
		}
		for n := 0; n < 2; n++ {
			_, err := tx.Exec(`INSERT INTO notifications(user_id,notification_type,title,body,link) VALUES($1,$2,$3,$4,$5)`, userID, "demo", []string{"Новое задание", "Расписание обновлено"}[n], []string{"Проверьте срок выполнения задания в планировщике.", "Откройте расписание, чтобы посмотреть занятия."}[n], "")
			if err != nil {
				return err
			}
		}
		// Demo portfolio examples.
		_, err = tx.Exec(`INSERT INTO portfolio_items(student_user_id,title,description,category,skills,resource_url,is_public) VALUES($1,$2,$3,$4,$5,$6,TRUE)`, userID, "Учебный проект YOUR DAY", "Демонстрационный проект для портфолио студента.", "project", "Go, SQL, HTML, CSS", "")
		if err != nil {
			return err
		}
	}
	// A demo group conversation, including messages, membership and read markers.
	var conversationID int64
	if err := tx.QueryRow(`INSERT INTO conversations(conversation_type,title,created_by) VALUES('group','Демо-группа ИТ-1-24',$1) RETURNING id`, students[0]).Scan(&conversationID); err != nil {
		return err
	}
	memberIDs := append([]int64{}, students[:min(10, len(students))]...)
	// teachers contains teacher-table IDs, but conversation_members requires users.id.
	var teacherUserID int64
	if err := tx.QueryRow(`SELECT user_id FROM teachers WHERE id=$1`, teachers[0]).Scan(&teacherUserID); err != nil {
		return err
	}
	memberIDs = append(memberIDs, teacherUserID)
	for _, uid := range memberIDs {
		_, err := tx.Exec(`INSERT INTO conversation_members(conversation_id,user_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, conversationID, uid)
		if err != nil {
			return err
		}
	}
	msgSamples := []string{"Всем привет! Проверяем демо-функции портала YOUR DAY.", "Не забудьте посмотреть задания и расписание.", "Материалы по дисциплинам уже добавлены в демо-раздел.", "Если есть вопросы, напишите преподавателю в чате.", "Удачи в подготовке к модульному контролю!"}
	for i, body := range msgSamples {
		_, err := tx.Exec(`INSERT INTO messages(conversation_id,sender_id,body) VALUES($1,$2,$3)`, conversationID, memberIDs[i%len(memberIDs)], body)
		if err != nil {
			return err
		}
	}
	// Example AI assistant history per selected demo account.
	for _, uid := range students[:min(5, len(students))] {
		_, err := tx.Exec(`INSERT INTO ai_chat_messages(user_id,role,content) VALUES($1,'user',$2),($1,'assistant',$3)`, uid, "Объясни, как работает база данных.", "База данных хранит информацию в таблицах. SQL используется для добавления, чтения, изменения и удаления данных. В YOUR DAY можно практиковаться на демо-данных.")
		if err != nil {
			return err
		}
	}
	return nil
}

// seedSchedule creates three classes per day, Monday through Saturday, for each demo group.
// It replaces old schedule entries, making repeated runs idempotent.
func seedSchedule(tx *sql.Tx, assignments, groups []int64) error {
	if len(groups) == 0 || len(assignments) == 0 {
		return nil
	}
	for _, assignmentID := range assignments {
		if _, err := tx.Exec(`DELETE FROM schedule_entries WHERE assignment_id=$1`, assignmentID); err != nil {
			return err
		}
	}
	days := []int{1, 2, 3, 4, 5, 6}
	starts := []string{"09:00", "10:45", "13:00"}
	ends := []string{"10:30", "12:15", "14:30"}
	rooms := []string{"Аудитория 201", "Аудитория 203", "Аудитория 205", "Аудитория 301", "Аудитория 303", "Аудитория 305", "Лаборатория 1", "Лаборатория 2", "Лаборатория 3"}
	groupCount := len(groups)
	teacherByAssignment := make([]int64, len(assignments))
	for i, assignmentID := range assignments {
		if err := tx.QueryRow(`SELECT teacher_id FROM teacher_assignments WHERE id=$1`, assignmentID).Scan(&teacherByAssignment[i]); err != nil {
			return err
		}
	}
	for dayIndex, day := range days {
		for slot := 0; slot < 3; slot++ {
			usedTeachers := map[int64]bool{}
			usedRooms := map[string]bool{}
			for groupIndex := 0; groupIndex < groupCount; groupIndex++ {
				chosen := -1
				for offset := 0; offset < len(subjectNames); offset++ {
					subjectIndex := (dayIndex*3 + slot + groupIndex*2 + offset) % len(subjectNames)
					assignmentIndex := subjectIndex*groupCount + groupIndex
					if assignmentIndex >= len(assignments) {
						continue
					}
					teacherID := teacherByAssignment[assignmentIndex]
					if !usedTeachers[teacherID] {
						chosen = assignmentIndex
						usedTeachers[teacherID] = true
						break
					}
				}
				if chosen < 0 {
					return fmt.Errorf("не удалось назначить преподавателя: день %d, пара %d, группа %d", day, slot+1, groupIndex+1)
				}
				room := rooms[(dayIndex*3+slot+groupIndex)%len(rooms)]
				if usedRooms[room] {
					room = rooms[(dayIndex*3+slot+groupIndex+4)%len(rooms)]
				}
				usedRooms[room] = true
				_, err := tx.Exec(`INSERT INTO schedule_entries(assignment_id,day_of_week,start_time,end_time,room,lesson_type) VALUES($1,$2,$3,$4,$5,$6)`, assignments[chosen], day, starts[slot], ends[slot], room, lessonTypes[slot])
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func upsertUser(tx *sql.Tx, name, email, hash, role string) (int64, error) {
	var id int64
	err := tx.QueryRow(`INSERT INTO users(full_name,email,password_hash,role,is_active) VALUES($1,$2,$3,$4,TRUE) ON CONFLICT(email) DO UPDATE SET full_name=EXCLUDED.full_name, password_hash=EXCLUDED.password_hash, role=EXCLUDED.role, is_active=TRUE RETURNING id`, name, email, hash, role).Scan(&id)
	return id, err
}
