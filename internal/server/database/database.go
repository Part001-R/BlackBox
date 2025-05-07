package database

import (
	"database/sql"
	"fmt"
	"os"

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
		return false, fmt.Errorf("ошибка проверки присутствия таблицы: %s", err)
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
func (db *DB_Object) CreateTables() (bool, error) {

	// Создание таблицы - настройки хоста
	Q := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
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
	`, os.Getenv("TABLE_HOST"))

	_, err := db.Ptr.Exec(Q)
	if err != nil {
		return false, fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - устройства
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
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
	`, os.Getenv("TABLE_DEVICES"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return false, fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - тэги
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
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
	`, os.Getenv("TABLE_TAGS"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return false, fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Создание таблицы - архивные данные
	Q = fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		id SERIAL PRIMARY KEY NOT NULL,
		dev VARCHAR(50) NOT NULL,
		name VARCHAR(50) NOT NULL,
		value NUMERIC NOT NULL,
		qual NUMERIC NOT NULL,
		timestamp TIMESTAMPTZ DEFAULT NOW()
	);
	`, os.Getenv("TABLE_DATA"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		return false, fmt.Errorf("ошибка при создании таблицы: %s", err)
	}

	// Проверка присутствия созданных таблиц
	ok, err := db.CheckTablesExist()
	if err != nil {
		return false, fmt.Errorf("ошибка проверки таблиц после их создания: %s", err)
	}
	if !ok {
		return false, nil
	}

	return true, nil
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
