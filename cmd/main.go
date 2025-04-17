package main

import (
	"blackbox/internal/database"
	"blackbox/internal/libre"
	loger "blackbox/internal/loger"
	"log"
	"os"

	"github.com/joho/godotenv"
)

var (
	db  database.DB_Object
	lgr loger.Log_Object
	cnf libre.ConfXLSX_Object
)

func main() {

	// Чтение переменных окружения
	err := godotenv.Load("configs/.env")
	if err != nil {
		log.Fatal(err)
	}

	// Создание логеров
	err = lgr.CreateOpenLog()
	if err != nil {
		log.Fatal(err)
	}
	defer lgr.CloseAll()
	lgr.I.Println("логер запущен")

	// Подключение к БД
	err = db.ConDB()
	if err != nil {
		lgr.E.Println("неудалось подключиться к БД")
		os.Exit(1)
	}
	defer db.Close()
	lgr.I.Println("подключение к БД выполнено")

	// Открытие конфигурационного файла
	err = cnf.Read()
	if err != nil {
		lgr.E.Println("неудалось открыть файл конфигурации", err)
		os.Exit(1)
	}
	lgr.I.Println("чтение конфигурационного файла выполнено")

	//modbustcpmaster.Test()

}
