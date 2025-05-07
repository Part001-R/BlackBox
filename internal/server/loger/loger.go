package log

import (
	"errors"
	"log"
	"os"
)

type (
	Log_Object struct {
		I        *log.Logger
		W        *log.Logger
		E        *log.Logger
		CloseAll func() error
		isRun    bool
	}

	Log_Intf interface {
		CreateOpenLog() error
	}
)

// Функция подключается в файлам логирования или создаёт их, в случае отсутствия.
func (l *Log_Object) CreateOpenLog() error {

	dirLogFiles := os.Getenv("LOG_PATH")

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
