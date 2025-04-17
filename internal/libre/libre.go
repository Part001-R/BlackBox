package libre

import (
	"fmt"
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

type (
	ConfXLSX_Object struct {
		Ptr *excelize.File
	}

	ConfXLSX_Intf interface {
		Open() error
		Read() error
	}
)

// Открытие файла конфигурации. Возвращает ошибку
func (e *ConfXLSX_Object) Open() error {

	var err error

	e.Ptr, err = excelize.OpenFile(os.Getenv("CONFIG_FILE_NAME"))
	if err != nil {
		return err
	}

	return nil
}

// Чтение файла
func (e *ConfXLSX_Object) Read() error {

	// Открытие файла
	file, err := excelize.OpenFile(os.Getenv("CONFIG_FILE_NAME"))
	if err != nil {
		return err
	}

	// Чтение содержимого вкладки
	rows, err := file.GetRows("Main")
	if err != nil {
		return err
	}

	// Вывод содержимого в терминал
	for _, row := range rows {
		for _, col := range row {
			fmt.Print(col, "\t")
		}
		fmt.Println()
	}

	return nil
}

// Демо пример по созданию xlsx файла
func CreateLibreFile_test() {
	file := excelize.NewFile()

	headers := []string{"ID", "Name", "Age"}
	for i, header := range headers {
		file.SetCellValue("Sheet1", fmt.Sprintf("%s%d", string(rune(65+i)), 1), header)
	}

	data := [][]interface{}{
		{1, "John", 30, 55},
		{2, "Alex", 20, 56},
		{3, "Bob", 40, 57},
	}

	for i, row := range data {
		dataRow := i + 2
		for j, col := range row {
			file.SetCellValue("Sheet1", fmt.Sprintf("%s%d", string(rune(65+j)), dataRow), col)
		}
	}

	if err := file.SaveAs("students.xlsx"); err != nil {
		log.Fatal(err)
	}
}
