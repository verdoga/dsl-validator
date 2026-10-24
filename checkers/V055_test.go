package checkers

import (
	"encoding/json"
	"slices"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const (
	v055TestMessage = "Основной заголовок находится в недопустимом месте."
	v055TestPrior   = "Перед заголовком находится недопустимая конструкция"
	v055TestLate    = "Заголовок предшествует первому объявлению @dsl-version"
	v055TestBlock   = "Заголовок находится внутри фигурного блока; Перед заголовком находится недопустимая конструкция"
	v055TestAll     = "Заголовок находится внутри фигурного блока; Перед заголовком находится недопустимая конструкция; Заголовок предшествует первому объявлению @dsl-version"
)

// v055TestWant содержит явные ожидания, независимые от функций диагностики.
// Тройки — номер строки, первая и исключающая последняя Unicode-колонки.
type v055TestWant struct {
	location [3]int
	detail   string
	related  [][3]int
}

func TestV055Registration(t *testing.T) {
	registry := v055TestRegistry(t)
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V055" {
			continue
		}
		count++
		if definition.Severity != diagnostics.SeverityError || definition.Scope != diagnostics.ScopeLine ||
			definition.Message != v055TestMessage || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V055 definitions = %d, want 1", count)
	}
	for _, version := range []string{"", "1.1", "1.2.0", "1.3", "2.1.3"} {
		t.Run("unsupported "+version, func(t *testing.T) {
			result := v055TestDocument(t, "")
			result.Document.DSLVersion = &version
			before := v055TestSnapshot(t, result)
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil || v055TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

func TestV055ReferenceAndExample(t *testing.T) {
	// Оба примера уже содержат необходимый контекст. Исходные строки
	// сохранены целиком; дополнительные заголовки и теги не добавляются.
	t.Run("reference", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson",
			v055TestVersion(t), v055TestTag(t, "section", "Studentbook", "name"),
			v055TestHeading(t, "Lesson"), v055TestText(t, "Read carefully.", "Read carefully.", 3, 1))
		section := "Studentbook"
		result.Document.Metadata.Section = &section
		v055TestRun(t, result, "current", nil)
	})
	t.Run("example", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson",
			v055TestVersion(t), v055TestText(t, "Read carefully.", "Read carefully.", 0, 0),
			v055TestHeading(t, "Lesson"))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 16}}},
		})
	})
}

func TestV055AllowedBeforeHeading(t *testing.T) {
	for _, tc := range []struct {
		name   string
		prefix []model.Line
	}{
		{"version only", nil},
		{"empty and whitespace lines", []model.Line{
			v055TestLine(t, "", "blank", 0, 0), v055TestLine(t, " \t", "blank", 0, 0),
		}},
		{"single-line editor", []model.Line{v055TestTag(t, "editor", "Note.", "content")}},
		{"uppercase editor", []model.Line{
			v055TestLine(t, "@EDITOR Note.", "tag", 0, 0,
				v055TestElement(t, "tag", "@EDITOR", "editor", 1, 8),
				v055TestElement(t, "content", "Note.", "Note.", 9, 14)),
		}},
		{"editor block including both boundaries", []model.Line{
			v055TestOpen(t, "editor", 0, 0),
			v055TestText(t, "# Literal", "# Literal", 2, 1),
			v055TestText(t, "@dsl-version 9.9", "@dsl-version 9.9", 2, 1),
			v055TestClose(t, 2, 1),
		}},
		{"successive editor blocks", []model.Line{
			v055TestOpen(t, "editor", 0, 0), v055TestText(t, "First.", "First.", 2, 1), v055TestClose(t, 2, 1),
			v055TestOpen(t, "editor", 0, 0), v055TestText(t, "Second.", "Second.", 5, 1), v055TestClose(t, 5, 1),
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			lines := append([]model.Line{v055TestVersion(t)}, tc.prefix...)
			lines = append(lines, v055TestHeading(t, "Lesson"))
			v055TestRun(t, v055TestDocument(t, "Lesson", lines...), "current", nil)
		})
	}
	t.Run("all metadata and interspersed comments", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson",
			v055TestVersion(t), v055TestTag(t, "document-id", "lesson", "identifier"),
			v055TestTag(t, "section", "Studentbook", "name"), v055TestLine(t, "", "blank", 0, 0),
			v055TestTag(t, "editor", "Note.", "content"), v055TestTag(t, "order", "1", "number"),
			v055TestTag(t, "resource-dir", "assets", "resource-path"), v055TestHeading(t, "Lesson"))
		id, section, order := "lesson", "Studentbook", "1"
		result.Document.Metadata.DocumentID = &id
		result.Document.Metadata.Section = &section
		result.Document.Metadata.Order = &order
		result.Document.Metadata.ResourceDirs = []string{"assets"}
		v055TestRun(t, result, "current", nil)
	})
	t.Run("metadata use canonical values without changing raw", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestLine(t, "@SECTION Studentbook", "tag", 0, 0,
				v055TestElement(t, "tag", "@SECTION", "section", 1, 9),
				v055TestElement(t, "name", "Studentbook", "Studentbook", 10, 21)),
			v055TestHeading(t, "Lesson"))
		section := "Studentbook"
		result.Document.Metadata.Section = &section
		v055TestRun(t, result, "current", nil)
	})
}

func TestV055FirstObstacleAndCandidates(t *testing.T) {
	version := v055TestVersion(t)
	heading := v055TestHeading(t, "Lesson")
	for _, tc := range []struct {
		name  string
		title string
		lines []model.Line
		want  []v055TestWant
	}{
		{"first of several obstacles", "Lesson", []model.Line{
			version, v055TestText(t, "First.", "First.", 0, 0), v055TestText(t, "Second.", "Second.", 0, 0), heading,
		}, []v055TestWant{{[3]int{4, 1, 2}, v055TestPrior, [][3]int{{2, 1, 7}}}}},
		{"obstacle after editor closes", "Lesson", []model.Line{
			version, v055TestOpen(t, "editor", 0, 0), v055TestText(t, "# Literal", "# Literal", 2, 1),
			v055TestClose(t, 2, 1), v055TestText(t, "First.", "First.", 0, 0), heading,
		}, []v055TestWant{{[3]int{6, 1, 2}, v055TestPrior, [][3]int{{5, 1, 7}}}}},
		{"escaped heading is text obstacle", "Lesson", []model.Line{
			version, v055TestText(t, "\\# Literal", "# Literal", 0, 0), heading,
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 11}}}}},
		{"escaped metadata is text obstacle", "Lesson", []model.Line{
			version, v055TestText(t, "\\@section X", "@section X", 0, 0), heading,
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 12}}}}},
		{"nonbreaking space does not start a heading", "Lesson", []model.Line{
			version, v055TestText(t, "\u00a0# Literal", "\u00a0# Literal", 0, 0), heading,
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 11}}}}},
		{"Unicode and full source obstacle", "Урок", []model.Line{
			version, v055TestText(t, "  Я😀 \t", "Я😀", 0, 0),
			v055TestLine(t, "\t# Урок", "heading", 0, 0,
				v055TestElement(t, "heading-level", "#", "1", 2, 3),
				v055TestElement(t, "title", "Урок", "Урок", 4, 8)),
		}, []v055TestWant{{[3]int{3, 2, 3}, v055TestPrior, [][3]int{{2, 1, 7}}}}},
		{"repeated heading after correct first", "", []model.Line{
			version, heading, v055TestText(t, "Read carefully.", "Read carefully.", 2, 1), v055TestHeading(t, "Again"),
		}, nil},
		{"only first of two misplaced headings", "", []model.Line{
			version, v055TestText(t, "Read carefully.", "Read carefully.", 0, 0), heading, v055TestHeading(t, "Again"),
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 16}}}}},
		{"no candidate", "", []model.Line{version, v055TestTag(t, "editor", "Note.", "content")}, nil},
		{"only escaped candidate", "", []model.Line{
			version, v055TestText(t, "\\# Literal", "# Literal", 0, 0),
		}, nil},
		{"marker inside text is not a candidate", "", []model.Line{
			version, v055TestText(t, "Text # Lesson", "Text # Lesson", 0, 0),
		}, nil},
		{"literal backslash is not decoded before recognition", "", []model.Line{
			version, v055TestText(t, "\\\\# Literal", "\\# Literal", 0, 0),
		}, nil},
		{"fullwidth sign is not the marker", "", []model.Line{
			version, v055TestText(t, "＃ Lesson", "＃ Lesson", 0, 0),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v055TestRun(t, v055TestDocument(t, tc.title, tc.lines...), "current", tc.want)
		})
	}
}

func TestV055MalformedFirstCandidate(t *testing.T) {
	for _, tc := range []struct {
		raw         string
		column, end int
	}{
		{"#Lesson", 1, 2},
		{"#", 1, 2},
		{"#\tLesson", 1, 2},
		{"# \t", 1, 2},
		{"  \t#Lesson", 4, 5},
		{"#Урок", 1, 2},
		{"#😀", 1, 2},
	} {
		t.Run(strconv.Quote(tc.raw), func(t *testing.T) {
			t.Run("misplaced", func(t *testing.T) {
				result := v055TestDocument(t, "", v055TestVersion(t),
					v055TestText(t, "Read carefully.", "Read carefully.", 0, 0),
					v055TestInvalid(t, tc.raw, 0, 0))
				v055TestRun(t, result, "current", []v055TestWant{
					{[3]int{3, tc.column, tc.end}, v055TestPrior, [][3]int{{2, 1, 16}}},
				})
			})
			t.Run("correct position consumes first candidate", func(t *testing.T) {
				// Ошибка формы принадлежит другой диагностике. Поздний
				// корректный заголовок не становится новым кандидатом V055.
				result := v055TestDocument(t, "Lesson", v055TestVersion(t),
					v055TestInvalid(t, tc.raw, 0, 0), v055TestHeading(t, "Lesson"))
				v055TestRun(t, result, "current", nil)
			})
		})
	}
}

func TestV055HeadingMarkerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		raw, marker, level string
		titleStart, end    int
	}{
		{"## Before", "##", "2", 4, 10},
		{"### Before", "###", "3", 5, 11},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			previous := v055TestLine(t, tc.raw, "heading", 0, 0,
				v055TestElement(t, "heading-level", tc.marker, tc.level, 1, tc.titleStart-1),
				v055TestElement(t, "title", "Before", "Before", tc.titleStart, tc.end))
			result := v055TestDocument(t, "Lesson", v055TestVersion(t), previous, v055TestHeading(t, "Lesson"))
			if tc.level == "2" {
				subtitle := "Before"
				result.Document.Metadata.Subtitle = &subtitle
			}
			v055TestRun(t, result, "current", []v055TestWant{
				{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, tc.end}}},
			})
		})
	}
	t.Run("unsupported level is obstacle but not candidate", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestInvalid(t, "#### Before", 0, 0), v055TestHeading(t, "Lesson"))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 12}}},
		})
	})
	t.Run("separated second hash belongs to title", func(t *testing.T) {
		result := v055TestDocument(t, "# Lesson", v055TestVersion(t),
			v055TestText(t, "First.", "First.", 0, 0), v055TestHeading(t, "# Lesson"))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 7}}},
		})
	})
}

func TestV055BlockContexts(t *testing.T) {
	version := v055TestVersion(t)
	heading := v055TestHeading(t, "Lesson")
	for _, tc := range []struct {
		name  string
		title string
		lines []model.Line
		want  []v055TestWant
	}{
		{"fragment definition is not skipped", "Lesson", []model.Line{
			version, v055TestFragment(t), v055TestText(t, "Text.", "Text.", 2, 1), v055TestClose(t, 2, 1), heading,
		}, []v055TestWant{{[3]int{5, 1, 2}, v055TestPrior, [][3]int{{2, 1, 14}}}}},
		{"candidate inside text block", "", []model.Line{
			version, v055TestOpen(t, "text", 0, 0), v055TestInvalid(t, "# Lesson", 2, 1), v055TestClose(t, 2, 1),
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestBlock, [][3]int{{2, 1, 8}}}}},
		{"two levels of block parents", "", []model.Line{
			version, v055TestFragment(t), v055TestOpen(t, "text", 2, 1),
			v055TestInvalid(t, "# Lesson", 3, 2), v055TestClose(t, 3, 2), v055TestClose(t, 2, 1),
		}, []v055TestWant{{[3]int{4, 1, 2}, v055TestBlock, [][3]int{{2, 1, 14}}}}},
		{"multifill body is literal but declaration is obstacle", "Lesson", []model.Line{
			version, v055TestOpen(t, "multifill", 0, 0), v055TestText(t, "# Literal", "# Literal", 2, 1),
			v055TestClose(t, 2, 1), heading,
		}, []v055TestWant{{[3]int{5, 1, 2}, v055TestPrior, [][3]int{{2, 1, 13}}}}},
		{"only candidate is literal multifill content", "", []model.Line{
			version, v055TestOpen(t, "multifill", 0, 0), v055TestText(t, "# Literal", "# Literal", 2, 1),
			v055TestClose(t, 2, 1),
		}, nil},
		{"nested first candidate prevents checking later root heading", "Again", []model.Line{
			version, v055TestOpen(t, "text", 0, 0), v055TestInvalid(t, "# Lesson", 2, 1),
			v055TestClose(t, 2, 1), v055TestHeading(t, "Again"),
		}, []v055TestWant{{[3]int{3, 1, 2}, v055TestBlock, [][3]int{{2, 1, 8}}}}},
		{"all reasons with only first obstacle related", "", []model.Line{
			v055TestOpen(t, "text", 0, 0), v055TestInvalid(t, "# Lesson", 1, 1),
			v055TestClose(t, 1, 1), version,
		}, []v055TestWant{{[3]int{2, 1, 2}, v055TestAll, [][3]int{{1, 1, 8}}}}},
		{"closed block does not imply nesting", "Lesson", []model.Line{
			version, v055TestOpen(t, "text", 0, 0), v055TestText(t, "Text.", "Text.", 2, 1),
			v055TestClose(t, 2, 1), heading,
		}, []v055TestWant{{[3]int{5, 1, 2}, v055TestPrior, [][3]int{{2, 1, 8}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v055TestRun(t, v055TestDocument(t, tc.title, tc.lines...), "current", tc.want)
		})
	}
	t.Run("logical task parent does not imply a brace block", func(t *testing.T) {
		// DSL 2.3.6: task имеет логическую область без фигурных скобок.
		// Неразрешённый внутри неё заголовок сохраняется как invalid.
		result := v055TestDocument(t, "", v055TestVersion(t),
			v055TestTag(t, "task", "a", "identifier"), v055TestInvalid(t, "# Lesson", 2, 1),
			v055TestLine(t, "@question Q?", "tag", 2, 1,
				v055TestElement(t, "tag", "@question", "question", 1, 10),
				v055TestElement(t, "content", "Q?", "Q?", 11, 13)),
			v055TestLine(t, "@answer A", "tag", 2, 1,
				v055TestElement(t, "tag", "@answer", "answer", 1, 8),
				v055TestElement(t, "content", "A", "A", 9, 10)))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 8}}},
		})
	})
}

func TestV055VersionOrder(t *testing.T) {
	version := v055TestVersion(t)
	heading := v055TestHeading(t, "Lesson")
	for _, tc := range []struct {
		name  string
		lines []model.Line
		want  []v055TestWant
	}{
		{"heading before existing version", []model.Line{heading, version},
			[]v055TestWant{{[3]int{1, 1, 2}, v055TestLate, nil}}},
		{"later duplicate version belongs to another diagnostic", []model.Line{version, heading, version}, nil},
		{"late first version followed by duplicate", []model.Line{heading, version, version},
			[]v055TestWant{{[3]int{1, 1, 2}, v055TestLate, nil}}},
		{"text before heading and late version", []model.Line{
			v055TestText(t, "First.", "First.", 0, 0), heading, version,
		}, []v055TestWant{{[3]int{2, 1, 2},
			"Перед заголовком находится недопустимая конструкция; Заголовок предшествует первому объявлению @dsl-version",
			[][3]int{{1, 1, 7}}}}},
		{"literal version in editor does not hide late real declaration", []model.Line{
			v055TestOpen(t, "editor", 0, 0), v055TestText(t, "@dsl-version 1.2", "@dsl-version 1.2", 1, 1),
			v055TestClose(t, 1, 1), heading, version,
		}, []v055TestWant{{[3]int{4, 1, 2}, v055TestLate, nil}}},
		{"literal version in multifill does not hide late real declaration", []model.Line{
			v055TestOpen(t, "multifill", 0, 0), v055TestText(t, "@dsl-version 1.2", "@dsl-version 1.2", 1, 1),
			v055TestClose(t, 1, 1), heading, version,
		}, []v055TestWant{{[3]int{4, 1, 2},
			"Перед заголовком находится недопустимая конструкция; Заголовок предшествует первому объявлению @dsl-version",
			[][3]int{{1, 1, 13}}}}},
		{"escaped version is text", []model.Line{
			v055TestText(t, "\\@dsl-version 1.2", "@dsl-version 1.2", 0, 0), heading, version,
		}, []v055TestWant{{[3]int{2, 1, 2},
			"Перед заголовком находится недопустимая конструкция; Заголовок предшествует первому объявлению @dsl-version",
			[][3]int{{1, 1, 18}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v055TestRun(t, v055TestDocument(t, "Lesson", tc.lines...), "current", tc.want)
		})
	}
}

func TestV055EndingsAndHistory(t *testing.T) {
	for _, tc := range []struct {
		name string
		eol  model.LineEnding
		last model.LineEnding
	}{
		{"LF", "\n", "\n"},
		{"CRLF", "\r\n", "\r\n"},
		{"no final EOL", "\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v055TestDocument(t, "Lesson", v055TestVersion(t),
				v055TestText(t, "Read carefully.", "Read carefully.", 0, 0), v055TestHeading(t, "Lesson"))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[2].LineEnding = tc.last
			want := []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 16}}}}
			v055TestRun(t, result, "first", want)
			v055TestRun(t, result, "second", want)
			// Новый документ между запусками не наследует первое препятствие.
			v055TestRun(t, v055TestDocument(t, "Lesson", v055TestVersion(t), v055TestHeading(t, "Lesson")), "clean", nil)
		})
	}
}

func TestV055UnknownVersionStopsBeforeChecks(t *testing.T) {
	for _, tc := range []struct {
		name  string
		title string
		lines []model.Line
	}{
		{"empty document", "", nil},
		{"heading without version declaration", "Lesson", []model.Line{v055TestHeading(t, "Lesson")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v055TestDocument(t, tc.title, tc.lines...)
			result.Document.DSLVersion = nil
			before := v055TestSnapshot(t, result)
			if err := validator.CheckModel(result); err != nil {
				t.Fatalf("input integrity: %v", err)
			}
			checks, err := validator.Prepare(result, v055TestRegistry(t))
			if err == nil || checks != nil || len(result.Diagnostics) != 0 || v055TestSnapshot(t, result) != before {
				t.Fatalf("unknown version: checks=%v, err=%v, diagnostics=%+v", checks, err, result.Diagnostics)
			}
		})
	}
}

func TestV055OtherPrecedingConstructions(t *testing.T) {
	t.Run("header is an obstacle", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestTag(t, "header", "Vocabulary", "title"), v055TestHeading(t, "Lesson"),
			v055TestText(t, "Read.", "Read.", 3, 1))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 19}}},
		})
	})
	t.Run("include is an obstacle despite forward definition", func(t *testing.T) {
		// DSL 3.12.9 разрешает ссылку до определения. Само определение
		// помещено после заголовка, чтобы единственным препятствием был include.
		fragment := v055TestFragment(t)
		parent := 3
		fragment.ParentLine, fragment.NestingLevel = &parent, 1
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestTag(t, "include", "f", "identifier"), v055TestHeading(t, "Lesson"),
			fragment, v055TestText(t, "Read.", "Read.", 4, 2), v055TestClose(t, 4, 2))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 11}}},
		})
	})
	t.Run("newpage is an obstacle", func(t *testing.T) {
		// Шаг newpage имеет отображаемое содержимое по DSL 2.7.6.
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestLine(t, "@newpage", "tag", 0, 0,
				v055TestElement(t, "tag", "@newpage", "newpage", 1, 9)),
			v055TestText(t, "Read.", "Read.", 2, 1), v055TestHeading(t, "Lesson"))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{4, 1, 2}, v055TestPrior, [][3]int{{2, 1, 9}}},
		})
	})
	t.Run("invalid construction before heading", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestInvalid(t, "@unknown X", 0, 0), v055TestHeading(t, "Lesson"))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 11}}},
		})
	})
	t.Run("recognized metadata with malformed argument remains allowed", func(t *testing.T) {
		// Нечисловой order — самостоятельная ошибка; V055 проверяет место
		// заголовка и не переопределяет список разрешённых метатегов.
		unparsed := v055TestElement(t, "unparsed", "nope", "", 8, 12)
		unparsed.Value = nil
		result := v055TestDocument(t, "Lesson", v055TestVersion(t),
			v055TestLine(t, "@order nope", "tag", 0, 0,
				v055TestElement(t, "tag", "@order", "order", 1, 7), unparsed),
			v055TestHeading(t, "Lesson"))
		v055TestRun(t, result, "current", nil)
	})
}

func TestV055PartiallyParsedFirstCandidate(t *testing.T) {
	for _, misplaced := range []bool{false, true} {
		t.Run("misplaced="+strconv.FormatBool(misplaced), func(t *testing.T) {
			// Парсер сохранил маркер, а название после отсутствующего
			// разделителя осталось unparsed. Повторный заголовок не проверяется.
			unparsed := v055TestElement(t, "unparsed", "Lesson", "", 2, 8)
			unparsed.Value = nil
			candidate := v055TestLine(t, "#Lesson", "invalid", 0, 0,
				v055TestElement(t, "heading-level", "#", "1", 1, 2), unparsed)
			lines := []model.Line{v055TestVersion(t)}
			var want []v055TestWant
			if misplaced {
				lines = append(lines, v055TestText(t, "First.", "First.", 0, 0))
				want = []v055TestWant{{[3]int{3, 1, 2}, v055TestPrior, [][3]int{{2, 1, 7}}}}
			}
			lines = append(lines, candidate, v055TestHeading(t, "Again"))
			v055TestRun(t, v055TestDocument(t, "Again", lines...), "current", want)
		})
	}
}

func TestV055BlockAncestorThroughLogicalParents(t *testing.T) {
	// DSL 4.9: полный блок с двумя непустыми ветвями. Родителями кандидата
	// являются task и variant, а фигурный предок variants находится выше.
	result := v055TestDocument(t, "", v055TestVersion(t), v055TestOpen(t, "variants", 0, 0),
		v055TestLine(t, "@variant A", "tag", 2, 1,
			v055TestElement(t, "tag", "@variant", "variant", 1, 9),
			v055TestElement(t, "name", "A", "A", 10, 11)),
		v055TestLine(t, "@task a", "tag", 3, 2,
			v055TestElement(t, "tag", "@task", "task", 1, 6),
			v055TestElement(t, "identifier", "a", "a", 7, 8)),
		v055TestInvalid(t, " \t# Lesson", 4, 3),
		v055TestText(t, "Read.", "Read.", 4, 3),
		v055TestLine(t, "@variant B", "tag", 2, 1,
			v055TestElement(t, "tag", "@variant", "variant", 1, 9),
			v055TestElement(t, "name", "B", "B", 10, 11)),
		v055TestText(t, "Write.", "Write.", 7, 2), v055TestClose(t, 2, 1))
	v055TestRun(t, result, "current", []v055TestWant{
		{[3]int{5, 3, 4}, v055TestBlock, [][3]int{{2, 1, 12}}},
	})
}

func TestV055OnlyLiteralCandidatesAndLateVersion(t *testing.T) {
	t.Run("editor candidates are all ignored", func(t *testing.T) {
		result := v055TestDocument(t, "", v055TestVersion(t), v055TestOpen(t, "editor", 0, 0),
			v055TestText(t, "# Literal", "# Literal", 2, 1),
			v055TestText(t, "#MissingSeparator", "#MissingSeparator", 2, 1),
			v055TestClose(t, 2, 1))
		v055TestRun(t, result, "current", nil)
	})
	t.Run("uppercase literal block and indented heading before version", func(t *testing.T) {
		result := v055TestDocument(t, "Lesson",
			v055TestLine(t, "@EDITOR {", "block-start", 0, 0,
				v055TestElement(t, "tag", "@EDITOR", "editor", 1, 8),
				v055TestElement(t, "block-open", "{", "{", 9, 10)),
			v055TestText(t, "# Literal", "# Literal", 1, 1), v055TestClose(t, 1, 1),
			v055TestLine(t, " \t# Lesson", "heading", 0, 0,
				v055TestElement(t, "heading-level", "#", "1", 3, 4),
				v055TestElement(t, "title", "Lesson", "Lesson", 5, 11)),
			v055TestLine(t, "@DSL-VERSION 1.2", "tag", 0, 0,
				v055TestElement(t, "tag", "@DSL-VERSION", "dsl-version", 1, 13),
				v055TestElement(t, "version", "1.2", "1.2", 14, 17)))
		v055TestRun(t, result, "current", []v055TestWant{
			{[3]int{4, 3, 4}, v055TestLate, nil},
		})
	})
}

// v055TestRegistry использует полный штатный каталог без подмены регистраций.
func v055TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v055TestDocument сохраняет явно заданные элементы и родителей без разбора Raw.
// Для unparsed добавляются исходные ошибки с необходимыми обратными ссылками.
// Пустой title означает отсутствие однозначного извлечённого основного заголовка.
func v055TestDocument(t *testing.T, title string, input ...model.Line) *model.Result {
	t.Helper()
	version, count := "1.2", len(input)
	lines := append([]model.Line{}, input...)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	if title != "" {
		result.Document.Metadata.Title = &title
	}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		for j := range line.Elements {
			element := &line.Elements[j]
			element.ErrorIDs = append([]string{}, element.ErrorIDs...)
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: diagnostics.SeverityError,
				DiagnosticScope: diagnostics.ScopeElement, Message: "Ошибка разбора строки.",
				Location: &model.Location{
					Start: model.Position{Line: line.Number, Column: element.Start},
					End:   model.Position{Line: line.Number, Column: element.End},
				},
				RelatedLocations: []model.Location{},
			})
		}
	}
	return result
}

// v055TestLine принимает готовую классификацию строки и её логического родителя.
func v055TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v055TestElement задаёт исходный фрагмент, семантическое значение и координаты.
func v055TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v055TestVersion задаёт обязательный технический тег без добавления других строк.
func v055TestVersion(t *testing.T) model.Line {
	t.Helper()
	return v055TestTag(t, "dsl-version", "1.2", "version")
}

// v055TestTag строит только заданную корректную однострочную форму тега.
func v055TestTag(t *testing.T, tag, value string, kind model.ElementType) model.Line {
	t.Helper()
	return v055TestLine(t, "@"+tag+" "+value, "tag", 0, 0,
		v055TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v055TestElement(t, kind, value, value, len(tag)+3, len(tag)+3+utf8.RuneCountInString(value)))
}

// v055TestHeading задаёт корректный корневой заголовок первого уровня.
func v055TestHeading(t *testing.T, title string) model.Line {
	t.Helper()
	return v055TestLine(t, "# "+title, "heading", 0, 0,
		v055TestElement(t, "heading-level", "#", "1", 1, 2),
		v055TestElement(t, "title", title, title, 3, utf8.RuneCountInString(title)+3))
}

// v055TestText сохраняет Raw и независимо заданное текстовое значение.
func v055TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v055TestLine(t, raw, "content", parent, depth,
		v055TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v055TestInvalid задаёт сохранённую ошибочную строку с unparsed и null Value.
func v055TestInvalid(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	element := v055TestElement(t, "unparsed", raw, "", 1, utf8.RuneCountInString(raw)+1)
	element.Value = nil
	return v055TestLine(t, raw, "invalid", parent, depth, element)
}

// v055TestOpen задаёт открытие блока без параметров и с явным родителем.
func v055TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v055TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v055TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v055TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v055TestFragment задаёт определение с обязательным идентификатором.
func v055TestFragment(t *testing.T) model.Line {
	t.Helper()
	return v055TestLine(t, "@fragment f {", "block-start", 0, 0,
		v055TestElement(t, "tag", "@fragment", "fragment", 1, 10),
		v055TestElement(t, "identifier", "f", "f", 11, 12),
		v055TestElement(t, "block-open", "{", "{", 13, 14))
}

// v055TestClose задаёт закрытие с тем же родителем, что у содержимого блока.
func v055TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v055TestLine(t, "}", "block-end", parent, depth,
		v055TestElement(t, "block-close", "}", "}", 1, 2))
}

// v055TestRun выполняет штатный конвейер и сравнивает только новые V055.
// Ожидания заданы тестом; помощник не вызывает функции проверяемой диагностики.
func v055TestRun(t *testing.T, result *model.Result, processingID string, want []v055TestWant) {
	t.Helper()
	before := v055TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v055TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v055TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V055" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V055 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := v055TestLocation(expected.location)
		if entry.Location == nil || *entry.Location != location || entry.ID == "" ||
			entry.Source != processingID || entry.SeverityLevel != diagnostics.SeverityError ||
			entry.DiagnosticScope != diagnostics.ScopeLine || entry.Fatal ||
			entry.Message != v055TestMessage+"\nконкретизация: "+expected.detail {
			t.Fatalf("V055 occurrence %d = %+v, want %+v", i, entry, expected)
		}
		if entry.RelatedLocations == nil || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("V055 related count = %d, want %d", len(entry.RelatedLocations), len(expected.related))
		}
		for j, related := range expected.related {
			if entry.RelatedLocations[j] != v055TestLocation(related) {
				t.Fatalf("V055 related %d = %+v, want %v", j, entry.RelatedLocations[j], related)
			}
		}
		if !result.Document.HasErrors || !result.Lines[expected.location[0]-1].HasErrors {
			t.Fatal("V055 error flags are missing")
		}
		for _, line := range result.Lines {
			for _, element := range line.Elements {
				if slices.Contains(element.ErrorIDs, entry.ID) {
					t.Fatal("line-scoped V055 created an element error link")
				}
			}
		}
	}
	v055TestPreserved(t, before, result)
}

// v055TestLocation переводит явную тройку ожидания в тип координат модели.
func v055TestLocation(position [3]int) model.Location {
	return model.Location{
		Start: model.Position{Line: position[0], Column: position[1]},
		End:   model.Position{Line: position[0], Column: position[2]},
	}
}

// v055TestSnapshot сериализует модель для проверки неизменности исходных данных.
func v055TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v055TestPreserved проверяет исходник и историю, исключая новые результаты
// и производные флаги/ссылки. Их согласованность проверяется штатным CheckModel.
func v055TestPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v055TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 ||
		len(restored.Diagnostics) < len(original.Diagnostics) {
		t.Fatal("source or history length changed")
	}
	restored.Document.HasErrors = original.Document.HasErrors
	for i := range restored.Lines {
		if len(restored.Lines[i].Elements) != len(original.Lines[i].Elements) {
			t.Fatal("elements changed")
		}
		restored.Lines[i].HasErrors = original.Lines[i].HasErrors
		for j := range restored.Lines[i].Elements {
			restored.Lines[i].Elements[j].ErrorIDs = original.Lines[i].Elements[j].ErrorIDs
		}
	}
	restored.Processing = restored.Processing[:len(original.Processing)]
	restored.Diagnostics = restored.Diagnostics[:len(original.Diagnostics)]
	if v055TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}
