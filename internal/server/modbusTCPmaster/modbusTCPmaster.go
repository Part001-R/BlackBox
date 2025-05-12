package modbustcpmaster

import (
	"log"
	"net"
	"time"

	modbus "github.com/thinkgos/gomodbus/v2"
)

type (
	Connect struct {
		Name      string
		SlaveIP   string
		SlavePort string
		HostIP    string
		HostPort  string
		Client    modbus.Client
		Close     func() error
		IsRun     bool
	}
)

// Создание подключения. Функция возвращает ошибку.
func (mb *Connect) Connect() error {

	host := mb.HostIP + ":" + mb.HostPort
	slave := mb.SlaveIP + ":" + mb.SlavePort

	localAddr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		log.Fatalf("Ошибка ResolveTCPAddr при создании TCP соединения: %v", err)
	}

	// Dialer с указанием локального адреса
	dialer := &net.Dialer{
		LocalAddr: localAddr,
		Timeout:   5 * time.Second,
	}

	// Установка TCP-соединение с использованием Dialer
	conn, err := dialer.Dial("tcp", slave)
	if err != nil {
		log.Fatalf("Ошибка Dial при создании TCP соединения: %v", err)
	}
	defer conn.Close()

	// Создаем  provider, но используем уже установленное соединение
	provider := modbus.NewTCPClientProvider(slave)

	// Функция закрытия подклчения
	mb.Close = func() error {
		err := provider.Close()
		return err
	}

	// Создание клиента для подключения к серверу
	mb.Client = modbus.NewClient(provider)

	// Подключение
	err = mb.Client.Connect()
	if err != nil {
		return err
	}

	// Установка признака активности
	mb.IsRun = true

	return nil
}

// Чтение данных. Функция возвращает результат чтения и ошибку.
//
// Параметры:
//
// slID - адрес слейва
// addr - начальный адрес регистра
// quant - количество запрашиваемых значений
func (mb *Connect) ReadData(slID byte, addr, quant uint16) (res []uint16, err error) {

	res, err = mb.Client.ReadHoldingRegisters(slID, addr, quant)
	if err != nil {
		return nil, err
	}
	return res, nil

}
