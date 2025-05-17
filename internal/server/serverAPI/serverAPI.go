package serverAPI

import (
	loger "blackbox/internal/server/loger"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"time"

	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

type (
	// Для передачи состояния сервера
	StatusServerCallT struct {
		TimeStart string
		MbRTU     []InfoModbusRTU
		MbTCP     []InfoModbusTCP
		SizeF     SizeFiles
		DB        *sql.DB
		Lgr       loger.Log_Object
	}

	// Для передачи состояния сервера
	StatusT struct {
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
	DataDBCall struct {
		StartDate string
		Data      []DataEl
		DB        *sql.DB
		Lgr       loger.Log_Object
	}

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

	// Для регистрации пользователя на https сервере
	LoginUser struct {
		DB  *sql.DB
		Lgr loger.Log_Object
	}
)

// Обработчик запроса на предоставление состояния Go рутин
func (el *StatusServerCallT) HandlStatusSrv(w http.ResponseWriter, r *http.Request) {

	w.Header().Set("Content-Type", "application/json")

	// Чтение заголовков звпроса
	token := r.Header.Get("authorization")
	if token == "" {
		el.Lgr.W.Println("https-status -> нет токена, в запроск")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Чтение параметров запроса
	queryParams := r.URL.Query()
	name := queryParams.Get("name")
	if name == "" {
		el.Lgr.W.Println("https-status -> нет имени пользователя, в запросе")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка, что принятое имя и его токен соответствуют
	tokenDB, err := readUserTokenByNameDB(name, el.DB)
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

	resp, err := json.Marshal(statusServer)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Передача данных
	el.Lgr.I.Printf("https-status -> пользователь {%s} запросил состояние сервера", name)
	w.WriteHeader(http.StatusOK)
	w.Write(resp)
}

// Обработчик запроса на экспорт архивных данных БД
func (el *DataDBCall) HandlExpDataDB(w http.ResponseWriter, r *http.Request) {

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
	tokenDB, err := readUserTokenByNameDB(name, el.DB)
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

	// Подготовка данных для ответа
	var data DataDB
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

// Обработчик регистрации пользователя на https сервере
func (el *LoginUser) HandleHttpsRegistration(w http.ResponseWriter, r *http.Request) {

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

	rxUsrName := slStr[0]
	rxUserPsw := slStr[1]

	// Чтение из БД хэша пароля пользователя
	dbPswHash, err := readPswUserDB(rxUsrName, el.DB)
	if err != nil {
		el.Lgr.W.Printf("https-registration -> попытка подключения пользователя {%s}, такого пользователя в БД нет\n", rxUsrName)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Проверка соответствия хэшей
	calcHash := fmt.Sprintf("%x", sha256.Sum256([]byte(rxUserPsw)))

	if dbPswHash != calcHash {
		el.Lgr.W.Printf("https-registration -> принят запрос пользователя {%s} с не верным паролем", rxUsrName)
		http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return
	}

	// Вычисление токена
	token := generateToken(rxUsrName, rxUserPsw)

	// Сохрание токена в БД
	err = saveTokenUserDB(rxUsrName, token, el.DB)
	if err != nil {
		el.Lgr.E.Printf("https-registration -> ошибка {%v} при сохранении в БД хэша пароля для пользователя {%s}\n", err, rxUsrName)
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}

	// Ответ
	el.Lgr.I.Printf("https-registration -> пользователь {%s} зарегистрировался на сервере\n", rxUsrName)

	w.Header().Set("Content-Type", "application-json")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(token))

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

// Получение имени пользователя по его токену. Возвращается имя и ошибка.
//
// Параметры:
//
// token - токен пользователя
// db - указатель на БД
func readUserNameByTokenDB(token string, db *sql.DB) (name string, err error) {

	q := fmt.Sprintf("SELECT name FROM %s.%s WHERE token = $1",
		os.Getenv("TABLE_SCHEMA"),
		os.Getenv("TABLE_USERS"))

	err = db.QueryRow(q, token).Scan(&name)
	if err != nil {
		return "", errors.New("ошибка: при порлучении имени пользователя по его токену")
	}

	return name, nil
}

// Получение токена по имени пользователя. Возвращается токен и ошибка.
//
// Параметры:
//
// token - токен пользователя
// db - указатель на БД
func readUserTokenByNameDB(name string, db *sql.DB) (token string, err error) {

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
