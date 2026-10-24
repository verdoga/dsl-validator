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

func TestV021Registration(t *testing.T) {
	definitions, err := v021Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V021" {
			continue
		}
		matches++
		if definition.Severity != "recommendation" || definition.Scope != "block" ||
			definition.Message != "Строка без завершающего знака продолжается строкой со строчной буквы." || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V021 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V021 definitions = %d, want 1", matches)
	}
}

func TestV021ReferenceAndExample(t *testing.T) {
	// Фрагменты таблицы сохранены буквально; <br> — символы DSL, а не
	// HTML-разметка таблицы. Добавлены только обязательные версия и заголовок.
	v021Run(t, v021Block(t, "text",
		v021Text(t, "Это законченное предложение.", 3, 2),
		v021Text(t, "Следующее предложение.", 3, 2)), "current", nil)
	v021Run(t, v021Block(t, "text",
		v021Text(t, "Это начало", 3, 2), v021Text(t, "<br>", 3, 2),
		v021Text(t, "одного предложения.", 3, 2)), "current", [][4]int{{4, 1, 6, 20}})
}

func TestV021TargetBlocks(t *testing.T) {
	for _, tag := range []string{"multifill", "text", "key", "note", "alt", "script", "instr"} {
		t.Run(tag, func(t *testing.T) {
			v021Run(t, v021Block(t, tag, v021Text(t, "Начало", 3, 2), v021Text(t, "далее", 3, 2)),
				"current", [][4]int{{4, 1, 5, 6}})
		})
	}
	for _, tag := range []string{"editor", "example", "hint", "table"} {
		t.Run("excluded "+tag, func(t *testing.T) {
			v021Run(t, v021Block(t, tag, v021Text(t, "Начало", 3, 2), v021Text(t, "далее", 3, 2)), "current", nil)
		})
	}
}

func TestV021PairConditions(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][4]int
	}{
		{"ordinary pair", "Начало", "далее", [][4]int{{4, 1, 5, 6}}},
		{"second ends with period", "Начало", "далее.", [][4]int{{4, 1, 5, 7}}},
		{"period", "Готово.", "далее", nil},
		{"question", "Готово?", "далее", nil},
		{"exclamation", "Готово!", "далее", nil},
		{"ellipsis", "Готово…", "далее", nil},
		{"three dots", "Готово...", "далее", nil},
		{"colon terminates line", "Начало:", "далее", nil},
		{"semicolon is not terminal", "Начало;", "далее", [][4]int{{4, 1, 5, 6}}},
		{"hyphen", "Начало-", "далее", nil},
		{"en dash", "Начало –", "далее", nil},
		{"em dash", "Начало —", "далее", nil},
		{"nonbreaking hyphen", "Начало‑", "далее", nil},
		{"Unicode Pd", "Начало⸺", "далее", nil},
		{"minus is not Pd", "Начало −", "далее", [][4]int{{4, 1, 5, 6}}},
		{"closing guillemet", "«Готово.»", "далее", nil},
		{"closing bracket", "[Готово.]", "далее", nil},
		{"several closing signs", "«[Готово.]»", "далее", nil},
		{"double quotes", `"Готово."`, "далее", nil},
		{"single quotes", "'Готово.'", "далее", nil},
		{"quoted dash", "«Начало—»", "далее", nil},
		{"opening guillemet", "Начало", "«далее»", [][4]int{{4, 1, 5, 8}}},
		{"opening bracket", "Начало", "[далее]", [][4]int{{4, 1, 5, 8}}},
		{"opening double quote", "Начало", `"далее"`, [][4]int{{4, 1, 5, 8}}},
		{"opening single quote", "Начало", "'далее'", [][4]int{{4, 1, 5, 8}}},
		{"Unicode Pi and Pf", "Начало", "‘далее’", [][4]int{{4, 1, 5, 8}}},
		{"Unicode Ps and Pe", "Начало", "（далее）", [][4]int{{4, 1, 5, 8}}},
		{"nested opening signs", "Начало", "«[далее]»", [][4]int{{4, 1, 5, 10}}},
		{"capital", "Начало", "Далее", nil},
		{"number", "Начало", "1 далее", nil},
		{"titlecase is not Ll", "Начало", "ǅалее", nil},
		{"uncased letter is not Ll", "Начало", "中", nil},
		{"non ASCII lower", "Начало", "é", [][4]int{{4, 1, 5, 2}}},
		{"uncased first letter", "中", "далее", [][4]int{{4, 1, 5, 6}}},
		{"no first letter", "123", "далее", nil},
		{"marks are not letters", "\u0301", "далее", nil},
		{"leading combining mark", "Начало", "\u0301далее", nil},
		{"combining mark after letter", "Начало", "е\u0301", [][4]int{{4, 1, 5, 3}}},
		{"Unicode columns with emoji", "Начало", "а🙂", [][4]int{{4, 1, 5, 3}}},
		{"leading emoji", "Начало", "🙂далее", nil},
		{"NBSP is not trimmed", "Начало", "\u00a0далее", nil},
		{"narrow NBSP is not trimmed", "Начало", "\u202fдалее", nil},
		{"NBSP after period remains", "Готово.\u00a0", "далее", [][4]int{{4, 1, 5, 6}}},
		{"internal space after opening quote", "Начало", "« далее»", nil},
		{"internal space before closing quote", "«Готово. »", "далее", [][4]int{{4, 1, 5, 6}}},
		{"only quotes in second line", "Начало", `""`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, "text", v021Text(t, tc.first, 3, 2), v021Text(t, tc.second, 3, 2)), "current", tc.want)
		})
	}
}

func TestV021HTML(t *testing.T) {
	for _, tc := range []struct {
		name, first, second string
		want                [][4]int
	}{
		{"em", "Начало", "<em>далее</em>", [][4]int{{4, 1, 5, 15}}},
		{"strong", "Начало", "<strong>далее</strong>", [][4]int{{4, 1, 5, 23}}},
		{"u", "Начало", "<u>далее</u>", [][4]int{{4, 1, 5, 13}}},
		{"mixed case", "Начало", "<Em>далее</eM>", [][4]int{{4, 1, 5, 15}}},
		{"nested HTML", "Начало", "<em><u>далее</u></em>", [][4]int{{4, 1, 5, 22}}},
		{"markup around terminal punctuation", "<strong>Готово.</strong>", "далее", nil},
		{"markup around dash", "<em>Начало—</em>", "далее", nil},
		{"letters inside markup counted", "<u>Начало</u>", "далее", [][4]int{{4, 1, 5, 6}}},
		{"marker name is not visible letter", "<em>123</em>", "далее", nil},
		{"transparent empty marker in middle", "Готово.<u></u>", "далее", nil},
		{"unknown tag is literal", "Начало", "<foo>далее</foo>", nil},
		{"attributes are not transparent", "Начало", "<em x=y>далее</em>", nil},
		{"inline br not transparent", "Начало", "<br>далее", nil},
		{"entity not decoded", "Начало", "&#1072;", nil},
		{"unpaired supported marker transparent", "Начало", "<em>далее", [][4]int{{4, 1, 5, 10}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, "text", v021Text(t, tc.first, 3, 2), v021Text(t, tc.second, 3, 2)), "current", tc.want)
		})
	}
}

func TestV021Sequences(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"one line", []model.Line{v021Text(t, "Начало", 3, 2)}, nil},
		{"overlapping pairs", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "далее", 3, 2),
			v021Text(t, "ещё", 3, 2)}, [][4]int{{4, 1, 5, 6}, {5, 1, 6, 4}}},
		{"restart after terminal line", []model.Line{v021Text(t, "Готово.", 3, 2), v021Text(t, "далее", 3, 2),
			v021Text(t, "ещё", 3, 2)}, [][4]int{{5, 1, 6, 4}}},
		{"one blank", []model.Line{v021Text(t, "Начало", 3, 2), v021Line(t, "", "blank", 3, 2),
			v021Text(t, "далее", 3, 2)}, [][4]int{{4, 1, 6, 6}}},
		{"two blanks", []model.Line{v021Text(t, "Начало", 3, 2), v021Line(t, "", "blank", 3, 2),
			v021Line(t, " \t", "blank", 3, 2), v021Text(t, "далее", 3, 2)}, [][4]int{{4, 1, 7, 6}}},
		{"leading and trailing blanks", []model.Line{v021Line(t, "", "blank", 3, 2), v021Text(t, "Начало", 3, 2),
			v021Text(t, "далее", 3, 2), v021Line(t, "", "blank", 3, 2)}, [][4]int{{5, 1, 6, 6}}},
		{"br and blank", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "<BR>", 3, 2),
			v021Line(t, "", "blank", 3, 2), v021Text(t, "<br>", 3, 2), v021Text(t, "далее", 3, 2)}, [][4]int{{4, 1, 8, 6}}},
		{"numeric line is not skipped", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "123", 3, 2),
			v021Text(t, "далее", 3, 2)}, nil},
		{"empty visible markup is not blank", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "<em></em>", 3, 2),
			v021Text(t, "далее", 3, 2)}, nil},
		// По шагам 1–3 неизвестная форма в content не пропускается, но её
		// буквальное содержимое может стать первой строкой следующей пары.
		{"self closing br as literal content", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "<br/>", 3, 2),
			v021Text(t, "далее", 3, 2)}, [][4]int{{5, 1, 6, 6}}},
		{"spaced br as literal content", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "<br />", 3, 2),
			v021Text(t, "далее", 3, 2)}, [][4]int{{5, 1, 6, 6}}},
		{"inline br as literal content", []model.Line{v021Text(t, "Начало", 3, 2), v021Text(t, "x<br>", 3, 2),
			v021Text(t, "далее", 3, 2)}, [][4]int{{4, 1, 5, 6}, {5, 1, 6, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, "text", tc.body...), "current", tc.want)
		})
	}
}

func TestV021RawAndEscaping(t *testing.T) {
	for _, tc := range []struct {
		name, tag, firstRaw, firstValue, secondRaw, secondValue string
		want                                                    [][4]int
	}{
		{"original indentation", "text", "\tНачало  ", "Начало", "  далее\t", "далее", [][4]int{{4, 1, 5, 9}}},
		{"trimmed terminal punctuation", "text", " Готово.\t", "Готово.", "далее", "далее", nil},
		{"escaped braces", "text", "Начало", "Начало", `\{далее\}`, "{далее}", [][4]int{{4, 1, 5, 10}}},
		{"escaped closing brace after period", "text", `\{Готово.\}`, "{Готово.}", "далее", "далее", nil},
		{"escaped HTML opening is literal", "text", "Начало", "Начало", `\<em>далее`, "<em>далее", nil},
		{"escaped HTML closing is literal", "text", "Начало", "Начало", `<em\>далее`, "<em>далее", nil},
		{"double backslash then real marker", "text", `Готово.\\<em>x</em>`, `Готово.\<em>x</em>`, "далее", "далее", [][4]int{{4, 1, 5, 6}}},
		{"escaped backslash is visible", "text", "Начало", "Начало", `\\<em>далее</em>`, `\<em>далее</em>`, nil},
		{"ordinary slash is not removed", "text", "Начало", "Начало", `\далее`, `\далее`, nil},
		{"quotes are not escaped in content", "text", "Начало", "Начало", `\"далее"`, `\"далее"`, nil},
		{"literal opening tag at line start", "text", "Начало", "Начало", `\@note далее`, "@note далее", nil},
		{"literal heading at line start", "text", "Начало", "Начало", `\# далее`, "# далее", nil},
		{"multifill answer tag escaped", "multifill", "Начало", "Начало", `\@answer далее`, "@answer далее", nil},
		{"multifill other tag is literal", "multifill", "Начало", "Начало", "@text далее", "@text далее", nil},
		{"key underscores do not escape", "key", "Начало", "Начало", `\_____ далее`, `\_____ далее`, nil},
		{"escaped underline sequence", "multifill", "Начало", "Начало", `\_____ далее`, "_____ далее", nil},
		{"literal special placeholder", "multifill", `\_____\{Готово.\}`, "_____{Готово.}", "далее", "далее", nil},
		{"HTML outside text", "note", "Начало", "Начало", "<em>далее</em>", "<em>далее</em>", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, tc.tag,
				v021Content(t, tc.firstRaw, tc.firstValue, 3, 2),
				v021Content(t, tc.secondRaw, tc.secondValue, 3, 2)), "current", tc.want)
		})
	}
	t.Run("indented br", func(t *testing.T) {
		v021Run(t, v021Block(t, "text", v021Text(t, "Начало", 3, 2),
			v021Content(t, " \t<Br> \t", "<Br>", 3, 2), v021Text(t, "далее", 3, 2)), "current", [][4]int{{4, 1, 6, 6}})
	})
	t.Run("br outside text not skipped", func(t *testing.T) {
		v021Run(t, v021Block(t, "note", v021Text(t, "Начало", 3, 2), v021Text(t, "<br>", 3, 2),
			v021Text(t, "далее", 3, 2)), "current", [][4]int{{5, 1, 6, 6}})
	})
}

func TestV021Placeholders(t *testing.T) {
	for _, tc := range []struct {
		name          string
		first, second model.Line
		want          [][4]int
	}{
		{"answer period is hidden", v021Line(t, "Начало _____{ответ.}", "content", 3, 2,
			v021Element(t, "content", "Начало ", "Начало ", 1, 8), v021Element(t, "placeholder", "_____{ответ.}", "ответ.", 8, 21)),
			v021Text(t, "далее", 3, 2), [][4]int{{4, 1, 5, 6}}},
		{"only hidden letters", v021Line(t, "_____{ответ}", "content", 3, 2,
			v021Element(t, "placeholder", "_____{ответ}", "ответ", 1, 13)), v021Text(t, "далее", 3, 2), nil},
		{"leading placeholder is not a letter", v021Text(t, "Начало", 3, 2),
			v021Line(t, "_____{ответ} далее", "content", 3, 2, v021Element(t, "placeholder", "_____{ответ}", "ответ", 1, 13),
				v021Element(t, "content", " далее", " далее", 13, 19)), nil},
		{"visible terminal punctuation", v021Line(t, "Начало _____{ответ}.", "content", 3, 2,
			v021Element(t, "content", "Начало ", "Начало ", 1, 8), v021Element(t, "placeholder", "_____{ответ}", "ответ", 8, 20),
			v021Element(t, "content", ".", ".", 20, 21)), v021Text(t, "далее", 3, 2), nil},
		{"escaped answer content hidden", v021Line(t, `Начало _____{\{a\}.}`, "content", 3, 2,
			v021Element(t, "content", "Начало ", "Начало ", 1, 8), v021Element(t, "placeholder", `_____{\{a\}.}`, "{a}.", 8, 21)),
			v021Text(t, "далее", 3, 2), [][4]int{{4, 1, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, "multifill", tc.first, tc.second), "current", tc.want)
		})
	}
	t.Run("ordinary and answered placeholders", func(t *testing.T) {
		ordinary := model.Element{ElementType: "placeholder", Raw: "_____", Start: 8, End: 13, ErrorIDs: []string{}}
		v021Run(t, v021Block(t, "multifill",
			v021Line(t, "Начало _____", "content", 3, 2, v021Element(t, "content", "Начало ", "Начало ", 1, 8), ordinary),
			v021Line(t, "далее _____{x}", "content", 3, 2, v021Element(t, "content", "далее ", "далее ", 1, 7),
				v021Element(t, "placeholder", "_____{x}", "x", 7, 15))), "current", nil)
	})
}

func TestV021IndependentRows(t *testing.T) {
	for _, tc := range []struct {
		name, tag, first, second string
		want                     [][4]int
	}{
		{"parallel raw gaps", "text", "hello _____", "good morning _____", nil},
		{"formatted parallel gaps", "text", "<em>hello _____</em>", "<u>good morning _____</u>", nil},
		{"parallel raw answers", "multifill", "hello _____{hi}", "good morning _____{morning}", nil},
		{"lettered raw answers", "multifill", "a. _____{the USA}", "b. _____{Spain}", nil},
		{"colon markers", "text", "a: first", "b: second", nil},
		{"space alone is not a marker", "text", "a first", "b second", [][4]int{{4, 1, 5, 9}}},
		{"NBSP alone is not a marker", "text", "а\u00a0первое", "б\u00a0второе", [][4]int{{4, 1, 5, 9}}},
		{"tab alone is not a marker", "text", "Начало", "a\tпункт", [][4]int{{4, 1, 5, 8}}},
		{"numeric list cannot start a paragraph pair", "text", "1. First item", "continuation", nil},
		{"marker in first line breaks adjacency", "text", "a. First item", "continuation", nil},
		{"formatting around marker", "text", "Начало", "<em>a</em>. пункт", nil},
		{"several letters are not a marker", "text", "Начало", "ab. пункт", [][4]int{{4, 1, 5, 10}}},
		{"colon later in word is not a marker", "text", "Начало", "word: text", [][4]int{{4, 1, 5, 11}}},
		{"short prose still reports", "text", "Это начало", "одного предложения.", [][4]int{{4, 1, 5, 20}}},
		{"single gap in prose still reports", "multifill", "Начало _____{ответ.}", "далее", [][4]int{{4, 1, 5, 6}}},
		{"gap inside prose still reports", "multifill", "Это _____{часть} начала", "одного предложения.", [][4]int{{4, 1, 5, 20}}},
		{"escaped gaps are literal", "text", `hello \_____`, `good morning \_____`, [][4]int{{4, 1, 5, 20}}},
		{"wrong gap length is not a template", "text", "hello ____", "good morning ____", [][4]int{{4, 1, 5, 18}}},
		{"key underscores are literal", "key", "hello _____", "good morning _____", [][4]int{{4, 1, 5, 19}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Block(t, tc.tag, v021Text(t, tc.first, 3, 2), v021Text(t, tc.second, 3, 2)), "current", tc.want)
		})
	}
	v021Run(t, v021Block(t, "text", v021Text(t, "Начало", 3, 2),
		v021Text(t, "a. самостоятельный пункт", 3, 2), v021Text(t, "далее", 3, 2)), "current", nil)
	v021Run(t, v021Block(t, "text", v021Text(t, "Начало", 3, 2),
		v021Text(t, "hello _____", 3, 2), v021Line(t, "", "blank", 3, 2),
		v021Text(t, "<br>", 3, 2), v021Text(t, "good morning _____", 3, 2),
		v021Text(t, "good evening _____", 3, 2), v021Text(t, "далее", 3, 2)), "current", nil)
	// Одиночные шаблоны в разных блоках не образуют общий список.
	v021Run(t, v021Document(t, v021Open(t, "text", 2, 1),
		v021Text(t, "hello _____", 3, 2), v021Close(t, 3, 2), v021Open(t, "text", 2, 1),
		v021Text(t, "good morning _____", 6, 2), v021Text(t, "далее", 6, 2), v021Close(t, 6, 2)),
		"current", [][4]int{{7, 1, 8, 6}})
}

func TestV021ContainerBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want [][4]int
	}{
		{"root text excluded", []model.Line{v021Text(t, "Начало", 2, 1), v021Text(t, "далее", 2, 1)}, nil},
		{"different blocks", []model.Line{v021Open(t, "note", 2, 1), v021Text(t, "Начало", 3, 2), v021Close(t, 3, 2),
			v021Open(t, "note", 2, 1), v021Text(t, "далее", 6, 2), v021Close(t, 6, 2)}, nil},
		{"inline values excluded", []model.Line{
			v021Line(t, "@note Начало", "tag", 2, 1, v021Element(t, "tag", "@note", "note", 1, 6),
				v021Element(t, "content", "Начало", "Начало", 7, 13)),
			v021Line(t, "@note далее", "tag", 2, 1, v021Element(t, "tag", "@note", "note", 1, 6),
				v021Element(t, "content", "далее", "далее", 7, 12))}, nil},
		{"title excluded", []model.Line{
			v021Line(t, "@text Начало {", "block-start", 2, 1, v021Element(t, "tag", "@text", "text", 1, 6),
				v021Element(t, "title", "Начало", "Начало", 7, 13), v021Element(t, "block-open", "{", "{", 14, 15)),
			v021Text(t, "далее", 3, 2), v021Close(t, 3, 2)}, nil},
		{"nested hint is a barrier", []model.Line{v021Open(t, "instr", 2, 1), v021Text(t, "Начало", 3, 2),
			v021Open(t, "hint", 3, 2), v021Text(t, "подсказка", 5, 3), v021Text(t, "продолжение", 5, 3),
			v021Close(t, 5, 3), v021Line(t, "", "blank", 3, 2), v021Text(t, "далее", 3, 2), v021Close(t, 3, 2)}, nil},
		{"inline hint is a barrier", []model.Line{v021Open(t, "instr", 2, 1), v021Text(t, "Начало", 3, 2),
			v021Line(t, "@hint Подсказка.", "tag", 3, 2, v021Element(t, "tag", "@hint", "hint", 1, 6),
				v021Element(t, "content", "Подсказка.", "Подсказка.", 7, 17)),
			v021Text(t, "далее", 3, 2), v021Close(t, 3, 2)}, nil},
		{"restart after hint", []model.Line{v021Open(t, "instr", 2, 1), v021Text(t, "Начало", 3, 2),
			v021Open(t, "hint", 3, 2), v021Text(t, "Подсказка.", 5, 3), v021Close(t, 5, 3),
			v021Text(t, "далее", 3, 2), v021Text(t, "ещё", 3, 2), v021Close(t, 3, 2)}, [][4]int{{8, 1, 9, 4}}},
		{"canonical block tag", []model.Line{
			v021Line(t, "@TeXt {", "block-start", 2, 1, v021Element(t, "tag", "@TeXt", "text", 1, 6),
				v021Element(t, "block-open", "{", "{", 7, 8)), v021Text(t, "Начало", 3, 2),
			v021Text(t, "далее", 3, 2), v021Close(t, 3, 2)}, [][4]int{{4, 1, 5, 6}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v021Run(t, v021Document(t, tc.body...), "current", tc.want)
		})
	}
	t.Run("target nested inside fragment", func(t *testing.T) {
		// DSL 3.12.9–3.12.10: определение фрагмента размещено после заголовка;
		// текстовый блок внутри него проверяется самостоятельно.
		fragment := v021Line(t, "@fragment sample {", "block-start", 2, 1,
			v021Element(t, "tag", "@fragment", "fragment", 1, 10), v021Element(t, "identifier", "sample", "sample", 11, 17),
			v021Element(t, "block-open", "{", "{", 18, 19))
		result := v021Document(t, fragment, v021Open(t, "text", 3, 2),
			v021Text(t, "Начало", 4, 3), v021Text(t, "далее", 4, 3), v021Close(t, 4, 3), v021Close(t, 3, 2))
		v021Run(t, result, "current", [][4]int{{5, 1, 6, 6}})
	})
}

func TestV021InvalidLineBreaksPair(t *testing.T) {
	// Неизвестный DSL-тег (1.16) согласован с unparsed и исходной ошибкой.
	invalid := v021Line(t, "@unknown", "invalid", 3, 2, model.Element{
		ElementType: "unparsed", Raw: "@unknown", Start: 1, End: 9, ErrorIDs: []string{"parser-error"},
	})
	invalid.HasErrors = true
	result := v021Block(t, "text", v021Text(t, "Начало", 3, 2), invalid,
		v021Text(t, "далее", 3, 2), v021Text(t, "ещё", 3, 2))
	result.Document.HasErrors = true
	result.Diagnostics = []model.Diagnostic{{ID: "parser-error", Source: "parser", DiagnosticCode: "fixture-parser-error",
		SeverityLevel: "error", DiagnosticScope: "element", Message: "Неизвестный DSL-тег.",
		Location:         &model.Location{Start: model.Position{Line: 5, Column: 1}, End: model.Position{Line: 5, Column: 9}},
		RelatedLocations: []model.Location{},
	}}
	v021Run(t, result, "current", [][4]int{{6, 1, 7, 4}})
}

func TestV021LineEndingsAndHistory(t *testing.T) {
	for _, ending := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""}, {"mixed", "\r\n", "\n"},
	} {
		t.Run(ending.name, func(t *testing.T) {
			result := v021Block(t, "text", v021Text(t, "Начало", 3, 2), v021Text(t, "далее", 3, 2))
			for i := range result.Lines {
				result.Lines[i].LineEnding = ending.eol
			}
			result.Lines[5].LineEnding = ending.last
			v021Run(t, result, "first", [][4]int{{4, 1, 5, 6}})
			v021Run(t, result, "second", [][4]int{{4, 1, 5, 6}})
			v021Run(t, v021Block(t, "text", v021Text(t, "Готово.", 3, 2), v021Text(t, "далее", 3, 2)), "current", nil)
		})
	}
}

func TestV021NoLinesAndVersions(t *testing.T) {
	result := v021Model(t)
	result.Document.LineCount = nil
	result.Document.Metadata.Title = nil
	v021Run(t, result, "current", nil)
	for _, version := range []string{"1.1", "1.3", "unknown"} {
		t.Run(version, func(t *testing.T) {
			result := v021Block(t, "text", v021Text(t, "Начало", 3, 2), v021Text(t, "далее", 3, 2))
			result.Document.DSLVersion = &version
			if version == "unknown" {
				result.Document.DSLVersion = nil
			}
			before := v021Snapshot(t, result)
			checks, err := validator.Prepare(result, v021Registry(t))
			if err == nil || checks != nil || v021Snapshot(t, result) != before {
				t.Fatalf("Prepare(%s) = %v, %v; want refusal without changes", version, checks, err)
			}
		})
	}
}

func TestV021CoexistsWithOtherChecks(t *testing.T) {
	result := v021Block(t, "text", v021Text(t, "Начало [", 3, 2), v021Text(t, "далее", 3, 2))
	v021Run(t, result, "current", [][4]int{{4, 1, 5, 6}})
	count := 0
	for _, entry := range result.Diagnostics {
		if entry.Source == "current" && entry.DiagnosticCode == "V008" {
			count++
			want := model.Location{Start: model.Position{Line: 4, Column: 8}, End: model.Position{Line: 4, Column: 9}}
			if entry.Location == nil || *entry.Location != want || entry.SeverityLevel != "warning" ||
				entry.Message != "Нарушена парность квадратных скобок в строке.\nконкретизация: Открывающая квадратная скобка не закрыта в этой строке" {
				t.Errorf("unexpected V008: %+v", entry)
			}
		}
	}
	if count != 1 {
		t.Fatalf("V008 count = %d, want 1", count)
	}
}

// v021Content задаёт различающиеся Raw и Value без повторного разбора текста.
func v021Content(t *testing.T, raw, value string, parent, depth int) model.Line {
	t.Helper()
	return v021Line(t, raw, "content", parent, depth,
		v021Element(t, "content", raw, value, 1, utf8.RuneCountInString(raw)+1))
}

// v021Block добавляет обрамление блока; строки тела явно заданы с parent=3.
func v021Block(t *testing.T, tag string, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{v021Open(t, tag, 2, 1)}
	lines = append(lines, body...)
	lines = append(lines, v021Close(t, 3, 2))
	return v021Document(t, lines...)
}

// v021Registry создаёт реестр только через штатный каталог регистраций.
func v021Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v021Document добавляет обязательные версию и заголовок к явно заданному телу.
func v021Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := []model.Line{
		v021Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v021Element(t, "tag", "@dsl-version", "dsl-version", 1, 13), v021Element(t, "version", "1.2", "1.2", 14, 17)),
		v021Line(t, "# Unit 1", "heading", 0, 0,
			v021Element(t, "heading-level", "#", "1", 1, 2), v021Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	return v021Model(t, append(lines, body...)...)
}

// v021Model задаёт физические номера, не выводя типы и структуру из Raw.
func v021Model(t *testing.T, lines ...model.Line) *model.Result {
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

// v021Line сохраняет явно заданную структуру строки без разбора Raw.
func v021Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, NestingLevel: depth,
		LineEnding: model.LineEndingLF, Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v021Element задаёт исходное и семантическое значения и диапазон [start,end).
func v021Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{}}
}

// v021Text задаёт текст с одинаковыми Raw и Value; длина нужна только для
// входного элемента и не используется для вычисления ожидаемых срабатываний.
func v021Text(t *testing.T, raw string, parent, depth int) model.Line {
	t.Helper()
	return v021Line(t, raw, "content", parent, depth,
		v021Element(t, "content", raw, raw, 1, utf8.RuneCountInString(raw)+1))
}

// v021Open задаёт блочное объявление без параметров с согласованными элементами.
func v021Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	end := utf8.RuneCountInString(tag) + 2
	return v021Line(t, "@"+tag+" {", "block-start", parent, depth,
		v021Element(t, "tag", "@"+tag, tag, 1, end), v021Element(t, "block-open", "{", "{", end+1, end+2))
}

// v021Close задаёт закрытие с явно указанными родителем и глубиной.
func v021Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v021Line(t, "}", "block-end", parent, depth, v021Element(t, "block-close", "}", "}", 1, 2))
}

// v021Run проверяет V021 текущего запуска штатного конвейера: точное число,
// метаданные и диапазоны [start,end), неизменность исходной модели и истории.
func v021Run(t *testing.T, result *model.Result, processingID string, want [][4]int) {
	t.Helper()
	before := v021Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v021Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v021Snapshot(t, result) != before {
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
		if i >= diagnosticCount && entry.DiagnosticCode == "V021" && entry.Source != processingID {
			t.Fatalf("new V021 source = %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V021" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V021 count = %d, want %d; occurrences: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{Start: model.Position{Line: expected[0], Column: expected[1]},
			End: model.Position{Line: expected[2], Column: expected[3]}}
		if entry.Location == nil || *entry.Location != location ||
			entry.Message != "Строка без завершающего знака продолжается строкой со строчной буквы.\nконкретизация: Проверьте, не разделён ли один абзац на несколько строк" ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "block" || entry.Fatal ||
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
	v021OtherFindings(t, &expected, result.Diagnostics[diagnosticCount:], processingID)
	if err := validator.CheckModel(result); err != nil {
		t.Fatal(err)
	}
	previous := *result
	previous.Processing, previous.Diagnostics = result.Processing[:processingCount], result.Diagnostics[:diagnosticCount]
	if v021Snapshot(t, &previous) != v021Snapshot(t, &expected) {
		t.Fatal("source, history, flags or element links differ from expected changes")
	}
}

// v021Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v021Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v021OtherFindings учитывает только разрешённые побочные изменения от других
// диагностик полного каталога. Ожидания самой проверки заданы явно в Run.
// Исходник, история, родители и связанные диапазоны не меняются.
func v021OtherFindings(t *testing.T, expected *model.Result, entries []model.Diagnostic, source string) {
	t.Helper()
	order := make(map[string]int)
	for i, entry := range append(append([]model.Diagnostic{}, expected.Diagnostics...), entries...) {
		order[entry.ID] = i
	}
	for _, entry := range entries {
		if entry.Source != source {
			t.Fatalf("unexpected diagnostic source %q", entry.Source)
		}
		if entry.DiagnosticCode == "V021" {
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
