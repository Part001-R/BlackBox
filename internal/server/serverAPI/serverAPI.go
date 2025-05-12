package serverAPI

import (
	"encoding/json"
	"net/http"
)

type (
	// Для передачи состояния сервера
	StatusServerT struct {
		TimeStart string          `json:"timeStart"`
		MbRTU     []InfoModbusRTU `json:"mbRTU"`
		MbTCP     []InfoModbusTCP `json:"mbTCP"`
		SizeF     SizeFiles       `json:"sizeFiles"`
	}
	InfoModbusRTU struct {
		ConName   string
		Con       string
		ConParams struct {
			BaudRate int
			DataBits int
			Parity   string
			StopBits int
		}
	}
	InfoModbusTCP struct {
		ConName string
		Con     string
	}
	SizeFiles struct {
		I int64
		W int64
		E int64
	}

	// Для передачи архивных данных БД
	DataDB struct {
		StartDate string   `json:"startdate"`
		Data      []DataEl `json:"datadb"`
	}
	DataEl struct {
		Name      string
		Value     string
		Qual      string
		TimeStamp string
	}
)

// Обработчик запроса на предоставление состояния Go рутин
func (el *StatusServerT) HandlStatusSrv(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	var statusServer StatusServerT

	statusServer.TimeStart = el.TimeStart
	statusServer.MbRTU = el.MbRTU
	statusServer.MbTCP = el.MbTCP
	statusServer.SizeF = el.SizeF

	resp, err := json.Marshal(statusServer)
	if err != nil {
		http.Error(w, `"Внутренняя ошибка сервера"`, http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Обработчик запроса на экспорт архивных данных БД
func (el *DataDB) HandlExpDataDB(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	resp, err := json.Marshal(el)
	if err != nil {
		http.Error(w, `"внутренняя ошибка сервера"`, http.StatusInternalServerError)
	}

	w.WriteHeader(http.StatusOK)
	w.Write(resp)

}
