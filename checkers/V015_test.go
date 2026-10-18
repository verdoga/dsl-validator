package checkers

import (
	"encoding/json"
	"fmt"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

// v015Finding хранит независимое ожидание: исходный диапазон и конкретизацию.
type v015Finding struct {
	line, start, end int
	detail           string
}

func TestV015Text(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		want             []v015Finding
	}{
		{"reference", "Read /ˈwɜːd/ ↗.", "Read /ˈwɜːd/ ↗.", nil},
		{"example", "Read □✓ the word.", "Read □✓ the word.", []v015Finding{{3, 6, 8, "Обнаружены U+25A1 и U+2713"}}},
		{"ordinary text", "Abc Ёж", "Abc Ёж", nil},
		{"one character", "□", "□", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}}},
		{"repeated character", "□□", "□□", []v015Finding{{3, 1, 3, "Обнаружены U+25A1 и U+25A1"}}},
		{"three characters", "□✓文", "□✓文", []v015Finding{{3, 1, 4, "Обнаружены U+25A1, U+2713 и U+6587"}}},
		{"allowed letter splits", "□a✓", "□a✓", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}}},
		{"space splits", "□ ✓", "□ ✓", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}}},
		{"other scripts", "α文", "α文", []v015Finding{{3, 1, 3, "Обнаружены U+03B1 и U+6587"}}},
		{"Greek exceptions", "βθχ", "βθχ", nil},
		{"Greek uppercase is not folded", "ΒΘΧ", "ΒΘΧ", []v015Finding{{3, 1, 4, "Обнаружены U+0392, U+0398 и U+03A7"}}},
		{"IPA without delimiters", "ˈwɜːd", "ˈwɜːd", nil},
		{"all arrows without context", "↑↓↗↘", "↑↓↗↘", nil},
		{"teaching transformation", "I am → I'm", "I am → I'm", nil},
		{"arrow splits forbidden characters", "□→✓", "□→✓", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}}},
		{"unclosed delimiters", "/↑ [↓", "/↑ [↓", nil},
		{"Unicode decimal digits", "0９١", "0９١", nil},
		{"other numbers", "²Ⅳ", "²Ⅳ", []v015Finding{{3, 1, 3, "Обнаружены U+00B2 и U+2163"}}},
		{"combining diacritic", "e\u0301", "e\u0301", nil},
		{"standalone combining diacritic", "\u0301", "\u0301", nil},
		{"combining non-diacritic", "a\u20DD", "a\u20DD", []v015Finding{{3, 2, 3, "Обнаружен U+20DD"}}},
		{"no Unicode normalization", "𝐀", "𝐀", []v015Finding{{3, 1, 2, "Обнаружен U+1D400"}}},
		{"Unicode columns and supplementary code", "Я😀✓", "Я😀✓", []v015Finding{{3, 2, 4, "Обнаружены U+1F600 и U+2713"}}},
		{"escaping is not removed", `\_____`, "_____", []v015Finding{{3, 2, 7, "Обнаружены U+005F, U+005F, U+005F, U+005F и U+005F"}}},
		{"HTML is just characters", "□<em>✓</em>", "□<em>✓</em>", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 6, 7, "Обнаружен U+2713"}}},
		{"pipe is just a character", "□|✓", "□|✓", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}}},
		{"entities are not decoded", "&#9633;", "&#9633;", nil},
		{"soft hyphen splits", "□\u00AD✓", "□\u00AD✓", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := v015Line(t, tc.raw, "content", 2, 1,
				v015Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v015Run(t, v015Document(t, line), "current", tc.want)
		})
	}
}

func TestV015AllowedAndExcludedCharacters(t *testing.T) {
	// Фигурные скобки и обратная косая черта корректно экранированы в DSL;
	// сама V015 рассматривает все исходные символы этих записей без изменений.
	base := `Text .,;:!?…'"-–—()[]\{\}/\\+=%°№§@#$€£₽`
	value := `Text .,;:!?…'"-–—()[]{}/\+=%°№§@#$€£₽`
	v015Run(t, v015Document(t, v015Line(t, base, "content", 2, 1,
		v015Element(t, "content", base, value, 1, utf8.RuneCountInString(base)+1))), "current", nil)
	v015Run(t, v015Document(t, v015Text(t, "βθχ‿‖|↘↗↑↓", 2, 1)), "current", nil)
	excluded := []rune{
		'\t', '\u000B', '\u000C', '\u0085', '\u00A0', '\u1680', '\u2000', '\u2001',
		'\u2002', '\u2003', '\u2004', '\u2005', '\u2006', '\u2007', '\u2008', '\u2009',
		'\u200A', '\u2028', '\u2029', '\u202F', '\u205F', '\u3000',
		'\u00AB', '\u00BB', '\u2018', '\u2019', '\u201A', '\u201B', '\u201C', '\u201D',
		'\u201E', '\u201F', '\u2039', '\u203A', '\u2E42', '\u300C', '\u300D', '\u300E',
		'\u300F', '\u301D', '\u301E', '\u301F', '\uFE41', '\uFE42', '\uFE43', '\uFE44',
		'\uFF02', '\uFF07', '\uFF62', '\uFF63',
		'^', '&', '*', '~', '`', '<', '>', '\u00AD', '\u2010', '\u2011', '\u2012',
		'\u2015', '\uFE58', '\uFE63', '\uFF0D',
	}
	for _, char := range excluded {
		t.Run(fmt.Sprintf("U+%04X", char), func(t *testing.T) {
			// Проверяется также то, что исключение прерывает две последовательности.
			v015Run(t, v015Document(t, v015Text(t, "□"+string(char)+"✓", 2, 1)), "current",
				[]v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {3, 3, 4, "Обнаружен U+2713"}})
		})
	}
}

func TestV015PhoneticCharacters(t *testing.T) {
	// Явные присвоенные участки Unicode 15.0 (таблицы Go 1.25.1).
	// Ожидание нулевое для каждого символа; проверяемая классификация не вызывается.
	for _, span := range [][2]rune{
		{0x0250, 0x02FF}, {0x1D00, 0x1DBF}, {0xA700, 0xA71F}, {0xAB30, 0xAB6B},
		{0x10780, 0x10785}, {0x10787, 0x107B0}, {0x107B2, 0x107BA},
		{0x1DF00, 0x1DF1E}, {0x1DF25, 0x1DF2A},
	} {
		for char := span[0]; char <= span[1]; char++ {
			t.Run(fmt.Sprintf("U+%04X", char), func(t *testing.T) {
				v015Run(t, v015Document(t, v015Text(t, string(char), 2, 1)), "current", nil)
			})
		}
	}
	for _, tc := range []struct {
		char   rune
		detail string
	}{
		{0x0242, ""},
		{0x0300, ""},
		{0x1CFF, "Обнаружен U+1CFF"},
		{0x1DC0, "Обнаружен U+1DC0"},
		{0xA6FF, "Обнаружен U+A6FF"},
		{0xA720, "Обнаружен U+A720"},
		{0xAB2F, "Обнаружен U+AB2F"},
		{0xAB6C, "Обнаружен U+AB6C"},
		{0xAB70, "Обнаружен U+AB70"},
		{0x1077F, "Обнаружен U+1077F"},
		{0x10786, "Обнаружен U+10786"},
		{0x107B1, "Обнаружен U+107B1"},
		{0x107BB, "Обнаружен U+107BB"},
		{0x107BF, "Обнаружен U+107BF"},
		{0x107C0, "Обнаружен U+107C0"},
		{0x1DEFF, "Обнаружен U+1DEFF"},
		{0x1DF1F, "Обнаружен U+1DF1F"},
		{0x1DF24, "Обнаружен U+1DF24"},
		{0x1DF2B, "Обнаружен U+1DF2B"},
		{0x1DFFF, "Обнаружен U+1DFFF"},
		{0x1E000, "Обнаружен U+1E000"},
	} {
		t.Run(fmt.Sprintf("boundary U+%04X", tc.char), func(t *testing.T) {
			var want []v015Finding
			if tc.detail != "" {
				want = []v015Finding{{3, 1, 2, tc.detail}}
			}
			v015Run(t, v015Document(t, v015Text(t, string(tc.char), 2, 1)), "current", want)
		})
	}
}

func TestV015Contexts(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v015Finding
	}{
		{"single-line comment", []model.Line{
			v015Line(t, "@editor □", "tag", 2, 1,
				v015Element(t, "tag", "@editor", "editor", 1, 8), v015Element(t, "content", "□", "□", 9, 10)),
		}, []v015Finding{{3, 9, 10, "Обнаружен U+25A1"}}},
		{"block comment", []model.Line{
			v015Open(t, "editor", 2, 1), v015Text(t, "□✓", 3, 2), v015Close(t, 3, 2),
		}, []v015Finding{{4, 1, 3, "Обнаружены U+25A1 и U+2713"}}},
		{"nested hint", []model.Line{
			v015Open(t, "instr", 2, 1), v015Text(t, "Read.", 3, 2),
			v015Open(t, "hint", 3, 2), v015Text(t, "□✓", 5, 3), v015Close(t, 5, 3), v015Close(t, 3, 2),
		}, []v015Finding{{6, 1, 3, "Обнаружены U+25A1 и U+2713"}}},
		{"HTML content", []model.Line{
			v015Open(t, "text", 2, 1), v015Text(t, "□<em>✓</em>", 3, 2), v015Close(t, 3, 2),
		}, []v015Finding{{4, 1, 2, "Обнаружен U+25A1"}, {4, 6, 7, "Обнаружен U+2713"}}},
		{"table content", []model.Line{
			v015Open(t, "table", 2, 1), v015Text(t, "□|✓", 3, 2), v015Close(t, 3, 2),
		}, []v015Finding{{4, 1, 2, "Обнаружен U+25A1"}, {4, 3, 4, "Обнаружен U+2713"}}},
		{"split table cell", []model.Line{
			v015Open(t, "table", 2, 1), v015Text(t, "<□|✓>", 3, 2), v015Close(t, 3, 2),
		}, []v015Finding{{4, 2, 3, "Обнаружен U+25A1"}, {4, 4, 5, "Обнаружен U+2713"}}},
		{"separator and block boundaries", []model.Line{
			v015Open(t, "example", 2, 1), v015Text(t, "□", 3, 2),
			v015Line(t, "---", "separator", 3, 2), v015Text(t, "✓", 3, 2), v015Close(t, 3, 2),
		}, []v015Finding{{4, 1, 2, "Обнаружен U+25A1"}, {6, 1, 2, "Обнаружен U+2713"}}},
		{"original element offset", []model.Line{
			v015Line(t, " \t□✓ \t", "content", 2, 1, v015Element(t, "content", "□✓", "□✓", 3, 5)),
		}, []v015Finding{{3, 3, 5, "Обнаружены U+25A1 и U+2713"}}},
		{"blank line", []model.Line{v015Line(t, " \t", "blank", 2, 1)}, nil},
		{"media source", []model.Line{
			v015Line(t, "@media image □✓.png", "tag", 2, 1,
				v015Element(t, "tag", "@media", "media", 1, 7),
				v015Element(t, "media-type", "image", "image", 8, 13),
				v015Element(t, "source", "□✓.png", "□✓.png", 14, 20)),
		}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v015Run(t, v015Document(t, tc.body...), "current", tc.want)
		})
	}
}

func TestV015Placeholders(t *testing.T) {
	for _, tc := range []struct {
		name, raw, value string
		noAnswer         bool
		want             []v015Finding
	}{
		{"answer", `_____{□✓}`, `□✓`, false, []v015Finding{{4, 7, 9, "Обнаружены U+25A1 и U+2713"}}},
		{"no answer", `_____`, "", true, nil},
		{"allowed answer", `_____{↑ˈwɜːd↓}`, `↑ˈwɜːd↓`, false, nil},
		{"arrow in answer", `_____{I am → I'm}`, `I am → I'm`, false, nil},
		{"raw escape is preserved", `_____{\{□✓}`, `{□✓`, false, []v015Finding{{4, 9, 11, "Обнаружены U+25A1 и U+2713"}}},
		{"escaped closing brace", `_____{a\}□✓}`, `a}□✓`, false, []v015Finding{{4, 10, 12, "Обнаружены U+25A1 и U+2713"}}},
		{"raw underscores in answer", `_____{\_}`, `\_`, false, []v015Finding{{4, 8, 9, "Обнаружен U+005F"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			element := v015Element(t, "placeholder", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1)
			if tc.noAnswer {
				element.Value = nil
			}
			v015Run(t, v015Document(t, v015Open(t, "multifill", 2, 1),
				v015Line(t, tc.raw, "content", 3, 2, element), v015Close(t, 3, 2)), "current", tc.want)
		})
	}
	// Соседние элементы не объединяются, даже если оба ответа запрещены целиком.
	v015Run(t, v015Document(t, v015Open(t, "multifill", 2, 1),
		v015Line(t, "_____{□}_____{✓}", "content", 3, 2,
			v015Element(t, "placeholder", "_____{□}", "□", 1, 9),
			v015Element(t, "placeholder", "_____{✓}", "✓", 9, 17)),
		v015Close(t, 3, 2)), "current",
		[]v015Finding{{4, 7, 8, "Обнаружен U+25A1"}, {4, 15, 16, "Обнаружен U+2713"}})
}

func TestV015MalformedAnswerKeepsTextChecks(t *testing.T) {
	// Полный каталог также выдаёт V005 на незакрытую фигурную скобку.
	// Проверка целостности в v015Run контролирует её флаги и обратные ссылки;
	// V012 и V017 обязаны независимо продолжать проверку содержимого.
	result := v021Block(t, "multifill", v021Text(t, "_____{1-12 Word ,", 3, 2))
	v015Run(t, result, "current", []v015Finding{{4, 1, 6, "Обнаружены U+005F, U+005F, U+005F, U+005F и U+005F"}})
	for _, tc := range []struct {
		code             diagnostics.Code
		line, start, end int
		severity         string
	}{
		{"V005", 3, 12, 13, "error"},
		{"V012", 4, 8, 9, "recommendation"},
		{"V017", 4, 16, 17, "warning"},
	} {
		count := 0
		for _, entry := range result.Diagnostics {
			if entry.DiagnosticCode != tc.code {
				continue
			}
			count++
			want := model.Location{Start: model.Position{Line: tc.line, Column: tc.start}, End: model.Position{Line: tc.line, Column: tc.end}}
			if entry.Location == nil || *entry.Location != want || string(entry.SeverityLevel) != tc.severity {
				t.Errorf("%s: unexpected finding %+v, location %+v", tc.code, entry, entry.Location)
			}
		}
		if count != 1 {
			t.Errorf("%s count = %d, want 1", tc.code, count)
		}
	}
}

// v015AnswerDocument задаёт две модели одной исходной строки. Границы
// входных элементов вычисляются по длинам явно заданных фрагментов;
// ожидаемые координаты диагностик каждый тест задаёт независимо.
func v015AnswerDocument(t *testing.T, before, answer, after string, separate bool) *model.Result {
	t.Helper()
	gap := "_____{" + answer + "}"
	raw := before + gap + after
	line := v015Text(t, raw, 3, 2)
	if separate {
		line.Elements = []model.Element{}
		if before != "" {
			line.Elements = append(line.Elements, v015Element(t, "content", before, before, 1, utf8.RuneCountInString(before)+1))
		}
		start := utf8.RuneCountInString(before) + 1
		end := start + utf8.RuneCountInString(gap)
		line.Elements = append(line.Elements, v015Element(t, "placeholder", gap, answer, start, end))
		if after != "" {
			line.Elements = append(line.Elements, v015Element(t, "content", after, after, end, end+utf8.RuneCountInString(after)))
		}
	}
	return v015Document(t, v015Open(t, "multifill", 2, 1), line, v015Close(t, 3, 2))
}

func TestV015ContentPlaceholders(t *testing.T) {
	const underscores = "Обнаружены U+005F, U+005F, U+005F, U+005F и U+005F"
	for _, tc := range []struct {
		name, raw, value string
		want             []v015Finding
	}{
		{"plain", "_____", "_____", nil},
		{"reported example", "1. _____{Brazil}: Brazilian", "1. _____{Brazil}: Brazilian", nil},
		{"answer stays checked", "_____{□✓}", "_____{□✓}", []v015Finding{{4, 7, 9, "Обнаружены U+25A1 и U+2713"}}},
		{"outside stays checked", "□_____✓", "□_____✓", []v015Finding{{4, 1, 2, "Обнаружен U+25A1"}, {4, 7, 8, "Обнаружен U+2713"}}},
		{"multiple answers", "_____{□}_____{✓}", "_____{□}_____{✓}", []v015Finding{{4, 7, 8, "Обнаружен U+25A1"}, {4, 15, 16, "Обнаружен U+2713"}}},
		{"underscores in answer are data", "_____{_____}", "_____{_____}", []v015Finding{{4, 7, 12, underscores}}},
		{"escaped closing brace", `_____{a\}□✓}`, `_____{a}□✓}`, []v015Finding{{4, 10, 12, "Обнаружены U+25A1 и U+2713"}}},
		{"even slashes close answer", `_____{a\\}_____`, `_____{a\}_____`, nil},
		{"escaped gap", `\_____`, "_____", []v015Finding{{4, 2, 7, underscores}}},
		{"paired slashes", `\\_____`, `\_____`, nil},
		{"odd slashes", `\\\_____`, `\_____`, []v015Finding{{4, 4, 9, underscores}}},
		{"single underscore", "a_b", "a_b", []v015Finding{{4, 2, 3, "Обнаружен U+005F"}}},
		{"no partial gap in longer run", "______", "______", []v015Finding{{4, 1, 7, "Обнаружены U+005F, U+005F, U+005F, U+005F, U+005F и U+005F"}}},
		{"unclosed answer falls back", "_____{□", "_____{□", []v015Finding{{4, 1, 6, underscores}, {4, 7, 8, "Обнаружен U+25A1"}}},
		{"Unicode offset and raw value", "Я _____{□}✓", "different", []v015Finding{{4, 9, 10, "Обнаружен U+25A1"}, {4, 11, 12, "Обнаружен U+2713"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			content := v015Line(t, tc.raw, "content", 3, 2,
				v015Element(t, "content", tc.raw, tc.value, 1, utf8.RuneCountInString(tc.raw)+1))
			v015Run(t, v015Document(t, v015Open(t, "multifill", 2, 1), content, v015Close(t, 3, 2)), "current", tc.want)
		})
	}
	for _, tag := range []string{"text", "instr", "editor", "key", "answer", "choice", "multichoice", "matching", "ordering"} {
		t.Run("context/"+tag, func(t *testing.T) {
			var want []v015Finding
			if tag != "text" && tag != "instr" {
				want = []v015Finding{{4, 1, 6, underscores}}
			}
			v015Run(t, v015Document(t, v015Open(t, tag, 2, 1), v015Text(t, "_____", 3, 2), v015Close(t, 3, 2)), "current", want)
		})
	}
	v015Run(t, v015Document(t, v015Text(t, "_____", 2, 1)), "current", nil)
	for _, tag := range []string{"question", "editor", "answer"} {
		t.Run("inline/"+tag, func(t *testing.T) {
			start := len(tag) + 3
			var want []v015Finding
			if tag != "question" {
				want = []v015Finding{{3, start, start + 5, underscores}}
			}
			v015Run(t, v015Document(t, v015Line(t, "@"+tag+" _____", "tag", 2, 1,
				v015Element(t, "tag", "@"+tag, tag, 1, start-1),
				v015Element(t, "content", "_____", "_____", start, start+5))), "current", want)
		})
	}
	t.Run("title is literal", func(t *testing.T) {
		result := v015Document(t)
		title := "_____"
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# _____"
		result.Lines[1].Elements[1] = v015Element(t, "title", title, title, 3, 8)
		v015Run(t, result, "current", []v015Finding{{2, 3, 8, underscores}})
	})
}

func TestV015HeadingAndMetadata(t *testing.T) {
	t.Run("title", func(t *testing.T) {
		result := v015Document(t, v015Text(t, "Text.", 2, 1))
		title := "□✓"
		result.Document.Metadata.Title = &title
		result.Lines[1].Raw = "# □✓"
		result.Lines[1].Elements[1] = v015Element(t, "title", "□✓", "□✓", 3, 5)
		v015Run(t, result, "current", []v015Finding{{2, 3, 5, "Обнаружены U+25A1 и U+2713"}})
	})
	for _, tc := range []struct {
		name, raw, tag, kind, value string
		tagEnd, start, end          int
		want                        []v015Finding
	}{
		{"name", "@section □✓", "section", "name", "□✓", 9, 10, 12, []v015Finding{{2, 10, 12, "Обнаружены U+25A1 и U+2713"}}},
		{"name is not a gap", "@section _____", "section", "name", "_____", 9, 10, 15,
			[]v015Finding{{2, 10, 15, "Обнаружены U+005F, U+005F, U+005F, U+005F и U+005F"}}},
		{"resource path", "@resource-dir □✓", "resource-dir", "resource-path", "□✓", 14, 15, 17, nil},
		{"identifier", "@document-id unit-1", "document-id", "identifier", "unit-1", 13, 14, 20, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v015Document(t)
			metadata := v015Line(t, tc.raw, "tag", 0, 0,
				v015Element(t, "tag", "@"+tc.tag, tc.tag, 1, tc.tagEnd),
				v015Element(t, model.ElementType(tc.kind), tc.value, tc.value, tc.start, tc.end))
			result.Lines = []model.Line{result.Lines[0], metadata, result.Lines[1]}
			for i := range result.Lines {
				result.Lines[i].Number = i + 1
			}
			*result.Document.LineCount = 3
			switch tc.kind {
			case "name":
				result.Document.Metadata.Section = &tc.value
			case "resource-path":
				result.Document.Metadata.ResourceDirs = []string{tc.value}
			case "identifier":
				result.Document.Metadata.DocumentID = &tc.value
			}
			v015Run(t, result, "current", tc.want)
		})
	}
}

func TestV015Unparsed(t *testing.T) {
	element := v015Element(t, "unparsed", "@□", "", 1, 3)
	element.Value, element.ErrorIDs = nil, []string{"d1"}
	result := v015Document(t, v015Line(t, "@□", "invalid", 2, 1, element))
	result.Document.HasErrors, result.Lines[2].HasErrors = true, true
	result.Diagnostics = []model.Diagnostic{{
		ID: "d1", Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error",
		Message: "Неизвестный тег.", DiagnosticScope: "element",
		Location:         &model.Location{Start: model.Position{Line: 3, Column: 1}, End: model.Position{Line: 3, Column: 3}},
		RelatedLocations: []model.Location{},
	}}
	v015Run(t, result, "current", []v015Finding{{3, 2, 3, "Обнаружен U+25A1"}})
}

func TestV015LineEndings(t *testing.T) {
	for _, tc := range []struct {
		name      string
		eol, last model.LineEnding
	}{
		{"LF", "\n", "\n"}, {"CRLF", "\r\n", "\r\n"},
		{"LF without final EOL", "\n", ""}, {"CRLF without final EOL", "\r\n", ""},
		{"mixed", "\r\n", "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v015Document(t, v015Text(t, "□", 2, 1), v015Text(t, "✓", 2, 1))
			for i := range result.Lines {
				result.Lines[i].LineEnding = tc.eol
			}
			result.Lines[3].LineEnding = tc.last
			v015Run(t, result, "current", []v015Finding{{3, 1, 2, "Обнаружен U+25A1"}, {4, 1, 2, "Обнаружен U+2713"}})
		})
	}
}

func TestV015UnavailableAndUnsupportedVersions(t *testing.T) {
	v015Run(t, v015UnavailableDocument(t, "1.2"), "current", nil)
	for _, version := range []string{"1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v015UnavailableDocument(t, version)
			before := v015Snapshot(t, result)
			checks, err := validator.Prepare(result, v015Registry(t))
			if err == nil || checks != nil {
				t.Fatalf("unsupported version accepted: %v, %v", checks, err)
			}
			if v015Snapshot(t, result) != before {
				t.Fatal("unsupported version changed the model")
			}
		})
	}
}

func TestV015HistoryAndOtherChecks(t *testing.T) {
	result := v015Document(t, v015Text(t, "A\t  “□”", 2, 1))
	want := []v015Finding{{3, 6, 7, "Обнаружен U+25A1"}}
	v015Run(t, result, "first", want)
	v015Run(t, result, "second", want)
	for _, source := range []string{"first", "second"} {
		for _, tc := range []struct {
			code       diagnostics.Code
			start, end int
		}{
			{"V001", 3, 5}, {"V002", 2, 3}, {"V009", 5, 6}, {"V009", 7, 8},
		} {
			count := 0
			for _, entry := range result.Diagnostics {
				if entry.Source != source || entry.DiagnosticCode != tc.code || entry.Location == nil {
					continue
				}
				location := model.Location{Start: model.Position{Line: 3, Column: tc.start}, End: model.Position{Line: 3, Column: tc.end}}
				if *entry.Location == location {
					count++
				}
			}
			if count != 1 {
				t.Errorf("%s at [%d,%d) in %s: count = %d, want 1", tc.code, tc.start, tc.end, source, count)
			}
		}
	}
	v015Run(t, v015Document(t, v015Text(t, "Clean.", 2, 1)), "first", nil)
}

// v015Open создаёт явно заданное объявление блока без параметров.
func v015Open(t *testing.T, tag string, parent, depth int) model.Line {
	t.Helper()
	return v015Line(t, "@"+tag+" {", "block-start", parent, depth,
		v015Element(t, "tag", "@"+tag, tag, 1, len(tag)+2),
		v015Element(t, "block-open", "{", "{", len(tag)+3, len(tag)+4))
}

// v015Close создаёт закрывающую строку с заданным родительским блоком.
func v015Close(t *testing.T, parent, depth int) model.Line {
	t.Helper()
	return v015Line(t, "}", "block-end", parent, depth, v015Element(t, "block-close", "}", "}", 1, 2))
}

func TestV015Registration(t *testing.T) {
	definitions, err := v015Registry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	matches := 0
	for _, definition := range definitions {
		if definition.Code != "V015" {
			continue
		}
		matches++
		if definition.Severity != "warning" || definition.Message != "В тексте присутствует символ вне разрешённого набора." ||
			definition.Scope != "line" || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Errorf("unexpected V015 definition: %+v", definition)
		}
	}
	if matches != 1 {
		t.Fatalf("V015 definitions = %d, want 1", matches)
	}
}

// v015UnavailableDocument сохраняет известную версию при недоступном исходнике.
func v015UnavailableDocument(t *testing.T, version string) *model.Result {
	t.Helper()
	return &model.Result{
		Format:      model.FormatVersion1,
		Document:    model.Document{DSLVersion: &version, Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       []model.Line{},
		Diagnostics: []model.Diagnostic{},
	}
}

// v015Registry создаёт только штатный реестр; тест не подменяет регистрации.
func v015Registry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v015Document добавляет обязательные @dsl-version и заголовок по DSL 1.2,
// пп. 1.1–1.3, 1.30 и 2.1.3. Обычный текст образует шаг по п. 2.1.15.
// Тело уже содержит явно заданные элементы, родителей и глубину; разбора нет.
func v015Document(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	version, title, encoding := "1.2", "Unit 1", "UTF-8"
	lines := []model.Line{
		v015Line(t, "@dsl-version 1.2", "tag", 0, 0,
			v015Element(t, "tag", "@dsl-version", "dsl-version", 1, 13),
			v015Element(t, "version", "1.2", "1.2", 14, 17)),
		v015Line(t, "# Unit 1", "heading", 0, 0,
			v015Element(t, "heading-level", "#", "1", 1, 2),
			v015Element(t, "title", "Unit 1", "Unit 1", 3, 9)),
	}
	lines = append(lines, body...)
	for i := range lines {
		lines[i].Number = i + 1
	}
	count := len(lines)
	return &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{
			DSLVersion: &version, Encoding: &encoding, LineCount: &count,
			Metadata: model.DocumentMetadata{Title: &title, ResourceDirs: []string{}},
		},
		Processing:  []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:       lines,
		Diagnostics: []model.Diagnostic{},
	}
}

// v015Line собирает строку с явно заданной структурой, без классификации Raw.
func v015Line(t *testing.T, raw string, kind model.LineType, parent, depth int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{
		Raw: raw, LineType: kind, NestingLevel: depth, LineEnding: model.LineEndingLF,
		Elements: append([]model.Element{}, elements...),
	}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v015Element сохраняет заданные исходное и семантическое значения;
// start и end — исходные Unicode-колонки, правая граница исключается.
func v015Element(t *testing.T, kind model.ElementType, raw, value string, start, end int) model.Element {
	t.Helper()
	return model.Element{
		ElementType: kind, Raw: raw, Value: &value, Start: start, End: end, ErrorIDs: []string{},
	}
}

// v015Text создаёт готовое неэкранированное содержимое без краевых пробелов.
// Длина используется только для входного элемента, не для ожидаемых срабатываний.
func v015Text(t *testing.T, text string, parent, depth int) model.Line {
	t.Helper()
	return v015Line(t, text, "content", parent, depth,
		v015Element(t, "content", text, text, 1, utf8.RuneCountInString(text)+1))
}

// v015Run проверяет штатный конвейер и явно заданные диапазоны и конкретизации.
// Правая граница исключается. Проверяются только V015 текущего запуска;
// прежние результаты и исходные строки должны сохраниться без изменений.
func v015Run(t *testing.T, result *model.Result, processingID string, want []v015Finding) {
	t.Helper()
	before := v015Snapshot(t, result)
	processingCount, diagnosticCount := len(result.Processing), len(result.Diagnostics)
	checks, err := validator.Prepare(result, v015Registry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v015Snapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	seenIDs := make(map[string]bool)
	for i, entry := range result.Diagnostics {
		if entry.ID == "" || seenIDs[entry.ID] {
			t.Fatalf("empty or duplicate diagnostic ID %q", entry.ID)
		}
		seenIDs[entry.ID] = true
		if i >= diagnosticCount && entry.DiagnosticCode == "V015" && entry.Source != processingID {
			t.Fatalf("new V015 has source %q, want %q", entry.Source, processingID)
		}
		if entry.DiagnosticCode == "V015" && entry.Source == processingID {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("current V015 count = %d, want %d; diagnostics: %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := model.Location{
			Start: model.Position{Line: expected.line, Column: expected.start},
			End:   model.Position{Line: expected.line, Column: expected.end},
		}
		if entry.Location == nil || *entry.Location != location {
			t.Errorf("occurrence %d location = %+v, want %+v", i, entry.Location, location)
		}
		if entry.DiagnosticCode != "V015" || entry.Source != processingID || entry.SeverityLevel != "warning" ||
			entry.Message != "В тексте присутствует символ вне разрешённого набора.\nконкретизация: "+expected.detail || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.RelatedLocations == nil || len(entry.RelatedLocations) != 0 {
			t.Errorf("occurrence %d has unexpected metadata: %+v", i, entry)
		}
	}
	if len(result.Processing) != processingCount+1 {
		t.Fatal("unexpected processing count")
	}
	v015AssertPreserved(t, before, result)
}

// v015Snapshot фиксирует модель для проверки отсутствия побочных изменений.
func v015Snapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

// v015AssertPreserved проверяет неизменность исходника и прежней истории.
// CheckModel независимо проверяет флаги и точные обратные ссылки всех сообщений,
// включая сопутствующие ошибки полного каталога. Только эти производные поля
// восстанавливаются в копии перед сравнением; ожидания V015 заданы явно выше.
func v015AssertPreserved(t *testing.T, before string, result *model.Result) {
	t.Helper()
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v015Snapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 || len(restored.Diagnostics) < len(original.Diagnostics) {
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
	if v015Snapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}
