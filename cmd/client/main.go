package main

import (
	clientapi "blackbox/internal/client/clientAPI"
	libre "blackbox/internal/client/libre"
	"errors"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
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

	menu()

}

// Вывод меню действия
func menu() {
	var str string

	for {
		fmt.Println("---------------------------")
		fmt.Println("1: Вывод информации сервера")
		fmt.Println("2: Запросить данные сервера")
		fmt.Println("3: Завершение работы")
		fmt.Print("->")
		_, err := fmt.Scanln(&str)
		if err != nil {
			log.Fatal("Ошибка чтения введённых данных")
		}

		switch str {
		case "1":
			err := showStatusServer()
			if err != nil {
				fmt.Println("Ошибка:", err)
				fmt.Println("Работа прервана")
				return
			}
			continue

		case "2":
			fmt.Println()
			fmt.Print("Введите дату экспорта (YYYY-MM-DD): ")
			fmt.Scanln(&str)

			err := partDataDB(str) // Запрос данных с дроблением. Запрос по 100 строк, до завершения.
			if err != nil {
				fmt.Printf("Ошибка запроса архивных данных: {%v}\n", err)
				fmt.Println("Работа прервана")
				return
			}
			fmt.Println("Данные приняты")
			fmt.Println()
			continue

		case "3":
			return

		default:
			fmt.Println("Ошибка ввода")
			fmt.Println()
		}
	}
}

// Запрос состояния сервера и вывод в терминал. Функция возвращает ошибку.
func showStatusServer() error {

	statusSrv := clientapi.RxStatusSrv{}

	err := statusSrv.ReqStatusServer()
	if err != nil {
		return fmt.Errorf("ошибка при запросе состояния сервера: %v", err)
	}

	fmt.Println()
	fmt.Println("Время запуска сервера :", statusSrv.TimeStart)
	fmt.Println()

	fmt.Println("Интерфейсов Modbus-RTU:", len(statusSrv.MbRTU))
	fmt.Println("Интерфейсов Modbus-TCP:", len(statusSrv.MbTCP))
	fmt.Println()

	for i, v := range statusSrv.MbRTU {
		fmt.Printf("Интерфейс Modbus-RTU {%d}\n", i+1)
		fmt.Println("Имя :", v.ConName)
		fmt.Println("Порт:", v.Con)
		fmt.Println("Параметры:", v.ConParams)
	}
	fmt.Println()

	for i, v := range statusSrv.MbTCP {
		fmt.Printf("Интерфейс Modbus-TCP {%d}\n", i+1)
		fmt.Println("Имя :", v.ConName)
		fmt.Println("Порт:", v.Con)
	}
	fmt.Println()

	fmt.Printf("Размер в МБ файла логирования - Информация    :{%d}\n", statusSrv.SizeF.I)
	fmt.Printf("Размер в МБ файла логирования - Предупреждение:{%d}\n", statusSrv.SizeF.W)
	fmt.Printf("Размер в МБ файла логирования - Ошибки        :{%d}\n", statusSrv.SizeF.E)
	fmt.Println()

	return nil
}

// Функция создаёт xlsx файл и сохраняет туда принятые данные от сервера. Возвращает ошибку.
func savePartDataXlsx(rxData []clientapi.DataEl, startDate string) (err error) {

	tn := time.Now().Format("02.01.2006-15:04:05")

	// Создание файла
	fileName, err := libre.CreateXlsx("./", "exportData_"+startDate+"___", tn, ".xlsx")
	if err != nil {
		return fmt.Errorf("ошибка при создании xlsx файла экспорта: {%v}", err)
	}

	// Открытие файла
	file, err := excelize.OpenFile(fileName)
	if err != nil {
		return fmt.Errorf("ошибка при открытии файла: {%v}", fileName)
	}

	// Заполнение файла

	nameSheet := "DataDB"

	// Формирование заголовков
	// Name:	Value:	Quality:	TimeStamp:
	err = file.SetCellValue(nameSheet, "A1", "Name:")
	if err != nil {
		return errors.New("ошибка при добавлении заголовка столбца Name")
	}
	err = file.SetCellValue(nameSheet, "B1", "Value:")
	if err != nil {
		return errors.New("ошибка при добавлении заголовка столбца Value")
	}
	err = file.SetCellValue(nameSheet, "C1", "Quality:")
	if err != nil {
		return errors.New("ошибка при добавлении заголовка столбца Quality")
	}
	err = file.SetCellValue(nameSheet, "D1", "TimeStamp:")
	if err != nil {
		return errors.New("ошибка при добавлении заголовка столбца TimeStamp")
	}

	// Перенос данных
	for i, str := range rxData {

		i += 2

		err = file.SetCellValue(nameSheet, fmt.Sprintf("A%d", i), str.Name)
		if err != nil {
			return fmt.Errorf("ошибка {%v} добавления значения {%s} в ячейку {A%d}", err, str.Name, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("B%d", i), str.Value)
		if err != nil {
			return fmt.Errorf("ошибка {%v} добавления значения {%s} в ячейку {B%d}", err, str.Value, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("C%d", i), str.Qual)
		if err != nil {
			return fmt.Errorf("ошибка {%v} добавления значения {%s} в ячейку {C%d}", err, str.Qual, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("D%d", i), str.TimeStamp)
		if err != nil {
			return fmt.Errorf("ошибка {%v} добавления значения {%s} в ячейку {D%d}", err, str.TimeStamp, i)
		}
	}

	// Сохрангение
	err = file.Save()
	if err != nil {
		return errors.New("ошибка при сохранении Xlsx файла")
	}

	return nil
}

// Функция реализует очередь запросов на сервер для выгрузки исходных данных. Возвращает ошибку.
//
// Параметры:
//
// startDate - дата экспорта данных
func partDataDB(startDate string) error {

	// Проверка корректности ввода даты
	t, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("ошибка ввода даты: {%s}", t)
	}

	var dataDB clientapi.RxDataDB

	dataDB.StartDate = startDate

	// Запрос количества строк в БД по указанной дате
	err = dataDB.ReqCntStrByDateDB()
	if err != nil {
		return fmt.Errorf("ошибка запроса количества строк: {%v}", err)
	}

	fmt.Printf("в БД на {%s}, содержится {%s} записей\n", dataDB.StartDate, dataDB.CntStr)

	// Вычисление количества необходимых запросов
	cntStr, err := strconv.Atoi(dataDB.CntStr)
	if err != nil {
		return fmt.Errorf("ошибка преобразования значения количества строк из типа string {%s}, в int", dataDB.CntStr)
	}
	iter := cntStr / 100

	collectRxDataDB := make([]clientapi.PartDataDB, 0)

	// Запросы
	if iter == 0 {

		rxData, err := clientapi.ReqPartDataDB(0, 100, 0, startDate)
		if err != nil {
			return errors.New("ошибка при выполнении запроса при количестве строк < 100")
		}
		collectRxDataDB = append(collectRxDataDB, rxData)

	} else {

		for i := 0; i < iter; i++ {

			// отображение процентов выполнения получения данных от сервера
			percentage := float64(i+1) / float64(iter) * 100
			fmt.Printf("Получено данных: %.2f%%\r", percentage)

			rxData, err := clientapi.ReqPartDataDB(i, 100, 100*i, startDate)
			if err != nil {
				return fmt.Errorf("ошибка при выполнении запроса на итерации {%d}, {%v}", i, err)
			}
			collectRxDataDB = append(collectRxDataDB, rxData)

			time.Sleep(10 * time.Millisecond) // установка небольшой паузы между очередным запросом
		}
	}
	fmt.Println()

	// Подготовка данных для сохранения в xlsx
	forXlsx := make([]clientapi.DataEl, 0)

	for _, v := range collectRxDataDB {
		forXlsx = append(forXlsx, v.Data...)
	}

	// Сохранение принятых данных в xlsx
	err = savePartDataXlsx(forXlsx, startDate)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении принятых данных в xlsx: {%v}", err)
	}

	return nil
}
