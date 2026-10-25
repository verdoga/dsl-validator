package checkers

import (
	"encoding/json"
	"sort"
	"strconv"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v155Expected задаёт исходные диапазоны [начало,конец) в Unicode-колонках.
type v155Expected struct {
	primary [4]int
	related [][4]int
}

func TestV155ReferenceAndExample(t *testing.T) {
	for _, tc := range []struct {
		name, second string
		want         []v155Expected
	}{
		{"reference", "They _____{live} there.", nil},
		{"example", "They _____ there.", []v155Expected{{[4]int{3, 1, 6, 2}, [][4]int{{5, 6, 5, 11}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Эталон и пример сохранены дословно. Только версия и основной
			// заголовок добавлены перед ними, как требует DSL 1.30.
			result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
				v155TestText(t, "She _____{lives} here.", "She _____{lives} here.", 3, 2),
				v155TestText(t, tc.second, tc.second, 3, 2), v155TestClose(t, 3, 2))
			v155TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV155ModesAndEscapes(t *testing.T) {
	first := []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}}}}
	for _, tc := range []struct {
		name, raw, value string
		want             []v155Expected
	}{
		{"ordinary then special", "_____ _____{A}", "_____ _____{A}", first},
		{"special then ordinary", "_____{A} _____", "_____{A} _____", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 10, 4, 15}}}}},
		{"all ordinary ranges", "_____ _____{A} _____", "_____ _____{A} _____", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}, {4, 16, 4, 21}}}}},
		{"adjacent alternatives", "_____{A}{B} _____", "_____{A}{B} _____", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 13, 4, 18}}}}},
		{"more than two alternatives", "_____ _____{A}{B}{C}{D}", "_____ _____{A}{B}{C}{D}", first},
		{"all special alternatives", "_____{A}{B}", "_____{A}{B}", nil},
		{"all ordinary", "_____ _____", "_____ _____", nil},
		{"no gaps", "Read this text.", "Read this text.", nil},
		{"escaped ordinary", `\_____ _____{A}`, "_____ _____{A}", nil},
		{"escaped special", `_____ \_____\{A\}`, "_____ _____{A}", nil},
		{"paired slash", `\\_____ _____{A}`, `\_____ _____{A}`, []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 3, 4, 8}}}}},
		{"odd slashes", `\\\_____ _____{A}`, `\_____ _____{A}`, nil},
		{"four slashes", `\\\\_____ _____{A}`, `\\_____ _____{A}`, []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 5, 4, 10}}}}},
		{"literal short run is excluded", `_____ _____{A} \__`, "_____ _____{A} __", first},
		{"literal long run is excluded", `_____ _____{A} \__________`, "_____ _____{A} __________", first},
		{"escaped brace after gap", `_____ \{A\} _____{B}`, "_____ {A} _____{B}", first},
		{"escaped opening in answer", `_____ _____{\{}`, "_____ _____{{}", first},
		{"escaped closing in answer", `_____ _____{\}}`, "_____ _____{}}", first},
		{"escaped slash in answer", `_____ _____{\\}`, `_____ _____{\}`, first},
		{"escape does not remove ordinary character", `_____ _____{\A}`, `_____ _____{\A}`, first},
		{"escape before space stays nonempty", `_____ _____{\ }`, `_____ _____{\ }`, first},
		{"escaped underscore in answer stays data", `_____ _____{\_}`, `_____ _____{\_}`, first},
		{"no rescan of decoded opening", `_____ _____{A}{\{B}`, "_____ _____{A}{{B}", first},
		{"no rescan of decoded closing", `_____ _____{A}{B\}}`, "_____ _____{A}{B}}", first},
		{"answer runs are not gaps", "_____ _____{_ ____ _____}", "_____ _____{_ ____ _____}", first},
		{"later answer runs are not gaps", "_____ _____{A}{_ ____ _____}", "_____ _____{A}{_ ____ _____}", first},
		{"only special containing ordinary spelling", "_____{_____}{A}", "_____{_____}{A}", nil},
		{"punctuation answer", "_____ _____{.}", "_____ _____{.}", first},
		{"punctuation later answer", "_____ _____{A}{; /}", "_____ _____{A}{; /}", first},
		{"Unicode letters and spaces", "_____ _____{ \tЁЖ\u00a0\u2003 }", "_____ _____{ \tЁЖ\u00a0\u2003 }", first},
		{"zero width space is not White_Space", "_____ _____{\u200b}", "_____ _____{\u200b}", first},
		{"Unicode source columns", "Я🙂 _____ _____{A}", "Я🙂 _____ _____{A}", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 4, 4, 9}}}}},
		{"edge spaces preserved in range", " \t_____ _____{A}\t ", "_____ _____{A}", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 3, 4, 8}}}}},
		{"touching placeholders", "_____{A}_____", "_____{A}_____", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 9, 4, 14}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
				v155TestText(t, tc.raw, tc.value, 3, 2), v155TestClose(t, 3, 2)), "current", tc.want)
		})
	}
}

func TestV155AmbiguitySuppressesWholeBlock(t *testing.T) {
	for _, tc := range []struct{ name, raw string }{
		{"single underscore", "_"}, {"two underscores", "__"}, {"four underscores", "____"},
		{"six underscores", "______"}, {"ten not two placeholders", "__________"},
		{"space before answer", "_____ {A}"}, {"tab before answer", "_____\t{A}"},
		{"Unicode gap before answer", "_____\u00a0\u2003{A}"},
		{"space between alternatives", "_____{A} {B}"}, {"tab between alternatives", "_____{A}\t{B}"},
		{"Unicode gap between alternatives", "_____{A}\u202f{B}"},
		{"empty first answer", "_____{}"}, {"empty second answer", "_____{A}{}"},
		{"empty third answer", "_____{A}{B}{}"},
		{"whitespace first answer", "_____{ \t\v\f\u0085\u00a0\u1680\u2000\u2001\u2002\u2003\u2004\u2005\u2006\u2007\u2008\u2009\u200a\u2028\u2029\u202f\u205f\u3000}"},
		{"whitespace second answer", "_____{A}{\t\u00a0\u2003}"},
		{"unclosed first answer", "_____{A"}, {"unclosed second answer", "_____{A}{B"},
		{"unclosed third answer", "_____{A}{B}{C"},
		{"inner opening first answer", "_____{A{B}"}, {"inner opening later answer", "_____{A}{B{C}"},
		{"escaped closing does not close", `_____{A\}`},
		{"paired slash leaves opening active", `_____{A\\{B}`},
	} {
		for _, before := range []bool{true, false} {
			t.Run(tc.name+" before="+strconv.FormatBool(before), func(t *testing.T) {
				mixed := v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2)
				broken := v155TestText(t, tc.raw, tc.raw, 3, 2)
				// Одиночный _ разрешён спецификацией 4.3.2 и остаётся content;
				// V155 подавляется по согласованному более строгому условию длины.
				// Остальной повреждённый синтаксис представлен unparsed:
				// его неизвестное семантическое значение не выдумывается.
				if tc.raw != "_" {
					broken.LineType = "invalid"
					broken.Elements[0].ElementType, broken.Elements[0].Value = "unparsed", nil
				}
				rows, badLine := []model.Line{mixed, broken}, 5
				if before {
					rows, badLine = []model.Line{broken, mixed}, 4
				}
				body := append([]model.Line{v155TestOpen(t, "multifill", 2, 1)}, rows...)
				result := v155TestDocument(t, append(body, v155TestClose(t, 3, 2))...)
				if tc.raw != "_" {
					v155TestError(t, result, badLine, 0, "Неоднозначный синтаксис пропуска или его ответа.")
				}
				v155TestRun(t, result, "current", nil)
			})
		}
	}
	t.Run("unclosed answer cannot continue on next physical line", func(t *testing.T) {
		result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "_____ _____{A", "_____ _____{A", 3, 2),
			v155TestText(t, "B} _____{C}", "B} _____{C}", 3, 2), v155TestClose(t, 3, 2))
		v155TestError(t, result, 4, -1, "Ответ не закрыт в пределах строки.")
		v155TestRun(t, result, "current", nil)
	})
}

func TestV155OwnContentAndModelRepresentations(t *testing.T) {
	t.Run("invalid instruction does not suppress body warning", func(t *testing.T) {
		result := v155TestDocument(t, v155TestBlock(t, "multifill", "content", "____", 2, 1),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2), v155TestClose(t, 3, 2))
		v155TestError(t, result, 3, -1, "Неверная длина пропуска в инструкции.")
		v155TestRun(t, result, "current", []v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}}}})
	})
	for _, tc := range []struct{ name, instruction, body string }{
		{"ordinary instruction", "_____", "_____{A}"},
		{"special instruction", "_____{A}", "_____"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v155TestDocument(t, v155TestBlock(t, "multifill", "content", tc.instruction, 2, 1),
				v155TestText(t, tc.body, tc.body, 3, 2), v155TestClose(t, 3, 2))
			if tc.name == "special instruction" {
				v155TestError(t, result, 3, -1, "Специальный пропуск вне предзаполненного содержимого.")
			}
			v155TestRun(t, result, "current", nil)
		})
	}
	t.Run("inline multifill has no prefilled content", func(t *testing.T) {
		result := v155TestDocument(t, v155TestTag(t, "multifill", "content", "_____ _____{A}", 2, 1))
		v155TestError(t, result, 3, -1, "Специальный пропуск вне предзаполненного содержимого.")
		v155TestRun(t, result, "current", nil)
	})
	t.Run("literal tag and heading lines still supply placeholders", func(t *testing.T) {
		v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "@note _____", "@note _____", 3, 2),
			v155TestText(t, "# _____{A}", "# _____{A}", 3, 2), v155TestClose(t, 3, 2)), "current",
			[]v155Expected{{[4]int{3, 1, 6, 2}, [][4]int{{4, 7, 4, 12}}}})
	})
	t.Run("answer tag is not a service answer", func(t *testing.T) {
		result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____", "_____", 3, 2),
			v155TestText(t, "@answer A", "@answer A", 3, 2), v155TestClose(t, 3, 2))
		v155TestError(t, result, 5, -1, "Тег @answer запрещён внутри multifill.")
		v155TestRun(t, result, "current", nil)
	})
	t.Run("editor contains only literal pseudo declarations", func(t *testing.T) {
		v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "editor", 2, 1),
			v155TestText(t, "@multifill {", "@multifill {", 3, 2),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2), v155TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("pseudo multifill does not create nested block", func(t *testing.T) {
		v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "@multifill _____", "@multifill _____", 3, 2),
			v155TestText(t, "_____{A}", "_____{A}", 3, 2), v155TestClose(t, 3, 2)), "current",
			[]v155Expected{{[4]int{3, 1, 6, 2}, [][4]int{{4, 12, 4, 17}}}})
	})
	for _, elements := range []bool{false, true} {
		t.Run("placeholder elements="+strconv.FormatBool(elements), func(t *testing.T) {
			line := v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2)
			if elements {
				gap := v155TestElement(t, "placeholder", "_____", "", 1, 6)
				gap.Value = nil
				line.Elements = []model.Element{gap, v155TestElement(t, "content", " ", " ", 6, 7),
					v155TestElement(t, "placeholder", "_____{A}", "A", 7, 15)}
			}
			v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1), line, v155TestClose(t, 3, 2)), "current",
				[]v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}}}})
		})
	}
}

func TestV155ContextsAndIndependentBlocks(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v155Expected
	}{
		{"task", []model.Line{v155TestTag(t, "task", "identifier", "first", 2, 1),
			v155TestOpen(t, "multifill", 3, 2), v155TestText(t, "_____ _____{A}", "_____ _____{A}", 4, 3),
			v155TestClose(t, 4, 3), v155TestTag(t, "endtask", "", "", 2, 1)},
			[]v155Expected{{[4]int{4, 1, 6, 2}, [][4]int{{5, 1, 5, 6}}}}},
		{"internal step", []model.Line{v155TestTag(t, "task", "identifier", "first", 2, 1),
			v155TestTag(t, "step", "title", "First", 3, 2), v155TestOpen(t, "multifill", 4, 3),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 5, 4), v155TestClose(t, 5, 4),
			v155TestTag(t, "endtask", "", "", 2, 1)},
			[]v155Expected{{[4]int{5, 1, 7, 2}, [][4]int{{6, 1, 6, 6}}}}},
		{"deep branch task and step", []model.Line{v155TestOpen(t, "variants", 2, 1),
			v155TestTag(t, "variant", "name", "A", 3, 2), v155TestTag(t, "task", "identifier", "first", 4, 3),
			v155TestTag(t, "step", "title", "First", 5, 4), v155TestOpen(t, "multifill", 6, 5),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 7, 6), v155TestClose(t, 7, 6),
			v155TestTag(t, "endtask", "", "", 4, 3), v155TestTag(t, "variant", "name", "B", 3, 2),
			v155TestOpen(t, "multifill", 11, 3), v155TestText(t, "_____ _____{B}", "_____ _____{B}", 12, 4),
			v155TestClose(t, 12, 4), v155TestClose(t, 3, 2)},
			[]v155Expected{{[4]int{7, 1, 9, 2}, [][4]int{{8, 1, 8, 6}}}, {[4]int{12, 1, 14, 2}, [][4]int{{13, 1, 13, 6}}}}},
		{"adjacent mixed blocks", []model.Line{v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2), v155TestClose(t, 3, 2),
			v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____ _____{B}", "_____ _____{B}", 6, 2), v155TestClose(t, 6, 2)},
			[]v155Expected{{[4]int{3, 1, 5, 2}, [][4]int{{4, 1, 4, 6}}}, {[4]int{6, 1, 8, 2}, [][4]int{{7, 1, 7, 6}}}}},
		{"ordinary and special in separate blocks", []model.Line{v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "_____", "_____", 3, 2), v155TestClose(t, 3, 2),
			v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____{A}", "_____{A}", 6, 2), v155TestClose(t, 6, 2)}, nil},
		{"text after close is outside", []model.Line{v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "_____{A}", "_____{A}", 3, 2), v155TestClose(t, 3, 2), v155TestText(t, "_____", "_____", 2, 1)}, nil},
		{"text before opening is outside", []model.Line{v155TestText(t, "_____", "_____", 2, 1),
			v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____{A}", "_____{A}", 4, 2), v155TestClose(t, 4, 2)}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v155TestRun(t, v155TestDocument(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("ambiguity belongs to its block only", func(t *testing.T) {
		result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
			v155TestText(t, "_____ _____{A} ____", "_____ _____{A} ____", 3, 2), v155TestClose(t, 3, 2),
			v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____ _____{B}", "_____ _____{B}", 6, 2), v155TestClose(t, 6, 2))
		v155TestError(t, result, 4, -1, "Неверная длина пропуска.")
		v155TestRun(t, result, "current", []v155Expected{{[4]int{6, 1, 8, 2}, [][4]int{{7, 1, 7, 6}}}})
	})
}

func TestV155BoundariesAndRepeatedRuns(t *testing.T) {
	for _, ending := range []string{"LF", "CRLF", "no final newline"} {
		t.Run(ending, func(t *testing.T) {
			result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
				v155TestText(t, "_____", "_____", 3, 2), v155TestBlank(t, " \t", 3, 2),
				v155TestText(t, "_____{A}", "_____{A}", 3, 2), v155TestText(t, "_____", "_____", 3, 2), v155TestClose(t, 3, 2))
			for index := range result.Lines {
				if ending == "LF" {
					result.Lines[index].LineEnding = "\n"
				} else if ending == "CRLF" {
					result.Lines[index].LineEnding = "\r\n"
				}
			}
			want := []v155Expected{{[4]int{3, 1, 8, 2}, [][4]int{{4, 1, 4, 6}, {7, 1, 7, 6}}}}
			v155TestRun(t, result, "first", want)
			v155TestRun(t, result, "second", want)
			v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
				v155TestText(t, "_____{A}{B}", "_____{A}{B}", 3, 2), v155TestClose(t, 3, 2)), "separate", nil)
		})
	}
	t.Run("empty block", func(t *testing.T) {
		result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1), v155TestClose(t, 3, 2))
		v155TestRun(t, result, "current", nil)
	})
	t.Run("blank body", func(t *testing.T) {
		v155TestRun(t, v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
			v155TestBlank(t, "", 3, 2), v155TestBlank(t, "\t ", 3, 2), v155TestClose(t, 3, 2)), "current", nil)
	})
	t.Run("unclosed block uses last body end", func(t *testing.T) {
		result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1), v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2))
		v155TestError(t, result, 3, -1, "Блок не закрыт.")
		v155TestRun(t, result, "current", []v155Expected{{[4]int{3, 1, 4, 15}, [][4]int{{4, 1, 4, 6}}}})
	})
	t.Run("indentation and mixed case declaration", func(t *testing.T) {
		v155TestRun(t, v155TestDocument(t,
			v155TestLine(t, " \t@MuLtIfIlL {", "block-start", 2, 1,
				v155TestElement(t, "tag", "@MuLtIfIlL", "multifill", 3, 13), v155TestElement(t, "block-open", "{", "{", 14, 15)),
			v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2),
			v155TestLine(t, " \t}  ", "block-end", 3, 2, v155TestElement(t, "block-close", "}", "}", 3, 4))), "current",
			[]v155Expected{{[4]int{3, 3, 5, 4}, [][4]int{{4, 1, 4, 6}}}})
	})
}

func TestV155EmptyAndRegistration(t *testing.T) {
	for _, known := range []bool{false, true} {
		t.Run("empty known count="+strconv.FormatBool(known), func(t *testing.T) {
			version, count := "1.2", 0
			result := &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version,
				Metadata: model.DocumentMetadata{ResourceDirs: []string{}}}, Processing: []model.Processing{}, Lines: []model.Line{}, Diagnostics: []model.Diagnostic{}}
			if known {
				result.Document.LineCount = &count
			}
			v155TestRun(t, result, "empty", nil)
		})
	}
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	definitions, err := registry.ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V155" {
			continue
		}
		count++
		if definition.Severity != "warning" || definition.Scope != "block" || definition.Check == nil ||
			definition.Message != "В @multifill смешаны пропуски с ответами и без ответов." ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V155 definitions = %d, want 1", count)
	}
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v155TestDocument(t, v155TestOpen(t, "multifill", 2, 1),
				v155TestText(t, "_____ _____{A}", "_____ _____{A}", 3, 2), v155TestClose(t, 3, 2))
			result.Document.DSLVersion = &version
			result.Lines[0] = v155TestTag(t, "dsl-version", "version", version, 0, 0)
			result.Lines[0].Number = 1
			checks, err := validator.Prepare(result, registry)
			if err == nil || checks != nil {
				t.Fatalf("unsupported version: checks=%v err=%v", checks, err)
			}
		})
	}
}

// v155TestBlock задаёт параметр объявления перед открывающей скобкой.
func v155TestBlock(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	line := v155TestTag(t, tag, kind, value, parent, depth)
	start := utf8.RuneCountInString(line.Raw) + 2
	line.Raw += " {"
	line.LineType = "block-start"
	line.Elements = append(line.Elements, v155TestElement(t, "block-open", "{", "{", start, start+1))
	return line
}

// v155TestRun запускает штатный конвейер и проверяет только V155 текущего запуска.
func v155TestRun(t *testing.T, result *model.Result, source string, want []v155Expected) {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	before, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	checks, err := validator.Prepare(result, registry)
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	after, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Prepare changed input")
	}
	result.Processing = append(result.Processing, model.Processing{ID: source, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, source); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V155" && diagnostic.Source == source {
			got = append(got, diagnostic)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V155 count = %d, want %d: %+v", len(got), len(want), got)
	}
	for index, expected := range want {
		entry := got[index]
		primary := model.Location{Start: model.Position{Line: expected.primary[0], Column: expected.primary[1]}, End: model.Position{Line: expected.primary[2], Column: expected.primary[3]}}
		if entry.ID == "" || entry.Fatal || entry.SeverityLevel != "warning" || entry.DiagnosticScope != "block" ||
			entry.Message != "В @multifill смешаны пропуски с ответами и без ответов.\nконкретизация: Весь блок будет единым многострочным полем без автоматической проверки. Проверьте полноту служебных ответов; отсутствующие ответы нельзя придумывать" ||
			entry.Location == nil || *entry.Location != primary || len(entry.RelatedLocations) != len(expected.related) {
			t.Fatalf("occurrence %d = %+v, want %+v", index, entry, expected)
		}
		for relatedIndex, span := range expected.related {
			location := model.Location{Start: model.Position{Line: span[0], Column: span[1]}, End: model.Position{Line: span[2], Column: span[3]}}
			if entry.RelatedLocations[relatedIndex] != location {
				t.Fatalf("related %d = %+v, want %+v", relatedIndex, entry.RelatedLocations[relatedIndex], location)
			}
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	var original model.Result
	if err := json.Unmarshal(before, &original); err != nil {
		t.Fatal(err)
	}
	for index, line := range original.Lines {
		current := result.Lines[index]
		current.HasErrors = line.HasErrors
		current.Elements = append([]model.Element{}, current.Elements...)
		for element := range current.Elements {
			current.Elements[element].ErrorIDs = line.Elements[element].ErrorIDs
		}
		left, _ := json.Marshal(line)
		right, _ := json.Marshal(current)
		if string(left) != string(right) {
			t.Fatalf("source changed on line %d", index+1)
		}
	}
	for index, old := range original.Diagnostics {
		left, _ := json.Marshal(old)
		right, _ := json.Marshal(result.Diagnostics[index])
		if string(left) != string(right) {
			t.Fatal("diagnostic history changed")
		}
	}
}

// v155TestError добавляет исходную ошибку: index=-1 для строки, иначе для элемента.
func v155TestError(t *testing.T, result *model.Result, number, index int, message string) {
	t.Helper()
	line := &result.Lines[number-1]
	id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
	scope, start, end := diagnostics.ScopeLine, 1, utf8.RuneCountInString(line.Raw)+1
	if index >= 0 {
		scope = diagnostics.ScopeElement
		element := &line.Elements[index]
		start, end = element.Start, element.End
		element.ErrorIDs = append(element.ErrorIDs, id)
	}
	line.HasErrors, result.Document.HasErrors = true, true
	result.Diagnostics = append(result.Diagnostics, model.Diagnostic{ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		DiagnosticScope: scope, Message: message, RelatedLocations: []model.Location{},
		Location: &model.Location{Start: model.Position{Line: number, Column: start}, End: model.Position{Line: number, Column: end}}})
	sort.SliceStable(result.Diagnostics, func(i, j int) bool {
		left, right := result.Diagnostics[i].Location.Start, result.Diagnostics[j].Location.Start
		return left.Line < right.Line || left.Line == right.Line && left.Column < right.Column
	})
}

// v155TestElement задаёт исходное и семантическое значения без преобразований.
func v155TestElement(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v155TestLine сохраняет явные тип, родителя, глубину и элементы строки.
func v155TestLine(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v155TestTag создаёт однострочное объявление с явно заданным параметром.
func v155TestTag(t *testing.T, tag string, kind model.ElementType, value string, parent, depth int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v155TestElement(t, "tag", raw, tag, 1, len(raw)+1)}
	if kind != "" {
		elements = append(elements, v155TestElement(t, kind, value, value, len(raw)+2, len(raw)+2+utf8.RuneCountInString(value)))
		raw += " " + value
	}
	return v155TestLine(t, raw, "tag", parent, depth, elements...)
}

// v155TestOpen создаёт объявление блока без параметров.
func v155TestOpen(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v155TestLine(t, "@"+tag+" {", "block-start", parent, depth,
		v155TestElement(t, "tag", "@"+tag, tag, 1, len(tag)+2), v155TestElement(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v155TestClose создаёт закрытие с заданным родителем.
func v155TestClose(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v155TestLine(t, "}", "block-end", parent, depth, v155TestElement(t, "block-close", "}", "}", 1, 2))
}

// v155TestText сохраняет раздельно заданные Raw и обработанное парсером Value.
func v155TestText(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v155TestLine(t, raw, "content", parent, depth, v155TestElement(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v155TestBlank задаёт обычную пустую строку без текстового элемента.
func v155TestBlank(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v155TestLine(t, raw, "blank", parent, depth)
}

// v155TestDocument добавляет обязательные версию и основной заголовок по DSL 1.30.
// Родители тела задаются тестами явно, с учётом двух вводных строк.
func v155TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title := "1.2", "Lesson"
	lines := append([]model.Line{v155TestTag(t, "dsl-version", "version", "1.2", 0, 0),
		v155TestLine(t, "# Lesson", "heading", 0, 0, v155TestElement(t, "heading-level", "#", "1", 1, 2), v155TestElement(t, "title", "Lesson", "Lesson", 3, 9))}, body...)
	for index := range lines {
		lines[index].Number = index + 1
	}
	lines[len(lines)-1].LineEnding = ""
	count := len(lines)
	return &model.Result{Format: "1.0", Document: model.Document{DSLVersion: &version, LineCount: &count,
		Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}}, Lines: lines, Diagnostics: []model.Diagnostic{}}
}
