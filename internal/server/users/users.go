package users

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"syscall"

	"golang.org/x/term"
)

type (
	UsersT struct {
		DB    *sql.DB
		Users []UserT
	}

	UserT struct {
		Id       int
		Name     string
		Password string
		Token    string
	}
)

// Функция выводит меню действий для работы с пользователями.
func (el *UsersT) MenuActionsUsers() {
	fmt.Println()
	fmt.Println("1. Просмотр списка пользователей")
	fmt.Println("2. Добавление пользователя")
	fmt.Println("3. Удаление пользователя")
	fmt.Println("4. Изменение имени пользователя")
	fmt.Println("5. Изменение пароля пользователя")
	fmt.Println("6. Завершение работы")
	fmt.Print("Введите номер -> ")
}

// Функция скрывает пароль, при его вводе в терминале. Возвращает введённый пароль и ошибку.
func (el *UsersT) ReadTerminal() (pwd string, err error) {

	fd := int(syscall.Stdin)

	data, err := term.ReadPassword(fd)
	if err != nil {
		return "", errors.New("ошибка при чтении ввода пользователя")
	}

	pwd = string(data)
	return pwd, nil
}

// Функция выводит список пользователей. Возвращается ошибка
func (el *UsersT) ReqDataUsersDB() error {

	// Чтение конфигурации хоста
	q := fmt.Sprintf("SELECT id, name, password, token FROM %s.%s",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	rows, err := el.DB.Query(q)
	if err != nil {
		return fmt.Errorf("ошибка при чтении таблицы пользователей: {%v} ", err)
	}
	defer rows.Close()

	el.Users = []UserT{}

	for rows.Next() {

		var str UserT

		err = rows.Scan(&str.Id, &str.Name, &str.Password, &str.Token)
		if err != nil {
			return fmt.Errorf("ошибка при чтении очередной строки ответа, при запросе данных пользователей: {%v}", err)
		}
		el.Users = append(el.Users, str)
	}

	if err = rows.Err(); err != nil {
		return fmt.Errorf("ошибка при считывании строк у полученных данных пользователей: {%v}", err)
	}

	return nil
}

// Функция выводит в терминал информацию о пользователях.
func (el *UsersT) ShowDataUsers() {

	fmt.Println()
	fmt.Printf("Количество пользователей: %d\n", len(el.Users))
	for _, v := range el.Users {
		fmt.Printf("id:%d  name:%s  password:%s  token:%s\n", v.Id, v.Name, v.Password, v.Token)
	}
}

// Функция вычисляет хэш пароля. Возвращает ошибку.
func (el *UsersT) CalcHashPassword(password string) (hash string, err error) {

	if password == "" {
		return "", errors.New("при генерации хэш пароля, принят пустой пароль")
	}
	hash = fmt.Sprintf("%x", sha256.Sum256([]byte(password)))
	return hash, nil
}

// Функция добавляет пользователя в БД. Возвращается ошибка.
//
// Параметры:
//
// name - имя пользователя
// hashPwd  - хэш пароля пользователя
func (el *UsersT) AddUserDB(name, hashPwd string) error {

	// Проверка входных данных
	if name == "" || hashPwd == "" {
		return fmt.Errorf("ошибка входных данных при добавлении пользователя в БД. имя:{%s} хэш:{%s}", name, hashPwd)
	}
	if el.DB == nil {
		return errors.New("пустой указатель на БД при добавлении пользователя")
	}

	// Добавление пользователя
	q := fmt.Sprintf("INSERT INTO %s.%s (name, password, token) VALUES ($1, $2, $3)",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"),
	)

	_, err := el.DB.Exec(q, name, hashPwd, "")
	if err != nil {
		return fmt.Errorf("ошибка {%v} при добавлении пользователя {%v} в БД", err, name)
	}

	return nil
}

// Функция удаляет пользователя из БД по его id. Возвращается ошибка.
//
// Параметры:
//
// id - номер пользователя в БД
func (el *UsersT) DelUserDB(id int) error {

	// Проверка входных данных
	if id < 1 {
		return fmt.Errorf("ошибка в значении id {%d}, при удалении пользователя", id)
	}
	if el.DB == nil {
		return errors.New("пустой указатель на БД при удалении пользователя")
	}

	// Выполнение запроса
	q := fmt.Sprintf("DELETE FROM %s.%s WHERE id = $1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	_, err := el.DB.Exec(q, id)

	if err != nil {
		return fmt.Errorf("ошибка {%v} при удалении пользователя {%d} из БД", err, id)
	}

	return nil
}

// Функция изменяет имя пользователя в БД. Возвращается ошибка.
//
// Параметры:
//
// id - номер пользователя в БД
// name - имя пользователя
func (el *UsersT) ChgUserNameDB(id int, name string) error {

	// Проверка входных данных
	if id < 1 {
		return fmt.Errorf("ошибка в значении id {%d}, при изменении имени пользователя в БД", id)
	}
	if name == "" {
		return errors.New("принято пустое значение имени, при изменении имени пользователя в БД")
	}
	if el.DB == nil {
		return errors.New("пустой указатель на БД при изменении имени пользователя в БД")
	}

	// Выполнение запроса
	q := fmt.Sprintf("UPDATE %s.%s SET name = $1 WHERE id = $2",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	_, err := el.DB.Exec(q, name, id)
	if err != nil {
		return fmt.Errorf("ошибка при изменении имени пользователя по id={%d}, на имя {%s}: {%v}", id, name, err)
	}

	return nil
}

// Функция изменяет имя пользователя в БД. Возвращается ошибка.
//
// Параметры:
//
// id - номер пользователя в БД
// hashPwd - новый хэш пароля пользователя
func (el *UsersT) ChgUserPasswordDB(id int, hashPwd string) error {

	// Проверка входных данных
	if id < 1 {
		return fmt.Errorf("ошибка в значении id {%d}, при изменении пароля пользователя в БД", id)
	}
	if hashPwd == "" {
		return errors.New("принято пустое значение хэша пароля, при изменении пароля пользователя в БД")
	}
	if el.DB == nil {
		return errors.New("пустой указатель на БД при изменении пароля пользователя в БД")
	}

	// Выполнение запроса
	q := fmt.Sprintf("UPDATE %s.%s SET password = $1 WHERE id = $2",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	_, err := el.DB.Exec(q, hashPwd, id)
	if err != nil {
		return fmt.Errorf("ошибка при изменении пароля пользователя по id={%d}: {%v}", id, err)
	}

	return nil
}

// Функция получает имя пользователя по его id. Возвращает имя пользователя и ошибку.
//
// Параметры:
//
// id - id пользователя в БД
func (el *UsersT) UserNameByIdDB(id int) (name string, err error) {

	if id < 1 {
		return "", fmt.Errorf("получение имени пользоателя по id -> ошибка: id = {%d}", id)
	}

	q := fmt.Sprintf("SELECT name FROM %s.%s WHERE id=$1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	qRow := el.DB.QueryRow(q, id)

	err = qRow.Scan(&name)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении имени пользователя из ответа на запрос к БД: {%v}", err)
	}

	return name, nil
}

// Функция получает хэш пароля по его id. Возвращает хэш пароля и ошибку.
//
// Параметры:
//
// id - id пользователя в БД
func (el *UsersT) UserPasswordByIdDB(id int) (name string, err error) {

	if id < 1 {
		return "", fmt.Errorf("получение хэша пароля пользоателя, по id -> ошибка: id = {%d}", id)
	}

	q := fmt.Sprintf("SELECT password FROM %s.%s WHERE id=$1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	qRow := el.DB.QueryRow(q, id)

	err = qRow.Scan(&name)
	if err != nil {
		return "", fmt.Errorf("ошибка при чтении хэша пароля пользователя, из ответа на запрос к БД: {%v}", err)
	}

	return name, nil
}

// Функция запрашивает повторение меню. Возвращает true/false для повторения и ошибку.
func (el *UsersT) RepeatMenu() (b bool, err error) {

	var yn string

	for {
		fmt.Println()
		fmt.Print("Продолжить работу? (Y/N): ")
		_, err := fmt.Scanln(&yn)
		if err != nil {
			return false, fmt.Errorf("запрос повтора вывода меню -> ошибка:{%v}", err)
		}

		if yn == "Y" {
			return true, nil
		}
		if yn == "N" {
			return false, nil
		}

		fmt.Println("Ошибка при вводе.")
		continue
	}
}
