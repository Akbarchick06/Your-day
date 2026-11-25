package main

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

type AcademicPeriodData struct {
	ID           int64  `json:"id"`
	AcademicYear string `json:"academic_year"`
	Semester     int    `json:"semester"`
	PeriodType   string `json:"period_type"`
	Title        string `json:"title"`
	StartDate    string `json:"start_date"`
	EndDate      string `json:"end_date"`
	IsOpen       bool   `json:"is_open"`
}

type AcademicPeriodUpdate struct {
	AcademicYear *string `json:"academic_year"`
	Semester     *int    `json:"semester"`
	PeriodType   *string `json:"period_type"`
	Title        *string `json:"title"`
	StartDate    *string `json:"start_date"`
	EndDate      *string `json:"end_date"`
	IsOpen       *bool   `json:"is_open"`
}

func adminPeriods(w http.ResponseWriter, r *http.Request, id int64) {
	switch r.Method {
	case http.MethodGet:
		rows, err := db.Query(`
			SELECT id, academic_year, semester, period_type,
			       title, start_date::text, end_date::text, is_open
			FROM academic_periods
			ORDER BY academic_year DESC, semester, start_date
		`)
		if err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Не удалось получить учебные периоды")
			return
		}
		defer rows.Close()

		items := []AcademicPeriodData{}

		for rows.Next() {
			var item AcademicPeriodData

			if err := rows.Scan(
				&item.ID,
				&item.AcademicYear,
				&item.Semester,
				&item.PeriodType,
				&item.Title,
				&item.StartDate,
				&item.EndDate,
				&item.IsOpen,
			); err != nil {
				writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения учебных периодов")
				return
			}

			items = append(items, item)
		}

		if err := rows.Err(); err != nil {
			writeAdminError(w, http.StatusInternalServerError, "Ошибка чтения учебных периодов")
			return
		}

		writeAdminJSON(w, http.StatusOK, items)

	case http.MethodPost:
		var item AcademicPeriodData

		if readAdminJSON(r, &item) != nil {
			writeAdminError(w, http.StatusBadRequest, "Некорректные данные периода")
			return
		}

		item.AcademicYear = strings.TrimSpace(item.AcademicYear)
		item.Title = strings.TrimSpace(item.Title)

		if !validAcademicPeriod(item.AcademicYear, item.Semester, item.PeriodType, item.Title, item.StartDate, item.EndDate) {
			writeAdminError(w, http.StatusBadRequest, "Проверьте учебный год, семестр, тип периода, название и даты")
			return
		}

		err := db.QueryRow(`
			INSERT INTO academic_periods
				(academic_year, semester, period_type, title, start_date, end_date, is_open)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id
		`,
			item.AcademicYear,
			item.Semester,
			item.PeriodType,
			item.Title,
			item.StartDate,
			item.EndDate,
			item.IsOpen,
		).Scan(&item.ID)

		if err != nil {
			writeAdminError(w, http.StatusConflict, "Не удалось создать период. Возможно, такой период для этого учебного года и семестра уже существует")
			return
		}

		writeAdminJSON(w, http.StatusCreated, item)

	case http.MethodPut:
		if id <= 0 {
			writeAdminError(w, http.StatusBadRequest, "Не указан ID периода")
			return
		}

		var update AcademicPeriodUpdate

		if readAdminJSON(r, &update) != nil {
			writeAdminError(w, http.StatusBadRequest, "Некорректные данные")
			return
		}

		// Частичное обновление статуса: кнопки «Открыть» и «Закрыть».
		if update.IsOpen != nil &&
			update.AcademicYear == nil &&
			update.Semester == nil &&
			update.PeriodType == nil &&
			update.Title == nil &&
			update.StartDate == nil &&
			update.EndDate == nil {

			result, err := db.Exec(`
				UPDATE academic_periods
				SET is_open = $1
				WHERE id = $2
			`, *update.IsOpen, id)

			if err != nil {
				writeAdminError(w, http.StatusInternalServerError, "Не удалось изменить статус периода")
				return
			}

			count, _ := result.RowsAffected()
			if count == 0 {
				writeAdminError(w, http.StatusNotFound, "Период не найден")
				return
			}

			writeAdminJSON(w, http.StatusOK, map[string]string{
				"message": "Статус периода обновлён",
			})
			return
		}

		// Полное редактирование периода.
		if update.AcademicYear == nil ||
			update.Semester == nil ||
			update.PeriodType == nil ||
			update.Title == nil ||
			update.StartDate == nil ||
			update.EndDate == nil ||
			update.IsOpen == nil {

			writeAdminError(w, http.StatusBadRequest, "Для редактирования заполните все поля периода")
			return
		}

		year := strings.TrimSpace(*update.AcademicYear)
		title := strings.TrimSpace(*update.Title)

		if !validAcademicPeriod(year, *update.Semester, *update.PeriodType, title, *update.StartDate, *update.EndDate) {
			writeAdminError(w, http.StatusBadRequest, "Проверьте учебный год, семестр, тип периода, название и даты")
			return
		}

		result, err := db.Exec(`
			UPDATE academic_periods
			SET academic_year = $1,
			    semester = $2,
			    period_type = $3,
			    title = $4,
			    start_date = $5,
			    end_date = $6,
			    is_open = $7
			WHERE id = $8
		`,
			year,
			*update.Semester,
			*update.PeriodType,
			title,
			*update.StartDate,
			*update.EndDate,
			*update.IsOpen,
			id,
		)

		if err != nil {
			writeAdminError(w, http.StatusConflict, "Не удалось изменить период. Проверьте данные и отсутствие дубликатов")
			return
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, http.StatusNotFound, "Период не найден")
			return
		}

		writeAdminJSON(w, http.StatusOK, map[string]string{
			"message": "Период обновлён",
		})

	case http.MethodDelete:
		if id <= 0 {
			writeAdminError(w, http.StatusBadRequest, "Не указан ID периода")
			return
		}

		result, err := db.Exec(`
			DELETE FROM academic_periods
			WHERE id = $1
		`, id)

		if err != nil {
			writeAdminError(w, http.StatusConflict, "Не удалось удалить период")
			return
		}

		count, _ := result.RowsAffected()
		if count == 0 {
			writeAdminError(w, http.StatusNotFound, "Период не найден")
			return
		}

		writeAdminJSON(w, http.StatusOK, map[string]string{
			"message": "Период удалён",
		})

	default:
		writeAdminError(w, http.StatusMethodNotAllowed, "Метод не поддерживается")
	}
}

func validAcademicPeriod(year string, semester int, periodType, title, startDate, endDate string) bool {
	if year == "" || title == "" {
		return false
	}

	if semester != 1 && semester != 2 {
		return false
	}

	switch periodType {
	case "module1", "module2", "exam":
	default:
		return false
	}

	start, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return false
	}

	end, err := time.Parse("2006-01-02", endDate)
	if err != nil {
		return false
	}

	return !end.Before(start)
}

// Используется для проверки корректного ID в URL при необходимости.
func academicPeriodID(value string) (int64, bool) {
	id, err := strconv.ParseInt(value, 10, 64)
	return id, err == nil && id > 0
}
