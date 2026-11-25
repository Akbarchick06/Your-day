package main

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"strings"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("================================")
	fmt.Println("       MY EDU ADMIN SETUP")
	fmt.Println("================================")

	fmt.Print("ФИО администратора: ")
	name, _ := reader.ReadString('\n')
	name = strings.TrimSpace(name)

	fmt.Print("Email администратора: ")
	email, _ := reader.ReadString('\n')
	email = strings.TrimSpace(email)

	fmt.Print("Пароль администратора: ")
	password, _ := reader.ReadString('\n')
	password = strings.TrimSpace(password)

	if name == "" || email == "" || len(password) < 8 {
		fmt.Println("Ошибка: заполните все поля. Пароль должен содержать минимум 8 символов.")
		return
	}

	dbPassword := os.Getenv("DB_PASSWORD")
	if dbPassword == "" {
		fmt.Println("Не задан DB_PASSWORD.")
		return
	}

	hash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		fmt.Println("Ошибка создания хеша:", err)
		return
	}

	connStr := fmt.Sprintf(
		"host=localhost port=5432 user=postgres password=%s dbname=my_edu sslmode=disable",
		dbPassword,
	)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println("Ошибка подключения:", err)
		return
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fmt.Println("Ошибка PostgreSQL:", err)
		return
	}

	var id int64

	err = db.QueryRow(`
		INSERT INTO users (full_name, email, password_hash, role)
		VALUES ($1, $2, $3, 'admin')
		RETURNING id
	`, name, email, string(hash)).Scan(&id)

	if err != nil {
		fmt.Println("Не удалось создать администратора:", err)
		return
	}

	fmt.Println("--------------------------------")
	fmt.Println("Администратор успешно создан!")
	fmt.Println("ID:", id)
	fmt.Println("Email:", email)
	fmt.Println("Роль: admin")
	fmt.Println("--------------------------------")
}
