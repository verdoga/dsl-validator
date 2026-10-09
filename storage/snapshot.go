package storage

import "os"

// Snapshot — закрытые сведения об исходном JSON для сохранения по тому же пути.
// Не является открытым дескриптором и не требует закрытия вызывающим кодом.
type Snapshot struct {
	// path — абсолютный путь открытого JSON, а не Document.FilePath исходного DSL.
	path string
	// original — собственные точные байты прочитанного JSON, включая неизвестные поля.
	original []byte
	// info — сведения об исходном обычном файле для проверки перед заменой.
	info os.FileInfo
}
