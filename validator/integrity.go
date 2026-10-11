package validator

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
)

// CheckModel проверяет целостность модели без изменения данных и без
// запуска диагностик: обязательные массивы, допустимые значения, ID, ссылки,
// координаты, порядок, связанные места и точность вычисленных hasErrors.
// Применяется Prepare до добавления Processing и Run к итоговой копии модели.
// Существующие DSL-ошибки сами по себе не запрещают валидацию.
// Нарушенные инварианты не исправляются молча и не выдаются за DSL-диагностику.
func CheckModel(result *model.Result) error {
	if result == nil {
		return fmt.Errorf("модель отсутствует")
	}
	if err := checkDocument(result); err != nil {
		return err
	}
	if err := checkProcessing(result.Processing); err != nil {
		return err
	}
	if err := checkLineSequence(result); err != nil {
		return err
	}
	for _, line := range result.Lines {
		if err := checkElementPositions(line); err != nil {
			return fmt.Errorf("строка %d: %w", line.Number, err)
		}
	}
	for _, check := range []func(*model.Result) error{
		checkDiagnosticSources, checkDiagnosticRanges, checkParentLinks,
		checkElementLinks, checkDiagnosticOrder, checkErrorFlags,
	} {
		if err := check(result); err != nil {
			return err
		}
	}
	return nil
}

// checkDocument проверяет Format, значения и взаимную согласованность полей
// Document, включая null, кодировку, неотрицательные размеры и форму SHA256.
// Не открывает исходный DSL и не проверяет существование Document.FilePath.
func checkDocument(result *model.Result) error {
	if result.Format != model.FormatVersion1 {
		return fmt.Errorf("неподдерживаемый formatVersion %q", result.Format)
	}
	d := result.Document
	for _, text := range []*string{d.DSLVersion, d.FileName, d.FilePath, d.Encoding, d.SHA256,
		d.Metadata.DocumentID, d.Metadata.Title, d.Metadata.Subtitle, d.Metadata.Section, d.Metadata.Order} {
		if text != nil && !utf8.ValidString(*text) {
			return fmt.Errorf("document содержит строку с неверным UTF-8")
		}
	}
	for _, path := range d.Metadata.ResourceDirs {
		if !utf8.ValidString(path) {
			return fmt.Errorf("document.metadata.resourceDirs содержит неверный UTF-8")
		}
	}
	if d.FileName != nil && (*d.FileName == "" || *d.FileName == "." || *d.FileName == ".." || filepath.Base(*d.FileName) != *d.FileName || strings.ContainsAny(*d.FileName, "/\x00")) {
		return fmt.Errorf("document.fileName не является именем файла")
	}
	if d.FilePath != nil {
		if !filepath.IsAbs(*d.FilePath) || filepath.Clean(*d.FilePath) != *d.FilePath || strings.ContainsRune(*d.FilePath, 0) {
			return fmt.Errorf("document.filePath должен быть абсолютным очищенным путём")
		}
		if d.FileName == nil || *d.FileName != filepath.Base(*d.FilePath) {
			return fmt.Errorf("document.fileName не согласован с filePath")
		}
	}
	if d.Encoding != nil && *d.Encoding != "UTF-8" {
		return fmt.Errorf("неподдерживаемая document.encoding %q", *d.Encoding)
	}
	if d.LineCount != nil && *d.LineCount < 0 || d.ByteLength != nil && *d.ByteLength < 0 {
		return fmt.Errorf("размеры документа не могут быть отрицательными")
	}
	if d.HasBOM != nil && *d.HasBOM && d.ByteLength != nil && *d.ByteLength < 3 {
		return fmt.Errorf("document.byteLength меньше размера UTF-8 BOM")
	}
	if d.SHA256 != nil {
		if len(*d.SHA256) != 64 || strings.Trim(*d.SHA256, "0123456789abcdef") != "" {
			return fmt.Errorf("document.sha256 должен содержать 64 шестнадцатеричных символа нижнего регистра")
		}
	}
	return nil
}

// checkProcessing проверяет непустые уникальные ID, Tool, формат известного
// StartedAt с часовым поясом и неотрицательные известные DurationMs.
func checkProcessing(entries []model.Processing) error {
	if entries == nil {
		return fmt.Errorf("processing должен быть массивом, а не null")
	}
	seen := make(map[string]bool, len(entries))
	timestamp := regexp.MustCompile(`^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(\.[0-9]+)?(Z|[+-](0[0-9]|1[0-9]|2[0-3]):[0-5][0-9])$`)
	for i, entry := range entries {
		if entry.ID == "" || seen[entry.ID] || entry.Tool == "" {
			return fmt.Errorf("processing[%d]: пустой или повторный ID либо пустой Tool", i)
		}
		seen[entry.ID] = true
		if !utf8.ValidString(entry.ID) || !utf8.ValidString(entry.Tool) || entry.Version != nil && !utf8.ValidString(*entry.Version) {
			return fmt.Errorf("processing[%d]: неверный UTF-8", i)
		}
		if entry.StartedAt != nil {
			if _, err := time.Parse(time.RFC3339Nano, *entry.StartedAt); err != nil || !timestamp.MatchString(*entry.StartedAt) {
				return fmt.Errorf("processing[%d].startedAt не является временем RFC 3339 с часовым поясом", i)
			}
		}
		if entry.DurationMs != nil && *entry.DurationMs < 0 {
			return fmt.Errorf("processing[%d].durationMs отрицательно", i)
		}
	}
	return nil
}

// checkLineSequence проверяет обязательный массив Lines, нумерацию 1..N,
// согласованность известного LineCount, типы строк, UTF-8, Raw и допустимый EOL.
// При неизвестном LineCount разрешён только пустой Lines; это не выдуманный
// пустой исходник, и диагностики должны уметь обрабатывать отсутствие строк.
func checkLineSequence(result *model.Result) error {
	if result.Lines == nil {
		return fmt.Errorf("lines должен быть массивом, а не null")
	}
	count := result.Document.LineCount
	if count == nil && len(result.Lines) != 0 || count != nil && *count != len(result.Lines) {
		return fmt.Errorf("document.lineCount не согласован с lines")
	}
	for i, line := range result.Lines {
		if line.Number != i+1 {
			return fmt.Errorf("lines[%d]: ожидается номер строки %d", i, i+1)
		}
		switch line.LineType {
		case model.LineTypeTag, model.LineTypeHeading, model.LineTypeContent, model.LineTypeBlockStart,
			model.LineTypeBlockEnd, model.LineTypeSeparator, model.LineTypeBlank, model.LineTypeInvalid:
		default:
			return fmt.Errorf("строка %d: неизвестный тип %q", line.Number, line.LineType)
		}
		if !utf8.ValidString(line.Raw) || strings.ContainsAny(line.Raw, "\r\n") || i == 0 && strings.HasPrefix(line.Raw, "\uFEFF") {
			return fmt.Errorf("строка %d: raw содержит неверный UTF-8, BOM или перевод строки", line.Number)
		}
		if line.LineEnding != model.LineEndingLF && line.LineEnding != model.LineEndingCRLF &&
			!(line.LineEnding == model.LineEndingNone && i == len(result.Lines)-1) {
			return fmt.Errorf("строка %d: недопустимый eol", line.Number)
		}
		if line.Raw == "" && line.LineEnding == model.LineEndingNone {
			return fmt.Errorf("строка %d: фиктивная пустая строка без исходного текста и eol", line.Number)
		}
	}
	return nil
}

// checkParentLinks проверяет предшествующего родителя, глубину и корневые
// заголовки. У block-end непустой родитель обязан иметь тип block-start.
// Сохранённая парсером ошибочная корневая block-end допустима только при
// существующей основной диагностике error на этой строке; коды P не зашиваются.
// Не восстанавливает дерево и не повторяет грамматический стек парсера.
func checkParentLinks(result *model.Result) error {
	for _, line := range result.Lines {
		if line.NestingLevel < 0 || line.ParentLine == nil && line.NestingLevel != 0 ||
			line.LineType == model.LineTypeHeading && (line.ParentLine != nil || line.NestingLevel != 0) {
			return fmt.Errorf("строка %d: недопустимая глубина или родитель заголовка", line.Number)
		}
		if line.ParentLine != nil {
			number := *line.ParentLine
			if number < 1 || number >= line.Number || number > len(result.Lines) {
				return fmt.Errorf("строка %d: родитель должен предшествовать строке", line.Number)
			}
			parent := result.Lines[number-1]
			if line.NestingLevel == 0 || line.NestingLevel-1 != parent.NestingLevel {
				return fmt.Errorf("строка %d: глубина не согласована с родителем", line.Number)
			}
			if line.LineType == model.LineTypeBlockEnd && parent.LineType != model.LineTypeBlockStart {
				return fmt.Errorf("строка %d: родитель block-end не является block-start", line.Number)
			}
		} else if line.LineType == model.LineTypeBlockEnd {
			found := false
			for _, entry := range result.Diagnostics {
				if entry.SeverityLevel == diagnostics.SeverityError && entry.Location != nil && entry.Location.Start.Line == line.Number {
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("строка %d: корневая block-end требует основную диагностику error", line.Number)
			}
		}
	}
	return nil
}

// checkElementPositions проверяет типы, допустимость Value, координаты Raw
// по кодовым точкам Unicode, порядок и отсутствие пересечений элементов.
// invalid требует unparsed, а unparsed — null Value и существующую ссылку;
// семантические ограничения конкретных DSL-тегов здесь не проверяются.
func checkElementPositions(line model.Line) error {
	if line.Elements == nil {
		return fmt.Errorf("elements должен быть массивом, а не null")
	}
	runes := []rune(line.Raw)
	previousStart, previousEnd, occupiedEnd := 0, 0, 0
	hasUnparsed, hasOpen, hasClose := false, false, false
	for i, element := range line.Elements {
		if element.Start < 1 || element.End < element.Start || element.End-1 > len(runes) ||
			!utf8.ValidString(element.Raw) || element.Raw != string(runes[element.Start-1:element.End-1]) {
			return fmt.Errorf("элемент %d: raw не согласован с Unicode-диапазоном", i)
		}
		if element.Start < previousStart || element.Start == previousStart && element.End < previousEnd ||
			element.Start < element.End && element.Start < occupiedEnd {
			return fmt.Errorf("элемент %d: нарушен порядок или диапазоны пересекаются", i)
		}
		previousStart, previousEnd = element.Start, element.End
		if element.Start < element.End {
			occupiedEnd = element.End
		}
		if element.ErrorIDs == nil || element.Value != nil && !utf8.ValidString(*element.Value) {
			return fmt.Errorf("элемент %d: errorIds равен null или value содержит неверный UTF-8", i)
		}
		if element.Value == nil && element.ElementType != model.ElementTypeUnparsed && element.ElementType != model.ElementTypePlaceholder && element.ElementType != model.ElementTypeMediaType {
			return fmt.Errorf("элемент %d: тип %q требует строковое value", i, element.ElementType)
		}
		switch element.ElementType {
		case model.ElementTypeTag:
			switch *element.Value {
			case "dsl-version", "document-id", "section", "order", "resource-dir", "header", "task", "endtask", "step", "speaking",
				"newpage", "editor", "media", "example", "wordlist", "table", "script", "text", "key", "instr", "note", "alt", "hint",
				"fragment", "include", "answer", "question", "multifill", "choice", "multichoice", "matching", "ordering", "variants", "variant":
			default:
				return fmt.Errorf("элемент %d: неизвестное каноническое имя тега", i)
			}
		case model.ElementTypeHeadingLevel:
			if (*element.Value != "1" && *element.Value != "2" && *element.Value != "3") || element.Raw != strings.Repeat("#", int((*element.Value)[0]-'0')) {
				return fmt.Errorf("элемент %d: неверный уровень заголовка", i)
			}
		case model.ElementTypeMediaType:
			if element.Value == nil && len(element.ErrorIDs) == 0 || element.Value != nil && *element.Value != "audio" && *element.Value != "video" && *element.Value != "image" {
				return fmt.Errorf("элемент %d: недопустимое value media-type", i)
			}
		case model.ElementTypeUnparsed:
			hasUnparsed = true
			if element.Value != nil || len(element.ErrorIDs) == 0 {
				return fmt.Errorf("элемент %d: unparsed требует null value и ссылку на диагностику", i)
			}
		case model.ElementTypeBlockOpen, model.ElementTypeBlockClose:
			brace := "{"
			if element.ElementType == model.ElementTypeBlockClose {
				brace = "}"
			}
			if element.Raw != brace || *element.Value != brace {
				return fmt.Errorf("элемент %d: неверное значение скобки", i)
			}
			hasOpen = hasOpen || element.ElementType == model.ElementTypeBlockOpen
			hasClose = hasClose || element.ElementType == model.ElementTypeBlockClose
		case model.ElementTypeTitle, model.ElementTypeContent, model.ElementTypeIdentifier, model.ElementTypeName,
			model.ElementTypeVersion, model.ElementTypeNumber, model.ElementTypeSource, model.ElementTypeResourcePath, model.ElementTypePlaceholder:
		default:
			return fmt.Errorf("элемент %d: неизвестный тип %q", i, element.ElementType)
		}
	}
	if line.LineType == model.LineTypeInvalid && !hasUnparsed || line.LineType == model.LineTypeBlockStart && !hasOpen || line.LineType == model.LineTypeBlockEnd && !hasClose {
		return fmt.Errorf("нет обязательного элемента для типа строки %q", line.LineType)
	}
	return nil
}

// checkDiagnosticSources проверяет непустые уникальные Diagnostic.ID,
// существующие Source и непустые Code без ограничения старых кодов префиксом V.
// Проверяет Severity/Scope и правило Fatal=true только при severity=error.
func checkDiagnosticSources(result *model.Result) error {
	if result.Diagnostics == nil {
		return fmt.Errorf("diagnostics должен быть массивом, а не null")
	}
	sources, seen := make(map[string]bool), make(map[string]bool)
	for _, entry := range result.Processing {
		sources[entry.ID] = true
	}
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] || !sources[entry.Source] || entry.DiagnosticCode == "" {
			return fmt.Errorf("diagnostics[%d]: неверный ID, Source или Code", i)
		}
		seen[entry.ID] = true
		for _, text := range []string{entry.ID, entry.Source, string(entry.DiagnosticCode), entry.Message} {
			if !utf8.ValidString(text) {
				return fmt.Errorf("диагностика %q: неверный UTF-8", entry.ID)
			}
		}
		switch entry.SeverityLevel {
		case diagnostics.SeverityError, diagnostics.SeverityWarning, diagnostics.SeverityRecommendation:
		default:
			return fmt.Errorf("диагностика %q: неизвестный severity", entry.ID)
		}
		switch entry.DiagnosticScope {
		case diagnostics.ScopeElement, diagnostics.ScopeLine, diagnostics.ScopeBlock, diagnostics.ScopeDocument:
		default:
			return fmt.Errorf("диагностика %q: неизвестный scope", entry.ID)
		}
		if entry.Fatal && entry.SeverityLevel != diagnostics.SeverityError {
			return fmt.Errorf("диагностика %q: fatal допустим только для error", entry.ID)
		}
	}
	return nil
}

// checkDiagnosticRanges проверяет Location, RelatedLocations, отсутствие
// повторов и допустимость области по существующим правилам JSON-контракта.
// Старые неэлементные записи могут иметь Location=nil, как допускает контракт.
func checkDiagnosticRanges(result *model.Result) error {
	for _, entry := range result.Diagnostics {
		if entry.RelatedLocations == nil {
			return fmt.Errorf("диагностика %q: relatedLocations должен быть массивом", entry.ID)
		}
		scope := entry.DiagnosticScope
		if entry.Location == nil && (scope == diagnostics.ScopeLine || scope == diagnostics.ScopeBlock) {
			scope = diagnostics.ScopeDocument
		}
		if err := checkOccurrence(result.Lines, scope, entry); err != nil {
			return fmt.Errorf("диагностика %q: %w", entry.ID, err)
		}
	}
	return nil
}

// checkElementLinks проверяет существование, уникальность и порядок ErrorIDs,
// область element и точное совпадение диапазона; каждая element-диагностика
// обязана иметь хотя бы одну обратную ссылку.
func checkElementLinks(result *model.Result) error {
	indices, linked := make(map[string]int), make(map[string]bool)
	for i, entry := range result.Diagnostics {
		indices[entry.ID] = i
	}
	for _, line := range result.Lines {
		for _, element := range line.Elements {
			previous := -1
			for _, id := range element.ErrorIDs {
				index, found := indices[id]
				if !found || index <= previous {
					return fmt.Errorf("строка %d: отсутствующая, повторная или неупорядоченная ссылка %q", line.Number, id)
				}
				entry := result.Diagnostics[index]
				location := entry.Location
				if entry.DiagnosticScope != diagnostics.ScopeElement || location == nil || location.Start.Line != line.Number || location.End.Line != line.Number || location.Start.Column != element.Start || location.End.Column != element.End {
					return fmt.Errorf("строка %d: ссылка %q не соответствует элементу", line.Number, id)
				}
				previous, linked[id] = index, true
			}
		}
	}
	for _, entry := range result.Diagnostics {
		if entry.DiagnosticScope == diagnostics.ScopeElement && !linked[entry.ID] {
			return fmt.Errorf("диагностика %q: нет обратной ссылки из элемента", entry.ID)
		}
	}
	return nil
}

// checkDiagnosticOrder проверяет порядок по Processing и позиции,
// не переставляя записи; любой существующий порядок равных позиций допустим.
func checkDiagnosticOrder(result *model.Result) error {
	indices := make(map[string]int, len(result.Processing))
	for i, entry := range result.Processing {
		indices[entry.ID] = i
	}
	previousSource, previousLine, previousColumn := -1, 0, 0
	for _, entry := range result.Diagnostics {
		source, found := indices[entry.Source]
		line, column := 0, 0
		if entry.Location != nil {
			line, column = entry.Location.Start.Line, entry.Location.Start.Column
		}
		if !found || source < previousSource || source == previousSource && (line < previousLine || line == previousLine && column < previousColumn) {
			return fmt.Errorf("диагностика %q: нарушен порядок по processing и позиции", entry.ID)
		}
		previousSource, previousLine, previousColumn = source, line, column
	}
	return nil
}

// checkErrorFlags проверяет эквивалентность hasErrors условию контракта,
// не исправляя исходную модель; как setErrorFlags, использует errorFlags.
func checkErrorFlags(result *model.Result) error {
	document, lines := errorFlags(result)
	if result.Document.HasErrors != document {
		return fmt.Errorf("document.hasErrors не соответствует диагностикам")
	}
	for _, line := range result.Lines {
		if line.HasErrors != lines[line.Number] {
			return fmt.Errorf("строка %d: hasErrors не соответствует диагностикам", line.Number)
		}
	}
	return nil
}

// errorFlags вычисляет ожидаемый признак документа и номера ошибочных строк
// по основной Location и ErrorIDs только записей error. Модель не изменяет;
// true означает наличие ошибки, false — её отсутствие. Карта принадлежит вызывающему.
func errorFlags(result *model.Result) (documentHasErrors bool, lineHasErrors map[int]bool) {
	lineHasErrors = make(map[int]bool)
	errors := make(map[string]bool)
	for _, entry := range result.Diagnostics {
		if entry.SeverityLevel == diagnostics.SeverityError {
			documentHasErrors = true
			errors[entry.ID] = true
			if entry.Location != nil {
				lineHasErrors[entry.Location.Start.Line] = true
			}
		}
	}
	for _, line := range result.Lines {
		for _, element := range line.Elements {
			for _, id := range element.ErrorIDs {
				if errors[id] {
					lineHasErrors[line.Number] = true
				}
			}
		}
	}
	return documentHasErrors, lineHasErrors
}
