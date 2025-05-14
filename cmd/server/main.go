package main

import (
	"blackbox/internal/server/database"
	"blackbox/internal/server/libre"
	loger "blackbox/internal/server/loger"
	modbusrtumaster "blackbox/internal/server/modbusRTUmaster"
	modbustcpmaster "blackbox/internal/server/modbusTCPmaster"
	serverAPI "blackbox/internal/server/serverAPI"
	"context"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
)

type (
	// коннекты
	connects struct {
		mbTCPmaster []modbustcpmaster.Connect
		mbRTUmaster []modbusrtumaster.Connect
	}

	// набор данных для запуска Go рутин
	goInst struct {
		db      []iGoDB
		queue   []iGoQueue
		drMbTCP []iGoDrModbusTCP
		drMbRTU []iGoDrModbusRTU
	}
	// Go - набор данных для запуска БД
	iGoDB struct {
		name string
		ctx  context.Context
		lgr  loger.Log_Object
		chRx chan []database.StoreType
		wg   *sync.WaitGroup
	}
	// Go  - набор данных для запуска потока очереди запросов
	iGoQueue struct {
		name   string
		ctx    context.Context
		lgr    loger.Log_Object
		chTxDr chan []libre.ChConfExt_Export
		wg     *sync.WaitGroup
		cnf    libre.ConfXLSX_Export
	}

	// Go - набор данных для запуска драйвера Modbus-TCP
	iGoDrModbusTCP struct {
		name   string
		ctx    context.Context
		lgr    loger.Log_Object
		con    modbustcpmaster.Connect
		chRxDr chan []libre.ChConfExt_Export
		chTxDB chan []database.StoreType
		wg     *sync.WaitGroup
	}
	// Go - набор данных для запуска драйвера Modbus-RTU
	iGoDrModbusRTU struct {
		name   string
		ctx    context.Context
		lgr    loger.Log_Object
		con    modbusrtumaster.Connect
		chRxDr chan []libre.ChConfExt_Export
		chTxDB chan []database.StoreType
		wg     *sync.WaitGroup
	}
)

var (
	db           database.DB_Object
	lgr          loger.Log_Object
	cnfImport    libre.ConfXLSX_Import
	cmdArgs      map[string][]string
	srvInfo      serverAPI.StatusServerT
	hostConnects connects
	httpMutex    sync.Mutex
)

const (
	maxEthernetDev = 2 // Ограничение на количество устройств Ethernet у хоста
	maxCOMDev      = 4 // Ограничение на количество устройств COM у хоста
)

// Точка входа
func main() {

	// подготовительные действия (открытие, создание, проверка и т.п.)
	prepare()

	// завершение работы (закрытие, отключение и т.п.)
	defer fin()

	// основная логика
	run()

}

// Подготовительные действия перед запуском приложения
func prepare() {

	// Фиксация времени запуска
	timeStart := time.Now()
	srvInfo.TimeStart = timeStart.Format("02-01-2006 15:04:05")

	// Чтение переменных окружения
	err := godotenv.Load("./configs/.env")
	if err != nil {
		log.Fatal(err)
	}

	// инициализация слайсов коннектов хоста
	hostConnects.mbRTUmaster = make([]modbusrtumaster.Connect, 0, maxCOMDev)
	hostConnects.mbTCPmaster = make([]modbustcpmaster.Connect, 0, maxEthernetDev)

	// Создание логеров
	err = lgr.CreateOpenLog()
	if err != nil {
		log.Fatal("ошибка при создании логеров", err)
	}
	lgr.I.Println("логер запущен")

	// Подключение к БД
	err = db.ConDB()
	if err != nil {
		lgr.E.Println("неудалось подключиться к БД")
		os.Exit(1)
	}
	lgr.I.Println("подключение к БД выполнено")

	// Заполнение мапы аргументов командной строки
	// Проверка набора аргументов командной строки
	cmdArgs = make(map[string][]string)
	cmdArgs["--run"] = []string{}
	cmdArgs["--do"] = []string{"DB-check", "DB-create", "DB-import", "DB-export", "DB-erase", "Xlsx-show"}

	err = checkArgs(os.Args)
	if err != nil {
		lgr.E.Println("ошибка в наборе аргументов командной строки при запуске:", os.Args)
		log.Fatal("проверьте корректность ввода командной строки ...")
	}
	lgr.I.Println("приложение запущено с командной строкой:", os.Args)

}

// Действия перед закрытием приложения
func fin() {

	// закрытие подключения БД
	err := db.Close()
	if err != nil {
		lgr.E.Println("ошибка закрытия подключения к БД: ", err)
	}

	// закрытие подключения Modbus-TCP
	for _, c := range hostConnects.mbTCPmaster {

		if c.IsRun {
			err := c.Close()
			if err != nil {
				lgr.E.Printf("ошибка: {%v} при закрытии подключения: {%s} Modbus-TCP:\n ", err, c.Name)
			}
		}
	}

	// Закрытие подключения Modbus-RTU
	for _, c := range hostConnects.mbRTUmaster {

		if c.IsRun {
			err := c.Close()
			if err != nil {
				lgr.E.Printf("ошибка: {%v} при закрытии подключения: {%s} Modbus-RTU:\n ", err, c.Name)
			}
		}
	}

	// закрытие логеров
	err = lgr.CloseAll()
	if err != nil {
		log.Fatal("ошибка при закрытии логеров:", err)
	}

}

// Основная логика
func run() {

	// Получение списка аргументов командной строки
	slArg, err := readArgs(os.Args)
	if err != nil {
		lgr.E.Println("ошибка в получении аргументов")
		fmt.Println("Работа прервана.")
		os.Exit(1)
	}

	// определение запускаемой функции согласно аргументов командной строки
	//
	switch slArg[0] {
	case "--run":
		fullRun() // полный запуск приложения (опрос + архивирование в БД + HTTP(S))

	case "--do":
		switch slArg[1] {
		case "DB-check":
			doDBcheck() // проверка БД

		case "DB-create":
			doDBcreate() // создание таблиц в БД

		case "DB-import":
			doDBimport() // передача конфигурации в БД

		case "DB-export":
			doDBexport() // экспорт конфигурации из БД

		case "DB-erase":
			doEraseDB() // очистка конфигурационных таблиц БД

		case "USER-show":

		case "USER-create":

		case "USER-edit":

		case "USER-delete":

		case "Xlsx-show":
			doXlsxShow() // вывод содержимого конфигурационного файла в терминал
		}

	default:
		lgr.E.Println("ошибка в аргументах командной строки при запуске приложения")
		os.Exit(1)
	}
}

// Функция выполняет проверку аргументов командной строки при запуске приложения. Возвращает ошибку.
func checkArgs(arg []string) error {

	if len(arg) < 2 {
		return fmt.Errorf("%v", arg)
	}

	for i, a := range arg {

		// поиск ключа в мапе
		if _, ok := cmdArgs[a]; ok {

			// если текущий индекс аргумента последний
			// проверка значения ключа на nil
			if i+1 == len(arg) {
				return nil
			}

			// если текущий индекс аргумента предпоследний
			// поиск в слайсе соответствия следующего аргумента
			if i+2 == len(arg) {

				list := cmdArgs[a]
				for _, addrReg := range list {

					if addrReg == arg[i+1] {
						return nil
					}
				}
			}
		}

	}

	return fmt.Errorf("%v", arg)
}

// Функция выполняет проверку аргументов командной строки при запуске приложения. Возвращает аргументы командной строки и ошибку.
//
// arg - аргументы командной строки
func readArgs(arg []string) (argSl []string, err error) {

	if len(arg) < 2 {
		return nil, fmt.Errorf("%v", arg)
	}

	for i, a := range arg {

		// поиск ключа в мапе
		if _, ok := cmdArgs[a]; ok {

			// если текущий индекс аргумента последний
			// проверка значения ключа на nil
			// возврат слайса с одного элемента
			if i+1 == len(arg) {

				argSl = append(argSl, arg[i])
				return argSl, nil
			}

			// если текущий индекс аргумента предпоследний
			// поиск в слайсе соответствия следующего аргумента
			// формирование слайса из двух элементов
			if i+2 == len(arg) {

				list := cmdArgs[a]
				for _, addrReg := range list {

					if addrReg == arg[i+1] {
						argSl = append(argSl, arg[i])
						argSl = append(argSl, arg[i+1])
						return argSl, nil
					}
				}
			}
		}

	}

	return nil, fmt.Errorf("%v", arg)
}

// Функция полноценного запуска приложения.
func fullRun() {

	// Чтение конфигурации из БД
	//
	cnfExport, err := rdConfDataDB()
	if err != nil {
		lgr.E.Println("ошибка при запуске опроса: ", err)
		fmt.Println("Работа прервана.")
		return
	}

	// Создание коннектов хоста
	//
	hostConnects, err = createConnect(cnfExport)
	if err != nil {
		lgr.E.Println("ошибка создания коннекта хоста: ", err)
		fmt.Println("Работа прервана.")
		return
	}

	// Подготовка данных для запуска Go рутин
	//
	goWait := sync.WaitGroup{}
	ctxStop, cancel := context.WithCancel(context.Background())
	defer cancel()

	dataGo, err := buildGoData(ctxStop, cnfExport, &hostConnects, &goWait, lgr)
	if err != nil {
		lgr.E.Printf("ошибка при подготовке данных для Go рутин: {%v}", err)
		return
	}

	// Запуск Go рутин
	//
	err = goStart(&dataGo)
	if err != nil {
		lgr.E.Println("ошибка при запуске Go рутин: ", err)
		fmt.Println("работа прервана.")
		return
	}

	// Запуск https сервера (для внешнего клиента)
	//
	if os.Getenv("HTTPS_SERVER_USE") == "true" {
		go goHttpsServer()
	}

	// Запуск http сервера (для локального клиента)
	//
	httpServer()

	fmt.Println("Завершение работы приложения")
	cancel()      // закрытие контекста
	goWait.Wait() // ожидание завершения запущенных Go рутин

}

// Функция проверки БД.
func doDBcheck() {

	ok, err := db.CheckTablesExist()
	if err != nil {
		lgr.E.Println("ошибка при проверке таблиц: ", err)
	}

	lgr.I.Println("выполнена проверка таблиц:", ok)

	if ok {
		fmt.Println("ok")
		return
	}
	fmt.Println("bad")
}

// Функция для создания таблиц в БД.
func doDBcreate() {

	// Предварительная проверка присутствия таблиц
	ok, _ := db.CheckTablesExist()
	if ok {
		fmt.Println("Таблицы БД присутствуют. Работа прервана")
		return
	}

	// Создание таблиц
	err := db.CreateTables()
	if err != nil {
		lgr.E.Println("ошибка при создании таблиц: ", err)
		fmt.Println("bad")
		return
	}
	lgr.I.Println("выполнено создание таблиц")

	// Добавление пользователя admin
	var user = "admin"

	err = db.AddUserTableDB(user)
	if err != nil {
		lgr.E.Println("ошибка при добавлении пользователя admin: ", err)
		fmt.Println("bad")
		return
	}

	// Установка пароля пользователю admin
	err = db.CheckSetUserPassword(user)
	if err != nil {
		lgr.E.Println("ошибка установки пароля admin:", err)
		fmt.Println("bad")
		return
	}

	lgr.I.Println("Таблицы БД созданы")
	lgr.I.Println("добавлен пользователь admin, установлен пароль")
	fmt.Println("ok")
}

// Функция для импорта конфигурации в БД.
func doDBimport() {

	// Очистка таблиц БД
	err := eraseTablesDB()
	if err != nil {
		lgr.E.Println("ошибка в очистке БД перед импортом: ", err)
		os.Exit(1)
	}

	// Чтение конфигурационного файла xlsx
	err = cnfImport.ReadImport()
	if err != nil {
		lgr.E.Println("ошибка при чтении файла конфигурации: ", err)
		os.Exit(1)
	}
	lgr.I.Println("чтение конфигурационного файла - выполнено")

	// Проверка присутствия таблиц в БД
	ok, err := db.CheckTablesExist()
	if err != nil {
		lgr.E.Println("ошибка при проверке конфигурационных таблиц БД: ", err)
		os.Exit(1)
	}

	lgr.I.Println("выполнена проверка таблиц БД: ", ok)

	// Передача данных конфигурации в БД
	err = wrConfDataDB()
	if err != nil {
		lgr.E.Println("ошибка при импорте конфигурации в БД: ", err)
		err = eraseTablesDB()
		if err != nil {
			lgr.E.Println("ошибка при очистке таблиц БД: ", err)
		}
		log.Fatal("ошибка при импорте конфигурации в БД")
	}
	lgr.I.Println("импорт конфигурации в БД - выполнено")

	fmt.Println("ok")
}

// Функция вывода содержимого конфигурационного файла в терминал
func doXlsxShow() {

	// Конфигурационный файл (тест)
	err := cnfImport.ReadImport()
	if err != nil {
		lgr.E.Println("ошибка при чтении файла конфигурации: ", err)
		os.Exit(1)
	}

	lgr.I.Println("чтение конфигурационного файла выполнено")

	cnfImport.ShowImport()
	fmt.Println("ok")

}

// Функция очистки конфигурационных таблиц в БД
func doEraseDB() {

	err := eraseTablesDB()
	if err != nil {
		lgr.E.Println("ошибка при очистке конфигурационных таблиц БД:", err)
	}

	lgr.I.Println("очистка конфигурационных таблиц БД выполнена")
	fmt.Println("ok")
}

// Функция для экспорта конфигурации из БД.
func doDBexport() {

	// Чтение конфигурационных таблиц БД
	conf, err := rdConfDataDB()
	if err != nil {
		lgr.E.Println("ошибка чтения данных конфигурации из БД: ", err)
		os.Exit(1)
	}

	// Создание нового файла xlsx для сохранения данных экспорта
	tn := time.Now().Format("02.01.2006-15:04:05")

	fileExport, err := conf.CreateXlsx(os.Getenv("EXPORT_FILE_PATH"), os.Getenv("EXPORT_FILE_NAME"), tn, os.Getenv("EXPORT_FILE_TYPE"))
	if err != nil {
		lgr.E.Println("ошибка при создании файла экспорта: ", err)
		os.Exit(1)
	}

	// Заполнение файла экспорта
	err = fillFileExport(fileExport, conf)
	if err != nil {
		lgr.E.Println("ошибка при заполнении файла экспорта: ", err)
		os.Exit(1)
	}

	lgr.I.Println("чтение конфигурации из БД выполнено успешно")
	fmt.Println("ok")

}

// Передача конфигурационных данных в таблицы БД. Функция возвращает ошибку.
func wrConfDataDB() error {

	// Запись конфигурации хоста
	for _, el := range cnfImport.SheetMain_Header {

		q := fmt.Sprintf("INSERT INTO %s.%s (host, contype, address, port, baudrate, databits, parity, stopbits) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)",
			os.Getenv("TABLE_SCHEMA"),
			os.Getenv("TABLE_HOST"))

		_, err := db.Ptr.Exec(q, el.Host, el.ConType, el.Address, el.Port, el.BaudRate, el.DataBits, el.Parity, el.StopBits)
		if err != nil {
			lgr.E.Printf("ошибка {%v} при записи строки конфигурации хоста {%v}\n", err, el)
			return err
		}

		lgr.I.Println("в БД добавлена строка конфигурации хоста: ", el)
	}

	// Запись настроек подключений
	for _, el := range cnfImport.SheetMain_Dev {

		q := fmt.Sprintf("INSERT INTO %s.%s (device, comment, host, type, address, ip, port) VALUES ($1, $2, $3, $4, $5, $6, $7)",
			os.Getenv("TABLE_SCHEMA"),
			os.Getenv("TABLE_DEVICES"))

		_, err := db.Ptr.Exec(q, el.Device, el.Comment, el.Host, el.Type_, el.Address, el.IP, el.Port)
		if err != nil {
			lgr.E.Printf("ошибка {%v} при записи строки конфигурации подключения устройства {%v}\n", err, el)
			return err
		}

		lgr.I.Println("в БД добавлена строка конфигурации подключения устройства: ", el)
	}

	// Запись настроек конфигурации тэгов
	//
	// Проход по устройствам
	for _, d := range cnfImport.SheetsDev {

		// Проход по настройкам конфигурации каналов устройства
		for _, ch := range d.Conf {

			q := fmt.Sprintf("INSERT INTO %s.%s (device, address, datatype, comment, timescan, functype, format ) VALUES ($1, $2, $3, $4, $5, $6, $7)",
				os.Getenv("TABLE_SCHEMA"),
				os.Getenv("TABLE_TAGS"))

			_, err := db.Ptr.Exec(q, d.Name, ch.Address, ch.DataType, ch.Comment, ch.TimeScan, ch.FuncType, ch.Format)
			if err != nil {
				lgr.E.Printf("ошибка {%v} при записи строки конфигурации канала {%v}\n", err, ch)
				return err
			}

			lgr.I.Println("в БД добавлена строка конфигурации канала: ", d.Name, ch.Address, ch.DataType, ch.Comment, ch.TimeScan, ch.FuncType, ch.Format)
		}

		lgr.I.Println("в БД добавлена конфигурация каналов для устройства: ", d.Name)
	}

	return nil
}

// Чтение конфигурации БД. Возвращается ошибка.
func rdConfDataDB() (conf libre.ConfXLSX_Export, err error) {

	// Чтение конфигурации хоста
	q := fmt.Sprintf("SELECT host, contype, address, port, baudrate, databits, parity, stopbits FROM %s.%s", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_HOST"))

	rows, err := db.Ptr.Query(q)
	if err != nil {
		lgr.E.Println("ошибка при чтении таблицы host: ", err)
		return libre.ConfXLSX_Export{}, err
	}
	defer rows.Close()

	for rows.Next() {

		var str libre.SheetMain_Head

		err = rows.Scan(&str.Host, &str.ConType, &str.Address, &str.Port, &str.BaudRate, &str.DataBits, &str.Parity, &str.StopBits)
		if err != nil {
			return libre.ConfXLSX_Export{}, errors.New(err.Error())
		}

		conf.SheetMain_Header = append(conf.SheetMain_Header, str)

	}

	if err = rows.Err(); err != nil {
		return libre.ConfXLSX_Export{}, errors.New(err.Error())
	}

	// Чтение конфигурации устройств
	q = fmt.Sprintf("SELECT device, comment, host, type, address, ip, port FROM %s.%s", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DEVICES"))

	rows, err = db.Ptr.Query(q)
	if err != nil {
		lgr.E.Println("ошибка при чтении таблицы host: ", err)
		return libre.ConfXLSX_Export{}, err
	}

	for rows.Next() {

		var str libre.SheetMain_Dev

		err = rows.Scan(&str.Device, &str.Comment, &str.Host, &str.Type_, &str.Address, &str.IP, &str.Port)
		if err != nil {
			return libre.ConfXLSX_Export{}, errors.New(err.Error())
		}

		conf.SheetMain_Dev = append(conf.SheetMain_Dev, str)
	}

	if err = rows.Err(); err != nil {
		return libre.ConfXLSX_Export{}, errors.New(err.Error())
	}

	// Чтение конфигурации каналов
	q = fmt.Sprintf("SELECT device, address, datatype, comment, timescan, functype, format FROM %s.%s",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_TAGS"))

	rows, err = db.Ptr.Query(q)
	if err != nil {
		lgr.E.Println("ошибка при чтении таблицы host: ", err)
		return libre.ConfXLSX_Export{}, err
	}

	for rows.Next() {

		var str libre.ChConf_Export

		err = rows.Scan(&str.Device, &str.Address, &str.DataType, &str.Comment, &str.TimeScan, &str.FuncType, &str.Format)
		if err != nil {
			return libre.ConfXLSX_Export{}, errors.New(err.Error())
		}

		conf.SheetChan = append(conf.SheetChan, str)
	}

	if err = rows.Err(); err != nil {
		return libre.ConfXLSX_Export{}, errors.New(err.Error())
	}

	conf.ConfDataReady = true // установка признака, что экспорт данных выполнен успешно
	return conf, nil
}

// Чтение данных каналов из ДБ по временной метке. Возвращается массив записей БД и ошибку.
//
// Параметры:
//
// timeScan - время опроса.
func rdChanByDevNameAndTimeScanDB(name string, timeScan int) (ch []libre.ChConf_Export, err error) {

	// Чтение конфигурации каналов
	ts := fmt.Sprintf("'%d'", timeScan)

	Q := fmt.Sprintf("SELECT device, address, datatype, comment, timescan, functype, format FROM %[1]s.%[2]s WHERE timescan=%[3]s AND device='%[4]s'",
		os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_TAGS"), ts, name)

	rows, err := db.Ptr.Query(Q)
	if err != nil {
		lgr.E.Println("ошибка при чтении таблицы host: ", err)
		return []libre.ChConf_Export{}, err
	}

	for rows.Next() {

		var str libre.ChConf_Export

		err = rows.Scan(&str.Device, &str.Address, &str.DataType, &str.Comment, &str.TimeScan, &str.FuncType, &str.Format)
		if err != nil {
			lgr.E.Println("ошибка при сканировании строки ответа БД: ", err)
			return []libre.ChConf_Export{}, err
		}

		ch = append(ch, str)
	}

	if err = rows.Err(); err != nil {
		lgr.E.Println("ошибка сканера строк ответа БД: ", err)
		return []libre.ChConf_Export{}, err
	}

	return ch, nil
}

// Очистка таблиц БД. Возвращается ошибка.
func eraseTablesDB() error {

	// Очистка содержимого таблицы host
	Q := fmt.Sprintf("TRUNCATE TABLE %s.%s", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_HOST"))

	_, err := db.Ptr.Exec(Q)
	if err != nil {
		lgr.E.Printf("ошибка {%v} при очистки таблицы host\n", err)
		return err
	}

	// Очистка содержимого таблицы devices
	Q = fmt.Sprintf("TRUNCATE TABLE %s.%s", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DEVICES"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		lgr.E.Printf("ошибка {%v} при очистки таблицы devices\n", err)
		return err
	}

	// Очистка содержимого таблицы devices
	Q = fmt.Sprintf("TRUNCATE TABLE %s.%s", os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_TAGS"))

	_, err = db.Ptr.Exec(Q)
	if err != nil {
		lgr.E.Printf("ошибка {%v} при очистки таблицы channels\n", err)
		return err
	}

	return nil

}

// Заполнение файла экспорта. Возвращается ошибка.
//
// Параметры:
//
// fileName - имя файла экспорта.
func fillFileExport(fileName string, conf libre.ConfXLSX_Export) error {

	cntRow := 1 // инициализация счётчика строк единицей, т.к. номера строк начинаются с единицы

	// Проверка содержимого данных экспорта
	if len(conf.SheetMain_Header) < 1 {
		return errors.New("нет данных конфигурации хоста при сохранении в файл экспорта")
	}

	if len(conf.SheetMain_Dev) < 1 {
		return errors.New("нет данных конфигурации устройств при сохранении в файл экспорта")
	}

	if len(conf.SheetChan) < 1 {
		return errors.New("нет данных конфигурации каналов при сохранении в файл экспорта")
	}

	file, err := excelize.OpenFile(fileName)
	if err != nil {
		return err
	}

	// Перенос заголовков хоста
	// Host:	ConType:	Address:	Port:
	err = file.SetCellValue("Main", "A1", "Host:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "B1", "ConType:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "C1", "Address:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "D1", "Port:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "E1", "BaudRate:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "F1", "DataBits:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "G1", "Parity:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", "H1", "StopBits:")
	if err != nil {
		return err
	}

	cntRow++

	// Перенос содержимого настроек хоста
	for _, str := range conf.SheetMain_Header {

		err = file.SetCellValue("Main", fmt.Sprintf("A%d", cntRow), str.Host)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("B%d", cntRow), str.ConType)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("C%d", cntRow), str.Address)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("D%d", cntRow), str.Port)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("E%d", cntRow), str.BaudRate)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("F%d", cntRow), str.DataBits)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("G%d", cntRow), str.Parity)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("H%d", cntRow), str.StopBits)
		if err != nil {
			return err
		}

		cntRow++
	}

	// Формирование пустой строки
	cntRow++

	// Перенос заголовков настройки устройств
	// Device:	Comment:	Host:   Type:	Address:
	err = file.SetCellValue("Main", fmt.Sprintf("A%d", cntRow), "Device:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", fmt.Sprintf("B%d", cntRow), "Comment:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", fmt.Sprintf("C%d", cntRow), "Host:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", fmt.Sprintf("D%d", cntRow), "Type:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Main", fmt.Sprintf("E%d", cntRow), "Address:")
	if err != nil {
		return err
	}

	cntRow++

	// Перенос содержимого настроек устройств
	for _, str := range conf.SheetMain_Dev {

		err = file.SetCellValue("Main", fmt.Sprintf("A%d", cntRow), str.Device)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("B%d", cntRow), str.Comment)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("C%d", cntRow), str.Host)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("D%d", cntRow), str.Type_)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Main", fmt.Sprintf("E%d", cntRow), str.Address)
		if err != nil {
			return err
		}

		cntRow++
	}

	// Перенос заголовков настройки устройств
	// Device: 	Address:	DataType:	Comment:   TimeScan:

	cntRow = 1

	err = file.SetCellValue("Channels", fmt.Sprintf("A%d", cntRow), "Device:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("B%d", cntRow), "Address:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("C%d", cntRow), "DataType:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("D%d", cntRow), "Comment:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("E%d", cntRow), "TimeScan:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("F%d", cntRow), "FuncType:")
	if err != nil {
		return err
	}

	err = file.SetCellValue("Channels", fmt.Sprintf("G%d", cntRow), "Format:")
	if err != nil {
		return err
	}

	cntRow++

	// Перенос содержимого настроек каналов
	for _, str := range conf.SheetChan {

		err = file.SetCellValue("Channels", fmt.Sprintf("A%d", cntRow), str.Device)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("B%d", cntRow), str.Address)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("C%d", cntRow), str.DataType)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("D%d", cntRow), str.Comment)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("E%d", cntRow), str.TimeScan)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("F%d", cntRow), str.FuncType)
		if err != nil {
			return err
		}

		err = file.SetCellValue("Channels", fmt.Sprintf("G%d", cntRow), str.Format)
		if err != nil {
			return err
		}

		cntRow++
	}

	// Сохрангение
	err = file.Save()
	if err != nil {
		return err
	}

	return nil
}

// Создание коннектов хоста. Возвращается коннект и ошибка.
//
// Параметры:
//
// listCon - список коннектов хорста из файла конфигурации
func createHostModbusTCPConn(conf libre.ConfXLSX_Export) (con []modbustcpmaster.Connect, err error) {

	// проверка входных данных
	if len(conf.SheetMain_Header) == 0 || len(conf.SheetMain_Dev) == 0 {
		return []modbustcpmaster.Connect{}, fmt.Errorf("нет конфигурационных данных для создания Modbus-TCP коннекта")
	}

	// поисх в заголовках соединения с типом TCP
	// добавление подключения в массив TCP подключений
	for _, c := range conf.SheetMain_Header {

		if c.ConType == "TCP" {

			// присвоение параметров подключения (скорость передачи, биты и т.д.)
			varCon, err := buildConfDataSlaveModbusTCP(conf, c.Host)
			if err != nil {
				return []modbustcpmaster.Connect{}, fmt.Errorf("ошибка параметрирования коннекта {%v}", err)
			}

			// добавление параметрироанного подключения к общему списку коннектов
			con = append(con, varCon)
		}
	}

	// подключение всех параметрированных коннектов
	for i, c := range con {
		_ = c

		err = con[i].Connect()
		if err != nil {
			return []modbustcpmaster.Connect{}, fmt.Errorf("ошибка при подключении: {%s} по COM: %v", con[i].Name, err)
		}
		con[i].IsRun = true
	}

	return con, nil
}

// Создание коннектов хоста. Возвращается коннект и ошибка.
//
// Параметры:
//
// conf - конфигурация
func createHostModbusRTUConn(conf libre.ConfXLSX_Export) (con []modbusrtumaster.Connect, err error) {

	// проверка входных данных
	if len(conf.SheetMain_Dev) == 0 || len(conf.SheetMain_Header) == 0 {
		return []modbusrtumaster.Connect{}, fmt.Errorf("нет конфигурационных данных для создания Modbus-RTU коннекта")
	}

	// поисх в заголовках соединения с типом COM
	// добавление подключения в массив СОМ подключений
	for _, c := range conf.SheetMain_Header {

		if c.ConType == "COM" {

			// присвоение параметров подключения (скорость передачи, биты и т.д.)
			varCon, err := buildConfDataSlaveModbusRTU(conf, c.Host)
			if err != nil {
				return []modbusrtumaster.Connect{}, fmt.Errorf("ошибка параметрирования коннекта {%v}", err)
			}

			// добавление параметрироанного подключения к общему списку коннектов
			con = append(con, varCon)
		}
	}

	// подключение всех параметрированных коннектов
	for i, c := range con {
		_ = c

		err = con[i].Connect()
		if err != nil {
			return []modbusrtumaster.Connect{}, fmt.Errorf("ошибка при подключении: {%s} по COM: %v", con[i].Name, err)
		}
		con[i].IsRun = true
	}

	return con, nil
}

// Создание коннектов хоста. Функция возвращает ошибку
//
// Параметры:
//
// hostCon - указатель на коннекты хоста.
// cnf - данные конфигурации.
func createConnect(cnf libre.ConfXLSX_Export) (hostCon connects, err error) {

	// Создание Modbus-TCP мастера
	//
	hostCon.mbTCPmaster, err = createHostModbusTCPConn(cnf)
	if err != nil {
		return connects{}, fmt.Errorf("ошибка при создании клиента Modbus-TCP хоста: {%v}", err)
	}

	// Создание Modbus-RTU мастера
	//
	hostCon.mbRTUmaster, err = createHostModbusRTUConn(cnf)
	if err != nil {
		return connects{}, fmt.Errorf("ошибка при создании клиента Modbus-RTU хоста: {%v}", err)
	}

	return hostCon, nil
}

// Функция создаёт экземпляры, для запуска Go рутин. Возвращается набор данных и ошибка
//
// Парметры:
//
// ctxStop - контекст для остановки
// cnf - конфигурация
// channels - каналы
// hostConnects - коннекты хоста
// wg - группа ожидания
// lgr - логер
func buildGoData(ctxStop context.Context, cnf libre.ConfXLSX_Export, hostConnects *connects, wg *sync.WaitGroup, lgr loger.Log_Object) (inst goInst, err error) {

	// DB
	iGoDB := iGoDB{
		name: "DB:Main",
		ctx:  ctxStop,
		lgr:  lgr,
		chRx: make(chan []database.StoreType, 10),
		wg:   wg,
	}
	inst.db = append(inst.db, iGoDB)

	// Queue
	for _, v := range cnf.SheetMain_Header {
		iGoQueue := iGoQueue{
			name:   "Queue:" + v.Host + ":" + v.ConType,
			ctx:    ctxStop,
			lgr:    lgr,
			chTxDr: make(chan []libre.ChConfExt_Export, 10),
			wg:     wg,
			cnf:    cnf,
		}
		inst.queue = append(inst.queue, iGoQueue)
	}

	// Driver
	for _, v := range inst.queue {

		sl := strings.Split(v.name, ":")
		if len(sl) != 3 {
			return goInst{}, fmt.Errorf("(код 1) ошибка в длинне имени: {%s}", v.name)
		}

		switch sl[2] {
		case "TCP":
			// определение коннекта их списка хост коннектов по номеру коннекта
			conn := modbustcpmaster.Connect{}
			ok := false
			for _, v := range hostConnects.mbTCPmaster {
				if sl[1] == v.Name {
					conn = v
					ok = true
				}
			}
			if !ok {
				return goInst{}, fmt.Errorf("нет присвоения коннекта ТСР: {%s}", v.name)
			}

			// сборка и добавление
			iGoDrModbusTCP := iGoDrModbusTCP{
				name:   "DriverModbusTCP:" + sl[1] + ":" + sl[2],
				ctx:    ctxStop,
				lgr:    lgr,
				con:    conn,
				chRxDr: v.chTxDr,
				chTxDB: inst.db[0].chRx,
				wg:     wg,
			}
			inst.drMbTCP = append(inst.drMbTCP, iGoDrModbusTCP)

		case "COM":
			// определение коннекта их списка хост коннектов по номеру коннекта
			conn := modbusrtumaster.Connect{}
			ok := false
			for _, v := range hostConnects.mbRTUmaster {
				if sl[1] == v.Name {
					conn = v
					ok = true
				}
			}
			if !ok {
				return goInst{}, fmt.Errorf("нет присвоения коннекта COM: {%s}", v.name)
			}

			// сборка и добавление
			iGoDrModbusRTU := iGoDrModbusRTU{
				name:   "DriverModbusRTU:" + sl[1] + ":" + sl[2],
				ctx:    ctxStop,
				lgr:    lgr,
				con:    conn,
				chRxDr: v.chTxDr,
				chTxDB: inst.db[0].chRx,
				wg:     wg,
			}
			inst.drMbRTU = append(inst.drMbRTU, iGoDrModbusRTU)

		default:
			return goInst{}, fmt.Errorf("нет нужного совпадения: {%s}", sl[2])
		}
	}

	return inst, nil
}

// Запуск Go рутин. Функция возвращает ошибку.
//
// Параметры:
//
// data - набор данных для запуска Go рутин
func goStart(data *goInst) error {

	// БД
	for _, v := range data.db {
		go goDriverDB(v.ctx, v.chRx, v.wg)
	}

	// goDriverModbusTCP
	for _, v := range data.drMbTCP {

		sl := strings.Split(v.name, ":")
		if len(sl) != 3 {
			return fmt.Errorf("нет соответствия в длинне наименования, при запуске goDriverModbusTCP: {%s} ", v.name)
		}

		switch sl[0] {
		case "DriverModbusTCP":
			go goDriverModbusTCP(v.ctx, v.lgr, v.con, v.chRxDr, v.chTxDB, v.wg)

		default:
			return fmt.Errorf("нет распознанного наименования (код 1): {%s} ", sl[0])
		}
	}

	// goDriverModbusRTU
	for _, v := range data.drMbRTU {

		sl := strings.Split(v.name, ":")
		if len(sl) != 3 {
			return fmt.Errorf("нет соответствия в длинне наименования, при запуске goDriverModbusRTU: {%s} ", v.name)
		}

		switch sl[0] {

		case "DriverModbusRTU":
			go goDriverModbusRTU(v.ctx, v.lgr, v.con, v.chRxDr, v.chTxDB, v.wg)

		default:
			return fmt.Errorf("нет распознанного наименования (код 2): {%s} ", sl[0])
		}
	}

	// goQueueForModbusTCP
	// goQueueForModbusRTU
	for _, v := range data.queue {

		sl := strings.Split(v.name, ":")
		if len(sl) != 3 {
			return fmt.Errorf("нет соответствия в длинне наименования, при запуске goQueue: {%s} ", v.name)
		}

		switch sl[2] {
		case "TCP":
			go goQueueForModbusTCP(v.ctx, v.cnf, v.chTxDr, v.wg)

		case "COM":
			go goQueueForModbusRTU(v.ctx, v.name, v.cnf, v.chTxDr, v.wg)

		default:
			return fmt.Errorf("нет распознанного наименования (код 2): {%s} ", sl[0])
		}
	}

	return nil

}

// Подготовка конфигурационных параметров слейва Modbus-RTU, перед коннектом. Возвращается RTU коннект и ошибка.
//
// Параметры:
//
// conf - конфигурация
// nameCon - имя коннекта
func buildConfDataSlaveModbusRTU(conf libre.ConfXLSX_Export, nameCon string) (mbRTUcon modbusrtumaster.Connect, err error) {

	mbRTUcon.Name = nameCon

	for _, dev := range conf.SheetMain_Header {

		if dev.Host == nameCon {

			// порт подключения
			for _, host := range conf.SheetMain_Header {

				if host.Host == mbRTUcon.Name {
					mbRTUcon.Port = os.Getenv("COM_PORT_PATH") + host.Port
				}
			}
			if mbRTUcon.Port == "" {
				return modbusrtumaster.Connect{}, fmt.Errorf("не найден СОМ порт подключения для {%s}", nameCon)
			}

			// сетевой адрес
			for _, v := range conf.SheetMain_Dev {

				if v.Host == nameCon {

					vInt, err := strconv.Atoi(v.Address)
					if err != nil {
						lgr.E.Println("создание Modbus-RTU коннекта. ошибка преобразования Port в число")
						return modbusrtumaster.Connect{}, err
					}
					mbRTUcon.SlaveAddr = byte(vInt)
					break
				}
			}

			// скорость передачи данных
			mbRTUcon.ParamsConn.BaudRate, err = strconv.Atoi(dev.BaudRate)
			if err != nil {
				lgr.E.Println("создание Modbus-RTU коннекта. ошибка преобразования BoudRate в число")
				return modbusrtumaster.Connect{}, err
			}

			// биты данных
			mbRTUcon.ParamsConn.DataBits, err = strconv.Atoi(dev.DataBits)
			if err != nil {
				lgr.E.Println("создание Modbus-RTU коннекта. ошибка преобразования DataBits в число")
				return modbusrtumaster.Connect{}, err
			}

			// четность
			mbRTUcon.ParamsConn.Parity = dev.Parity

			// стоп биты
			mbRTUcon.ParamsConn.StopBits, err = strconv.Atoi(dev.StopBits)
			if err != nil {
				lgr.E.Println("создание Modbus-RTU коннекта. ошибка преобразования StopBits в число")
				return modbusrtumaster.Connect{}, err
			}
		}
	}

	return mbRTUcon, nil
}

// Подготовка конфигурационных параметров слейва Modbus-TCP, перед коннектом. Возвращается TCP коннект и ошибка.
//
// Параметры:
//
// conf - конфигурация
// nameCon - имя коннекта
func buildConfDataSlaveModbusTCP(conf libre.ConfXLSX_Export, nameCon string) (mbTCPcon modbustcpmaster.Connect, err error) {

	mbTCPcon.Name = nameCon

	var cnt = 0
	// получение данных хоста
	for _, con := range conf.SheetMain_Header {
		if con.Host == nameCon {
			mbTCPcon.HostIP = con.Address
			mbTCPcon.HostPort = con.Port
			cnt++
		}
	}

	// получение данных устройства
	for _, con := range conf.SheetMain_Dev {
		if con.Host == nameCon {
			mbTCPcon.SlaveIP = con.IP
			mbTCPcon.SlavePort = con.Port
			cnt++
		}
	}

	// проверка результата выборки данных
	if cnt != 2 {
		return modbustcpmaster.Connect{}, fmt.Errorf("ошибка считывания данных IP и Port перед установкой TCP коннекта")
	}
	return mbTCPcon, nil
}

// Go. Сохранение данных в БД
//
// Параметры:
//
// ctx - контекст для завершения работы
// chStore - канал приёма данных для сохранения в БД
// wg - WaitGroup для отслеживания завершения Go рутины
func goDriverDB(ctx context.Context, chStore <-chan []database.StoreType, wg *sync.WaitGroup) {

	wg.Add(1)

	defer func() {
		wg.Done()
	}()

	for {
		select {
		// Завершение работы Go рутины
		case <-ctx.Done():
			return
		// Приём очередных данных
		case newReq, ok := <-chStore:

			if ok {
				// Запись конфигурации хоста
				for _, el := range newReq {

					Q := fmt.Sprintf("INSERT INTO %s.%s (dev, name, value, qual) VALUES ($1, $2, $3, $4)",
						os.Getenv("TABLE_SCHEMA"),
						os.Getenv("TABLE_DATA"))

					_, err := db.Ptr.Exec(Q, el.Dev, el.Name, el.Value, el.Qual)
					if err != nil {
						lgr.E.Printf("goDriverDB. ошибка [%v] записи данных [%v] в БД \n", err, el)
						os.Exit(1)
					}
				}

			} else {
				lgr.E.Println("goDriverDB. закрыт канал чтения запросов")
				os.Exit(1)
			}
		// Нет событий
		default:
			time.Sleep(100 * time.Microsecond)
		}
	}

}

// Go. Опрос устройств по Modbus-TCP
//
// Параметры:
//
// ctx - контекст для завершения работы
// lgr - логер
// con - коннект
// chForModbusTCP - канал приёма запросов на опрос
// chForDB - канал передачи данных на архивирование в Go БД
func goDriverModbusTCP(ctx context.Context, lgr loger.Log_Object, con modbustcpmaster.Connect, chForModbusTCP <-chan []libre.ChConfExt_Export, chForDB chan<- []database.StoreType, wg *sync.WaitGroup) {

	wg.Add(1)

	defer func() {
		wg.Done()
	}()

	if con.Client == nil {
		lgr.E.Println("Modbus-TCP драйвер. отсутствует подключение по:", con.Name)
		os.Exit(1)
	}

	// ожидание поступления новых данных в канал
	//
	for {
		select {

		//Завершение работы по контексту
		case <-ctx.Done():
			return

		case newReq, ok := <-chForModbusTCP: // приём новых данных для запроса
			if !ok {
				lgr.E.Println("GoDriverModbusTCP. Закрыт канал чтения запросов.")
				os.Exit(1)
			}

			slRx := make([]database.StoreType, 0)

			for _, v := range newReq {

				// подготовка данных для запроса
				slaveID, address, quantity, err := prepareDataClientModbusTCP(v)
				if err != nil {
					lgr.E.Printf("ошибка [%v] в преобразовании входных данных [%v]\n", err, v)
					os.Exit(1)
				}

				rx := database.StoreType{}

				// запрос
				rxUint16, rxByte, err := selectFuncMbTCPDo(con, v.FuncType, slaveID, address, quantity)

				// Обработка результата запроса
				switch v.FuncType {
				case "ReadHoldingRegisters", "ReadInputRegisters":
					if err != nil {
						lgr.W.Printf("ошибка {%v} при запросе Modbus-TCP: слейв {%d}, функция {%s}, адрес регистра {%d}, количество регистров {%d} ", err, slaveID, v.FuncType, address, quantity)
						rx.Value = 0
						rx.Qual = 0
					} else {
						val, err := buildValFromUint16(rxUint16, v.DataType, v.Format)
						if err != nil {
							lgr.E.Println("ошибка в обработке принятых данных Modbus-TCP (код 1): ", err)
							os.Exit(1)
						}
						rx.Value = val
						rx.Qual = 1
					}
					rx.Dev = v.DeviceName
					rx.Name = v.Comment
					slRx = append(slRx, rx)

				case "ReadDiscreteInputs", "ReadCoil":
					if err != nil {
						lgr.W.Printf("ошибка {%v} при запросе Modbus-TCP: слейв {%d}, функция {%s}, адрес регистра {%d}, количество регистров {%d} ", err, slaveID, v.FuncType, address, quantity)
						rx.Value = 0
						rx.Qual = 0
					} else {
						val, err := buildValFromByte(rxByte, v.DataType, v.Format)
						if err != nil {
							lgr.E.Println("ошибка в обработке принятых данных Modbus-TCP (код 2): ", err)
							os.Exit(1)
						}
						rx.Value = val
						rx.Qual = 1
					}
					rx.Dev = v.DeviceName
					rx.Name = v.Comment
					slRx = append(slRx, rx)

				default:
					lgr.E.Println("ошибка распознавания функции: ", v.FuncType)
					os.Exit(1)
				}

			}

			// передача сформированного слайса в канал
			chForDB <- slRx

		// ведение опроса слева
		default:
			time.Sleep(time.Microsecond * 1)
		}
	}

}

// Go. Опрос устройств по Modbus-RTU
//
// Параметры:
//
// ctx - контекст для завершения работы
// lgr - логер
// con - коннект
// chForModbusRTU - канал приёма запросов на опрос
// chForDB - канал передачи данных на архивирование в Go БД
func goDriverModbusRTU(ctx context.Context, lgr loger.Log_Object, con modbusrtumaster.Connect, chForModbusRTU <-chan []libre.ChConfExt_Export, chForDB chan<- []database.StoreType, wg *sync.WaitGroup) {

	wg.Add(1)

	defer func() {
		wg.Done()
	}()

	if con.Client == nil {
		lgr.E.Println("Modbus-RTU драйвер. отсутствует подключение по:", con.Name)
		os.Exit(1)
	}

	// ожидание поступления новых данных в канал
	//
	for {
		select {

		// Завершение работы по контексту
		case <-ctx.Done():
			return

		case newReq, ok := <-chForModbusRTU: // приём новых данных для запроса
			if !ok {
				lgr.E.Println("GoDriverModbusTCP. Закрыт канал чтения запросов.")
				os.Exit(1)
			}

			slRx := make([]database.StoreType, 0)

			// Выполнение запросов согласно принятым данным
			for _, v := range newReq {

				// подготовка данных для запроса
				slaveID, address, quantity, err := prepareDataClientModbusRTU(v)
				if err != nil {
					lgr.E.Printf("ошибка [%v] в преобразовании входных данных [%v]\n", err, v)
					os.Exit(1)
				}

				rx := database.StoreType{}

				// запрос
				rxByte, err := selectFuncMbRTUDo(con, v.FuncType, slaveID, address, quantity)
				if err != nil {
					lgr.W.Printf("ошибка {%v} при запросе Modbus-RTU: слейв {%d}, функция {%s}, адрес регистра {%d}, количество регистров {%d} ", err, slaveID, v.FuncType, address, quantity)
					rx.Value = 0
					rx.Qual = 0
				} else {
					val, err := buildValFromByte(rxByte, v.DataType, v.Format)
					if err != nil {
						lgr.E.Println("ошибка в обработке принятых данных Modbus-RTU: ", err)
						os.Exit(1)
					}
					rx.Value = val
					rx.Qual = 1
				}
				rx.Dev = v.DeviceName
				rx.Name = v.Comment
				slRx = append(slRx, rx)
			}

			// передача сформированного слайса в канал
			chForDB <- slRx

		// ведение опроса слева
		default:
			time.Sleep(time.Microsecond * 1)
		}
	}
}

// Go. Формирование очериди опроса для драйвера Modbus-TCP
//
// Параметры:
//
// ctx - контекст, для завершения работы
// cnf - конфигурация, для формирования запросов
// forModbusTCP - канал, для передачи запросов в драйвер Modbus-TCP
// wg - учёт Go
func goQueueForModbusTCP(ctx context.Context, cnf libre.ConfXLSX_Export, forModbusTCP chan<- []libre.ChConfExt_Export, wg *sync.WaitGroup) {

	wg.Add(1)

	defer func() {
		wg.Done()
	}()

	// Определение перечня разных временных меток опроса
	//
	listTimeScan := make(map[string]int)
	listChByTimeScan := make(map[int][]libre.ChConf_Export)
	listChByTimeScanExt := make(map[int][]libre.ChConfExt_Export)

	_ = listChByTimeScanExt // переменная используется, но выдаётся сообщение, что нет (сделано обращение)

	// фиксация первой в списке записи временной метки опроса
	t, err := strconv.Atoi(cnf.SheetChan[0].TimeScan)
	if err != nil {
		lgr.E.Println("ошибка 1 в преобразовании строки в число: ", cnf.SheetChan[0].TimeScan)
		os.Exit(1)
	}

	listTimeScan[cnf.SheetChan[0].TimeScan] = t

	// поиск временных меток отличных от списка
	for _, v := range cnf.SheetChan {

		if _, ok := listTimeScan[v.TimeScan]; !ok {

			t, err := strconv.Atoi(v.TimeScan)
			if err != nil {
				lgr.E.Println("ошибка 2 в преобразовании строки в число: ", v.TimeScan)
				os.Exit(1)
			}
			listTimeScan[v.TimeScan] = t
		}
	}

	// Запросы к БД. Получение спосков каналов с привязкой ко времени опроса
	//
	for k, v := range listTimeScan {

		resp, err := rdChanByDevNameAndTimeScanDB("Dev1", v) //=========== измени привязку имени в аргументах (сейчас заглушка)
		if err != nil {
			lgr.E.Println("работа Go рутины goQueueModbusTCP завершена из-за ошибки: ", err)
			os.Exit(1)
		}

		kInt, err := strconv.Atoi(k)
		if err != nil {
			lgr.E.Println("работа Go рутины goQueueModbusTCP завершена из-за ошибки преобразовании: ", err)
			os.Exit(1)
		}

		// добавление записи в мапу по ключу времени сканирования
		listChByTimeScan[kInt] = resp
	}

	// В полученных списках, замена имён устройств на их адреса
	//

	// получение соответствий имен к их адресам
	nameAddr := make(map[string]string)

	for _, v := range cnf.SheetMain_Dev {

		if _, ok := nameAddr[v.Device]; !ok {
			nameAddr[v.Device] = v.Address
		}
	}

	// перенос данных полученных мап в расширенную
	listChByTimeScanExt = moveToExt(listChByTimeScan)

	// замена в подготовленных списках имени устройста на его сетевой адрес
	for k, v := range listChByTimeScanExt {

		for kk, vv := range v {

			listChByTimeScanExt[k][kk].DeviceAddr = nameAddr[vv.DeviceName]
		}
	}

	// Создание таймер-тиков, для формирования тиков передачи данных в канал
	//

	// подготовка слайса для создания таймер-тикеров
	slTime := make([]int, 0)

	for _, v := range listTimeScan {
		slTime = append(slTime, v)
	}

	// Создание таймер-тикеров
	slTicker := createTickers(slTime)

	// Запуск Go рутин для каждого таймер-тика
	for i, ticker := range slTicker {

		wg.Add(1)

		go func(ctx context.Context, wg *sync.WaitGroup, t *time.Ticker, data []libre.ChConfExt_Export) {

			for {
				select {
				case <-t.C:
					forModbusTCP <- data // передача данных в канал
				case <-ctx.Done():
					wg.Done()
					return
				default:
					time.Sleep(time.Microsecond * 1)
				}
			}
		}(ctx, wg, ticker, listChByTimeScanExt[slTime[i]])
	}

	// генерация запущена
	// данные передаются в драйвер Modbus-TCP

}

// Go. Формирование очериди опроса для драйвера Modbus-RTU
//
// Параметры:
//
// ctx - контекст, для завершения работы
// cnf - конфигурация, для формирования запросов
// forModbusRTU - канал, для передачи запросов в драйвер Modbus-RTU
// wg - учёт Go
func goQueueForModbusRTU(ctx context.Context, name string, cnf libre.ConfXLSX_Export, forModbusRTU chan []libre.ChConfExt_Export, wg *sync.WaitGroup) {

	wg.Add(1)

	defer func() {
		wg.Done()
	}()

	// Определение наименований устройств, по типу коннекта
	//
	// выделение имени хост коннекта
	n := strings.Split(name, ":")
	if len(n) != 3 {
		lgr.E.Println("ошибка при запуске goQueueForModbusRTU -> нет соответствия в длинне имени:", name)
		return
	}
	if n[1] == "" {
		lgr.E.Println("ошибка при запуске goQueueForModbusRTU -> имя коннекта пустое:", name)
		return
	}
	hostCon := n[1]

	// определение наименований устройств сконфигурированных на этот коннект
	listDev := make([]string, 0)
	for _, v := range cnf.SheetMain_Dev {
		if hostCon == v.Host {
			listDev = append(listDev, v.Device)
		}
	}

	// Определение перечня разных временных меток опроса
	//
	listTimeScan := make(map[string]int)
	listChByTimeScan := make(map[int][]libre.ChConf_Export)
	listChByTimeScanExt := make(map[int][]libre.ChConfExt_Export)

	_ = listChByTimeScanExt // переменная используется, но выдаётся сообщение, что нет (сделано дополнительное обращение)

	// фиксация первой в списке записи временной метки опроса
	t, err := strconv.Atoi(cnf.SheetChan[0].TimeScan)
	if err != nil {
		lgr.E.Println("ошибка 1 в преобразовании строки в число: ", cnf.SheetChan[0].TimeScan)
		os.Exit(1)
	}

	listTimeScan[cnf.SheetChan[0].TimeScan] = t

	// поиск временных меток отличных от списка
	for _, v := range cnf.SheetChan {

		if _, ok := listTimeScan[v.TimeScan]; !ok {

			t, err := strconv.Atoi(v.TimeScan)
			if err != nil {
				lgr.E.Println("ошибка 2 в преобразовании строки в число: ", v.TimeScan)
				os.Exit(1)
			}
			listTimeScan[v.TimeScan] = t
		}
	}

	// Запросы к БД. Получение спосков каналов с привязкой ко времени опроса
	// Учитываются все устройства настроенные на данный коннект
	for _, nameD := range listDev {

		for k, v := range listTimeScan {

			resp, err := rdChanByDevNameAndTimeScanDB(nameD, v)
			if err != nil {
				lgr.E.Println("работа Go рутины goQueueModbusTCP завершена из-за ошибки в запросе к БД: ", err)
				os.Exit(1)
			}

			kInt, err := strconv.Atoi(k)
			if err != nil {
				lgr.E.Println("работа Go рутины goQueueModbusTCP завершена из-за ошибки преобразовании: ", err)
				os.Exit(1)
			}

			// добавление записи в мапу по ключу времени сканирования
			listChByTimeScan[kInt] = append(listChByTimeScan[kInt], resp...)
		}
	}

	// В полученных списках, замена имён устройств на их адреса
	//

	// получение соответствий имен к их адресам
	nameAddr := make(map[string]string)

	for _, v := range cnf.SheetMain_Dev {

		if _, ok := nameAddr[v.Device]; !ok {
			nameAddr[v.Device] = v.Address
		}
	}

	// перенос данных полученных мап в расширенную
	listChByTimeScanExt = moveToExt(listChByTimeScan)

	// замена в подготовленных списках имени устройста на его сетевой адрес
	for k, v := range listChByTimeScanExt {

		for kk, vv := range v {

			listChByTimeScanExt[k][kk].DeviceAddr = nameAddr[vv.DeviceName]
		}
	}

	// Создание таймер-тиков, для формирования тиков передачи данных в канал
	//

	// подготовка слайса для создания таймер-тикеров
	slTime := make([]int, 0)

	for _, v := range listTimeScan {
		slTime = append(slTime, v)
	}

	// Создание таймер-тикеров
	slTicker := createTickers(slTime)

	// Запуск Go рутин для каждого таймер-тика
	for i, ticker := range slTicker {

		wg.Add(1)

		go func(ctx context.Context, wg *sync.WaitGroup, t *time.Ticker, data []libre.ChConfExt_Export) {

			for {
				select {
				case <-t.C:
					forModbusRTU <- data // передача данных в канал
				case <-ctx.Done():
					wg.Done()
					return
				default:
					time.Sleep(time.Microsecond * 1)
				}
			}
		}(ctx, wg, ticker, listChByTimeScanExt[slTime[i]])
	}

	// генерация запущена
	// данные передаются в драйвер Modbus-TCP

}

// Создание таймер-тикеров. Для передачи данных в драйвер. Возвращается массив таймер-тикеров.
//
// Параметры:
//
// intervals - массив интервалов, под которые создаются тамер-тики
func createTickers(intervals []int) []*time.Ticker {

	// Создание слайса тикеров
	tickers := make([]*time.Ticker, len(intervals))

	// Инициализация для каждого интервала
	for i, interval := range intervals {
		tickers[i] = time.NewTicker(time.Duration(interval) * time.Millisecond)
	}

	return tickers
}

// Подготовка данных для клиента Modbus-TCP. Возвращается адрес слева, адрес регистра, количество регистров и ошибка.
//
// Параметры:
//
// srcData - набор данных для формирования выборки
func prepareDataClientModbusTCP(srcData libre.ChConfExt_Export) (slaveID byte, address uint16, quantity uint16, err error) {

	// формирование адреса устройства
	local_slaveID, err := strconv.Atoi(srcData.DeviceAddr)
	if err != nil {
		return 0, 0, 0, err
	}
	slaveID = byte(local_slaveID)

	// формирование стартового адреса регистра
	local_address, err := strconv.Atoi(srcData.Address)
	if err != nil {
		return 0, 0, 0, err
	}
	address = uint16(local_address)

	// формирование количества регистров на опрос
	numbReg := 0

	switch srcData.DataType {
	case "Word", "ShortInt", "Bool":
		numbReg = 1
	case "Integer", "DWord", "Float":
		numbReg = 2
	case "Int64", "Double":
		numbReg = 4
	default:
		return 0, 0, 0, fmt.Errorf("ошибка формирования количества регистров по входным данным: %v", srcData.DataType)
	}

	quantity = uint16(numbReg)
	err = nil

	// возврат сформированных значений
	return

}

// Подготовка данных для клиента Modbus-RTU. Возвращается адрес слева, адрес регистра, количество регистров и ошибка.
//
// Параметры:
//
// srcData - набор данных для формирования выборки
func prepareDataClientModbusRTU(srcData libre.ChConfExt_Export) (slaveID byte, address uint16, quantity uint16, err error) {

	// формирование адреса устройства
	local_slaveID, err := strconv.Atoi(srcData.DeviceAddr)
	if err != nil {
		return 0, 0, 0, err
	}
	slaveID = byte(local_slaveID)

	// формирование стартового адреса регистра
	local_address, err := strconv.Atoi(srcData.Address)
	if err != nil {
		return 0, 0, 0, err
	}
	address = uint16(local_address)

	// формирование количества регистров на опрос
	numbReg := 0

	switch srcData.DataType {
	case "Word", "ShortInt", "Bool":
		numbReg = 1
	case "Integer", "DWord", "Float":
		numbReg = 2
	case "Int64", "Double":
		numbReg = 4
	default:
		return 0, 0, 0, fmt.Errorf("ошибка формирования количества регистров по входным данным: %v", srcData.DataType)
	}

	quantity = uint16(numbReg)
	err = nil

	// возврат сформированных значений
	return

}

// Перенос содержимого в расширенную мапу. Возвращается расширенная мапа.
//
// Параметры:
//
// srcMap - исходная мапа
func moveToExt(srcMap map[int][]libre.ChConf_Export) (targMap map[int][]libre.ChConfExt_Export) {

	m := make(map[int][]libre.ChConfExt_Export)

	for k, v := range srcMap {

		sl := make([]libre.ChConfExt_Export, 0)

		for _, vv := range v {

			el := libre.ChConfExt_Export{}

			el.DeviceAddr = " " // новое добавленное поле
			el.DeviceName = vv.Device
			el.Address = vv.Address
			el.DataType = vv.DataType
			el.Comment = vv.Comment
			el.TimeScan = vv.TimeScan
			el.FuncType = vv.FuncType
			el.Format = vv.Format

			sl = append(sl, el)

		}

		m[k] = sl // передача подготовленного слайса
	}

	return m
}

// Выбор функции запроса Modbus и выполнение. Возвращается массив uint16 или byte и ошибка.
//
// Параметры:
//
// con - соединение хоста.
// function - название функции.
// slaveID - адрес слейва.
// quantity - количество регистров запроса.
func selectFuncMbTCPDo(con modbustcpmaster.Connect, function string, slaveID byte, address uint16, quantity uint16) (resUint16 []uint16, resByte []byte, err error) {

	switch function {
	case "ReadHoldingRegisters":
		resUint16, err = con.Client.ReadHoldingRegisters(slaveID, address, quantity)
		if err != nil {
			return []uint16{}, []byte{}, err
		}
		return resUint16, []byte{}, nil

	case "ReadInputRegisters":
		resUint16, err = con.Client.ReadInputRegisters(slaveID, address, quantity)
		if err != nil {
			return []uint16{}, []byte{}, err
		}
		return resUint16, []byte{}, nil

	case "ReadDiscreteInputs":
		resByte, err = con.Client.ReadDiscreteInputs(slaveID, address, quantity)

		if err != nil {
			return []uint16{}, []byte{}, err
		}
		return []uint16{}, resByte, nil

	case "ReadCoil":
		resByte, err = con.Client.ReadCoils(slaveID, address, quantity)
		if err != nil {
			return []uint16{}, []byte{}, err
		}
		return []uint16{}, resByte, nil

	case "WriteSingleRegister":
		err = con.Client.WriteSingleRegister(slaveID, address, quantity)
		if err != nil {
			return []uint16{}, []byte{}, err
		}
		return []uint16{}, []byte{}, nil

	default:
		return []uint16{}, []byte{}, fmt.Errorf("ошибка при выборе функции Modbus: %v ", function)
	}

}

// Выбор функции запроса Modbus и выполнение. Возвращается массив byte и ошибка.
//
// Параметры:
//
// con - соединение хоста.
// function - название функции.
// slaveID - адрес слейва.
// quantity - количество регистров запроса.
func selectFuncMbRTUDo(con modbusrtumaster.Connect, function string, slaveID byte, address uint16, quantity uint16) (resByte []byte, err error) {

	// установка адреса ведомого устройства
	con.ChangeSlaveID(slaveID)

	// выполнение запроса
	switch function {
	case "ReadHoldingRegisters":
		resBytes, err := con.Client.ReadHoldingRegisters(address, quantity)
		if err != nil {
			return []byte{}, err
		}
		return resBytes, nil

	case "ReadInputRegisters":
		resByte, err := con.Client.ReadInputRegisters(address, quantity)
		if err != nil {
			return []byte{}, err
		}
		return resByte, nil

	case "ReadDiscreteInputs":
		resByte, err := con.Client.ReadDiscreteInputs(address, quantity)

		if err != nil {
			return []byte{}, err
		}
		return resByte, nil

	case "ReadCoil":
		resByte, err := con.Client.ReadCoils(address, quantity)
		if err != nil {
			return []byte{}, err
		}
		return resByte, nil

	case "WriteSingleRegister":
		resByte, err := con.Client.WriteSingleRegister(address, 111)
		if err != nil {
			return []byte{}, err
		}
		return resByte, nil

	default:
		return []byte{}, fmt.Errorf("ошибка при выборе функции Modbus: %v ", function)
	}
}

// Приведение принятых данных к типу. Функция возвращает значение интерфесом и ошибку.
//
// Параметры:
//
// srcData - исходные данные.
// dataType - запрашиваемый тип данных.
// format - формат данных (чередование байт).
func buildValFromUint16(srcData []uint16, dataType string, format string) (value interface{}, err error) {

	if len(srcData) == 0 || dataType == "" || format == "" {
		return nil, fmt.Errorf("ошибка в содержимом аргументов функции преобразования типа: [%v] [%v] [%v]", srcData, dataType, format)
	}

	// Формирование слайса байт из принятого слайса uint16
	slByte := make([]byte, 0)

	for _, v := range srcData {
		slByte = append(slByte, byte(v))
		slByte = append(slByte, byte(v>>8))
	}

	// Формирование слайса из принятого формата (чередования байт)
	strNumb := strings.Split(format, "_")

	intNumb := make(map[int]int)

	for i, v := range strNumb {
		_ = i
		vv, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("ошибка при преобразовании символа [%v] в число", v)
		}
		intNumb[i] = vv
	}

	// Формирование значения нужного типа по указанной очерёдности байт
	switch dataType {

	case "Word":
		var val uint16 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= uint16(slByte[i]) << 8
			case 1:
				val |= uint16(slByte[i])
			}
		}
		return val, nil

	case "ShortInt":
		var val int16 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= int16(slByte[i]) << 8
			case 1:
				val |= int16(slByte[i])
			}
		}
		return val, nil

	case "Integer":
		var valBuf uint32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint32(slByte[i]) << 8
			case 1:
				valBuf |= uint32(slByte[i])
			case 2:
				valBuf |= uint32(slByte[i]) << 24
			case 3:
				valBuf |= uint32(slByte[i]) << 16
			}
		}
		return int32(valBuf), nil

	case "DWord":
		var val uint32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= uint32(slByte[i]) << 8
			case 1:
				val |= uint32(slByte[i])
			case 2:
				val |= uint32(slByte[i]) << 24
			case 3:
				val |= uint32(slByte[i]) << 16
			}
		}
		return val, nil

	case "Float":
		var valBuf uint32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint32(slByte[i]) << 8
			case 1:
				valBuf |= uint32(slByte[i])
			case 2:
				valBuf |= uint32(slByte[i]) << 24
			case 3:
				valBuf |= uint32(slByte[i]) << 16
			}
		}
		floatValue := math.Float32frombits(valBuf)
		return floatValue, nil

	case "Int64":
		var valBuf uint64 = 0

		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint64(slByte[i]) << 8
			case 1:
				valBuf |= uint64(slByte[i])
			case 2:
				valBuf |= uint64(slByte[i]) << 24
			case 3:
				valBuf |= uint64(slByte[i]) << 16
			case 4:
				valBuf |= uint64(slByte[i]) << 40
			case 5:
				valBuf |= uint64(slByte[i]) << 32
			case 6:
				valBuf |= uint64(slByte[i]) << 56
			case 7:
				valBuf |= uint64(slByte[i]) << 48
			}
		}
		return int64(valBuf), nil

	case "Double":
		var valBuf uint64 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint64(slByte[i]) << 8
			case 1:
				valBuf |= uint64(slByte[i])
			case 2:
				valBuf |= uint64(slByte[i]) << 24
			case 3:
				valBuf |= uint64(slByte[i]) << 16
			case 4:
				valBuf |= uint64(slByte[i]) << 40
			case 5:
				valBuf |= uint64(slByte[i]) << 32
			case 6:
				valBuf |= uint64(slByte[i]) << 56
			case 7:
				valBuf |= uint64(slByte[i]) << 48
			}
		}
		floatValue := math.Float64frombits(valBuf)
		return floatValue, nil

	default:
		return nil, fmt.Errorf("неподдерживаемый тип данных: %s", dataType)
	}
}

// Приведение принятых данных к типу. Функция возвращает значение интерфесом и ошибку.
//
// Параметры:
//
// srcData - исходные данные.
// dataType - запрашиваемый тип данных.
// format - формат данных (чередование байт).
func buildValFromByte(srcData []byte, dataType string, format string) (value interface{}, err error) {

	if len(srcData) == 0 || dataType == "" || format == "" {
		return nil, fmt.Errorf("ошибка в содержимом аргументов функции преобразования типа: [%v] [%v] [%v]", srcData, dataType, format)
	}

	slByte := srcData

	// Формирование слайса из принятого формата (чередования байт)
	strNumb := strings.Split(format, "_")

	intNumb := make(map[int]int)

	for i, v := range strNumb {
		_ = i
		vv, err := strconv.Atoi(v)
		if err != nil {
			return 0, fmt.Errorf("ошибка при преобразовании символа [%v] в число", v)
		}
		intNumb[i] = vv
	}

	// Формирование значения нужного типа по указанной очерёдности байт
	switch dataType {

	case "Word":
		var val uint16 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= uint16(slByte[i]) << 8
			case 1:
				val |= uint16(slByte[i])
			}
		}
		return val, nil

	case "ShortInt":

		var val int16 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= int16(slByte[i]) << 8
			case 1:
				val |= int16(slByte[i])
			}
		}

		return val, nil

	case "Integer":
		var valBuf int32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= int32(slByte[i]) << 24
			case 1:
				valBuf |= int32(slByte[i]) << 16
			case 2:
				valBuf |= int32(slByte[i]) << 8
			case 3:
				valBuf |= int32(slByte[i])
			}
		}

		return valBuf, nil

	case "DWord":
		var val uint32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				val |= uint32(slByte[i]) << 24
			case 1:
				val |= uint32(slByte[i]) << 16
			case 2:
				val |= uint32(slByte[i]) << 8
			case 3:
				val |= uint32(slByte[i])
			}
		}
		return val, nil

	case "Float":
		var valBuf uint32 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint32(slByte[i]) << 24
			case 1:
				valBuf |= uint32(slByte[i]) << 16
			case 2:
				valBuf |= uint32(slByte[i]) << 8
			case 3:
				valBuf |= uint32(slByte[i])
			}
		}

		floatValue := math.Float32frombits(valBuf)
		return floatValue, nil

	case "Int64":
		var valBuf int64 = 0

		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= int64(slByte[i]) << 56
			case 1:
				valBuf |= int64(slByte[i]) << 48
			case 2:
				valBuf |= int64(slByte[i]) << 40
			case 3:
				valBuf |= int64(slByte[i]) << 32
			case 4:
				valBuf |= int64(slByte[i]) << 24
			case 5:
				valBuf |= int64(slByte[i]) << 16
			case 6:
				valBuf |= int64(slByte[i]) << 8
			case 7:
				valBuf |= int64(slByte[i])
			}
		}
		return valBuf, nil

	case "Double":
		var valBuf uint64 = 0
		for i, v := range intNumb {
			switch v {
			case 0:
				valBuf |= uint64(slByte[i]) << 56
			case 1:
				valBuf |= uint64(slByte[i]) << 48
			case 2:
				valBuf |= uint64(slByte[i]) << 40
			case 3:
				valBuf |= uint64(slByte[i]) << 32
			case 4:
				valBuf |= uint64(slByte[i]) << 24
			case 5:
				valBuf |= uint64(slByte[i]) << 16
			case 6:
				valBuf |= uint64(slByte[i]) << 8
			case 7:
				valBuf |= uint64(slByte[i])
			}
		}
		floatValue := math.Float64frombits(valBuf)
		return floatValue, nil

	case "Bool":
		var valBuf byte
		valBuf = slByte[0]

		return valBuf, nil

	default:
		return nil, fmt.Errorf("неподдерживаемый тип данных: %s", dataType)
	}
}

// Http сервер
func httpServer() {

	fmt.Println("Запуск HTTP сервера.")

	// Ручки HTTP сервера
	r := chi.NewRouter()

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {
		httpMutex.Lock()
		defer httpMutex.Unlock()

		collectServInfo()
		srvInfo.HandlStatusSrv(w, r)
	})

	r.Get("/datadb", func(w http.ResponseWriter, r *http.Request) {

		httpMutex.Lock()
		defer httpMutex.Unlock()

		d := serverAPI.DataDB{
			StartDate: "",
			Data:      make([]serverAPI.DataEl, 0),
		}

		qParams := r.URL.Query()
		d.StartDate = qParams.Get("startdate")
		if d.StartDate == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadGateway)
			return
		}

		// Чтение данных из БД
		err := readDataDB(&d)
		if err != nil {
			lgr.E.Printf("ошибка при чтении данных из БД: {%v}", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Передача данных клиенту
		d.HandlExpDataDB(w, r)
	})

	// Запуск HTTP сервера
	err := http.ListenAndServe(os.Getenv("HTTP_SERVER_IP")+":"+os.Getenv("HTTP_SERVER_PORT"), r)
	if err != nil {
		lgr.E.Println("ошибка запуска HTTP сервера:", err)
		log.Fatalf("ошибка запуска HTTP сервера: %v", err)
	}
}

// HTTPS сервер (для внешних клиентов)
func goHttpsServer() {

	fmt.Println("Запуск HTTPS сервера.")

	// Ручки HTTP сервера
	r := chi.NewRouter()

	r.Get("/status", func(w http.ResponseWriter, r *http.Request) {

		httpMutex.Lock()
		defer httpMutex.Unlock()

		collectServInfo()
		srvInfo.HandlStatusSrv(w, r)
	})

	r.Get("/datadb", func(w http.ResponseWriter, r *http.Request) {

		httpMutex.Lock()
		defer httpMutex.Unlock()

		d := serverAPI.DataDB{
			StartDate: "",
			Data:      make([]serverAPI.DataEl, 0),
		}

		qParams := r.URL.Query()
		d.StartDate = qParams.Get("startdate")
		if d.StartDate == "" {
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadGateway)
			return
		}

		// Чтение данных из БД
		err := readDataDB(&d)
		if err != nil {
			lgr.E.Printf("ошибка при чтении данных из БД: {%v}", err)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}

		// Передача данных клиенту
		d.HandlExpDataDB(w, r)
	})

	// Запуск HTTPS сервера
	err := http.ListenAndServeTLS(
		os.Getenv("HTTPS_SERVER_IP")+":"+os.Getenv("HTTPS_SERVER_PORT"),
		os.Getenv("HTTPS_SERVER_KEY_PUBLIC"),
		os.Getenv("HTTPS_SERVER_KEY_PRIVATE"),
		r)

	if err != nil {
		lgr.E.Println("ошибка запуска HTTPS сервера:", err)
		log.Fatalf("ошибка запуска HTTPS сервера: %v", err)
	}
}

// Функция выполняет сбор данных сервера.
func collectServInfo() {

	srvInfo.MbRTU = make([]serverAPI.InfoModbusRTU, 0)
	srvInfo.MbTCP = make([]serverAPI.InfoModbusTCP, 0)

	// Сбор информации по Modbus-RTU
	for _, v := range hostConnects.mbRTUmaster {
		mbRTU := serverAPI.InfoModbusRTU{}
		mbRTU.ConName = v.Name
		mbRTU.Con = v.Port
		mbRTU.ConParams.BaudRate = v.ParamsConn.BaudRate
		mbRTU.ConParams.DataBits = v.ParamsConn.DataBits
		mbRTU.ConParams.Parity = v.ParamsConn.Parity
		mbRTU.ConParams.StopBits = v.ParamsConn.StopBits

		srvInfo.MbRTU = append(srvInfo.MbRTU, mbRTU)
	}

	// Сбор информации по Modbus-TCP
	for _, v := range hostConnects.mbTCPmaster {
		mbTCP := serverAPI.InfoModbusTCP{}
		mbTCP.ConName = v.Name
		mbTCP.Con = v.HostIP

		srvInfo.MbTCP = append(srvInfo.MbTCP, mbTCP)
	}

	// Получение информации о размерности файлов логера
	var err error

	srvInfo.SizeF.I, srvInfo.SizeF.W, srvInfo.SizeF.E, err = lgr.SizeFiles()
	if err != nil {
		lgr.E.Printf("ошибка при получении размерности файлов логера:{%v}\n", err)
		return
	}

}

// Функция организации чтения данных из БД. Возвращает ошибку.
//
// Параметры:
//
// data - стартовая дата выборки и результат выборки.
func readDataDB(data *serverAPI.DataDB) (err error) {

	// Проверка входных данных
	if data == nil {
		return errors.New("основная функция запросов -> принят пустой указатель")
	}

	limit := 10
	offset := 100
	sizeRx := limit
	cnt := 0

	// Реализация запроса данных
	for sizeRx == limit {

		sizeRx, err = readDataDBReq(data, limit, offset*cnt)
		if err != nil {
			return err
		}
		cnt++
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
func readDataDBReq(data *serverAPI.DataDB, limit, offset int) (size int, err error) {

	// Проверка указателя
	if data == nil {
		return 0, fmt.Errorf("запрос данных. пустой указатель: {%v}", data)
	}
	// Проверка корректной даты
	rxDate, err := time.Parse("2006-01-02", data.StartDate)
	if err != nil {
		return 0, fmt.Errorf("запрос данных. значение начальной даты: {%s}", data.StartDate)
	}
	// Проверка корректности значения limit
	if limit < 1 {
		return 0, fmt.Errorf("запрос данных. значение limit:{%d} меньше 1", limit)
	}
	// Проверка корректности значения offset
	if offset < 0 {
		return 0, fmt.Errorf("запрос данных. значение offset:{%d} меньше 0", offset)
	}

	// Подготовка даты для запроса
	reqDate := rxDate.Format("2006-01-02")

	q := fmt.Sprintf(`
	 SELECT name, value, qual, timestamp
     FROM %s.%s
     WHERE date(timestamp) = '%v'
	 ORDER By timestamp DESC
	 LIMIT %d OFFSET %d
	 ;              
	`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"), reqDate, limit, offset)

	// Запрос
	rows, err := db.Ptr.Query(q)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	// Обработка ответа
	cnt := 0
	localData := make([]serverAPI.DataEl, 0)

	for rows.Next() {
		var str serverAPI.DataEl

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
