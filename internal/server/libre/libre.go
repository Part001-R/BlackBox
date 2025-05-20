package libre

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type (
	// Формат данных при импорте
	ConfXLSX_Import struct {
		Ptr              *excelize.File
		SheetMain_Header []SheetMain_Head
		SheetMain_Dev    []SheetMain_Dev
		SheetsDev        []Dev
		ConfDataReady    bool
	}

	// Формат данных при экспорте
	ConfXLSX_Export struct {
		Ptr              *excelize.File
		SheetMain_Header []SheetMain_Head
		SheetMain_Dev    []SheetMain_Dev
		SheetChan        []ChConf_Export
		ConfDataReady    bool
	}

	// Данные для запуска Go рутин
	GoData struct {
		Host   []SheetMain_Head // описание устройства
		Device []Device         // устройства
	}

	Device struct {
		Name    string
		ChGroup []GroupTime // группа каналов по времени опроса
	}
	// Группа каналов по времени опроса
	GroupTime struct {
		TimeScan string   // время опроса
		Chan     []string // список каналов
	}

	// Заголовок главной вкладки
	SheetMain_Head struct {
		Host     string
		ConType  string
		Address  string
		Port     string
		BaudRate string
		DataBits string
		Parity   string
		StopBits string
	}

	SheetMain_Dev struct {
		Device  string
		Comment string
		Host    string
		Type_   string
		Address string
		IP      string
		Port    string
	}

	Dev struct {
		Name string           // наименование устройства
		Conf []DevConf_Import // содержимое вкладки

	}

	DevConf_Import struct {
		Address  string
		Name     string
		DataType string
		Comment  string
		TimeScan string
		FuncType string
		Format   string
	}

	ChConf_Export struct {
		Device   string
		Address  string
		DataType string
		Comment  string
		TimeScan string
		FuncType string
		Format   string
	}

	ChConfExt_Export struct {
		DeviceName string
		DeviceAddr string
		Address    string
		DataType   string
		Comment    string
		TimeScan   string
		FuncType   string
		Format     string
	}
)

var (
	// список типов коннектов хоста
	listConnType = map[string]bool{
		"TCP": true,
		"COM": false,
	}

	// список поддерживаемых функций протоколов
	listFuncType = map[string]bool{
		"ReadCoil":             true,
		"ReadDiscreteInputs":   true,
		"ReadHoldingRegisters": true,
		"ReadInputRegisters":   true,
		"WriteSingleRegister":  true,
	}

	// список поддерживаемых типов данных
	listDataType = map[string]bool{
		"Word":     true,
		"ShortInt": true,
		"Bool":     true,
		"Integer":  true,
		"DWord":    true,
		"Float":    true,
		"Int64":    true,
		"Double":   true,
	}

	// список поддерживаемых протоколов
	listProtocolType = map[string]bool{
		"Modbus-TCP": true,
		"Modbus-RTU": true,
	}

	// список типов данных с привязкой к функции
	listDataTypeByFunc = map[string][]string{
		"ReadCoil":             {"Bool"},
		"ReadDiscreteInputs":   {"Bool"},
		"ReadHoldingRegisters": {"Word", "ShortInt", "Integer", "DWord", "Float", "Int64", "Double"},
		"ReadInputRegisters":   {"Word", "ShortInt", "Integer", "DWord", "Float", "Int64", "Double"},
		"WriteSingleRegister":  {"Word", "ShortInt", "Integer", "DWord", "Float", "Int64", "Double"},
	}

	// список типов данных с привязкой к количеству файт
	listDataTypeByBytes = map[string]int{
		"Bool":     2,
		"Word":     2,
		"ShortInt": 2,
		"Integer":  4,
		"DWord":    4,
		"Float":    4,
		"Int64":    8,
		"Double":   8,
	}
)

// Открытие файла конфигурации. Возвращает ошибку
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func (e *ConfXLSX_Import) Open() error {

	var err error

	e.Ptr, err = excelize.OpenFile(os.Getenv("XLSX_NAME_FILE_IMPORT"))
	if err != nil {
		return err
	}

	return nil
}

// Чтение файла. Возвращается ошибка.
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func (e *ConfXLSX_Import) ReadImport() error {

	e.ConfDataReady = false
	var err error

	// Открытие файла
	e.Ptr, err = excelize.OpenFile(os.Getenv("IMPORT_FILE_NAME"))
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
	err = readMainSheet(e)
	if err != nil {
		return fmt.Errorf("ошибка чтения главной вкладки: [%v]", err)
	}

	// Чтение вкладок устройств
	err = readDevSheet(e)
	if err != nil {
		return fmt.Errorf("ошибка чтения данных устройств: [%v]", err)
	}

	// Проверка данных импорта на корректность
	err = checkImportData(e)
	if err != nil {
		return fmt.Errorf("ошибка проверки данных импорта: [%v]", err)
	}

	e.ConfDataReady = true
	return nil
}

// Вывод конфигурационных данных в терминал.
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func (e *ConfXLSX_Import) ShowImport() {

	fmt.Println("[Настойки хоста]")
	for _, el := range e.SheetMain_Header {
		fmt.Printf("Host: %s    ConType: %s    Address: %s    Port: %s   BaudRate: %s  DataBits: %s  Parity: %s  StopBits: %s \n",
			el.Host, el.ConType, el.Address, el.Port, el.BaudRate, el.DataBits, el.Parity, el.StopBits)
	}
	fmt.Println()

	fmt.Println("[Настойки подключений]")
	for _, el := range e.SheetMain_Dev {
		fmt.Printf("Device: %s    Comment: %s    Host: %s   Type: %s   Address: %s  IP: %s  Port:  %s\n",
			el.Device, el.Comment, el.Host, el.Type_, el.Address, el.IP, el.Port)
	}
	fmt.Println()

	fmt.Println("[Настройки опроса]")
	for _, el := range e.SheetsDev {
		fmt.Printf("[%s]\n", el.Name)

		for _, e := range el.Conf {
			fmt.Printf("Address: %s    Name: %s    DataType: %s    Comment: %s    Timescan: %s    FuncType: %s    Format: %s\n",
				e.Address, e.Name, e.DataType, e.Comment, e.TimeScan, e.FuncType, e.Format)
		}
	}
}

// Чтение основной вкладки. Возвращается ошибка.
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func readMainSheet(e *ConfXLSX_Import) error {

	// Чтение содержимого вкладки Main
	rows, err := e.Ptr.GetRows("Main")
	if err != nil {
		return err
	}

	// Выборка данных
	needFillHead := false
	needFillDev := false
	cntHeader := 0
	cntDev := 0

	for _, row := range rows {

		// пропуск пустой строки
		if len(row) == 0 {
			if needFillHead {
				needFillHead = false
			}
			continue
		}

		// Обнаружение строки заголовка коннектов хоста
		if len(row) == 8 && !needFillHead {
			if row[0] == "Host:" &&
				row[1] == "ConType:" &&
				row[2] == "Address:" &&
				row[3] == "Port:" &&
				row[4] == "BaudRate:" &&
				row[5] == "DataBits:" &&
				row[6] == "Parity:" &&
				row[7] == "StopBits:" {
				needFillHead = true
				continue
			}
		}

		// Обнаружение строки заголовка устройств (для Modbus-RTU)
		if len(row) == 7 && !needFillDev {
			if row[0] == "Device:" &&
				row[1] == "Comment:" &&
				row[2] == "Host:" &&
				row[3] == "Type:" &&
				row[4] == "Address:" &&
				row[5] == "IP:" &&
				row[6] == "Port:" {
				needFillDev = true
				continue
			}
		}

		// Получение данных коннектов хоста
		if needFillHead {
			var el SheetMain_Head

			switch row[1] {
			case "TCP":
				if row[0] != "" && row[2] != "" && row[3] != "" {
					el.Host = row[0]
					el.ConType = row[1]
					el.Address = row[2]
					el.Port = row[3]

					e.SheetMain_Header = append(e.SheetMain_Header, el)
					cntHeader++
					continue
				}
				return fmt.Errorf("есть пустое поле в строке: {%v} ", row)

			case "COM":
				if row[0] != "" && row[3] != "" && row[4] != "" && row[5] != "" && row[6] != "" && row[7] != "" {
					el.Host = row[0]
					el.ConType = row[1]
					el.Address = row[2]
					el.Port = row[3]
					el.BaudRate = row[4]
					el.DataBits = row[5]
					el.Parity = row[6]
					el.StopBits = row[7]

					e.SheetMain_Header = append(e.SheetMain_Header, el)
					cntHeader++
					continue
				}
				return fmt.Errorf("есть пустое поле в строке: {%v} ", row)

			default:
				return fmt.Errorf("ошибка распознования типа коннекта хоста")
			}
		}

		// Получение данных устройств
		if needFillDev {
			var el SheetMain_Dev

			switch row[3] {
			case "Modbus-TCP":
				if row[0] != "" && row[1] != "" && row[2] != "" && row[4] != "" {

					el.Device = row[0]
					el.Comment = row[1]
					el.Host = row[2]
					el.Type_ = row[3]
					el.Address = row[4]
					el.IP = row[5]
					el.Port = row[6]

					e.SheetMain_Dev = append(e.SheetMain_Dev, el)
					cntDev++
					continue
				}
				return fmt.Errorf("есть пустое поле в строке: {%v} ", row)

			case "Modbus-RTU":
				if row[0] != "" && row[1] != "" && row[2] != "" && row[4] != "" {

					el.Device = row[0]
					el.Comment = row[1]
					el.Host = row[2]
					el.Type_ = row[3]
					el.Address = row[4]

					e.SheetMain_Dev = append(e.SheetMain_Dev, el)
					cntDev++
					continue
				}
				return fmt.Errorf("есть пустое поле в строке: {%v} ", row)

			default:
				return fmt.Errorf("ошибка распознования типа коннекта в устройствах")
			}
		}
	}

	if cntHeader >= 1 && cntDev >= 1 {
		return nil
	}

	return errors.New("ошибка считывания данных файла конфигурации")
}

// Чтение вкладок устройств. Возвращается ошибка
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func readDevSheet(e *ConfXLSX_Import) error {

	if len(e.SheetMain_Dev) < 1 {
		return errors.New("в конфигурации основного листа не указаны устройства")
	}

	// проход по вкладкам устройств
	for _, d := range e.SheetMain_Dev {

		var device Dev

		device.Name = d.Device

		rows, err := e.Ptr.GetRows(d.Device)
		if err != nil {
			return err
		}

		// проход по строкам вкладки
		for i, row := range rows {

			if i == 0 { // пропуск итерации, чтобы несохранять наименования столбцов
				continue
			}

			if row != nil {
				if row[0] != "" && row[1] != "" && row[2] != "" && row[3] != "" && row[4] != "" && row[5] != "" && row[6] != "" {
					var dConf DevConf_Import
					dConf.Address = row[0]
					dConf.Name = row[1]
					dConf.DataType = row[2]
					dConf.Comment = row[3]
					dConf.TimeScan = row[4]
					dConf.FuncType = row[5]
					dConf.Format = row[6]

					device.Conf = append(device.Conf, dConf)
				}
			}
		}

		e.SheetsDev = append(e.SheetsDev, device)
	}
	return nil
}

// Создание xlsx файла, конфигурации. Возвращается имя файла и ошибка.
//
// Параметры:
//
// path - путь к файлу
// name - имя файла
// time - время создания файла
// typ - тип файла
func (e *ConfXLSX_Export) CreateXlsxConfig(path, name, time, typ string) (nameFile string, err error) {

	file := excelize.NewFile()

	_, err = file.NewSheet("Main") // добавление вкладки
	if err != nil {
		return "", err
	}

	_, err = file.NewSheet("Channels") // добавление вкладки
	if err != nil {
		return "", err
	}

	err = file.DeleteSheet("Sheet1") // удаление созданной по умолчанию вкладки
	if err != nil {
		return "", err
	}

	f := path + name + "-" + time + typ

	err = file.SaveAs(f)
	if err != nil {
		return "", err
	}

	return f, nil
}

// Создание xlsx файла, экспорта данных. Возвращается имя файла и ошибка.
//
// Параметры:
//
// name - имя файла
func CreateXlsxDataDB(name string) (nameFile string, err error) {

	file := excelize.NewFile()

	_, err = file.NewSheet("DataDB") // добавление вкладки
	if err != nil {
		return "", err
	}

	err = file.DeleteSheet("Sheet1") // удаление созданной по умолчанию вкладки
	if err != nil {
		return "", err
	}

	err = file.SaveAs(name)
	if err != nil {
		return "", err
	}

	return name, nil
}

// Проверка корректности данных импорта. Возвращается ошибка
//
// Параметры:
//
// *ConfXLSX_Import - указатель на данные импорта.
func checkImportData(e *ConfXLSX_Import) error {

	// Проверка корректности данных конфигурации хоста
	err := checkConfHost(e)
	if err != nil {
		return err
	}

	// Проверка корректности данных конфигурации устройств
	err = checkConfDev(e)
	if err != nil {
		return err
	}

	// Проверка данных конфигурации тегов устройств
	err = checkConfChannels(e)
	if err != nil {
		return err
	}

	return nil
}

// Проверка корректности данных в строке конфигурации хоста. Возвращается ошибка.
//
// Параметры:
//
// SheetMain_Head - конфигурации хоста.
func checkConfHost(e *ConfXLSX_Import) error {

	for _, host := range e.SheetMain_Header {

		v, ok := listConnType[host.ConType]
		if !ok {
			return fmt.Errorf("ошибка при проверке конфигурации хоста -> неподдерживаемый интерфейс: %v ", host.ConType)
		}
		if !v {
			return fmt.Errorf("ошибка при проверке конфигурации хоста -> отключена поддержка интерфейса: %v ", host.ConType)
		}

		switch host.ConType {
		case "TCP":
			// проверка типа коннекта
			listByte := strings.Split(host.Address, ".")
			if len(listByte) != 4 {
				return fmt.Errorf("ошибка при проверке конфигурации хоста -> количество байт адреса != 4: %v ", host.Address)
			}
			// проверка адреса
			for _, v := range listByte {
				_, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("ошибка при проверке конфигурации хоста -> в адресе указано не число: %v ", v)
				}
			}
			// проверка порта
			_, err := strconv.Atoi(host.Port)
			if err != nil {
				return fmt.Errorf("ошибка при проверке конфигурации хоста -> в номере порта не число: %v ", host.Port)
			}
			// успех
			return nil

		case "COM":

		default:
			return fmt.Errorf("ошибка при проверке конфигурации хоста -> ошибка в программы 1")
		}
	}

	return nil
}

// Проверка корректности данных в строках конфигурации устройств. Возвращается ошибка.
//
// параметры:
//
// *ConfXLSX_Import - указатель на полученные данные из файла импорта
func checkConfDev(e *ConfXLSX_Import) error {

	for _, dev := range e.SheetMain_Dev {

		devHost := dev.Host
		var conExist bool = false

		// проверка, что указанный коннект существует в конфигурации хоста
		for _, v := range e.SheetMain_Header {
			hostCon := v.Host
			if devHost == hostCon {
				conExist = conExist || true
			}
		}
		if !conExist {
			return fmt.Errorf("проверка конфигурации устройств -> указан несуществующий коннект: {%v} в строке: {%v}", devHost, dev)
		}

		// проверка указанного протокола
		v, ok := listProtocolType[dev.Type_]
		if !ok {
			return fmt.Errorf("проверка конфигурации устройств -> указан неподдерживаемый протокол: {%v} в строке: {%v}", dev.Type_, dev)
		}
		if !v {
			return fmt.Errorf("проверка конфигурации устройств -> отключена поддержка протокала: {%v} в строке: {%v}", dev.Type_, dev)
		}

		// проверка корректности сетевого адреса
		_, err := strconv.Atoi(dev.Address)
		if err != nil {
			return fmt.Errorf("проверка конфигурации устройств -> в качестве адреса не число: {%v} в строке: {%v}", dev.Address, dev)
		}

	}

	return nil
}

// Проверка корректности данных каналов устройств. Возвращается ошибка.
//
// Параметры:
//
// *ConfXLSX_Import - указатель на полученные данные из файла импорта
func checkConfChannels(e *ConfXLSX_Import) error {

	// перебор групп каналов устройств
	for _, chGroup := range e.SheetsDev {

		// перебор строк конфигураций каналов в устройстве
		for _, tag := range chGroup.Conf {

			// проверка адреса тега
			_, err := strconv.Atoi(tag.Address)
			if err != nil {
				return fmt.Errorf("проверка конфигурации тэгов устройств -> указан не адрес тэга: {%v} в строке: {%v}", tag.Address, tag)
			}

			// проверка наименования тега
			if len(tag.Name) == 0 {
				return fmt.Errorf("проверка конфигурации тэгов устройств -> нет наименования тэга в строке: {%v}", tag)
			}

			// проверка комментария тега
			if len(tag.Comment) == 0 {
				return fmt.Errorf("проверка конфигурации тэгов устройств -> нет коментария тэга в строке: {%v}", tag)
			}

			// проверка указанной функции
			val, ok := listFuncType[tag.FuncType]
			if !ok {
				return fmt.Errorf("проверка конфигурации тэгов устройств -> нет поддержки указанной функции: {%v} в строке {%v}: ", tag.FuncType, tag)
			}
			if !val {
				return fmt.Errorf("проверка конфигурации тэгов устройств ->  функция: {%v} отключена. строка {%v}: ", tag.FuncType, tag)
			}

			// проверка указанного типа данных
			_, ok = listDataType[tag.DataType]
			if !ok {
				return fmt.Errorf("проверка конфигурации тэгов устройств ->  указан неизвестный тип данных: {%v} в строке {%v} ", tag.DataType, tag)
			}

			// проверка типа данных к указанной функции
			sl, ok := listDataTypeByFunc[tag.FuncType]
			if !ok {
				return fmt.Errorf("проверка конфигурации тэгов устройств ->  ошибка программы 1: {%v} ", tag.FuncType)
			}

			var typeExist bool = false
			for _, v := range sl {
				if v == tag.DataType {
					typeExist = typeExist || true
				}
			}
			if !typeExist {
				return fmt.Errorf("проверка конфигурации тэгов устройств -> тип данных тэга: {%v}, не соответствует указанной функции: {%v}, в строке: {%v} ", tag.DataType, tag.FuncType, tag)
			}

			// проверка формата данных тега
			slFormat := strings.Split(tag.Format, "_")

			valBytes, ok := listDataTypeByBytes[tag.DataType]
			if !ok {
				return fmt.Errorf("проверка конфигурации тэгов устройств ->  ошибка программы 2: {%v} ", tag.DataType)
			}

			if valBytes != len(slFormat) {
				return fmt.Errorf("проверка конфигурации тэгов устройств ->  нет соответствия в формате данных: {%v}, к типу данных: {%v}, в строке: {%v}", tag.Format, tag.DataType, tag)
			}

			// проверка содержимого в формате типа данных
			for _, v := range slFormat {
				cntEl := 0
				for _, vv := range slFormat {
					if v == vv {
						cntEl++
					}
				}
				if cntEl > 1 {
					return fmt.Errorf("проверка конфигурации тэгов устройств ->  ошибка в формате данных, повторяемость: {%s}, в строке {%v}", v, tag)
				}
			}
		}
	}
	return nil
}
