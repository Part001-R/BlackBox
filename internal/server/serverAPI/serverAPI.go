package serverAPI

import (
	loger "blackbox/internal/server/loger"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type (
	// Для передачи количества строк
	CntStrT struct {
		CntStr string `json:"cntstr"`
	}

	// Для даты и имени
	DateNameT struct {
		Date string `json:"date"`
		Name string `json:"name"`
	}

	// Для имени
	NameT struct {
		Name string `json:"name"`
	}

	// Для токена
	TokenT struct {
		Token string `json:"token"`
	}

	// Для передачи состояния сервера
	StatusServerT struct {
		TimeStart string
		MbRTU     []InfoModbusRTUT
		MbTCP     []InfoModbusTCPT
		SizeF     SizeFilesT
		DB        *sql.DB
		Lgr       loger.Log_Object
	}

	// Для передачи состояния сервера
	StatusT struct {
		TimeStart string           `json:"timeStart"`
		MbRTU     []InfoModbusRTUT `json:"mbRTU"`
		MbTCP     []InfoModbusTCPT `json:"mbTCP"`
		SizeF     SizeFilesT       `json:"sizeFiles"`
	}
	InfoModbusRTUT struct {
		ConName   string
		Con       string
		ConParams struct {
			BaudRate int
			DataBits int
			Parity   string
			StopBits int
		}
	}
	InfoModbusTCPT struct {
		ConName string
		Con     string
	}
	SizeFilesT struct {
		I int64
		W int64
		E int64
	}

	// Для передачи архивных данных БД
	DataDBCallT struct {
		StartDate string
		CntStrDB  int
		Data      []DataElT
		DB        *sql.DB
		Lgr       loger.Log_Object
		FileName  string
	}

	DataDBT struct {
		StartDate string    `json:"startdate"`
		Data      []DataElT `json:"datadb"`
	}
	DataElT struct {
		Name      string
		Value     string
		Qual      string
		TimeStamp string
	}

	// Для регистрации пользователя на https сервере
	LoginUserT struct {
		DB  *sql.DB
		Lgr loger.Log_Object
	}

	// Для получения количества строк БД по дате
	CntStrByDateT struct {
		DB  *sql.DB
		Lgr loger.Log_Object
	}

	// Для получения части строк БД
	PartDataT struct {
		DB  *sql.DB
		Lgr loger.Log_Object
	}

	// Для передачи сразу всех данных по дате
	AllDataByDateT struct {
		DB  *sql.DB
		Lgr loger.Log_Object
	}

	// Для передачи данных при частичной выгрузке
	PartDataDBT struct {
		NumbReq int       `json:"numbreq"`
		Data    []DataElT `json:"data"`
	}
)

// Обработчик запроса на предоставление состояния Go рутин
func (el *StatusServerT) HandlHttpsStatusSrv(w http.ResponseWriter, r *http.Request) {

	// Проверка указателя на БД
	if el.DB == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Проверка указателей логеров
	if el.Lgr.I == nil || el.Lgr.W == nil || el.Lgr.E == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Проверка метода запроса
	if r.Method != http.MethodPost {
		el.Lgr.W.Printf("https-status -> принят запрос с методом {%s}, а нужен {%s}", r.Method, http.MethodPost)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение заголовков звпроса
	token := r.Header.Get("authorization")
	if token == "" {
		el.Lgr.W.Println("https-status -> нет токена, в запросе")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение тела запроса
	var rxBody NameT

	bytesBody, err := io.ReadAll(r.Body)
	if err != nil {
		el.Lgr.W.Println("https-status -> ошибка чтения тела запроса")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			el.Lgr.E.Println("https-status -> ошибка закрытия потока чтения тела запроса при завершении работы обработчика запроса")
		}
	}()

	err = json.Unmarshal(bytesBody, &rxBody)
	if err != nil {
		el.Lgr.W.Println("https-status -> ошибка десериализации данных тела запроса")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка, что принятое имя и его токен соответствуют
	tokenDB, err := ReadUserTokenByNameDB(rxBody.Name, el.DB)
	if err != nil {
		el.Lgr.W.Printf("https-status -> ошибка при получении токена, по имени пользователя {%v}", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if token != tokenDB {
		el.Lgr.W.Println("https-status -> принятый токен и токен из БД не соответствуют")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Подготовка данных для отправки
	var statusServer StatusT

	statusServer.TimeStart = el.TimeStart
	statusServer.MbRTU = el.MbRTU
	statusServer.MbTCP = el.MbTCP
	statusServer.SizeF = el.SizeF

	// Проверка содержимого ответа
	if _, err = time.Parse("2006-01-02 15:04:05", statusServer.TimeStart); err != nil {
		el.Lgr.E.Printf("https-status -> данные времени запуска сервера не в формате YYYY-MM-DD HH:MM:SS, содержится - {%s}", statusServer.TimeStart)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(statusServer.MbRTU) == 0 {
		el.Lgr.E.Println("https-status -> отсутствуют массив данных MbRTU")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(statusServer.MbTCP) == 0 {
		el.Lgr.E.Println("https-status -> отсутствуют массив данных MbTCP")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.I < 0 {
		el.Lgr.E.Printf("https-status -> отрицательный размер файла логера I - {%d}", statusServer.SizeF.I)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.W < 0 {
		el.Lgr.E.Printf("https-status -> отрицательный размер файла логера W - {%d}", statusServer.SizeF.W)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.E < 0 {
		el.Lgr.E.Printf("https-status -> отрицательный размер файла логера E - {%d}", statusServer.SizeF.E)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	for i, v := range statusServer.MbRTU {
		if v.Con == "" {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле Con", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConName == "" {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле ConName", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.BaudRate == 0 {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле BaudRate", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.DataBits == 0 {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле DataBits", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.StopBits == 0 {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле StopBits", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.Parity == "" {
			el.Lgr.E.Printf("https-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле Parity", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	for i, v := range statusServer.MbTCP {
		if v.Con == "" {
			el.Lgr.E.Printf("https-status -> массив MbTCP по индексу {%d} - отсутствует содержимое в поле Con", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConName == "" {
			el.Lgr.E.Printf("https-status -> массив MbTCP по индексу {%d} - отсутствует содержимое в поле ConName", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Сериализация данных ответа
	resp, err := json.Marshal(statusServer)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Передача данных
	el.Lgr.I.Printf("https-status -> пользователь {%s} запросил состояние сервера", rxBody.Name)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Обработчик запроса на предоставление состояния Go рутин
func (el *StatusServerT) HandlHttpStatusSrv(w http.ResponseWriter, r *http.Request) {

	// Проверка указателя на БД
	if el.DB == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Проверка указателей логеров
	if el.Lgr.I == nil || el.Lgr.W == nil || el.Lgr.E == nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Проверка метода запроса
	if r.Method != http.MethodGet {
		el.Lgr.W.Printf("http-status -> принят запрос с методом {%s}, а нужен {%s}", r.Method, http.MethodPost)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Подготовка данных для отправки
	var statusServer StatusT

	statusServer.TimeStart = el.TimeStart
	statusServer.MbRTU = el.MbRTU
	statusServer.MbTCP = el.MbTCP
	statusServer.SizeF = el.SizeF

	// Проверка содержимого ответа
	if _, err := time.Parse("2006-01-02 15:04:05", statusServer.TimeStart); err != nil {
		el.Lgr.E.Printf("http-status -> данные времени запуска сервера не в формате YYYY-MM-DD HH:MM:SS, содержится - {%s}", statusServer.TimeStart)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(statusServer.MbRTU) == 0 {
		el.Lgr.E.Println("http-status -> отсутствуют массив данных MbRTU")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if len(statusServer.MbTCP) == 0 {
		el.Lgr.E.Println("http-status -> отсутствуют массив данных MbTCP")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.I < 0 {
		el.Lgr.E.Printf("http-status -> отрицательный размер файла логера I - {%d}", statusServer.SizeF.I)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.W < 0 {
		el.Lgr.E.Printf("http-status -> отрицательный размер файла логера W - {%d}", statusServer.SizeF.W)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	if statusServer.SizeF.E < 0 {
		el.Lgr.E.Printf("http-status -> отрицательный размер файла логера E - {%d}", statusServer.SizeF.E)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	for i, v := range statusServer.MbRTU {
		if v.Con == "" {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле Con", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConName == "" {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле ConName", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.BaudRate == 0 {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле BaudRate", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.DataBits == 0 {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле DataBits", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.StopBits == 0 {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле StopBits", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConParams.Parity == "" {
			el.Lgr.E.Printf("http-status -> массив MbRTU по индексу {%d} - отсутствует содержимое в поле Parity", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}
	for i, v := range statusServer.MbTCP {
		if v.Con == "" {
			el.Lgr.E.Printf("http-status -> массив MbTCP по индексу {%d} - отсутствует содержимое в поле Con", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		if v.ConName == "" {
			el.Lgr.E.Printf("http-status -> массив MbTCP по индексу {%d} - отсутствует содержимое в поле ConName", i)
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
	}

	// Сериализация данных
	resp, err := json.Marshal(statusServer)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Передача данных
	w.Header().Set("Content-Type", "application/json")

	el.Lgr.I.Printf("http-status -> локальный запрос состояние сервера")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Обработчик запроса на экспорт архивных данных БД
func (el *DataDBCallT) HandlHttpsExpDataDB(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	// Чтение заголовка
	token := r.Header.Get("authorization")
	if token == "" {
		el.Lgr.W.Println("https-dataDB -> нет токена")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение параметров запроса
	qPrm := r.URL.Query()
	name := qPrm.Get("name")
	if name == "" {
		el.Lgr.W.Println("https-dataDB -> нет данных по параметру name")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка, что принятое имя и его токен соответствуют
	tokenDB, err := ReadUserTokenByNameDB(name, el.DB)
	if err != nil {
		el.Lgr.W.Printf("https-dataDB -> ошибка при получении токена, по имени пользователя {%v}", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if token != tokenDB {
		el.Lgr.W.Println("https-dataDB -> принятый токен и токен из БД не соответствуют")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	cntStr := strconv.Itoa(el.CntStrDB)
	w.Header().Set("Count-Strings", cntStr)

	// Подготовка данных для ответа
	var data DataDBT
	data.StartDate = el.StartDate
	data.Data = el.Data

	resp, err := json.Marshal(data)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	el.Lgr.I.Printf("https-dataDB -> пользователь {%s} запросил архивные данные на %s", name, el.StartDate)
	w.WriteHeader(http.StatusOK)
	w.Write(resp)

}

// Обработчик запроса на экспорт архивных данных БД
func (el *DataDBCallT) HandlHttpExpDataDB(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	// Подготовка данных для ответа
	var data DataDBT
	data.StartDate = el.StartDate
	data.Data = el.Data

	cntStr := strconv.Itoa(el.CntStrDB)
	w.Header().Set("Count-Strings", cntStr)

	resp, err := json.Marshal(data)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	el.Lgr.I.Println("http-dataDB -> локальный запрос исторических данных")
	w.WriteHeader(http.StatusOK)
	w.Write(resp)

}

// Обработчик запроса на передачу xlsx фала архивных данных БД
func (el *DataDBCallT) HandlHttpXlsxDataDB(w http.ResponseWriter, r *http.Request) {

	excelFilePath := "./" + el.FileName

	file, err := os.Open(excelFilePath)
	if err != nil {
		el.Lgr.E.Printf("Hndl-xlsx -> ошибка {%v} при открытии файла {./%s}", err, el.FileName)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		err := file.Close()
		if err != nil {
			el.Lgr.E.Printf("Hndl-xlsx -> ошибка {%v} закрытия файла {./%s} по завершению работы обработчика", err, el.FileName)
		}
	}()

	cntStr := strconv.Itoa(el.CntStrDB)
	w.Header().Set("Count-Strings", cntStr)

	fileInfo, err := file.Stat()
	if err != nil {
		el.Lgr.E.Printf("Hndl-xlsx -> ошибка {%v} при получении информации о файле {%s}", err, el.FileName)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", fileInfo.Size()))

	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename="+el.FileName)

	_, err = io.Copy(w, file)
	if err != nil {
		el.Lgr.E.Printf("Hndl-xlsx -> ошибка {%v} при передаче файла {./%s}", err, el.FileName)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
}

// Обработчик регистрации пользователя на https сервере
func (el *LoginUserT) HandlHttpsRegistration(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		el.Lgr.E.Printf("https-registration -> принят запрос с методом: {%s}, а нужен: {POST}", r.Method)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rcvStr, err := io.ReadAll(r.Body)
	if err != nil {
		el.Lgr.E.Println("https-registration -> ошибка при чтении тела запроса")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	defer func() {
		_ = r.Body.Close()
	}()

	slStr := strings.Split(string(rcvStr), " ")
	if len(slStr) != 2 {
		el.Lgr.W.Println("https-registration -> принят запрос с не верным содержимым")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка принятых данных
	if len(slStr) != 2 {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxUsrName := slStr[0]
	rxUsrPsw := slStr[1]

	if rxUsrName == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if rxUsrPsw == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение из БД хэша пароля пользователя
	dbPswHash, err := readPswUserDB(rxUsrName, el.DB)
	if err != nil {
		el.Lgr.W.Printf("https-registration -> попытка подключения пользователя {%s}, такого пользователя в БД нет\n", rxUsrName)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка соответствия хэшей
	calcHash := fmt.Sprintf("%x", sha256.Sum256([]byte(rxUsrPsw)))

	if dbPswHash != calcHash {
		el.Lgr.W.Printf("https-registration -> принят запрос пользователя {%s} с не верным паролем", rxUsrName)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Вычисление токена
	var dataToken TokenT
	dataToken.Token = generateToken(rxUsrName, rxUsrPsw)

	// Сохрание токена в БД
	err = saveTokenUserDB(rxUsrName, dataToken.Token, el.DB)
	if err != nil {
		el.Lgr.E.Printf("https-registration -> ошибка {%v} при сохранении в БД хэша пароля для пользователя {%s}\n", err, rxUsrName)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	dataTx, err := json.Marshal(dataToken)
	if err != nil {
		el.Lgr.E.Printf("https-registration -> ошибка {%v} сериализации данных {%v}", err, dataToken)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	el.Lgr.I.Printf("https-registration -> пользователь {%s} зарегистрировался на сервере\n", rxUsrName)

	w.Header().Set("Content-Type", "application-json")
	w.WriteHeader(http.StatusOK)
	w.Write(dataTx)
}

// Обработка запроса на количество строк в БД по дате.
func (el *CntStrByDateT) HandlHttpsCntStrByDate(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		el.Lgr.W.Printf("https-cntstr -> принят запрос с методом:{%s}, а нужен:{%s}", r.Method, http.MethodPost)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение заголовка
	token := r.Header.Get("authorization")
	if token == "" {
		el.Lgr.W.Println("https-cntstr -> в принятом запросе нет токена")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Тело запроса
	bytesBody, err := io.ReadAll(r.Body)
	if err != nil {
		el.Lgr.W.Println("https-cntstr -> ошибка чтения тела запроса")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			el.Lgr.W.Println("https-cntstr -> ошибка закрытия потока чтения тела ответа при завершении работы обработчика")
		}
	}()

	var reqBoddy DateNameT
	err = json.Unmarshal(bytesBody, &reqBoddy)
	if err != nil {
		el.Lgr.W.Printf("https-cntstr -> ошибка при десериализации {%s}", reqBoddy.Date)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка данных тела запроса
	_, err = time.Parse("2006-01-02", reqBoddy.Date)
	if err != nil {
		el.Lgr.W.Printf("https-cntstr -> в принятом запросе, дата {%s} не в формате YYYY-MM-DD", reqBoddy.Date)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if reqBoddy.Name == "" {
		el.Lgr.W.Println("https-cntstr -> в принятом запросе нет данных имени")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка, что принятое имя и его токен соответствуют
	tokenDB, err := ReadUserTokenByNameDB(reqBoddy.Name, el.DB)
	if err != nil {
		el.Lgr.W.Printf("https-cntstr -> ошибка получения токена, по имени пользователя:{%s} {%v}", reqBoddy.Name, err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if token != tokenDB {
		el.Lgr.W.Println("https-cntstr -> принятый токен и токен из БД не соответствуют")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Получение количества строк по указанной дате
	d := DataDBCallT{
		StartDate: reqBoddy.Date,
	}
	d.DB = el.DB
	d.Lgr = el.Lgr

	err = readCntStrDataDB(&d)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	cntStr := strconv.Itoa(d.CntStrDB)

	cntInfo := CntStrT{
		CntStr: cntStr,
	}
	bTx, err := json.Marshal(cntInfo)
	if err != nil {
		el.Lgr.W.Println("https-cntstr -> ошибка сериализации данных счётчика строк")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application-json")

	el.Lgr.I.Printf("https-cntstr -> выполнен запрос количество строк в БД на {%s}", reqBoddy.Date)
	w.WriteHeader(http.StatusOK)
	w.Write(bTx)
}

// Обработка запроса на количество строк в БД по дате.
func (el *CntStrByDateT) HandlHttpCntStrByDate(w http.ResponseWriter, r *http.Request) {

	// Проверка входных данных
	if r.Method != http.MethodGet {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение параметров запроса
	qP := r.URL.Query()

	dateExp := qP.Get("date")
	if dateExp == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка корректности даты
	_, err := time.Parse("2006-01-02", dateExp)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Получение количества строк по указанной дате
	d := DataDBCallT{
		StartDate: dateExp,
	}
	d.DB = el.DB
	d.Lgr = el.Lgr

	err = readCntStrDataDB(&d)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Подготовка ответа
	cntStr := strconv.Itoa(d.CntStrDB)

	cntInfo := CntStrT{
		CntStr: cntStr,
	}
	bTx, err := json.Marshal(cntInfo)
	if err != nil {
		el.Lgr.W.Println("https-cntstr -> ошибка сериализации данных счётчика строк")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	el.Lgr.I.Printf("клиент http -> выполнен запрос количество строк в БД на {%s}", dateExp)
	w.WriteHeader(http.StatusOK)
	w.Write(bTx)
}

// Обработка запроса на загрузку части строк
func (el *PartDataT) HandlHttpsPartDataDB(w http.ResponseWriter, r *http.Request) {

	if r.Method != http.MethodPost {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос:{%s}, а ожидается POST", r.Method)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение заголовков
	token := r.Header.Get("authorization")
	if token == "" {
		el.Lgr.W.Println("hdlr-partdatadb -> нет токена")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Тело запроса
	var reqBody DateNameT

	bytesBody, err := io.ReadAll(r.Body)
	if err != nil {
		el.Lgr.W.Println("https-partdatadb -> ошибка чтения тела запроса")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer func() {
		err = r.Body.Close()
		if err != nil {
			el.Lgr.W.Println("https-partdatadb -> ошибка закрытия потока чтения тела ответа при завершении работы обработчика")
		}
	}()

	err = json.Unmarshal(bytesBody, &reqBody)
	if err != nil {
		el.Lgr.W.Println("https-partdatadb -> ошибка десериализации тела запроса")
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Проверка данных тела запроса
	_, err = time.Parse("2006-01-02", reqBody.Date)
	if err != nil {
		el.Lgr.W.Printf("https-partdatadb -> в date не дате {%s}", reqBody.Date)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if reqBody.Name == "" {
		el.Lgr.W.Println("https-partdatadb -> нет данных в name")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение параметров запроса. Проверка.
	qP := r.URL.Query()

	RxNumbReg := qP.Get("numbReg")
	if RxNumbReg == "" {
		el.Lgr.W.Println("hdlr-partdatadb -> принят запрос с пустым содержимым numbReg")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	numbReq, err := strconv.Atoi(RxNumbReg)
	if err != nil {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос где в numbReg не число {%s}", RxNumbReg)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if numbReq < 0 {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос c отрицательным numbReg {%s}", RxNumbReg)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxStrLimit := qP.Get("strLimit")
	if rxStrLimit == "" {
		el.Lgr.W.Println("hdlr-partdatadb -> принят запрос с пустым содержимым strLimit")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(rxStrLimit)
	if err != nil {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос где в strLimit не число {%s}", rxStrLimit)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if limit < 0 {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос c отрицательным limit {%s}", rxStrLimit)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxStrOffSet := qP.Get("strOffSet")
	if rxStrOffSet == "" {
		el.Lgr.W.Println("hdlr-partdatadb -> принят запрос с пустым содержимым strOffSet")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	OffSet, err := strconv.Atoi(rxStrOffSet)
	if err != nil {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос где в strOffSet не число {%s}", rxStrOffSet)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if OffSet < 0 {
		el.Lgr.W.Printf("hdlr-partdatadb -> принят запрос c отрицательным OffSet {%s}", rxStrOffSet)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка, что принятое имя и его токен соответствуют
	tokenDB, err := ReadUserTokenByNameDB(reqBody.Name, el.DB)
	if err != nil {
		el.Lgr.W.Printf("hdlr-partdatadb -> ошибка при получении токена, по имени пользователя {%v}", err)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	if token != tokenDB {
		el.Lgr.W.Println("hdlr-partdatadb -> принятый токен и токен из БД не соответствуют")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение данных БД
	rdDataDB, err := readPartDataDBReq(el.DB, reqBody.Date, limit, OffSet)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	dataForTx := PartDataDBT{
		NumbReq: numbReq,
		Data:    rdDataDB,
	}

	txByte, err := json.Marshal(dataForTx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	//el.Lgr.I.Printf("https-PartData -> Предоставлены данные. По дате:{%s}, номер запроса:{%d}, строк:{%d}, смещение:{%d}", reqBody.Date, numbReq, len(rdDataDB), OffSet)

	w.Header().Set("Content-Type", "application-json")
	w.WriteHeader(http.StatusOK)
	w.Write(txByte)
}

// Обработка запроса на загрузку части строк
func (el *PartDataT) HandlHttpPartDataDB(w http.ResponseWriter, r *http.Request) {

	// Проверка метода запроса
	if r.Method != http.MethodGet {
		el.Lgr.W.Printf("http-partdatadb -> принят запрос с методом:{%s}, а ожидается POST", r.Method)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение параметров запроса. Проверка.
	qP := r.URL.Query()

	RxNumbReg := qP.Get("numbReg")
	if RxNumbReg == "" {
		el.Lgr.W.Println("http-partdatadb -> принят запрос с пустым содержимым numbReg")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	numbReq, err := strconv.Atoi(RxNumbReg)
	if err != nil {
		el.Lgr.W.Printf("http-partdatadb -> принят запрос где в numbReg не число {%s}", RxNumbReg)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if numbReq < 0 {
		el.Lgr.W.Printf("http-partdatadb -> отрицательное число в RxNumbReg  {%s}", RxNumbReg)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxStrLimit := qP.Get("strLimit")
	if rxStrLimit == "" {
		el.Lgr.W.Println("http-partdatadb -> принят запрос с пустым содержимым strLimit")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	limit, err := strconv.Atoi(rxStrLimit)
	if err != nil {
		el.Lgr.W.Printf("http-partdatadb -> принят запрос где в strLimit не число {%s}", rxStrLimit)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if limit < 0 {
		el.Lgr.W.Printf("http-partdatadb -> отрицательное число в limit {%s}", rxStrLimit)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	rxStrOffSet := qP.Get("strOffSet")
	if rxStrOffSet == "" {
		el.Lgr.W.Println("http-partdatadb -> принят запрос с пустым содержимым strOffSet")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	OffSet, err := strconv.Atoi(rxStrOffSet)
	if err != nil {
		el.Lgr.W.Printf("http-partdatadb -> принят запрос где в strOffSet не число {%s}", rxStrOffSet)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	if OffSet < 0 {
		el.Lgr.W.Printf("http-partdatadb -> отрицательное число в OffSet {%s}", rxStrOffSet)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	dateDB := qP.Get("dateDB")
	if dateDB == "" {
		el.Lgr.W.Println("http-partdatadb -> принят запрос с пустым содержимым dateDB")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	_, err = time.Parse("2006-01-02", dateDB)
	if err != nil {
		el.Lgr.W.Printf("http-partdatadb -> принят запрос где в dateDB не дата {%s}", dateDB)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение данных БД
	rdDataDB, err := readPartDataDBReq(el.DB, dateDB, limit, OffSet)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Ответ
	dataForTx := PartDataDBT{
		NumbReq: numbReq,
		Data:    rdDataDB,
	}

	txByte, err := json.Marshal(dataForTx)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write(txByte)
}

// Чтение пароля пользователя. Возвращается хэш пароля и ошибка.
//
// Параметры:
//
// name - имя пользователя
// db - указатель на БД
func readPswUserDB(name string, db *sql.DB) (psw string, err error) {

	q := fmt.Sprintf("SELECT password FROM %s.%s WHERE name = $1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	err = db.QueryRow(q, name).Scan(&psw)
	if err != nil {
		return "", fmt.Errorf("ошибка: {%v} при чтении пароля пользователя: {%s}", err, name)
	}

	return psw, nil
}

// Получение токена по имени пользователя. Возвращается токен и ошибка.
//
// Параметры:
//
// name - имя пользователя
// db - указатель на БД
func ReadUserTokenByNameDB(name string, db *sql.DB) (token string, err error) {

	// Проверка принятых данных
	if name == "" {
		return "", errors.New("ошибка: при порлучении токена - нет имени пользователя")
	}
	if db == nil {
		return "", errors.New("ошибка: нет указател на БД")
	}

	q := fmt.Sprintf("SELECT token FROM %s.%s WHERE name = $1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	err = db.QueryRow(q, name).Scan(&token)
	if err != nil {
		return "", errors.New("ошибка: при порлучении токена, по имени пользователя")
	}

	return token, nil
}

// Генерация токена. Возвращается токен
//
// Параметры:
//
// name - имя пользователя
// pwd - пароль пользователя
func generateToken(name, pwd string) string {

	timestamp := time.Now().Unix()

	data := fmt.Sprintf("%s:%s:%d", name, pwd, timestamp)

	token := fmt.Sprintf("%x", sha256.Sum256([]byte(data)))

	return token
}

// Запись в БД токена пользователя. Возвращется ошибка.
//
// Парметры:
// name - имя пользователя
// token - токен
func saveTokenUserDB(name, token string, db *sql.DB) error {

	q := fmt.Sprintf("UPDATE %s.%s SET token = $1 WHERE name = $2",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"),
	)

	_, err := db.Exec(q, token, name)
	if err != nil {
		return fmt.Errorf("ошибка {%v} при обновлении пароля у пользователя: {%s}", err, name)
	}

	return nil
}

// Функция выполняет запрос с подсчётом количества строк по указанной дате. Возвращает ошибку.
//
// Параметры:
//
// data - набор данных
func readCntStrDataDB(data *DataDBCallT) error {

	// Проверка входных данных
	if data == nil {
		return errors.New("принят пустой указатель")
	}

	q := fmt.Sprintf(`
	SELECT COUNT(*)
	FROM %s.%s 
	WHERE date(timestamp) = $1
	;`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"))

	err := data.DB.QueryRow(q, data.StartDate).Scan(&data.CntStrDB)
	if err != nil {
		return fmt.Errorf("ошибка при запросе количества строк по дате {%d}: {%v}", data.CntStrDB, err)
	}

	return nil
}

// Функция выполняет чтение из БД архивных данных, по начальной дате. Возвращается ошибка.
//
// Параметры:
//
// db - указатель на БД
// data - стартовая дата выборки и результат выборки.
// limit - количество строк выборки.
// offset - смещение выборки.
func readPartDataDBReq(db *sql.DB, date string, limit, offset int) (rdData []DataElT, err error) {

	// Проверка аргументов
	if db == nil {
		return nil, errors.New("запрос данных -> нет указателя на БД")
	}
	_, err = time.Parse("2006-01-02", date)
	if err != nil {
		return nil, fmt.Errorf("запрос данных -> значение начальной даты: {%s}", date)
	}
	if limit < 1 {
		return nil, fmt.Errorf("запрос данных -> значение limit:{%d} меньше 1", limit)
	}
	if offset < 0 {
		return nil, fmt.Errorf("запрос данных -> значение offset:{%d} меньше 0", offset)
	}

	// Подготовка запроса
	q := fmt.Sprintf(`
	 SELECT name, value, qual, timestamp
     FROM %s.%s
     WHERE date(timestamp) = '%v'
	 ORDER By timestamp ASC
	 LIMIT %d OFFSET %d
	 ;              
	`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"), date, limit, offset)

	// Запрос
	rows, err := db.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Обработка ответа
	cnt := 0
	rdData = make([]DataElT, 0)

	for rows.Next() {
		var str DataElT

		err = rows.Scan(&str.Name, &str.Value, &str.Qual, &str.TimeStamp)
		if err != nil {
			return nil, err
		}
		rdData = append(rdData, str)
		cnt++
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return rdData, nil
}
