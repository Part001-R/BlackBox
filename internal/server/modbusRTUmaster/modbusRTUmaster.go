package modbusrtumaster

import (
	"fmt"
	"time"

	"github.com/goburrow/modbus"
)

type (
	Connect struct {
		Name          string
		Port          string
		Client        modbus.Client
		Provider      *modbus.RTUClientHandler
		ParamsConn    Params
		Close         func() error
		ChangeSlaveID func(slaveId byte)
		SlaveAddr     byte
		IsRun         bool
	}

	Params struct {
		BaudRate int    // Скорость передачи
		DataBits int    // Количество бит данных
		Parity   string // Четность (N - None, E - Even, O - Odd)
		StopBits int    // Количество стоп-битов
	}
)

// Создание подключения. Функция возвращает ошибку.
func (mb *Connect) Connect() error {

	provider := modbus.NewRTUClientHandler(mb.Port) // COM-порт "/dev/ttyUSB0"
	provider.BaudRate = mb.ParamsConn.BaudRate      // Скорость передачи
	provider.DataBits = mb.ParamsConn.DataBits      // Количество бит данных
	provider.Parity = mb.ParamsConn.Parity          // Четность (N - None, E - Even, O - Odd)
	provider.StopBits = mb.ParamsConn.StopBits      // Количество стоп-битов
	provider.SlaveId = mb.SlaveAddr                 // ID ведомого устройства
	provider.Timeout = 50 * time.Millisecond        // Таймаут ответа
	//provider.Logger = log.New(os.Stdout, "modbus: ", log.LstdFlags)

	// Создание коннекта
	err := provider.Connect()
	if err != nil {
		return fmt.Errorf("ошибка создания СОМ коннекта: {%v}", err)
	}

	// Создание Modbus клиента
	mb.Client = modbus.NewClient(provider)
	if mb.Client == nil {
		return fmt.Errorf("клиент для {%s} не создан", mb.Name)
	}

	// Закрытие подключения при завершении работы
	mb.Close = func() error {
		err := provider.Close()
		if err != nil {
			return fmt.Errorf("ошибка {%v} при закрытии СОМ подключения по порту: {%v}", err, mb.Port)
		}
		return nil
	}

	mb.ChangeSlaveID = func(slaveId byte) {
		provider.SlaveId = slaveId
	}

	mb.IsRun = true
	return nil
}
