package serverAPI

import (
	loger "blackbox/internal/server/loger"
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =====================================================
// ====                Тесты HTTPS                  ====
// =====================================================

// Обработчик запроса на состояние сервера (Успешность)
func Test_HandlHttpsStatusSrv_Success(t *testing.T) {

	// Данные пользователя из БД
	userName := "user222"
	userToken := "b5570b12e802f1a4930921edc737065fce2a95d168ace66e749d4852d74410b0"

	// Фиксация времени запуска
	tn := time.Now()
	timeStart := tn.Format("02-01-2006 15:04:05")

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	// Подготовка сводных данных сервера
	infoMbRTU := make([]InfoModbusRTUT, 0)
	iMbRTU := InfoModbusRTUT{
		ConName: "Con2",
		Con:     "/dev/ttyUSB0",
		ConParams: struct {
			BaudRate int
			DataBits int
			Parity   string
			StopBits int
		}{
			BaudRate: 9600,
			DataBits: 8,
			Parity:   "N",
			StopBits: 1,
		},
	}
	infoMbRTU = append(infoMbRTU, iMbRTU)

	infoMbTCP := make([]InfoModbusTCPT, 0)
	iMbTCP := InfoModbusTCPT{
		ConName: "Con1",
		Con:     "192.168.122.1",
	}
	infoMbTCP = append(infoMbTCP, iMbTCP)

	infoFileSize := SizeFilesT{
		I: 10,
		W: 11,
		E: 12,
	}

	var serverInfo StatusServerT
	serverInfo.TimeStart = timeStart
	serverInfo.DB = db
	serverInfo.Lgr = lger
	serverInfo.MbRTU = infoMbRTU
	serverInfo.MbTCP = infoMbTCP
	serverInfo.SizeF = infoFileSize

	// Создание запроса
	infoTx := NameT{
		Name: userName,
	}
	bytesBody, err := json.Marshal(infoTx)
	require.NoErrorf(t, err, "ошибка при сериализации данных: {%s}", fmt.Sprintf("%v", err))

	reqBody := bytes.NewBuffer(bytesBody)

	req := httptest.NewRequest(http.MethodPost, "/status", reqBody)

	req.Header.Set("authorization", userToken)

	// Создание приёмника ответа от обработчика запроса
	res := httptest.NewRecorder()

	// Обработчик
	serverInfo.HandlHttpsStatusSrv(res, req)
	require.Equalf(t, http.StatusOK, res.Result().StatusCode, "ожидался код %d, а принят %d", http.StatusOK, res.Result().StatusCode)

	// Чтение тела ответа
	respBody, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "чтение тела ответа - принята ошибка {%s}", fmt.Sprintf("%v", err))

	var dataRx StatusT
	err = json.Unmarshal(respBody, &dataRx)
	require.NoErrorf(t, err, "ошибка при десериализации данных: {%s}", fmt.Sprintf("%v", err))

	// Сравнение статусных данных сервера что передано и что принято
	assert.Equalf(t, serverInfo.TimeStart, dataRx.TimeStart, "нет соответствия врмени - ожидалось {%s}, а принято {%s}", serverInfo.TimeStart, dataRx.TimeStart)
	assert.Equalf(t, serverInfo.SizeF.I, dataRx.SizeF.I, "нет соответствия размерности логера I - ожидалось {%s}, а принято {%s}", serverInfo.SizeF.I, dataRx.SizeF.I)
	assert.Equalf(t, serverInfo.SizeF.W, dataRx.SizeF.W, "нет соответствия размерности логера W - ожидалось {%s}, а принято {%s}", serverInfo.SizeF.W, dataRx.SizeF.W)
	assert.Equalf(t, serverInfo.SizeF.E, dataRx.SizeF.E, "нет соответствия размерности логера E - ожидалось {%s}, а принято {%s}", serverInfo.SizeF.E, dataRx.SizeF.E)
	require.Equalf(t, len(serverInfo.MbRTU), len(dataRx.MbRTU), "нет соответствия в размере массива данных MbRTU - ожидалось {%d}, а принято {%d}", len(serverInfo.MbRTU), len(dataRx.MbRTU))
	require.Equalf(t, len(serverInfo.MbTCP), len(dataRx.MbTCP), "нет соответствия в размере массива данных MbTTCP - ожидалось {%d}, а принято {%d}", len(serverInfo.MbTCP), len(dataRx.MbTCP))
	for i, v := range serverInfo.MbRTU {
		assert.Equalf(t, v.ConName, dataRx.MbRTU[i].ConName, "нет соответствия в названии конекта MbRTU по id[%d] - ожидалось {%s}, а принято {%s}", i, v.ConName, dataRx.MbRTU[i].ConName)
		assert.Equalf(t, v.Con, dataRx.MbRTU[i].Con, "нет соответствия в конекте MbRTU по id[%d] - ожидалось {%s}, а принято {%s}", i, v.Con, dataRx.MbRTU[i].Con)
		assert.Equalf(t, v.ConParams, dataRx.MbRTU[i].ConParams, "нет соответствия в параметрах коннекта id[%d] - ожидалось {%s}, а принято {%s}", i, v.ConParams, dataRx.MbRTU[i].ConParams)
	}
	for i, v := range serverInfo.MbTCP {
		assert.Equalf(t, v.ConName, dataRx.MbTCP[i].ConName, "нет соответствия в названии конекта MbTCP по id[%d] - ожидалось {%s}, а принято {%s}", i, v.ConName, dataRx.MbTCP[i].ConName)
		assert.Equalf(t, v.Con, dataRx.MbTCP[i].Con, "нет соответствия в названии конекта MbTCP по id[%d] - ожидалось {%s}, а принято {%s}", i, v.Con, dataRx.MbTCP[i].Con)
	}

	// Закрытие подключений к логеру и БД
	err = closeConn(db, &lger)
	if err != nil {
		log.Fatal(err)
	}
}

// =====================================================
// ====          Вспомогательные функции            ====
// =====================================================

// Подключение к логерам и БД
func openConnLogDB(lgr *loger.Log_Object) (dbPtr *sql.DB, err error) {

	// Чтение переменных окружения
	pathEnv, err := pathEnvFile()
	if err != nil {
		return nil, errors.New("ошибка при формировании пути к env файлу")
	}
	err = godotenv.Load(pathEnv)
	if err != nil {
		return nil, fmt.Errorf("ошибка {%v} при чтении переменных окружения", err)
	}

	// Логеры
	err = createOpenLog(lgr)
	if err != nil {
		return nil, err
	}

	// БД
	dbPtr, err = conDB()
	if err != nil {
		return nil, err
	}

	return dbPtr, nil
}

// Закрытие подключений
func closeConn(dbPtr *sql.DB, lgr *loger.Log_Object) error {

	if dbPtr == nil {
		return errors.New("закрытие подключений - нет указателя на БД")
	}
	if lgr == nil {
		return errors.New("закрытие подключений - нет указателя на логеры")
	}

	// закрытие подключения БД
	err := dbPtr.Close()
	if err != nil {
		lgr.E.Println("ошибка закрытия подключения к БД: ", err)
	}

	// закрытие логеров
	err = lgr.CloseAll()
	if err != nil {
		log.Fatal("ошибка при закрытии логеров:", err)
	}

	return nil
}

// Подключение к БД
func conDB() (dbPtr *sql.DB, err error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		os.Getenv("DB_HOST"),
		os.Getenv("DB_HOST_PORT"),
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_NAME"),
		os.Getenv("DB_SSLMODE"))

	// Подключение
	dbPtr, err = sql.Open("postgres", dsn) // Изменяем dbptr на dbPtr
	if err != nil {
		return nil, err
	}

	// Проверка подключения
	err = dbPtr.Ping()
	if err != nil {
		return nil, err
	}

	return dbPtr, nil
}

// Функция подключается в файлам логирования или создаёт их, в случае отсутствия. Возвращает ошибку
func createOpenLog(l *loger.Log_Object) error {

	dirLogFiles, err := pathLogFile()
	if err != nil {
		return errors.New("ошибка при построении путит к файлам логера")
	}

	// Создание директории для файлов лога
	//
	os.Mkdir(dirLogFiles, os.FileMode(0777))

	// Создание (подключение) файлов для логирования
	//
	ptrLogI, err := os.OpenFile(dirLogFiles+"log_info.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New("ошибка подключения к логеру информации")
	}

	ptrLogW, err := os.OpenFile(dirLogFiles+"log_warn.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New("ошибка подключения к логеру предупреждений")
	}

	ptrLogE, err := os.OpenFile(dirLogFiles+"log_error.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		return errors.New("ошибка подключения к логеру ошибок")
	}

	// Создание логеров
	//
	l.I = log.New(ptrLogI, "INFO:\t", log.Ldate|log.Ltime|log.Lshortfile)
	l.W = log.New(ptrLogW, "WARN:\t", log.Ldate|log.Ltime|log.Lshortfile)
	l.E = log.New(ptrLogE, "ERROR:\t", log.Ldate|log.Ltime|log.Lshortfile)

	// Закрытие файлов
	l.CloseAll = func() error {

		var e []error

		err := ptrLogI.Close()
		if err != nil {
			e = append(e, err)
		}

		err = ptrLogW.Close()
		if err != nil {
			e = append(e, err)
		}

		err = ptrLogE.Close()
		if err != nil {
			e = append(e, err)
		}

		if len(e) != 0 {
			return errors.New("ошибка при закрытии логеров")
		}

		return nil
	}

	// Возврат указателей

	return nil

}

// Создание пути к .env файлу
func pathEnvFile() (path string, err error) {

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("ошибка при получении пути рабочей директории: {%v}", err)
	}

	slPath := strings.Split(wd, "/")
	ind := 0
	found := false
	for i, v := range slPath {
		if v == "BlackBox" {
			i++
			ind = i
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("ошибка - в слайсе пути: {%v}, нет директории:{%s}", slPath, "BlackBox")
	}

	wdBase := slPath[:ind]
	pathWdBase := strings.Join(wdBase, "/")
	pathEnv := pathWdBase + "/configs/.env"

	return pathEnv, nil

}

// Создание пути к файлам логера
func pathLogFile() (path string, err error) {

	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("ошибка при получении пути рабочей директории: {%v}", err)
	}

	slPath := strings.Split(wd, "/")
	ind := 0
	found := false
	for i, v := range slPath {
		if v == "BlackBox" {
			i++
			ind = i
			found = true
			break
		}
	}

	if !found {
		return "", fmt.Errorf("ошибка - в слайсе пути: {%v}, нет директории:{%s}", slPath, "BlackBox")
	}

	wdBase := slPath[:ind]
	pathWdBase := strings.Join(wdBase, "/")
	pathLog := pathWdBase + "/LogServer/"

	return pathLog, nil

}
