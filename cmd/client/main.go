package main

import (
	clientapi "blackbox/internal/client/clientAPI"
	"fmt"
	"log"

	"github.com/joho/godotenv"
)

func main() {

	prepare()
	run()

}

// Подготовительные действия
func prepare() {

	err := godotenv.Load("./configs/.env")
	if err != nil {
		log.Fatal("ошибка чтения переменных окружения:", err)
	}
}

// Работа
func run() {

	reqStatusServer() // запрос состояния сервера
}

// Запрос состояния сервера
func reqStatusServer() {

	var statusSrv clientapi.RxJson

	err := statusSrv.StatusServer()
	if err != nil {
		log.Fatalf("ошибка при запросе состояния сервера: %v", err)
	}

	if statusSrv.DriverDB == "true" && statusSrv.DriverModbusTCP == "true" && statusSrv.QueueModbusTCP == "true" {
		fmt.Println("сервер работает")
	} else {
		fmt.Println("отказ сервера!")
	}

}
