package log

import (
	"errors"
	"fmt"
	"log"
	"os"
)

type (
	Log_Object struct {
		I        *log.Logger
		W        *log.Logger
		E        *log.Logger
		CloseAll func() error
		//isRun    bool
	}
)

// Функция подключается в файлам логирования или создаёт их, в случае отсутствия. Возвращает ошибку
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

// Получение размеров файлов. Возвращает размеры файлов и ошибку.
func (l *Log_Object) SizeFiles() (sizeIMB, sizeWMB, sizeEMB int64, err error) {

	dirLogFiles := os.Getenv("LOG_PATH")

	listFiles := []string{"log_info.log", "log_warn.log", "log_error.log"}

	for i, v := range listFiles {

		// Открываем файл
		var file *os.File
		file, err = os.OpenFile(dirLogFiles+v, os.O_RDONLY, 0400)
		if err != nil {
			return 0, 0, 0, fmt.Errorf("ошибка {%v} при открытии файла", err)
		}
		defer file.Close()

		// Получаем информацию о файле
		var fileInfo os.FileInfo
		fileInfo, err = file.Stat()
		if err != nil {
			return 0, 0, 0, fmt.Errorf("ошибка {%v} при получении информации о файле", err)
		}

		switch i {
		case 0:
			sizeIMB = fileInfo.Size() / 2048
		case 1:
			sizeWMB = fileInfo.Size() / 2048
		case 2:
			sizeEMB = fileInfo.Size() / 2048
		}
	}

	return sizeIMB, sizeWMB, sizeEMB, nil
}
