package modbustcpmaster

import (
	"fmt"
	"time"

	modbus "github.com/thinkgos/gomodbus/v2"
)

func Test() {

	// Подключение к серверу
	// Указывается IP сервера и порт
	p := modbus.NewTCPClientProvider("192.169.1.111:502")

	// Создание клиента для подключения к серверу
	client := modbus.NewClient(p)
	err := client.Connect()
	if err != nil {
		fmt.Println("connect failed, ", err)
		return
	}
	defer client.Close()

	fmt.Println("starting")

	// Опрос
	for {
		results, err := client.ReadHoldingRegisters(1, 0, 3)
		if err != nil {
			fmt.Println(err.Error())
		} else {
			fmt.Printf("Read_1: %d\n", results)
		}

		time.Sleep(time.Second * 5)
	}

}
