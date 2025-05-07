package serverAPI

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type (
	GoStatus struct {
		DriverDB        bool
		DriverModbusTCP bool
		QueueModbusTCP  bool
		DriverModbusRTU bool
		QueueModbusRTU  bool
	}
)

var (
	txJson struct {
		DriverDB        string `json:"driverDB"`
		DriverModbusTCP string `json:"driverModbusTCP"`
		DriverModbusRTU string `json:"driverModbusRTU"`
		QueueModbusTCP  string `json:"queueModbusTCP"`
	}
)

// Обработчик запроса на предоставление состояния Go рутин
func (el *GoStatus) HandlGoStatus(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	txJson.DriverDB = fmt.Sprintf("%t", el.DriverDB)
	txJson.DriverModbusTCP = fmt.Sprintf("%t", el.DriverModbusTCP)
	txJson.DriverModbusRTU = fmt.Sprintf("%t", el.DriverModbusRTU)
	txJson.QueueModbusTCP = fmt.Sprintf("%t", el.QueueModbusTCP)

	resp, err := json.Marshal(txJson)
	if err != nil {
		http.Error(w, `"Внутренняя ошибка сервера"`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)

}
