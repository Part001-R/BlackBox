package clientapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
)

type (
	// JSON для приёма данных состояния сервера
	RxStatusSrv struct {
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

	// JSON для приёма архивных данных БД
	RxDataDB struct {
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

// Получение статуса сервера. Возвращается ошибка.
func (rx *RxStatusSrv) ReqStatusServer() error {

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

// Получение архивных данных БД. Возвращается ошибка
func (rx *RxDataDB) ReqDataDB() error {

	u := fmt.Sprintf("http://%s:%s/datadb", os.Getenv("HTTP_SERVER_IP"), os.Getenv("HTTP_SERVER_PORT"))

	parseU, err := url.Parse(u)
	if err != nil {
		return fmt.Errorf("ошибка парсинга URL при запросе архивных данных БД: {%v}", err)
	}

	rawQ := url.Values{}
	rawQ.Add("startdate", rx.StartDate)

	parseU.RawQuery = rawQ.Encode()

	req, err := http.NewRequest(http.MethodGet, parseU.String(), nil)
	if err != nil {
		return fmt.Errorf("ошибка формирования запроса: {%v}", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса к серверу: {%v}", err)
	}

	dataResp, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка чтения тела ответа: {%v}", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	err = json.Unmarshal(dataResp, &rx)
	if err != nil {
		return fmt.Errorf("ошибка при десериализации принятых данных от сервера: {%v}", err)
	}

	return nil
}
