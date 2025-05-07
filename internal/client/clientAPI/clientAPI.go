package clientapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

type (
	RxJson struct {
		DriverDB        string `json:"driverDB"`
		DriverModbusTCP string `json:"driverModbusTCP"`
		DriverModbusRTU string `json:"driverModbusRTU"`
		QueueModbusTCP  string `json:"queueModbusTCP"`
	}
)

// Получение статуса сервера. Возвращается ошибка.
func (rx *RxJson) StatusServer() error {

	u := "http://" + os.Getenv("HTTP_SERVER_IP") + ":" + os.Getenv("HTTP_SERVER_PORT") + "/status"

	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("ошибка создания запроса: %v", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка Get запроса: %v", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка при чтении тела ответа: %v", err)
	}

	err = json.Unmarshal(respBody, rx)
	if err != nil {
		return fmt.Errorf("ошибка обработки данных ответа: %v", err)
	}

	return nil
}
