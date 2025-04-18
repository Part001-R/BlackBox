package libre

import (
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

type (
	ConfXLSX_Object struct {
		Ptr              *excelize.File
		SheetMain_Header []sheetMain_Head
		SheetMain_Dev    []sheetMain_Dev
		SheetsDev        []dev
		ConfDataReady    bool
	}

	// Заголовок главной вкладки
	sheetMain_Head struct {
		host    string
		conType string
		address string
		port    string
	}

	sheetMain_Dev struct {
		device  string
		comment string
		host    string
		netAddr string
	}

	dev struct {
		name string    // наименование устройства
		conf []devConf // содержимое вкладки

	}

	devConf struct {
		address  string
		name     string
		dataType string
		comment  string
		timeScan string
	}

	ConfXLSX_Intf interface {
		Open() error
		Read() error
		Show()
	}
)

// Открытие файла конфигурации. Возвращает ошибку
func (e *ConfXLSX_Object) Open() error {

	var err error

	e.Ptr, err = excelize.OpenFile(os.Getenv("CONFIG_FILE_NAME"))
	if err != nil {
		return err
	}

	return nil
}

// Чтение файла
func (e *ConfXLSX_Object) Read() error {

	var err error

	// Открытие файла
	e.Ptr, err = excelize.OpenFile(os.Getenv("CONFIG_FILE_NAME"))
	if err != nil {
		return err
	}
	defer func() {
		err = e.Ptr.Close()
		if err != nil {
			log.Fatal("ошибка при закрытии подключения к конфигурационному файлу")
		}
	}()

	// Чтение основной вкладки
	err = read_MainSheet(e)
	if err != nil {
		return err
	}

	// Чтение вкладок устройств
	err = read_DevSheet(e)
	if err != nil {
		return err
	}

	return nil
}

// Вывод конфигурационных данных в терминал
func (e *ConfXLSX_Object) Show() {

	fmt.Println("[Настойки хоста]")
	for _, el := range e.SheetMain_Header {
		fmt.Printf("Host: %s    ConType: %s    Address: %s    Port: %s \n", el.host, el.conType, el.address, el.port)
	}
	fmt.Println()

	fmt.Println("[Настойки подключений]")
	for _, el := range e.SheetMain_Dev {
		fmt.Printf("Device: %s    Comment: %s    Host: %s    netAddr: %s \n", el.device, el.comment, el.host, el.netAddr)
	}
	fmt.Println()

	fmt.Println("[Настройка опроса]")
	for _, el := range e.SheetsDev {
		fmt.Printf("[%s]\n", el.name)

		for _, e := range el.conf {
			fmt.Printf("Address: %s    Name: %s    DataType: %s    Comment: %s    Timescan: %s \n",
				e.address, e.name, e.dataType, e.comment, e.timeScan)
		}
	}
}

// Чтение основной вкладки
func read_MainSheet(e *ConfXLSX_Object) error {

	// Чтение содержимого вкладки Main
	rows, err := e.Ptr.GetRows("Main")
	if err != nil {
		return err
	}

	// Выборка данных
	needFillHead := false
	needFillBody := false
	cntHeadeer := 0
	cntBody := 0

	for _, row := range rows {

		// Обнаружение строки заголовка коннектов хоста
		if len(row) == 4 && !needFillHead {
			if row[0] == "Host:" && row[1] == "ConType:" && row[2] == "Address:" && row[3] == "Port:" {
				needFillHead = true
				continue
			}
		}

		// Обнаружение строки заголовка устройств
		if len(row) == 4 && !needFillBody {
			if row[0] == "Device:" && row[1] == "Comment:" && row[2] == "Host:" && row[3] == "NetAddr:" {
				needFillBody = true
				needFillHead = false
				continue
			}
		}

		// Получение данных коннектов хоста
		if needFillHead && row != nil {
			if row[0] != "" && row[1] != "" && row[2] != "" && row[3] != "" {
				var el sheetMain_Head
				el.host = row[0]
				el.conType = row[1]
				el.address = row[2]
				el.port = row[3]

				e.SheetMain_Header = append(e.SheetMain_Header, el)
				cntHeadeer++
				continue
			}
		}

		// Получение данных устройств
		if needFillBody && row != nil {
			if row[0] != "" && row[1] != "" && row[2] != "" && row[3] != "" {
				var el sheetMain_Dev
				el.device = row[0]
				el.comment = row[1]
				el.host = row[2]
				el.netAddr = row[3]

				e.SheetMain_Dev = append(e.SheetMain_Dev, el)
				cntBody++
				continue
			}
		}
	}

	if cntHeadeer >= 1 && cntBody >= 1 {
		return nil
	}

	return errors.New("ошибка в чтении файла конфигурации")
}

// Чтение вкладок устройств
func read_DevSheet(e *ConfXLSX_Object) error {

	if len(e.SheetMain_Dev) < 1 {
		return errors.New("в конфигурации главного листа неуказаны устройства")
	}

	// проход по вкладкам устройств
	for _, d := range e.SheetMain_Dev {

		var device dev

		device.name = d.device

		rows, err := e.Ptr.GetRows(d.device)
		if err != nil {
			return err
		}

		// проход по строкам вкладки
		for i, row := range rows {

			if i == 0 { // пропуск итерации, чтобы несохранять наименования столбцов
				continue
			}

			if row != nil {
				if row[0] != "" && row[1] != "" && row[2] != "" && row[3] != "" && row[4] != "" {
					var dConf devConf
					dConf.address = row[0]
					dConf.name = row[1]
					dConf.dataType = row[2]
					dConf.comment = row[3]
					dConf.timeScan = row[4]

					device.conf = append(device.conf, dConf)
				}
			}
		}

		e.SheetsDev = append(e.SheetsDev, device)
	}
	return nil
}
