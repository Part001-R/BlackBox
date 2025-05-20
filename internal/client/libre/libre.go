package libre

import (
	"errors"
	"fmt"

	"github.com/xuri/excelize/v2"
)

// Создание xlsx файла. Возвращается имя файла и ошибка.
//
// Параметры:
//
// path - путь к файлу
// name - имя файла
// time - время создания файла
// typ - тип файла
func CreateXlsx(path, name, time, typ string) (nameFile string, err error) {

	file := excelize.NewFile()

	_, err = file.NewSheet("DataDB") // добавление вкладки
	if err != nil {
		return "", errors.New("ошибка при добавлении вкладки DataDB")
	}

	err = file.DeleteSheet("Sheet1")
	if err != nil {
		return "", errors.New("ошибка при удалении вкладки Sheet1")
	}

	f := path + name + time + typ

	err = file.SaveAs(f)
	if err != nil {
		return "", fmt.Errorf("ошибка сохранения файла {%s}", f)
	}

	return f, nil
}
