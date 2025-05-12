package libre

import (
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
		return "", err
	}

	f := path + name + "-" + time + typ

	err = file.SaveAs(f)
	if err != nil {
		return "", err
	}

	return f, nil
}
