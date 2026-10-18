package checkers

import (
	"encoding/json"
	"slices"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV031Registration(t *testing.T) {
	definitions, err := v031Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V031" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Scope != "line" ||
			definition.Message != "Служебное обозначение содержит прописные буквы." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V031 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V031 definitions = %d, want 1", matches)
	}
}

func TestV031ReferenceAndExample(t *testing.T) {
	// Фрагменты сохранены без изменения символов DSL. v031Document добавляет
	// только обязательные версию и заголовок; &lt;/&gt; таблицы означают < и >.
	v031Run(t, v031Document(t, v031Open(t, "text", "text", 2, 1),
		v031Text(t, "<strong>Read</strong>.", 3, 2), v031Close(t, 3, 2)), "current", nil)
	v031Run(t, v031Document(t, v031Open(t, "Text", "text", 2, 1),
		v031Text(t, "<STRONG>Read</STRONG>.", 3, 2), v031Close(t, 3, 2)), "current",
		[][3]int{{3, 1, 6}, {4, 1, 9}, {4, 13, 22}})
}

func TestV031DSLTagsAndMedia(t *testing.T) {
	for _, tc := range []struct {
		name, spelling string
		want           [][3]int
	}{
		{"lowercase", "note", nil},
		{"one uppercase", "Note", [][3]int{{3, 1, 6}}},
		{"all uppercase gives one message", "NOTE", [][3]int{{3, 1, 6}}},
		{"mixed case", "nOtE", [][3]int{{3, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v031Line(t, "@"+tc.spelling+" TEXT", "tag", 2, 1,
				v031Element(t, "tag", "@"+tc.spelling, "note", 1, 6), v031Element(t, "content", "TEXT", "TEXT", 7, 11))
			v031Run(t, v031Document(t, line), "current", tc.want)
		})
	}
	for _, tc := range []struct {
		name, tag, kind, value string
		want                   [][3]int
	}{
		{"lowercase audio", "media", "audio", "audio", nil},
		{"lowercase video", "media", "video", "video", nil},
		{"lowercase image", "media", "image", "image", nil},
		{"both fields", "MeDiA", "Audio", "audio", [][3]int{{4, 1, 7}, {4, 8, 13}}},
		{"audio", "media", "Audio", "audio", [][3]int{{4, 8, 13}}},
		{"video", "media", "VIDEO", "video", [][3]int{{4, 8, 13}}},
		{"image", "media", "Image", "image", [][3]int{{4, 8, 13}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			base := v031Document(t)
			path := v031Line(t, "@resource-dir .", "tag", 0, 0,
				v031Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v031Element(t, "resource-path", ".", ".", 15, 16))
			line := v031Line(t, "@"+tc.tag+" "+tc.kind+` "Clip.MP3"`, "tag", 3, 1,
				v031Element(t, "tag", "@"+tc.tag, "media", 1, 7), v031Element(t, "media-type", tc.kind, tc.value, 8, 13),
				v031Element(t, "source", `"Clip.MP3"`, "Clip.MP3", 14, 24))
			result := v031Model(t, base.Lines[0], path, base.Lines[1], line)
			result.Document.Metadata.ResourceDirs = []string{"."}
			v031Run(t, result, "current", tc.want)
		})
	}
	t.Run("first line", func(t *testing.T) {
		result := v031Document(t)
		result.Lines[0].Raw = "@DSL-VERSION 1.2"
		result.Lines[0].Elements[0].Raw = "@DSL-VERSION"
		v031Run(t, result, "current", [][3]int{{1, 1, 13}})
	})
	t.Run("original indentation", func(t *testing.T) {
		line := v031Line(t, " \t@Note TEXT  ", "tag", 2, 1,
			v031Element(t, "tag", "@Note", "note", 3, 8), v031Element(t, "content", "TEXT", "TEXT", 9, 13))
		v031Run(t, v031Document(t, line), "current", [][3]int{{3, 3, 8}})
	})
	t.Run("nested declaration", func(t *testing.T) {
		v031Run(t, v031Document(t, v031Open(t, "instr", "instr", 2, 1), v031Text(t, "Read.", 3, 2),
			v031Line(t, "@Hint Read.", "tag", 3, 2, v031Element(t, "tag", "@Hint", "hint", 1, 6),
				v031Element(t, "content", "Read.", "Read.", 7, 12)), v031Close(t, 3, 2)), "current", [][3]int{{5, 1, 6}})
	})
}

func TestV031HTML(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             [][3]int
	}{
		{"lowercase with uppercase text", "<em>READ</em>", "<em>READ</em>", nil},
		{"both markers", "<EM>x</EM>", "<EM>x</EM>", [][3]int{{4, 1, 5}, {4, 6, 11}}},
		{"opening only", "<EM>x</em>", "<EM>x</em>", [][3]int{{4, 1, 5}}},
		{"closing only", "<em>x</EM>", "<em>x</EM>", [][3]int{{4, 6, 11}}},
		{"strong mixed case", "<StRoNg>x</sTrOnG>", "<StRoNg>x</sTrOnG>", [][3]int{{4, 1, 9}, {4, 10, 19}}},
		{"underline", "<U>x</U>", "<U>x</U>", [][3]int{{4, 1, 4}, {4, 5, 9}}},
		{"nested markers", "<EM><U>x</U></EM>", "<EM><U>x</U></EM>", [][3]int{{4, 1, 5}, {4, 5, 8}, {4, 9, 13}, {4, 13, 18}}},
		{"Unicode prefix", "Я🙂<EM>x</EM>", "Я🙂<EM>x</EM>", [][3]int{{4, 3, 7}, {4, 8, 13}}},
		{"combining mark prefix", "е\u0301<EM>x</EM>", "е\u0301<EM>x</EM>", [][3]int{{4, 3, 7}, {4, 8, 13}}},
		{"standalone br", "<BR>", "<BR>", [][3]int{{4, 1, 5}}},
		{"mixed case br", "<bR>", "<bR>", [][3]int{{4, 1, 5}}},
		{"lowercase br", "<br>", "<br>", nil},
		{"br with edge whitespace", " \t<BR>\t ", "<BR>", [][3]int{{4, 3, 7}}},
		{"br with NBSP is not standalone", "\u00a0<BR>", "\u00a0<BR>", nil},
		{"inline br", "x<BR>y", "x<BR>y", nil},
		{"self closing br", "<BR/>", "<BR/>", nil},
		{"spaced self closing br", "<BR />", "<BR />", nil},
		{"closing br", "</BR>", "</BR>", nil},
		{"attribute", `<EM class="x">`, `<EM class="x">`, nil},
		{"trailing space in marker", "<EM >", "<EM >", nil},
		{"unknown marker", "<SPAN>x</SPAN>", "<SPAN>x</SPAN>", nil},
		{"incomplete marker", "<EM", "<EM", nil},
		{"escaped opening", `\<EM>`, "<EM>", nil},
		{"escaped closing angle", `<EM\>`, "<EM>", nil},
		{"escaped closing marker", `\</EM>`, "</EM>", nil},
		{"two backslashes", `\\<EM>`, `\<EM>`, [][3]int{{4, 3, 7}}},
		{"three backslashes", `\\\<EM>`, `\<EM>`, nil},
		{"four backslashes", `\\\\<EM>`, `\\<EM>`, [][3]int{{4, 5, 9}}},
		{"escaped marker then real marker", `\<EM> <U>x</U>`, "<EM> <U>x</U>", [][3]int{{4, 7, 10}, {4, 11, 15}}},
		{"escaped br", `\<BR>`, "<BR>", nil},
		{"entities not decoded", "&lt;EM&gt;", "&lt;EM&gt;", nil},
		{"unpaired opening", "<EM>", "<EM>", [][3]int{{4, 1, 5}}},
		{"unpaired closing", "</EM>", "</EM>", [][3]int{{4, 1, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := v031Line(t, tc.raw, "content", 3, 2,
				v031Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v031Run(t, v031Document(t, v031Open(t, "text", "text", 2, 1), content, v031Close(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("content offset", func(t *testing.T) {
		line := v031Line(t, " \t<BR> ", "content", 3, 2, v031Element(t, "content", "<BR>", "<BR>", 3, 7))
		v031Run(t, v031Document(t, v031Open(t, "text", "text", 2, 1), line, v031Close(t, 3, 2)), "current", [][3]int{{4, 3, 7}})
	})
}

func TestV031ExcludedContexts(t *testing.T) {
	for _, tag := range []string{"editor", "multifill", "note", "table"} {
		t.Run("HTML outside text in "+tag, func(t *testing.T) {
			v031Run(t, v031Document(t, v031Open(t, tag, tag, 2, 1),
				v031Text(t, "<STRONG>x</STRONG>", 3, 2), v031Close(t, 3, 2)), "current", nil)
		})
	}
	for _, tc := range []struct {
		tag, spelling string
		want          [][3]int
	}{
		{"editor", "editor", nil}, {"editor", "Editor", [][3]int{{3, 1, 8}}},
		{"multifill", "multifill", nil}, {"multifill", "Multifill", [][3]int{{3, 1, 11}}},
	} {
		t.Run("literal content of "+tc.spelling, func(t *testing.T) {
			v031Run(t, v031Document(t, v031Open(t, tc.spelling, tc.tag, 2, 1),
				v031Text(t, "@Text", 3, 2), v031Text(t, "<STRONG>", 3, 2), v031Close(t, 3, 2)), "current", tc.want)
		})
	}
	t.Run("inline comment declaration", func(t *testing.T) {
		line := v031Line(t, "@Editor @Text", "tag", 2, 1,
			v031Element(t, "tag", "@Editor", "editor", 1, 8), v031Element(t, "content", "@Text", "@Text", 9, 14))
		v031Run(t, v031Document(t, line), "current", [][3]int{{3, 1, 8}})
	})
	t.Run("free text", func(t *testing.T) {
		v031Run(t, v031Document(t, v031Text(t, "READ <STRONG> and @Text", 2, 1)), "current", nil)
	})
	t.Run("text title", func(t *testing.T) {
		opening := v031Line(t, "@text <STRONG> {", "block-start", 2, 1,
			v031Element(t, "tag", "@text", "text", 1, 6), v031Element(t, "title", "<STRONG>", "<STRONG>", 7, 15),
			v031Element(t, "block-open", "{", "{", 16, 17))
		v031Run(t, v031Document(t, opening, v031Text(t, "READ", 3, 2), v031Close(t, 3, 2)), "current", nil)
	})
	t.Run("block context ends", func(t *testing.T) {
		v031Run(t, v031Document(t, v031Open(t, "text", "text", 2, 1), v031Text(t, "<EM>", 3, 2),
			v031Close(t, 3, 2), v031Text(t, "<EM>", 2, 1)), "current", [][3]int{{4, 1, 5}})
	})
	t.Run("no cross line marker", func(t *testing.T) {
		v031Run(t, v031Document(t, v031Open(t, "text", "text", 2, 1), v031Text(t, "<EM", 3, 2),
			v031Text(t, ">", 3, 2), v031Close(t, 3, 2)), "current", nil)
	})
	t.Run("placeholder answer", func(t *testing.T) {
		line := v031Line(t, "_____{<EM>}", "content", 3, 2, v031Element(t, "placeholder", "_____{<EM>}", "<EM>", 1, 12))
		v031Run(t, v031Document(t, v031Open(t, "multifill", "multifill", 2, 1), line, v031Close(t, 3, 2)), "current", nil)
	})
	t.Run("heading title", func(t *testing.T) {
		base := v031Document(t)
		heading := v031Line(t, "# TITLE", "heading", 0, 0, v031Element(t, "heading-level", "#", "1", 1, 2),
			v031Element(t, "title", "TITLE", "TITLE", 3, 8))
		result := v031Model(t, base.Lines[0], heading)
		title := "TITLE"
		result.Document.Metadata.Title = &title
		v031Run(t, result, "current", nil)
	})
	t.Run("section name and resource path", func(t *testing.T) {
		base := v031Document(t)
		section := v031Line(t, "@section TITLE", "tag", 0, 0, v031Element(t, "tag", "@section", "section", 1, 9),
			v031Element(t, "name", "TITLE", "TITLE", 10, 15))
		path := v031Line(t, "@resource-dir Assets/UPPER", "tag", 0, 0,
			v031Element(t, "tag", "@resource-dir", "resource-dir", 1, 14), v031Element(t, "resource-path", "Assets/UPPER", "Assets/UPPER", 15, 27))
		result := v031Model(t, base.Lines[0], section, path, base.Lines[1])
		name := "TITLE"
		result.Document.Metadata.Section = &name
		result.Document.Metadata.ResourceDirs = []string{"Assets/UPPER"}
		v031Run(t, result, "current", nil)
	})
	t.Run("identifier and nested text block", func(t *testing.T) {
		// DSL 3.12.9–3.12.10: fragment находится после заголовка и допускает text.
		fragment := v031Line(t, "@fragment SAMPLE {", "block-start", 2, 1,
			v031Element(t, "tag", "@fragment", "fragment", 1, 10), v031Element(t, "identifier", "SAMPLE", "sample", 11, 17),
			v031Element(t, "block-open", "{", "{", 18, 19))
		v031Run(t, v031Document(t, fragment, v031Open(t, "text", "text", 3, 2),
			v031Text(t, "<EM>x</em>", 4, 3), v031Close(t, 4, 3), v031Close(t, 3, 2)), "current", [][3]int{{5, 1, 5}})
	})
}

func TestV031UnrecognizedElements(t *testing.T) {
	t.Run("unknown tag inside text", func(t *testing.T) {
		line := v031Line(t, "@Unknown <EM>", "invalid", 3, 2, model.Element{
			ElementType: "unparsed", Raw: "@Unknown <EM>", Start: 1, End: 14, ErrorIDs: []string{"parser-error"},
		})
		line.HasErrors = true
		result := v031Document(t, v031Open(t, "text", "text", 2, 1), line,
			v031Text(t, "<EM>", 3, 2), v031Close(t, 3, 2))
		v031ParserError(t, result, 4, 1, 14, "Неизвестный DSL-тег.")
		v031Run(t, result, "current", [][3]int{{5, 1, 5}})
	})
	for _, tc := range []struct {
		tag  string
		want [][3]int
	}{
		{"media", nil}, {"Media", [][3]int{{3, 1, 7}}},
	} {
		t.Run("unknown media type with "+tc.tag, func(t *testing.T) {
			line := v031Line(t, "@"+tc.tag+` SOUND "Clip.MP3"`, "tag", 2, 1,
				v031Element(t, "tag", "@"+tc.tag, "media", 1, 7),
				model.Element{ElementType: "media-type", Raw: "SOUND", Start: 8, End: 13, ErrorIDs: []string{"parser-error"}},
				v031Element(t, "source", `"Clip.MP3"`, "Clip.MP3", 14, 24))
			line.HasErrors = true
			result := v031Document(t, line)
			v031ParserError(t, result, 3, 8, 13, "Неизвестный тип медиа.")
			v031Run(t, result, "current", tc.want)
		})
	}
}

func TestV031LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      [][3]int
	}{
		{"lowercase", "<em>x</em>", nil},
		{"uppercase", "<EM>x</EM>", [][3]int{{4, 1, 5}, {4, 6, 11}}},
	} {
		for _, ending := range []struct {
			name      string
			eol, last model.LineEnding
		}{
			{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
			{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""}, {"mixed", "\r\n", "\n"},
		} {
			t.Run(tc.name+"/"+ending.name, func(t *testing.T) {
				result := v031Document(t, v031Open(t, "text", "text", 2, 1), v031Text(t, tc.raw, 3, 2), v031Close(t, 3, 2))
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending.eol
				}
				result.Lines[4].LineEnding = ending.last
				v031Run(t, result, "current", tc.want)
			})
		}
	}
	t.Run("last line declaration without EOL", func(t *testing.T) {
		line := v031Line(t, "@Note Text.", "tag", 2, 1, v031Element(t, "tag", "@Note", "note", 1, 6),
			v031Element(t, "content", "Text.", "Text.", 7, 12))
		line.LineEnding = model.LineEndingNone
		v031Run(t, v031Document(t, line), "current", [][3]int{{3, 1, 6}})
	})
}

func TestV031EmptyAndVersions(t *testing.T) {
	result := v031Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v031Run(t, result, "current", nil)
	v031Run(t, v031Document(t, v031Line(t, "", "blank", 2, 1), v031Line(t, " \t", "blank", 2, 1)), "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v031Document(t)
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v031Snapshot(t, result)
			checks, err := validator.Prepare(result, v031Registry(t))
			if err == nil || checks != nil || v031Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV031HistoryAndOtherChecks(t *testing.T) {
	line := v031Line(t, "@Note TEXT!!", "tag", 2, 1, v031Element(t, "tag", "@Note", "note", 1, 6),
		v031Element(t, "content", "TEXT!!", "TEXT!!", 7, 13))
	result := v031Document(t, line)
	v031Run(t, result, "first", [][3]int{{3, 1, 6}})
	v031Run(t, result, "second", [][3]int{{3, 1, 6}})
	v031Run(t, v031Document(t, v031Text(t, "READ", 2, 1)), "current", nil)
	for _, source := range []string{"first", "second"} {
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.Source != source || entry.DiagnosticCode != "V026" {
				continue
			}
			count++
			want := model.Location{Start: model.Position{Line: 3, Column: 11}, End: model.Position{Line: 3, Column: 13}}
			if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" ||
				entry.Message != "Одинаковый знак препинания повторён подряд." || entry.Fatal ||
				entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
				t.Errorf("unexpected V026: %+v", entry)
			}
		}
		if count != 1 {
			t.Errorf("V026 count for %s = %d, want 1", source, count)
		}
	}
}

// v031ParserError добавляет исходную ошибку для согласованного unparsed либо
// media-type без Value. Связь элемента с parser-error задаётся во входе явно.
func v031ParserError(t *testing.T, result *model.Result, line, start, end int, message string) {
	t.Helper()
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: message,
		Location:         &model.Location{Start: model.Position{Line: line, Column: start}, End: model.Position{Line: line, Column: end}},
		RelatedLocations: []model.Location{},
	}}
}

// v031Registry создаёт реестр только через штатный каталог регистраций.
func v031Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v031Document добавляет обязательные версию и заголовок к явно заданному телу.
func v031Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v031Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v031Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v031Element(t, "version", "1.2", "1.2", 14, 17)),
		v031Line(t, "# Unit 1", "heading", 0, 0,
			v031Element(t, "heading-level", "#", "1", 1, 2), v031Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v031Model(t, append(lines, body...)...)
}

// v031Model задаёт физические номера, не выводя типы и структуру из Raw.
func v031Model(t *testing.T, lines ...model.Line) *model.Result {
	t.Helper()
	version, encoding, title := "1.2", "UTF-8", "Unit 1"
	count := len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      append([]model.Line{}, lines...), Diagnostics: []model.Diagnostic{},
	}
	for i := range result.Lines {
		result.Lines[i].Number = i + 1
	}
	return result
}

// v031Line сохраняет явно заданную структуру строки без разбора Raw.
func v031Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v031Element задаёт исходное и семантическое значения и диапазон [start,end).
func v031Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v031Text задаёт текст с одинаковыми Raw и Value; длина нужна только для
// входного элемента и не используется для вычисления ожидаемых срабатываний.
func v031Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v031Line(t, raw, "content", parent, depth,
		v031Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v031Open задаёт блочное объявление без параметров с согласованными элементами.
func v031Open(t *testing.T, spelling, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(spelling) + 2
	return v031Line(t, "@"+spelling+" {", "block-start", parent, depth,
		v031Element(t, "tag", "@"+spelling, tag, 1, end), v031Element(t, "block-open", "{", "{", end+1, end+2))
}

// v031Close задаёт закрытие с явно указанными родителем и глубиной.
func v031Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v031Line(t, "}", "block-end", parent, depth, v031Element(t, "block-close", "}", "}", 1, 2))
}

// v031Run проверяет V031 текущего запуска штатного конвейера: точное число,
// метаданные и диапазоны [start,end), неизменность исходной модели и истории.
func v031Run(t *testing.T, result *model.Result, processingID string, want [][3]int) {
	t.Helper()
	before := v031Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v031Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v031Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seen := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seen[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seen[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V031" && entry.Source != processingID {
			t.Fatalf("new V031 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V031" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V031 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[0], Column: expected[2]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Служебное обозначение содержит прописные буквы." ||
			entry.SeverityLevel != "warning" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d = %+v, location %+v; want %+v", i, entry, entry.Location, location)
		}
	}
	if len(result.Processing) != processingCount+1 || len(result.Diagnostics) < diagnosticCount {
		t.Fatal("unexpected processing or diagnostic history length")
	}
	var expected model.Result
	if err := json.Unmarshal([]byte(before), &expected); err != nil {
		t.Fatal(err)
	}
	v031OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v031Snapshot(t, &previous) != v031Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v031Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v031Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v031OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v031OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V031" {
			continue
		}
		if entry.SeverityLevel == "error" {
			expected.Document.HasErrors = true
			if entry.Location != nil {
				expected.Lines[entry.Location.Start.Line-1].HasErrors = true
			}
		}
		if entry.DiagnosticScope != "element" {
			continue
		}
		if entry.Location == nil {
			t.Fatal("element diagnostic has no location")
		}
		matched := false
		for i := range expected.Lines[entry.Location.Start.Line-1].Elements {
			element := &expected.Lines[entry.Location.Start.Line-1].Elements[i]
			if element.Start == entry.Location.Start.Column && element.End == entry.Location.End.Column {
				element.ErrorIDs = append(element.ErrorIDs, entry.ID)
				matched = true
				break
			}
		}
		if !matched {
			t.Fatal("element diagnostic does not identify an element")
		}
	}
	for i := range expected.Lines {
		for j := range expected.Lines[i].Elements {
			slices.SortStableFunc(expected.Lines[i].Elements[j].ErrorIDs, func(a, b string) int { return order[a] - order[b] })
		}
	}
}
