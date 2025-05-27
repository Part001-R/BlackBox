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
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/lib/pq"

	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	// Для тестов необходима учётная запись в БД
	userName  = "user222"
	userPassw = "222"
	dateReqDB = "2025-05-17"
)

// =====================================================
// ====                Тесты HTTPS                  ====
// =====================================================

// Регистрация пользователя на сервере (Успешность)
func Test_HandlHttpsRegistration_Success(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Тело запроса (имя + пароль)
	bodyReq := bytes.NewBuffer([]byte(fmt.Sprintf("%s %s", userName, userPassw)))

	// Создание запроса и приёмника ответа
	req := httptest.NewRequest(http.MethodPost, "/registration", bodyReq)
	res := httptest.NewRecorder()

	// Инициализация и запрос
	var user LoginUserT
	user.DB = db
	user.Lgr = lger
	user.HandlHttpsRegistration(res, req)

	// Чтение тела ответа
	byteRx, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "ошибка чтения тела ответа: {%v}", err)

	var dataRx TokenT
	err = json.Unmarshal(byteRx, &dataRx)
	require.NoErrorf(t, err, "ошибка десериализации тела ответа: {%v}", err)

	// Чтение токена из БД
	tokenDB, err := ReadUserTokenByNameDB(userName, db)
	require.NoErrorf(t, err, "ошибка при чтении токена из БД по имени пользователя: {%v}", err)

	// Проверка соответствия токенов
	assert.Equalf(t, tokenDB, dataRx.Token, "токены не равны. в БД {%s}, а принят {%s}", tokenDB, dataRx.Token)
}

// Регистрация пользователя на сервере (Ошибки)
func Test_HandlHttpsRegistration_Error(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	dataTest := []struct {
		testName   string
		methodHttp string
		user       string
		pwd        string
		wantErr    int
	}{
		{
			testName:   "метод запроса не POST",
			methodHttp: "GET",
			user:       userName,
			pwd:        userPassw,
			wantErr:    400,
		},
		{
			testName:   "пустое поле user",
			methodHttp: "POST",
			user:       "",
			pwd:        userPassw,
			wantErr:    400,
		},
		{
			testName:   "пустое поле pwd",
			methodHttp: "POST",
			user:       userName,
			pwd:        "",
			wantErr:    400,
		},
	}

	for _, tt := range dataTest {
		t.Run(tt.testName, func(t *testing.T) {

			// Тело запроса (имя + пароль)
			bodyReq := bytes.NewBuffer([]byte(fmt.Sprintf("%s %s", tt.user, tt.pwd)))

			req := httptest.NewRequest(tt.methodHttp, "/registration", bodyReq)
			res := httptest.NewRecorder()

			// Инициализация и запрос
			var user LoginUserT
			user.DB = db
			user.Lgr = lger
			user.HandlHttpsRegistration(res, req)
			assert.Equalf(t, tt.wantErr, res.Result().StatusCode, "ожидаля код: {%d}, а принят: {%d}", tt.wantErr, res.Result().StatusCode)
		})
	}
}

// Обработчик запроса на состояние сервера (Успешность)
func Test_HandlHttpsStatusSrv_Success(t *testing.T) {

	// Фиксация времени запуска
	tn := time.Now()
	timeStart := tn.Format("2006-01-02 15:04:05")

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

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

	// Чтение токена из БД
	userToken, err := ReadUserTokenByNameDB(userName, db)
	require.NoErrorf(t, err, "ошибка при чтении токена из БД: {%v}", err)

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
}

// Обработчик запроса на состояние сервера (Ошибки)
func Test_HandlHttpsStatusSrv_Error(t *testing.T) {

	// Фиксация времени запуска
	tn := time.Now()
	timeStart := tn.Format("2006-01-02 15:04:05")

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Чтение токена из БД
	userToken, err := ReadUserTokenByNameDB(userName, db)
	require.NoErrorf(t, err, "ошибка при чтении токена из БД: {%v}", err)

	// Набор данных для тестов
	dataTest := []struct {
		testName   string
		httpMethod string
		serverInfo StatusServerT
		wantErr    int
	}{
		{
			testName:   "неподдерживаемый метод запроса",
			httpMethod: "GET",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 400,
		},
		{
			testName:   "нет даты запуска",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: "",
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет соответствия в формате времени запуска. Формат YYYY-MM-DD HH-MM-SS",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: "2006-01-02",
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU:     []InfoModbusRTUT{},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных в названии коннекта RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных в подключения RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по скорости, в параметрах коннекта RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 0,
							DataBits: 8,
							Parity:   "N",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по битам данных, в параметрах коннекта RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 9600,
							DataBits: 0,
							Parity:   "N",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по чётности, в параметрах коннекта RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 9600,
							DataBits: 8,
							Parity:   "",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по стоп битам, в параметрах коннекта RTU",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
							StopBits: 0,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по TCP",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по имени коннекта TCP",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет данных по коннекту TCP",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "отрицательный размер файла логера I",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: -1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "отрицательный размер файла логера W",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: -2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "отрицательный размер файла логера E",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: -3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет указателя на БД",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  nil,
				Lgr: lger,
			},
			wantErr: 500,
		},
		{
			testName:   "нет указателя на логер",
			httpMethod: "POST",
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: loger.Log_Object{},
			},
			wantErr: 500,
		},
	}

	for _, tt := range dataTest {
		t.Run(tt.testName, func(t *testing.T) {
			// Создание запроса
			infoTx := NameT{
				Name: userName,
			}
			bytesBody, err := json.Marshal(infoTx)
			require.NoErrorf(t, err, "ошибка при сериализации данных: {%s}", fmt.Sprintf("%v", err))

			reqBody := bytes.NewBuffer(bytesBody)

			req := httptest.NewRequest(tt.httpMethod, "/status", reqBody)

			req.Header.Set("authorization", userToken)

			res := httptest.NewRecorder()

			// Запрос
			tt.serverInfo.HandlHttpsStatusSrv(res, req)
			assert.Equalf(t, tt.wantErr, res.Code, "тест {%s} - ожидался код {%d}, а принят {%d}", tt.testName, tt.wantErr, res.Code)
		})
	}
}

// Обработчик запроса на определение количества строк в БД по дате (Успешность)
func Test_HandlHttpsCntStrByDate_Success(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Тело запроса
	infoTx := DateNameT{
		Date: dateReqDB,
		Name: userName,
	}

	bytesBody, err := json.Marshal(infoTx)
	require.NoErrorf(t, err, "ошибка сериализации данных: {%v}", err)

	reqBody := bytes.NewBuffer(bytesBody)

	req := httptest.NewRequest(http.MethodPost, "/cntstr", reqBody)

	// Чтение токена из БД
	userToken, err := ReadUserTokenByNameDB(userName, db)
	require.NoErrorf(t, err, "ошибка при чтении токена из БД: {%v}", err)

	req.Header.Set("authorization", userToken)

	// Запрос
	res := httptest.NewRecorder()

	cntStrDB, err := сntStrDataDB(dateReqDB, db)
	require.NoErrorf(t, err, "ошибка: {%v} при чтении количества строк из БД по дате: {%s}", err, dateReqDB)

	var cntStr CntStrByDateT
	cntStr.DB = db
	cntStr.Lgr = lger
	cntStr.HandlHttpsCntStrByDate(res, req)
	require.Equalf(t, 200, res.Result().StatusCode, "ожидался код 200, а принят {%d}", res.Result().StatusCode)

	// Ответ
	dataResp, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа: {%v}", err)

	rxJson := CntStrT{}

	err = json.Unmarshal(dataResp, &rxJson)
	require.NoErrorf(t, err, "ошибка при десериализации ответа: {%v}", err)

	cntS, err := strconv.Atoi(rxJson.CntStr)
	require.NoErrorf(t, err, "ошибка:{%v} преобразования строки:{%s} в число:{%d}", err, rxJson.CntStr, cntS)

	// Проверка результата
	assert.Equalf(t, cntStrDB, cntS, "нет соответствия в количестве. ожидалось:{%d}, а принято:{%d}", cntStrDB, cntS)
}

// Обработчик запроса на определение количества строк в БД по дате (Ошибки)
func Test_HandlHttpsCntStrByDate_Error(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Данные для теста
	var dataTest = []struct {
		testName   string
		httpMethod string
		user       string
		date       string
		useToken   string
		wantErr    int
	}{
		{
			testName:   "Метод запроса не POST",
			httpMethod: http.MethodGet,
			user:       userName,
			date:       dateReqDB,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Нет имени пользователя",
			httpMethod: http.MethodPost,
			user:       "",
			date:       dateReqDB,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Фейковое имя пользователя",
			httpMethod: http.MethodPost,
			user:       "Такого_имени_нет",
			date:       dateReqDB,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Нет даты",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "",
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Дата не в формате YYYY-MM-DD",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "02-03-2025",
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Нет токена",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			useToken:   "false",
			wantErr:    400,
		},
	}

	// Тесты
	for _, tt := range dataTest {
		t.Run(tt.testName, func(t *testing.T) {

			// Тело запроса
			infoTx := DateNameT{
				Date: tt.date,
				Name: tt.user,
			}

			bytesBody, err := json.Marshal(infoTx)
			require.NoErrorf(t, err, "ошибка сериализации данных: {%v}", err)

			reqBody := bytes.NewBuffer(bytesBody)

			req := httptest.NewRequest(tt.httpMethod, "/cntstr", reqBody)

			// Чтение токена из БД
			userToken, err := ReadUserTokenByNameDB(userName, db)
			require.NoErrorf(t, err, "ошибка чтения токена из БД: {%v}", err)

			if tt.useToken == "false" {
				userToken = ""
			}
			req.Header.Set("authorization", userToken)

			// Запрос
			res := httptest.NewRecorder()

			var cntStr CntStrByDateT
			cntStr.DB = db
			cntStr.Lgr = lger
			cntStr.HandlHttpsCntStrByDate(res, req)
			assert.Equalf(t, tt.wantErr, res.Result().StatusCode, "ожидался код:{%d}, а принят:{%d}", tt.wantErr, res.Result().StatusCode)
		})
	}
}

// Обработчик запроса строк из БД (Успешность)
func Test_HandlHttpsPartDataDB_Success(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Тело запроса
	infoTx := DateNameT{
		Date: dateReqDB,
		Name: userName,
	}

	bytesBody, err := json.Marshal(infoTx)
	require.NoErrorf(t, err, "ошибка сериализации данных тела запроса: %s", fmt.Sprintf("%v", err))

	reqBody := bytes.NewBuffer(bytesBody)

	// URL и параметры запроса
	u := "/partdatadb"
	parseU, err := url.Parse(u)
	require.NoErrorf(t, err, "ошибка парсинга URL: %s", fmt.Sprintf("%v", err))

	strLimit, err := сntStrDataDB(dateReqDB, db)
	require.NoErrorf(t, err, "ошибка при чтении количества строк по дате:{%v}", err)

	var numbReg = 10
	var strOffSet = 0
	if strLimit > 100 {
		strLimit = 100
	}

	qP := url.Values{}
	qP.Set("numbReg", fmt.Sprintf("%d", numbReg))
	qP.Set("strLimit", fmt.Sprintf("%d", strLimit))
	qP.Set("strOffSet", fmt.Sprintf("%d", strOffSet))
	parseU.RawQuery = qP.Encode()

	// Создание запроса и приёмника ответа
	req := httptest.NewRequest(http.MethodPost, parseU.String(), reqBody)
	res := httptest.NewRecorder()

	// Чтение токена из БД
	userToken, err := ReadUserTokenByNameDB(userName, db)
	require.NoErrorf(t, err, "ошибка при чтении токена из БД: {%v}", err)

	req.Header.Set("authorization", userToken)

	// Запрос данных БД
	var partData PartDataT
	partData.DB = db
	partData.Lgr = lger
	partData.HandlHttpsPartDataDB(res, req)

	require.Equalf(t, 200, res.Result().StatusCode, "нет соответствия статуса запроса. Ожидался:{%d}, а принят:{%d}", 200, res.Result().StatusCode)

	// Обработка ответа
	var dataRx PartDataDBT

	bodyRes, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа: {%v}", err)

	err = json.Unmarshal(bodyRes, &dataRx)
	require.NoErrorf(t, err, "ошибка десериализации ответа: {%v}", err)

	// Проверка результата
	assert.Equalf(t, numbReg, dataRx.NumbReq, "нет соответствия в номере запроса. Ожидался номер:{%d}, а принят:{%d}", numbReg, dataRx.NumbReq)
	assert.Equalf(t, strLimit, len(dataRx.Data), "нет соответствия в размере массива. Ожидался:{%d}, а принят:{%d}", strLimit, len(dataRx.Data))
}

// Обработчик запроса строк из БД (Ошибки)
func Test_HandlHttpsPartDataDB_Error(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Табличные данные для тестов
	var testData = []struct {
		testName   string
		httpMethod string
		user       string
		date       string
		numbReg    int
		strOffSet  int
		strLimit   int
		useToken   string
		wantErr    int
	}{
		{
			testName:   "Метод запроса не POST",
			httpMethod: http.MethodGet,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Нет имени пользователя",
			httpMethod: http.MethodPost,
			user:       "",
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Фейковое имя пользователя",
			httpMethod: http.MethodPost,
			user:       "Нет_такого_пользователя",
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Нет даты",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "",
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Дата не в формате YYYY-MM-DD",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "01-02-2025",
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Отрицательное число в numbReg",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    -1,
			strOffSet:  0,
			strLimit:   0,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Отрицательное число в strOffSet",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  -1,
			strLimit:   0,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Отрицательное число в strLimit",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   -1,
			useToken:   "true",
			wantErr:    400,
		},
		{
			testName:   "Запрос без токена",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			useToken:   "false",
			wantErr:    400,
		},
	}

	// тесты
	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Тело запроса
			infoTx := DateNameT{
				Date: tt.date,
				Name: tt.user,
			}

			bytesBody, err := json.Marshal(infoTx)
			require.NoErrorf(t, err, "ошибка сериализации данных тела запроса: %s", fmt.Sprintf("%v", err))

			reqBody := bytes.NewBuffer(bytesBody)

			// URL и параметры запроса
			u := "/partdatadb"
			parseU, err := url.Parse(u)
			require.NoErrorf(t, err, "ошибка парсинга URL: %s", fmt.Sprintf("%v", err))

			strLimit, err := сntStrDataDB(dateReqDB, db)
			require.NoErrorf(t, err, "ошибка при чтении количества строк по дате:{%v}", err)

			if strLimit < tt.strLimit {
				tt.strLimit = strLimit
			}

			qP := url.Values{}
			qP.Set("numbReg", fmt.Sprintf("%d", tt.numbReg))
			qP.Set("strLimit", fmt.Sprintf("%d", tt.strLimit))
			qP.Set("strOffSet", fmt.Sprintf("%d", tt.strOffSet))
			parseU.RawQuery = qP.Encode()

			// Создание запроса и приёмника ответа
			req := httptest.NewRequest(tt.httpMethod, parseU.String(), reqBody)
			res := httptest.NewRecorder()

			// Добавление токена
			userToken, err := ReadUserTokenByNameDB(tt.user, db)

			if tt.useToken == "true" && err == nil {
				req.Header.Set("authorization", userToken)
			}

			// Запрос данных БД
			var partData PartDataT
			partData.DB = db
			partData.Lgr = lger
			partData.HandlHttpsPartDataDB(res, req)

			assert.Equalf(t, 400, res.Result().StatusCode, "нет соответствия статуса запроса. Ожидался:{%d}, а принят:{%d}", 400, res.Result().StatusCode)
		})
	}
}

// =====================================================
// ====                Тесты HTTP                   ====
// =====================================================

// Обработчик запроса на предоставление данных сервера (Успешность)
func Test_HandlHttpStatusSrv_Success(t *testing.T) {

	// Фиксация времени запуска
	tn := time.Now()
	timeStart := tn.Format("2006-01-02 15:04:05")

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

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

	// Создание запроса и приёмника ответа
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	res := httptest.NewRecorder()

	// Обработчик
	serverInfo.HandlHttpStatusSrv(res, req)
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
}

// Обработчик запроса на состояние сервера (Ошибки)
func Test_HandlHttpStatusSrv_Error(t *testing.T) {

	// Фиксация времени запуска
	tn := time.Now()
	timeStart := tn.Format("2006-01-02 15:04:05")

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Набор данных для тестов
	dataTest := []struct {
		testName   string
		httpMethod string
		serverInfo StatusServerT
		wantCode   int
	}{
		{
			testName:   "Метод запроса не GET",
			httpMethod: http.MethodPost,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusBadRequest,
		},
		{
			testName:   "нет даты запуска",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: "",
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет соответствия в формате времени запуска. Формат YYYY-MM-DD HH-MM-SS",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: "2006-01-02",
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU:     []InfoModbusRTUT{},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных в названии коннекта RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных в подключения RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по скорости, в параметрах коннекта RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 0,
							DataBits: 8,
							Parity:   "N",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по битам данных, в параметрах коннекта RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 9600,
							DataBits: 0,
							Parity:   "N",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по чётности, в параметрах коннекта RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
						Con:     "/dev/ttyUSB0",
						ConParams: struct {
							BaudRate int
							DataBits int
							Parity   string
							StopBits int
						}{
							BaudRate: 9600,
							DataBits: 8,
							Parity:   "",
							StopBits: 1,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по стоп битам, в параметрах коннекта RTU",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
							StopBits: 0,
						},
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по TCP",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по имени коннекта TCP",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет данных по коннекту TCP",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
						ConName: "Con1",
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "отрицательный размер файла логера I",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: -1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "отрицательный размер файла логера W",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: -2,
					E: 3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "отрицательный размер файла логера E",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: -3,
				},
				DB:  db,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет указателя на БД",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  nil,
				Lgr: lger,
			},
			wantCode: http.StatusInternalServerError,
		},
		{
			testName:   "нет указателя на логер",
			httpMethod: http.MethodGet,
			serverInfo: StatusServerT{
				TimeStart: timeStart,
				MbRTU: []InfoModbusRTUT{
					{
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
					},
				},
				MbTCP: []InfoModbusTCPT{
					{
						ConName: "Con1",
						Con:     "192.168.122.1",
					},
				},
				SizeF: SizeFilesT{
					I: 1,
					W: 2,
					E: 3,
				},
				DB:  db,
				Lgr: loger.Log_Object{},
			},
			wantCode: http.StatusInternalServerError,
		},
	}

	// Тесты
	for _, tt := range dataTest {
		t.Run(tt.testName, func(t *testing.T) {
			// Создание запроса и приёмника ответа
			req := httptest.NewRequest(tt.httpMethod, "/status", nil)
			res := httptest.NewRecorder()

			// Запрос
			tt.serverInfo.HandlHttpStatusSrv(res, req)
			assert.Equalf(t, tt.wantCode, res.Code, "тест {%s} - ожидался код {%d}, а принят {%d}", tt.testName, tt.wantCode, res.Code)
		})
	}
}

// Обработчик запроса на определение количества строк в БД по дате (Успешность)
func Test_HandlHttpCntStrByDate_Success(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Создание запроса и приёмника ответа
	uP, err := url.Parse("/cntstr")
	require.NoErrorf(t, err, "ошибка парсинга URL:{%v}", err)

	qP := url.Values{}
	qP.Set("date", dateReqDB)

	uP.RawQuery = qP.Encode()

	req := httptest.NewRequest(http.MethodGet, uP.String(), nil)
	res := httptest.NewRecorder()

	// Предварительное чтение количества стор БД
	cntStrDB, err := сntStrDataDB(dateReqDB, db)
	require.NoErrorf(t, err, "ошибка: {%v} при чтении количества строк из БД по дате: {%s}", err, dateReqDB)

	// Запрос
	var cntStr CntStrByDateT
	cntStr.DB = db
	cntStr.Lgr = lger
	cntStr.HandlHttpCntStrByDate(res, req)
	require.Equalf(t, 200, res.Result().StatusCode, "ожидался код 200, а принят {%d}", res.Result().StatusCode)

	// Ответ
	dataResp, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа: {%v}", err)

	rxJson := CntStrT{}

	err = json.Unmarshal(dataResp, &rxJson)
	require.NoErrorf(t, err, "ошибка при десериализации ответа: {%v}", err)

	cntS, err := strconv.Atoi(rxJson.CntStr)
	require.NoErrorf(t, err, "ошибка:{%v} преобразования строки:{%s} в число:{%d}", err, rxJson.CntStr, cntS)

	// Проверка результата
	assert.Equalf(t, cntStrDB, cntS, "нет соответствия в количестве. ожидалось:{%d}, а принято:{%d}", cntStrDB, cntS)
}

// Обработчик запроса на определение количества строк в БД по дате (Ошибки)
func Test_HandlHttpCntStrByDate_Error(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Данные для теста
	var dataTest = []struct {
		testName   string
		httpMethod string
		user       string
		date       string
		wantCode   int
	}{
		{
			testName:   "Метод запроса не GET",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Нет даты",
			httpMethod: http.MethodGet,
			user:       userName,
			date:       "",
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Дата не в формате YYYY-MM-DD",
			httpMethod: http.MethodGet,
			user:       userName,
			date:       "02-03-2025",
			wantCode:   http.StatusBadRequest,
		},
	}

	// Тесты
	for _, tt := range dataTest {
		t.Run(tt.testName, func(t *testing.T) {

			// Создание запроса и приёмника ответа
			uP, err := url.Parse("/cntstr")
			require.NoErrorf(t, err, "ошибка парсинга URL:{%v}", err)

			qP := url.Values{}
			qP.Set("date", tt.date)

			uP.RawQuery = qP.Encode()

			req := httptest.NewRequest(tt.httpMethod, uP.String(), nil)
			res := httptest.NewRecorder()

			// Запрос
			var cntStr CntStrByDateT
			cntStr.DB = db
			cntStr.Lgr = lger
			cntStr.HandlHttpCntStrByDate(res, req)
			assert.Equalf(t, tt.wantCode, res.Result().StatusCode, "ожидался код:{%d}, а принят:{%d}", tt.wantCode, res.Result().StatusCode)
		})
	}
}

// Обработчик запроса строк из БД (Успешность)
func Test_HandlHttpPartDataDB_Success(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// URL и параметры запроса
	uP, err := url.Parse("/partdatadb")
	require.NoErrorf(t, err, "ошибка парсинга URL: %s", fmt.Sprintf("%v", err))

	strLimit, err := сntStrDataDB(dateReqDB, db)
	require.NoErrorf(t, err, "ошибка при чтении количества строк по дате:{%v}", err)

	var numbReg = 10
	var strOffSet = 0
	if strLimit > 100 {
		strLimit = 100
	}

	qP := url.Values{}
	qP.Set("numbReg", fmt.Sprintf("%d", numbReg))
	qP.Set("strLimit", fmt.Sprintf("%d", strLimit))
	qP.Set("strOffSet", fmt.Sprintf("%d", strOffSet))
	qP.Set("dateDB", dateReqDB)
	uP.RawQuery = qP.Encode()

	// Создание запроса и приёмника ответа
	req := httptest.NewRequest(http.MethodGet, uP.String(), nil)
	res := httptest.NewRecorder()

	// Запрос данных БД
	var partData PartDataT
	partData.DB = db
	partData.Lgr = lger
	partData.HandlHttpPartDataDB(res, req)

	require.Equalf(t, http.StatusOK, res.Result().StatusCode, "нет соответствия статуса запроса. Ожидался:{%d}, а принят:{%d}", 200, res.Result().StatusCode)

	// Обработка ответа
	var dataRx PartDataDBT

	bodyRes, err := io.ReadAll(res.Body)
	require.NoErrorf(t, err, "ошибка при чтении тела ответа: {%v}", err)

	err = json.Unmarshal(bodyRes, &dataRx)
	require.NoErrorf(t, err, "ошибка десериализации ответа: {%v}", err)

	// Проверка результата
	assert.Equalf(t, numbReg, dataRx.NumbReq, "нет соответствия в номере запроса. Ожидался номер:{%d}, а принят:{%d}", numbReg, dataRx.NumbReq)
	assert.Equalf(t, strLimit, len(dataRx.Data), "нет соответствия в размере массива. Ожидался:{%d}, а принят:{%d}", strLimit, len(dataRx.Data))
}

// Обработчик запроса строк из БД (Ошибки)
func Test_HandlHttpPartDataDB_Error(t *testing.T) {

	// Подключение к логеру и БД
	var lger loger.Log_Object

	db, err := openConnLogDB(&lger)
	require.NoErrorf(t, err, "подключение к БД и Логеру - ожидалось отсутствие ошибки, а принято: %s", fmt.Sprintf("%v", err))

	defer func() {
		err = closeConn(db, &lger)
		if err != nil {
			log.Fatal(err)
		}
	}()

	// Табличные данные для тестов
	var testData = []struct {
		testName   string
		httpMethod string
		user       string
		date       string
		numbReg    int
		strOffSet  int
		strLimit   int
		wantCode   int
	}{
		{
			testName:   "Метод запроса не GET",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Нет даты",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "",
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Дата не в формате YYYY-MM-DD",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       "01-02-2025",
			numbReg:    0,
			strOffSet:  0,
			strLimit:   10,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Отрицательное число в numbReg",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    -1,
			strOffSet:  0,
			strLimit:   0,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Отрицательное число в strOffSet",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  -1,
			strLimit:   0,
			wantCode:   http.StatusBadRequest,
		},
		{
			testName:   "Отрицательное число в strLimit",
			httpMethod: http.MethodPost,
			user:       userName,
			date:       dateReqDB,
			numbReg:    0,
			strOffSet:  0,
			strLimit:   -1,
			wantCode:   http.StatusBadRequest,
		},
	}

	// тесты
	for _, tt := range testData {
		t.Run(tt.testName, func(t *testing.T) {

			// Тело запроса
			infoTx := DateNameT{
				Date: tt.date,
				Name: tt.user,
			}

			bytesBody, err := json.Marshal(infoTx)
			require.NoErrorf(t, err, "ошибка сериализации данных тела запроса: %s", fmt.Sprintf("%v", err))

			reqBody := bytes.NewBuffer(bytesBody)

			// URL и параметры запроса
			u := "/partdatadb"
			parseU, err := url.Parse(u)
			require.NoErrorf(t, err, "ошибка парсинга URL: %s", fmt.Sprintf("%v", err))

			strLimit, err := сntStrDataDB(dateReqDB, db)
			require.NoErrorf(t, err, "ошибка при чтении количества строк по дате:{%v}", err)

			if strLimit < tt.strLimit {
				tt.strLimit = strLimit
			}

			qP := url.Values{}
			qP.Set("numbReg", fmt.Sprintf("%d", tt.numbReg))
			qP.Set("strLimit", fmt.Sprintf("%d", tt.strLimit))
			qP.Set("strOffSet", fmt.Sprintf("%d", tt.strOffSet))
			qP.Set("dateDB", tt.date)
			parseU.RawQuery = qP.Encode()

			// Создание запроса и приёмника ответа
			req := httptest.NewRequest(tt.httpMethod, parseU.String(), reqBody)
			res := httptest.NewRecorder()

			// Запрос данных БД
			var partData PartDataT
			partData.DB = db
			partData.Lgr = lger
			partData.HandlHttpPartDataDB(res, req)

			assert.Equalf(t, tt.wantCode, res.Result().StatusCode, "нет соответствия статуса запроса. Ожидался:{%d}, а принят:{%d}", 400, res.Result().StatusCode)
		})
	}
}

// =====================================================
// ====          Вспомогательные функции            ====
// =====================================================

// Подключение к логерам и БД. Возвращается указатель на БД и ошибка.
//
// Параметры:
//
// lgr - указатель на логеры
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
//
// Параметры:
//
// dbPtr - указатель на БД
// lgr - указатель на логеры
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

// Подключение к БД. Возвращается указатель на БД и ошибка.
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
//
// Параметры:
//
// l - указатель на логеры
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

// Создание пути к .env файлу. Возвращается путь и ошибка
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

// Создание пути к файлам логера. Возвращается путь и ошибка
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

// Функция выполняет запрос с подсчётом количества строк по указанной дате. Возвращает ошибку.
//
// Параметры:
//
// data - набор данных
// db - указатель на БД
func сntStrDataDB(date string, db *sql.DB) (cnt int, err error) {

	// Проверка входных данных
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return 0, fmt.Errorf("принятая дата не в формате YYYY-MM-DD {%v}", err)
	}
	if db == nil {
		return 0, errors.New("нет указателя на БД")
	}

	q := fmt.Sprintf(`
	SELECT COUNT(*)
	FROM %s.%s 
	WHERE date(timestamp) = $1
	;`, os.Getenv("TABLE_SCHEMA"), os.Getenv("TABLE_DATA"))

	err = db.QueryRow(q, date).Scan(&cnt)
	if err != nil {
		return 0, fmt.Errorf("ошибка при запросе количества строк по дате {%s}: {%v}", date, err)
	}

	return cnt, nil
}
