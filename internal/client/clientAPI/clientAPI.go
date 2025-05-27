package clientapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type (
	// Для передачи количества строк
	CntStrT struct {
		CntStr string `json:"cntstr"`
	}

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
		CntStr    string   `json:"cntstr"`
		Data      []DataEl `json:"datadb"`
	}
	DataEl struct {
		Name      string
		Value     string
		Qual      string
		TimeStamp string
	}

	// Для хранения всех запрошенных частей
	AllDataDB struct {
		AllData []PartDataDB
	}
	PartDataDB struct {
		NumbReq int      `json:"numbreq"`
		Data    []DataEl `json:"data"`
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

	// Проверка успешности запроса
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("принят код статуса отличный от ОК: {%v}", resp.StatusCode)
	}

	rx.CntStr = resp.Header.Get("Count-Strings")
	if rx.CntStr == "" {
		return errors.New("нет принятых данных в заголовке Count-Strings")
	}

	// Чтение тела
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

// Получение архивных данных БД в виде xlsx файла. Возвращается ошибка
func (rx *RxDataDB) ReqXlsxDataDB() error {

	u := fmt.Sprintf("http://%s:%s/xlsx", os.Getenv("HTTP_SERVER_IP"), os.Getenv("HTTP_SERVER_PORT"))

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

	// Параметрирование клиента и запрос
	client := &http.Client{
		Timeout: 20 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("ошибка выполнения запроса к серверу: {%v}", err)
	}

	// Проверка успешности запроса
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("принят код статуса отличный от ОК: {%v}", resp.StatusCode)
	}

	rx.CntStr = resp.Header.Get("Count-Strings")
	if rx.CntStr == "" {
		return errors.New("нет данных по количеству строк в файле")
	}

	// Получение имени файла из данных заголовка
	contentDisposition := resp.Header.Get("Content-Disposition")
	if contentDisposition == "" {
		return errors.New("нет данных в заголовке Content-Disposition")
	}
	var fileName string
	parts := strings.Split(contentDisposition, "filename=")
	if len(parts) > 1 {
		fileName = parts[1]
		fileName = strings.Trim(fileName, "\"") // Удаление кавычек, если они есть
	}

	// Создание файла для сохранения данных
	outFile, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("ошибка {%v} создания xlsx файла {%s} для сохранения данных", err, fileName)
	}
	defer outFile.Close()

	// Копирование данных из HTTP-ответа в файл
	_, err = io.Copy(outFile, resp.Body)
	if err != nil {
		return fmt.Errorf("ошибка {%v} копирования принятых данных в xlsx файл {%s}", err, fileName)
	}

	return nil
}

// Получение количество строк БД по указанной дате. Возвращается ошибка
func (rx *RxDataDB) ReqCntStrByDateDB() error {

	u := fmt.Sprintf("http://%s:%s/cntstr", os.Getenv("HTTP_SERVER_IP"), os.Getenv("HTTP_SERVER_PORT"))

	parseU, err := url.Parse(u)
	if err != nil {
		return errors.New("req-cntStr -> ошибка парсинга URL")
	}

	qP := url.Values{}
	qP.Set("date", rx.StartDate)

	parseU.RawQuery = qP.Encode()

	// Формирование запроса
	req, err := http.NewRequest(http.MethodGet, parseU.String(), nil)
	if err != nil {
		return fmt.Errorf("req-cntStr -> ошибка {%v} при создании запроса", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("req-cntStr -> ошибка {%v} запроса", err)
	}

	// Проверка кода ответа
	if resp.StatusCode != http.StatusOK {
		return errors.New("req-cntStr -> сервер не отправил код 200")
	}

	// Чтение тела ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return errors.New("req-cntStr -> ошибка чтения тела ответа")
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	var cntInfo CntStrT

	err = json.Unmarshal(body, &cntInfo)
	if err != nil {
		return fmt.Errorf("req-cntStr -> ошибка десиреализации тела ответа:{%v}", err)
	}

	// Проверка содержимого тела ответа
	if cntInfo.CntStr == "" {
		return errors.New("req-cntStr -> в теле ответа нет данных о количестве строк БД")
	}
	_, err = strconv.Atoi(cntInfo.CntStr)
	if err != nil {
		return fmt.Errorf("req-cntStr -> принятое значение {%s} количества строк, не является числом", cntInfo.CntStr)
	}

	// сохранение количества строк
	rx.CntStr = cntInfo.CntStr

	return nil
}

// Частичный запрос строк БД по дате, количеству строк и смещению. Возвращается результат запроса и ошибка.
//
// Параметры:
//
// numbReq - номер запроса
// strLimit - количество строк
// strOffSet - смещение номеров строк
// dataDB - дата
func ReqPartDataDB(numbReg, strLimit, strOffSet int, dateDB string) (data PartDataDB, err error) {

	// Проверка значений аргументов
	if numbReg < 0 {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> значение аргумента numbReg {%d}, меньше нуля", numbReg)
	}
	if strLimit < 0 {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> значение аргумента strLimit {%d}, меньше нуля", strLimit)
	}
	if strOffSet < 0 {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> значение аргумента strOffSet {%d}, меньше нуля", strOffSet)
	}
	_, err = time.Parse("2006-01-02", dateDB)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> значение аргумента dataDB {%s}, не дата", dateDB)
	}

	// Подготовка URL
	u := fmt.Sprintf("http://%s:%s/partdatadb", os.Getenv("HTTP_SERVER_IP"), os.Getenv("HTTP_SERVER_PORT"))

	parseU, err := url.Parse(u)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> ошибка парсинга URL {%v}", err)
	}

	qP := url.Values{}
	qP.Set("numbReg", fmt.Sprintf("%d", numbReg))
	qP.Set("strLimit", fmt.Sprintf("%d", strLimit))
	qP.Set("strOffSet", fmt.Sprintf("%d", strOffSet))
	qP.Set("dateDB", dateDB)

	parseU.RawQuery = qP.Encode()

	// Формирование запроса
	req, err := http.NewRequest(http.MethodGet, parseU.String(), nil)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> ошибка формирования запроса {%v}", err)
	}

	// Запрос к серверу
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> ошибка выполнения запроса {%v}", err)
	}

	// Проверка статус-кода ответа сервера на 200
	if resp.StatusCode != http.StatusOK {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> сервер вернул код {%d}", resp.StatusCode)
	}

	// Обработка ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> ошибка чтения тела ответа {%v}", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	err = json.Unmarshal(body, &data)
	if err != nil {
		return PartDataDB{}, fmt.Errorf("req-partdatadb -> ошибка десиарелизации ответа {%v}", err)
	}

	return data, nil
}
