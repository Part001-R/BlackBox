package main

import (
	clientapi "blackbox/internal/client/clientAPI"
	libre "blackbox/internal/client/libre"
	"errors"
	"fmt"
	"log"
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
	case "2":
		fmt.Println()
		fmt.Print("Введите дату экспорта (YYYY-MM-DD): ")
		fmt.Scanln(&str)

		err := expDataDB(str)
		if err != nil {
			fmt.Println("ошибка при экспорте данных из БД", err)
			fmt.Println("Работа прервана")
			return
		}

		fmt.Println("Экспорт данных выполнен")

	case "3":
		return

	default:
		fmt.Println("Ошибка ввода. Работа завершена")
		return
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

// Запрос архивных данных БД. Функция возвращает ошибку
func expDataDB(startDate string) error {

	// Проверка корректности ввода даты
	t, err := time.Parse("2006-01-02", startDate)
	if err != nil {
		return fmt.Errorf("ошибка ввода даты: {%s}", t)
	}

	var dataDB clientapi.RxDataDB

	dataDB.StartDate = startDate

	// Запрос архивных данных БД
	err = dataDB.ReqDataDB()
	if err != nil {
		return fmt.Errorf("ошибка запроса архивных данных ДБ: {%v}", err)
	}

	fmt.Printf("принято %s строк\n", dataDB.CntStr)

	// Формирование Exlx файла данных
	err = saveDataXlsx(dataDB)
	if err != nil {
		return fmt.Errorf("ошибка при сохранении данных в xlsx файл: {%v}", err)
	}

	return nil
}

// Функция зодаёт xlsx файл и сохраняет туда принятые данные от сервера. Возвращает ошибку.
func saveDataXlsx(data clientapi.RxDataDB) (err error) {

	tn := time.Now().Format("02.01.2006-15:04:05")

	// Создание файла
	fileName, err := libre.CreateXlsx("./", "exportData", tn, ".xlsx")
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
	for i, str := range data.Data {

		i++

		err = file.SetCellValue(nameSheet, fmt.Sprintf("A%d", i), str.Name)
		if err != nil {
			return fmt.Errorf("ошибка добавления значения {%s} в ячейку {A%d}", str.Name, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("B%d", i), str.Value)
		if err != nil {
			return fmt.Errorf("ошибка добавления значения {%s} в ячейку {B%d}", str.Value, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("C%d", i), str.Qual)
		if err != nil {
			return fmt.Errorf("ошибка добавления значения {%s} в ячейку {C%d}", str.Qual, i)
		}

		err = file.SetCellValue(nameSheet, fmt.Sprintf("D%d", i), str.TimeStamp)
		if err != nil {
			return fmt.Errorf("ошибка добавления значения {%s} в ячейку {D%d}", str.TimeStamp, i)
		}
	}

	// Сохрангение
	err = file.Save()
	if err != nil {
		return errors.New("ошибка при сохранении Xlsx файла")
	}

	return nil
}
