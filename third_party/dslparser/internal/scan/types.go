package scan

// Kind задаёт предварительный вид нормализованной строки.
type Kind int

// Виды строк-кандидатов.
const (
	Empty Kind = iota
	Version
	Heading
	Tag
	BlockOpen
	BlockClose
	InvalidBoundary
	Text
	OpaqueLine
)

// Mode задаёт контекст предварительной классификации.
type Mode int

// Режимы классификации содержимого.
const (
	Structural Mode = iota
	Opaque
)

// Candidate хранит результат предварительной классификации строки.
type Candidate struct {
	// Kind является видом строки.
	Kind Kind
	// Raw сохраняет нормализованную строку.
	Raw string
	// Name сохраняет исходное имя тега.
	Name string
	// CanonicalName хранит имя тега в нижнем регистре.
	CanonicalName string
	// Tail сохраняет полный сырой хвост после имени.
	Tail string
	// NameStart задаёт начальный Unicode-столбец имени.
	NameStart int
	// NameEnd задаёт исключённый конечный Unicode-столбец имени.
	NameEnd int
}

// Diagnostic хранит локальную ошибку сканирования.
type Diagnostic struct {
	// Code является стабильным машинным кодом.
	Code string
	// Message является кратким сообщением на русском языке.
	Message string
	// Start задаёт начальный Unicode-столбец.
	Start int
	// End задаёт исключённый конечный Unicode-столбец.
	End int
}

// HeadingResult хранит разобранные части заголовка.
type HeadingResult struct {
	Level                                    int
	Text                                     string
	LevelStart, LevelEnd, TextStart, TextEnd int
	Parsed                                   bool
	Diagnostics                              []Diagnostic
}

// Token хранит сырой и декодированный токен с полуоткрытыми Unicode-столбцами.
type Token struct {
	Raw, Value  string
	Start, End  int
	Parsed      bool
	Diagnostics []Diagnostic
}

// Tokens хранит полный сырой хвост и частичный результат токенизации.
type Tokens struct {
	Raw         string
	Values      []Token
	Parsed      bool
	Diagnostics []Diagnostic
}
