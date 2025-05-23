package database

import (
	"blackbox/internal/server/serverAPI"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"
	"unicode/utf8"

	_ "github.com/lib/pq"
)

type (

	// Информация о БД
	DB_Object struct {
		Ptr   *sql.DB      // указатель
		Close func() error // закрытие подключения
		isRun bool         // подключение активно
	}

	// Тип данных для передачи в БД
	StoreType struct {
		Dev   string      // наименование устройства предоставившего данные
		Name  string      // наименование переменной
		Value interface{} // значение переменной
		Qual  byte        // значение качества переменной
	}
)

// Подключение к БД. Функция возвращает ошибку, если подключеиться неудалось.
func (db *DB_Object) ConDB() error {

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_HOST_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"))

	// Подключение
	dbptr, err := sql.Open("postgres", dsn)
	if err != nil {
		return err
	}

	// Проверка подключения
	err = dbptr.Ping()
	if err != nil {
		return err
	}

	// Передача данных в экземпляр
	db.Ptr = dbptr

	// Функция закрытия подключения
	db.Close = func() error {
		err := db.Ptr.Close()
		return err
	}

	db.isRun = true

	return nil
}

// Проверка присутствия необходимых таблиц. Функция возвращает false, если таблицы не соответствуют перечню. Иначе - true
func (db *DB_Object) CheckTablesExist() (bool, error) {

	// Проверка присутствия таблицы - хост
	exist, err := tableExists(db, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_HOST"))
	if err != nil {
		return false, fmt.Errorf("ошибка проверки присутствия таблицы: %s", err)
	}
	if !exist {
		return false, fmt.Errorf("ошибка. нет такой таблицы: %s", err)
	}

	// Проверка присутствия таблицы - конфигурация
	exist, err = tableExists(db, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DEVICES"))
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке присутствия таблицы: %s", err)
	}
	if !exist {
		return false, fmt.Errorf("ошибка. нет такой таблицы: %s", err)
	}

	// Проверка присутствия таблицы - каналы
	exist, err = tableExists(db, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_TAGS"))
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке присутствия таблицы: %s", err)
	}
	if !exist {
		return false, fmt.Errorf("ошибка. нет такой таблицы: %s", err)
	}

	// Проверка присутствия таблицы - данные
	exist, err = tableExists(db, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"))
	if err != nil {
		return false, fmt.Errorf("ошибка при проверке присутствия таблицы: %s", err)
	}
	if !exist {
		return false, fmt.Errorf("ошибка. нет такой таблицы: %s", err)
	}

	return true, nil

}

// Функция выполняет создание таблиц в БД.
func (db *DB_Object) CreateTables() error {

	// Создание таблицы - пользователи
	Q := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		id SERIAL PRIMARY KEY NOT NULL,
		name VARCHAR(50) UNIQUE NOT NULL,
		password VARCHAR(64),
		token VARCHAR(64),
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	_, err := db.Ptr.Exec(Q)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - настройки хоста
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		id SERIAL PRIMARY KEY NOT NULL,
		host VARCHAR(50) NOT NULL,
		conType VARCHAR(50) NOT NULL,
		address VARCHAR(50) NOT NULL,
		port VARCHAR(50) NOT NULL,
		baudrate VARCHAR(7),
		databits VARCHAR(3),
		parity VARCHAR(5),
		stopbits VARCHAR(3),
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_HOST"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - устройства
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		id SERIAL PRIMARY KEY NOT NULL,
		device VARCHAR(50) NOT NULL,
		comment VARCHAR(50) NOT NULL,
		host VARCHAR(50) NOT NULL,
		type VARCHAR(50) NOT NULL,
		address VARCHAR(5) NOT NULL,
		ip VARCHAR(15) NOT NULL,
		port VARCHAR(5) NOT NULL,
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_DEVICES"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - тэги
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		id SERIAL PRIMARY KEY NOT NULL,
		device VARCHAR(50) NOT NULL,
		address VARCHAR(50) NOT NULL,
		datatype VARCHAR(50) NOT NULL,
		comment VARCHAR(100) NOT NULL,
		timeScan VARCHAR(30) NOT NULL,
		functype VARCHAR(30) NOT NULL,
		format VARCHAR(30) NOT NULL,
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_TAGS"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - архивные данные
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		id SERIAL PRIMARY KEY NOT NULL,
		dev VARCHAR(50) NOT NULL,
		name VARCHAR(50) NOT NULL,
		value NUMERIC NOT NULL,
		qual NUMERIC NOT NULL,
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_DATA"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Проверка присутствия созданных таблиц
	_, err = db.CheckTablesExist()
	if err != nil {
		return fmt.Errorf("ошибка проверки таблиц после их создания: %s", err)
	}

	return nil
}

// Внутренняя функция. Проверка присутствия таблицы по её имени.
func tableExists(db *DB_Object, schema, tableName string) (bool, error) {

	if db == nil || schema == "" || tableName == "" {
		return false, fmt.Errorf("ошибка в аргументах: БД{%v}, схема{%s}, таблица{%s}", db, schema, tableName)
	}

	var exists bool

	err := db.Ptr.QueryRow("SELECT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)",
		schema, tableName).Scan(&exists)

	if err != nil {
		return false, fmt.Errorf("ошибка при выполнении запроса: %v", err)
	}

	if !exists {
		return false, nil
	}

	return true, nil
}

// Функция создаёт пользователя admin, в таблице пользователей. Возвращается ошибка.
func (db *DB_Object) AddUserTableDB(name string) error {

	// Добавление пользователя admin
	Q := fmt.Sprintf("INSERT INTO %s.%s (name, password) VALUES ($1, $2)",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	_, err := db.Ptr.Exec(Q, name, "")
	if err != nil {
		return fmt.Errorf("ошибка добавления пользователя admin: {%v}", err)
	}

	return nil
}

// Проверка и установка пароля (если его нет), для учетной записи admin. Возвращается ошибка.
func (db *DB_Object) CheckSetUserPassword(name string) error {

	psw, err := db.ReadPswUser(name)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	if utf8.RuneCountInString(psw) == 64 {
		return nil
	}

	err = db.SetPswUser(name)
	if err != nil {
		return fmt.Errorf("%v", err)
	}

	return nil
}

// Чтение пароля пользователя. Возвращается хэш пароля и ошибка.
//
// Параметры:
//
// name - имя пользователя
func (db *DB_Object) ReadPswUser(name string) (psw string, err error) {

	q := fmt.Sprintf("SELECT password FROM %s.%s WHERE name = '%s'", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_USERS"), name)

	err = db.Ptr.QueryRow(q).Scan(&psw)
	if err != nil {
		return "", fmt.Errorf("ошибка: {%v} при чтении пароля пользователя: {%s}", err, name)
	}

	return psw, nil
}

// Установка пароля пользователя. Возвращается ошибка.
//
// Параметры:
//
// name - имя пользователя
func (db *DB_Object) SetPswUser(name string) error {

	var psw1, psw2, pswHash string

	fmt.Println()
	fmt.Printf("Установка пароля для пользователя: %s\n", name)

	for {
		fmt.Println("---")
		fmt.Print("Введите пароль: ")
		fmt.Scanln(&psw1)
		fmt.Print("Повторите пароль: ")
		fmt.Scanln(&psw2)
		if psw1 != psw2 {
			fmt.Println("Ошибка при вводе пароля. Повторите попытку.")
			continue
		}

		pswHash = fmt.Sprintf("%x", sha256.Sum256([]byte(psw1)))
		break
	}
	fmt.Println("---")

	q := fmt.Sprintf("UPDATE %s.%s SET password = '%s' WHERE name = '%s'",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"),
		pswHash,
		name)

	_, err := db.Ptr.Exec(q)
	if err != nil {
		return fmt.Errorf("ошибка {%v} при обновлении пароля у пользователя: {%s}", err, name)
	}

	return nil
}

// Функция организации чтения данных из БД. Возвращает ошибку.
//
// Параметры:
//
// data - данные для выполнения чтения
func ReadDataDB(data *serverAPI.DataDBCallT) error {

	// Проверка входных данных
	if data == nil {
		return errors.New("основная функция запросов -> принят пустой указатель")
	}
	if data.DB == nil {
		return errors.New("основная функция запросов -> нет указателя на БД")
	}

	limit := 100
	offset := 100
	sizeRx := limit
	cnt := 0

	// Запрос на количества строк по указанной дате
	err := ReadCntStrDataDB(data)
	if err != nil {
		return err
	}

	// Реализация запроса данных
	for sizeRx == limit {

		sizeRx, err = ReadDataDBReq(data, limit, offset*cnt)
		if err != nil {
			return err
		}
		cnt++
	}

	return nil
}

// Функция выполняет запрос с подсчётом количества строк по указанной дате. Возвращает ошибку.
func ReadCntStrDataDB(data *serverAPI.DataDBCallT) error {

	// Проверка входных данных
	if data == nil {
		return errors.New("принят пустой указатель")
	}

	q := fmt.Sprintf(`
	SELECT COUNT(*)
	FROM %s.%s 
	WHERE date(timestamp) = $1
	;`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"))

	err := data.DB.QueryRow(q, data.StartDate).Scan(&data.CntStrDB)
	if err != nil {
		return fmt.Errorf("ошибка при запросе количества строк по дате {%d}: {%v}", data.CntStrDB, err)
	}

	return nil
}

// Функция выполняет чтение из БД архивных данных, по начальной дате. Возвращается ошибка.
//
// Параметры:
//
// data - стартовая дата выборки и результат выборки.
// limit - количество строк выборки.
// offset - смещение выборки.
func ReadDataDBReq(data *serverAPI.DataDBCallT, limit, offset int) (size int, err error) {

	// Проверка аргументов
	if data == nil {
		return 0, fmt.Errorf("запрос данных. пустой указатель: {%v}", data)
	}
	if data.DB == nil {
		return 0, errors.New("запрос данных. нет указателя на БД")
	}
	rxDate, err := time.Parse("2006-01-02", data.StartDate)
	if err != nil {
		return 0, fmt.Errorf("запрос данных. значение начальной даты: {%s}", data.StartDate)
	}
	if limit < 1 {
		return 0, fmt.Errorf("запрос данных. значение limit:{%d} меньше 1", limit)
	}
	if offset < 0 {
		return 0, fmt.Errorf("запрос данных. значение offset:{%d} меньше 0", offset)
	}

	// Подготовка даты для запроса
	reqDate := rxDate.Format("2006-01-02")

	q := fmt.Sprintf(`
	 SELECT name, value, qual, timestamp
     FROM %s.%s
     WHERE date(timestamp) = '%v'
	 ORDER By timestamp ASC
	 LIMIT %d OFFSET %d
	 ;              
	`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"), reqDate, limit, offset)

	// Запрос
	rows, err := data.DB.Query(q)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// Обработка ответа
	cnt := 0
	localData := make([]serverAPI.DataElT, 0)

	for rows.Next() {
		var str serverAPI.DataElT

		err = rows.Scan(&str.Name, &str.Value, &str.Qual, &str.TimeStamp)
		if err != nil {
			return 0, err
		}
		localData = append(localData, str)
		cnt++
	}

	if err = rows.Err(); err != nil {
		return 0, err
	}

	// Передача локольного содержимого
	data.Data = append(data.Data, localData...)

	return cnt, nil
}
