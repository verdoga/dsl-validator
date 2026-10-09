package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/console"
	"github.com/verdoga/dsl-validator/model"
)

// readFile получает байты и FileInfo из одного открытого дескриптора,
// сверяет тип с PathInfo и обязательно закрывает дескриптор на всех путях.
// Ошибки чтения и закрытия объединяет через errors.Join.
func readFile(path string) (source []byte, info os.FileInfo, err error) {
	expected, err := console.PathInfo(path)
	if err != nil {
		return nil, nil, err
	}
	if !expected.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("путь %q не указывает на обычный файл", path)
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось открыть %q: %w", path, err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("не удалось закрыть %q: %w", path, closeErr))
		}
		if err != nil {
			source, info = nil, nil
		}
	}()
	info, err = file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось проверить открытый файл %q: %w", path, err)
	}
	if !info.Mode().IsRegular() || !os.SameFile(expected, info) {
		return nil, nil, fmt.Errorf("файл %q изменился при открытии", path)
	}
	source, err = io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("не удалось прочитать %q: %w", path, err)
	}
	return source, info, nil
}

// decodeResult принимает ровно один UTF-8 JSON-объект без JSONC-комментариев,
// BOM и хвоста после объекта; пробельный хвост допустим.
// Отклоняет повторные ключи, отсутствие обязательных полей, неверные JSON-типы
// и неподдерживаемый formatVersion. Неизвестные поля игнорирует семантически.
// Наличие DSL-версии 1.2 не является сигнатурой файла: null и неизвестная
// версия могут быть законным результатом парсера и сохраняются без подмены.
func decodeResult(source []byte) (*model.Result, error) {
	if err := checkJSONSyntax(source); err != nil {
		return nil, err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(source, &root); err != nil {
		return nil, err
	}
	if err := checkRootShape(root); err != nil {
		return nil, err
	}
	// Декодируем только точные имена: Unmarshal структуры принимает также
	// неизвестные контракту варианты регистра и мог бы подменить ими данные.
	var result model.Result
	var document, metadata map[string]json.RawMessage
	if err := json.Unmarshal(root["document"], &document); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(document["metadata"], &metadata); err != nil {
		return nil, err
	}
	// Значения карты — адреса полей разных типов для стандартного JSON-декодера.
	for name, target := range map[string]any{
		"dslVersion": &result.Document.DSLVersion, "fileName": &result.Document.FileName,
		"filePath": &result.Document.FilePath, "encoding": &result.Document.Encoding,
		"hasBom": &result.Document.HasBOM, "lineCount": &result.Document.LineCount,
		"byteLength": &result.Document.ByteLength, "sha256": &result.Document.SHA256,
		"hasErrors": &result.Document.HasErrors,
	} {
		if err := json.Unmarshal(document[name], target); err != nil {
			return nil, fmt.Errorf("document.%s: %w", name, err)
		}
	}
	for name, target := range map[string]any{
		"documentId": &result.Document.Metadata.DocumentID, "title": &result.Document.Metadata.Title,
		"subtitle": &result.Document.Metadata.Subtitle, "section": &result.Document.Metadata.Section,
		"order": &result.Document.Metadata.Order, "resourceDirs": &result.Document.Metadata.ResourceDirs,
	} {
		if err := json.Unmarshal(metadata[name], target); err != nil {
			return nil, fmt.Errorf("document.metadata.%s: %w", name, err)
		}
	}
	if err := json.Unmarshal(root["formatVersion"], &result.Format); err != nil {
		return nil, err
	}
	if result.Format != model.FormatVersion1 {
		return nil, fmt.Errorf("неподдерживаемая версия JSON-контракта %q", result.Format)
	}
	var processing, lines, diagnostics []map[string]json.RawMessage
	for name, target := range map[string]any{
		"processing": &processing, "lines": &lines, "diagnostics": &diagnostics,
	} {
		if err := json.Unmarshal(root[name], target); err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
	}
	result.Processing = make([]model.Processing, len(processing))
	for i, raw := range processing {
		entry := &result.Processing[i]
		for name, target := range map[string]any{
			"id": &entry.ID, "tool": &entry.Tool, "version": &entry.Version,
			"startedAt": &entry.StartedAt, "durationMs": &entry.DurationMs,
		} {
			if err := json.Unmarshal(raw[name], target); err != nil {
				return nil, fmt.Errorf("processing[%d].%s: %w", i, name, err)
			}
		}
	}
	result.Lines = make([]model.Line, len(lines))
	for i, raw := range lines {
		line := &result.Lines[i]
		for name, target := range map[string]any{
			"line": &line.Number, "type": &line.LineType, "nestingLevel": &line.NestingLevel,
			"parentLine": &line.ParentLine, "raw": &line.Raw, "eol": &line.LineEnding, "hasErrors": &line.HasErrors,
		} {
			if err := json.Unmarshal(raw[name], target); err != nil {
				return nil, fmt.Errorf("lines[%d].%s: %w", i, name, err)
			}
		}
		var elements []map[string]json.RawMessage
		if err := json.Unmarshal(raw["elements"], &elements); err != nil {
			return nil, err
		}
		line.Elements = make([]model.Element, len(elements))
		for j, rawElement := range elements {
			element := &line.Elements[j]
			for name, target := range map[string]any{
				"type": &element.ElementType, "raw": &element.Raw, "value": &element.Value,
				"start": &element.Start, "end": &element.End, "errorIds": &element.ErrorIDs,
			} {
				if err := json.Unmarshal(rawElement[name], target); err != nil {
					return nil, fmt.Errorf("lines[%d].elements[%d].%s: %w", i, j, name, err)
				}
			}
		}
	}
	result.Diagnostics = make([]model.Diagnostic, len(diagnostics))
	for i, raw := range diagnostics {
		entry := &result.Diagnostics[i]
		for name, target := range map[string]any{
			"id": &entry.ID, "source": &entry.Source, "code": &entry.DiagnosticCode,
			"severity": &entry.SeverityLevel, "message": &entry.Message,
			"scope": &entry.DiagnosticScope, "fatal": &entry.Fatal,
		} {
			if err := json.Unmarshal(raw[name], target); err != nil {
				return nil, fmt.Errorf("diagnostics[%d].%s: %w", i, name, err)
			}
		}
		var ranges []json.RawMessage
		if err := json.Unmarshal(raw["relatedLocations"], &ranges); err != nil {
			return nil, err
		}
		entry.RelatedLocations = make([]model.Location, len(ranges))
		targets := make([]*model.Location, len(ranges))
		for j := range targets {
			targets[j] = &entry.RelatedLocations[j]
		}
		if !bytes.Equal(bytes.TrimSpace(raw["location"]), []byte("null")) {
			entry.Location = new(model.Location)
			ranges = append(ranges, raw["location"])
			targets = append(targets, entry.Location)
		}
		for j, rawRange := range ranges {
			var location map[string]json.RawMessage
			if err := json.Unmarshal(rawRange, &location); err != nil {
				return nil, err
			}
			for name, position := range map[string]*model.Position{"start": &targets[j].Start, "end": &targets[j].End} {
				var coordinates map[string]json.RawMessage
				if err := json.Unmarshal(location[name], &coordinates); err != nil {
					return nil, err
				}
				for coordinate, target := range map[string]*int{"line": &position.Line, "column": &position.Column} {
					if err := json.Unmarshal(coordinates[coordinate], target); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	return &result, nil
}

// checkJSONSyntax проверяет единственность корневого объекта, корректность
// UTF-8/Unicode-последовательностей и уникальность ключей на всех уровнях.
// Не допускает молчаливой замены повреждённого текста декодером JSON.
func checkJSONSyntax(source []byte) error {
	if !utf8.Valid(source) {
		return fmt.Errorf("JSON содержит повреждённый UTF-8")
	}
	trimmed := bytes.TrimSpace(source)
	if len(trimmed) == 0 || trimmed[0] != '{' || !json.Valid(source) {
		return fmt.Errorf("ожидался ровно один корректный JSON-объект")
	}
	// JSON-декодер заменяет непарные суррогаты на U+FFFD; проверяем их до разбора.
	for i := 0; i < len(source); i++ {
		if source[i] != '\\' {
			continue
		}
		i++
		if source[i] != 'u' {
			continue
		}
		code, _ := strconv.ParseUint(string(source[i+1:i+5]), 16, 16)
		i += 4
		if code >= 0xDC00 && code <= 0xDFFF {
			return fmt.Errorf("непарный младший Unicode-суррогат у байта %d", i-4)
		}
		if code < 0xD800 || code > 0xDBFF {
			continue
		}
		if i+6 >= len(source) || source[i+1] != '\\' || source[i+2] != 'u' {
			return fmt.Errorf("непарный старший Unicode-суррогат у байта %d", i-4)
		}
		low, parseErr := strconv.ParseUint(string(source[i+3:i+7]), 16, 16)
		if parseErr != nil || low < 0xDC00 || low > 0xDFFF {
			return fmt.Errorf("неверная пара Unicode-суррогатов у байта %d", i-4)
		}
		i += 6
	}
	decoder := json.NewDecoder(bytes.NewReader(source))
	decoder.UseNumber()
	var keys []map[string]bool
	var expectingKey []bool
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("не удалось разобрать JSON: %w", err)
		}
		if delimiter, ok := token.(json.Delim); ok && (delimiter == '}' || delimiter == ']') {
			keys = keys[:len(keys)-1]
			expectingKey = expectingKey[:len(expectingKey)-1]
			continue
		}
		last := len(keys) - 1
		if last >= 0 && keys[last] != nil {
			if expectingKey[last] {
				key, ok := token.(string)
				if !ok {
					return fmt.Errorf("ожидалось имя поля JSON")
				}
				if keys[last][key] {
					return fmt.Errorf("повторное поле JSON %q", key)
				}
				keys[last][key] = true
				expectingKey[last] = false
				continue
			}
			expectingKey[last] = true
		}
		if delimiter, ok := token.(json.Delim); ok {
			var objectKeys map[string]bool
			if delimiter == '{' {
				objectKeys = make(map[string]bool)
			}
			keys = append(keys, objectKeys)
			expectingKey = append(expectingKey, delimiter == '{')
		}
	}
}

// checkRootShape проверяет наличие formatVersion, document, processing,
// lines, diagnostics и вызывает проверки форм вложенных объектов.
// Все обязательные массивы представлены [], а не null; исключение
// metadata.resourceDirs допускает null согласно контракту.
func checkRootShape(root map[string]json.RawMessage) error {
	version := bytes.TrimSpace(root["formatVersion"])
	if len(version) == 0 || version[0] != '"' || !json.Valid(version) {
		return fmt.Errorf("formatVersion: ожидалась обязательная строка")
	}
	for name, check := range map[string]func(json.RawMessage) error{
		"document": checkDocumentShape, "processing": checkProcessingShape,
		"lines": checkLinesShape, "diagnostics": checkDiagnosticsShape,
	} {
		if err := check(root[name]); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	return nil
}

// checkDocumentShape проверяет обязательные поля document и metadata,
// их JSON-типы и разрешённый null без проверки достоверности метаданных.
func checkDocumentShape(raw json.RawMessage) error {
	var document, metadata map[string]json.RawMessage
	if err := json.Unmarshal(raw, &document); err != nil {
		return fmt.Errorf("ожидался объект document: %w", err)
	}
	if err := json.Unmarshal(document["metadata"], &metadata); err != nil {
		return fmt.Errorf("metadata: ожидался объект: %w", err)
	}
	objects := []map[string]json.RawMessage{document, metadata}
	paths := []string{"document", "document.metadata"}
	rules := []map[string]string{{
		"dslVersion": "string|null", "fileName": "string|null", "filePath": "string|null",
		"encoding": "string|null", "hasBom": "boolean|null", "lineCount": "integer|null",
		"byteLength": "integer|null", "sha256": "string|null", "metadata": "object", "hasErrors": "boolean",
	}, {
		"documentId": "string|null", "title": "string|null", "subtitle": "string|null",
		"section": "string|null", "order": "string|null", "resourceDirs": "strings|null",
	}}

	for i, object := range objects {
		if object == nil {
			return fmt.Errorf("%s: ожидался объект", paths[i])
		}
		for name, rule := range rules[i] {
			field := bytes.TrimSpace(object[name])
			if len(field) == 0 {
				return fmt.Errorf("%s.%s: отсутствует обязательное поле", paths[i], name)
			}
			if strings.HasSuffix(rule, "|null") && bytes.Equal(field, []byte("null")) {
				continue
			}
			valid := false
			switch strings.TrimSuffix(rule, "|null") {
			case "string":
				valid = field[0] == '"'
			case "boolean":
				valid = bytes.Equal(field, []byte("true")) || bytes.Equal(field, []byte("false"))
			case "integer":
				bits := strconv.IntSize
				if name == "byteLength" {
					bits = 64
				}
				_, err := strconv.ParseInt(string(field), 10, bits)
				valid = err == nil
			case "object":
				valid = field[0] == '{'
			case "array":
				valid = field[0] == '['
			case "strings":
				var entries []json.RawMessage
				valid = field[0] == '[' && json.Unmarshal(field, &entries) == nil
				for _, entry := range entries {
					entry = bytes.TrimSpace(entry)
					valid = valid && len(entry) > 0 && entry[0] == '"'
				}
			}
			if !valid {
				return fmt.Errorf("%s.%s: ожидался тип %s", paths[i], name, rule)
			}
		}
	}
	return nil
}

// checkProcessingShape проверяет обязательные поля и JSON-типы записей processing.
func checkProcessingShape(raw json.RawMessage) error {
	var objects []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &objects); err != nil {
		return fmt.Errorf("ожидался массив processing: %w", err)
	}
	if objects == nil {
		return fmt.Errorf("ожидался массив processing, а не null")
	}
	paths := make([]string, len(objects))
	rules := make([]map[string]string, len(objects))
	for i := range objects {
		paths[i] = fmt.Sprintf("processing[%d]", i)
		rules[i] = map[string]string{
			"id": "string", "tool": "string", "version": "string|null",
			"startedAt": "string|null", "durationMs": "integer|null",
		}
	}

	for i, object := range objects {
		if object == nil {
			return fmt.Errorf("%s: ожидался объект", paths[i])
		}
		for name, rule := range rules[i] {
			field := bytes.TrimSpace(object[name])
			if len(field) == 0 {
				return fmt.Errorf("%s.%s: отсутствует обязательное поле", paths[i], name)
			}
			if strings.HasSuffix(rule, "|null") && bytes.Equal(field, []byte("null")) {
				continue
			}
			valid := false
			switch strings.TrimSuffix(rule, "|null") {
			case "string":
				valid = field[0] == '"'
			case "boolean":
				valid = bytes.Equal(field, []byte("true")) || bytes.Equal(field, []byte("false"))
			case "integer":
				bits := strconv.IntSize
				if name == "byteLength" {
					bits = 64
				}
				_, err := strconv.ParseInt(string(field), 10, bits)
				valid = err == nil
			}
			if !valid {
				return fmt.Errorf("%s.%s: ожидался тип %s", paths[i], name, rule)
			}
		}
	}
	return nil
}

// checkLinesShape проверяет обязательные поля строк и элементов, включая
// наличие hasErrors, errorIds и допустимость null у parentLine/value.
func checkLinesShape(raw json.RawMessage) error {
	var lines []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &lines); err != nil {
		return fmt.Errorf("ожидался массив lines: %w", err)
	}
	if lines == nil {
		return fmt.Errorf("ожидался массив lines, а не null")
	}
	var objects []map[string]json.RawMessage
	var paths []string
	var rules []map[string]string
	for i, line := range lines {
		objects = append(objects, line)
		paths = append(paths, fmt.Sprintf("lines[%d]", i))
		rules = append(rules, map[string]string{
			"line": "integer", "type": "string", "nestingLevel": "integer", "parentLine": "integer|null",
			"raw": "string", "eol": "string", "hasErrors": "boolean", "elements": "array",
		})
		var elements []map[string]json.RawMessage
		if err := json.Unmarshal(line["elements"], &elements); err != nil {
			return fmt.Errorf("lines[%d].elements: %w", i, err)
		}
		for j, element := range elements {
			objects = append(objects, element)
			paths = append(paths, fmt.Sprintf("lines[%d].elements[%d]", i, j))
			rules = append(rules, map[string]string{
				"type": "string", "raw": "string", "value": "string|null",
				"start": "integer", "end": "integer", "errorIds": "strings",
			})
		}
	}

	for i, object := range objects {
		if object == nil {
			return fmt.Errorf("%s: ожидался объект", paths[i])
		}
		for name, rule := range rules[i] {
			field := bytes.TrimSpace(object[name])
			if len(field) == 0 {
				return fmt.Errorf("%s.%s: отсутствует обязательное поле", paths[i], name)
			}
			if strings.HasSuffix(rule, "|null") && bytes.Equal(field, []byte("null")) {
				continue
			}
			valid := false
			switch strings.TrimSuffix(rule, "|null") {
			case "string":
				valid = field[0] == '"'
			case "boolean":
				valid = bytes.Equal(field, []byte("true")) || bytes.Equal(field, []byte("false"))
			case "integer":
				bits := strconv.IntSize
				if name == "byteLength" {
					bits = 64
				}
				_, err := strconv.ParseInt(string(field), 10, bits)
				valid = err == nil
			case "object":
				valid = field[0] == '{'
			case "array":
				valid = field[0] == '['
			case "strings":
				var entries []json.RawMessage
				valid = field[0] == '[' && json.Unmarshal(field, &entries) == nil
				for _, entry := range entries {
					entry = bytes.TrimSpace(entry)
					valid = valid && len(entry) > 0 && entry[0] == '"'
				}
			}
			if !valid {
				return fmt.Errorf("%s.%s: ожидался тип %s", paths[i], name, rule)
			}
		}
	}
	return nil
}

// checkDiagnosticsShape проверяет обязательные поля диагностик и диапазонов,
// включая сохранённое поле fatal; ID и координаты проверяет validator.
func checkDiagnosticsShape(raw json.RawMessage) error {
	var diagnostics []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &diagnostics); err != nil {
		return fmt.Errorf("ожидался массив diagnostics: %w", err)
	}
	if diagnostics == nil {
		return fmt.Errorf("ожидался массив diagnostics, а не null")
	}
	var objects []map[string]json.RawMessage
	var paths []string
	var rules []map[string]string
	for i, diagnostic := range diagnostics {
		objects = append(objects, diagnostic)
		paths = append(paths, fmt.Sprintf("diagnostics[%d]", i))
		rules = append(rules, map[string]string{
			"id": "string", "source": "string", "code": "string", "severity": "string",
			"message": "string", "scope": "string", "fatal": "boolean",
			"location": "object|null", "relatedLocations": "array",
		})
		var ranges []json.RawMessage
		if err := json.Unmarshal(diagnostic["relatedLocations"], &ranges); err != nil {
			return fmt.Errorf("diagnostics[%d].relatedLocations: %w", i, err)
		}
		rangePaths := make([]string, len(ranges))
		for j := range ranges {
			rangePaths[j] = fmt.Sprintf("diagnostics[%d].relatedLocations[%d]", i, j)
		}
		if !bytes.Equal(bytes.TrimSpace(diagnostic["location"]), []byte("null")) {
			ranges = append(ranges, diagnostic["location"])
			rangePaths = append(rangePaths, fmt.Sprintf("diagnostics[%d].location", i))
		}
		for j, rawRange := range ranges {
			var location map[string]json.RawMessage
			if err := json.Unmarshal(rawRange, &location); err != nil {
				return fmt.Errorf("%s: %w", rangePaths[j], err)
			}
			objects = append(objects, location)
			paths = append(paths, rangePaths[j])
			rules = append(rules, map[string]string{"start": "object", "end": "object"})
			for _, name := range []string{"start", "end"} {
				var position map[string]json.RawMessage
				if err := json.Unmarshal(location[name], &position); err != nil {
					return fmt.Errorf("%s.%s: %w", rangePaths[j], name, err)
				}
				objects = append(objects, position)
				paths = append(paths, rangePaths[j]+"."+name)
				rules = append(rules, map[string]string{"line": "integer", "column": "integer"})
			}
		}
	}

	for i, object := range objects {
		if object == nil {
			return fmt.Errorf("%s: ожидался объект", paths[i])
		}
		for name, rule := range rules[i] {
			field := bytes.TrimSpace(object[name])
			if len(field) == 0 {
				return fmt.Errorf("%s.%s: отсутствует обязательное поле", paths[i], name)
			}
			if strings.HasSuffix(rule, "|null") && bytes.Equal(field, []byte("null")) {
				continue
			}
			valid := false
			switch strings.TrimSuffix(rule, "|null") {
			case "string":
				valid = field[0] == '"'
			case "boolean":
				valid = bytes.Equal(field, []byte("true")) || bytes.Equal(field, []byte("false"))
			case "integer":
				bits := strconv.IntSize
				if name == "byteLength" {
					bits = 64
				}
				_, err := strconv.ParseInt(string(field), 10, bits)
				valid = err == nil
			case "object":
				valid = field[0] == '{'
			case "array":
				valid = field[0] == '['
			}
			if !valid {
				return fmt.Errorf("%s.%s: ожидался тип %s", paths[i], name, rule)
			}
		}
	}
	return nil
}
