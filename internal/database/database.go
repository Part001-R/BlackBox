package database

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

type DB_Object struct {
	Ptr   *sql.DB
	Close func()
}

type DB_Intf interface {
	ConDB(db *DB_Object) error
}

// Подключение к БД.
// Функция возвращает ошибку, если подключеиться неудалось.
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
	db.Close = func() { _ = db.Ptr.Close() }

	return nil
}
