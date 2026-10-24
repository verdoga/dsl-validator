package checkers

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/verdoga/dsl-validator/diagnostics"
	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

const v060TestMessage = "Нарушена числовая последовательность заголовков шагов."

// v060TestWant содержит явно заданные исходные диапазоны с исключающим концом.
// Нулевой related означает отсутствие связанных диапазонов.
type v060TestWant struct {
	location [3]int
	detail   string
	related  [3]int
}

func TestV060Registration(t *testing.T) {
	definitions, err := v060TestRegistry(t).ForVersion("1.2")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, definition := range definitions {
		if definition.Code != "V060" {
			continue
		}
		count++
		if definition.Severity != "recommendation" || definition.Scope != "line" ||
			definition.Message != v060TestMessage || definition.Check == nil ||
			definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "1.2"}) {
			t.Fatalf("unexpected definition: %+v", definition)
		}
	}
	if count != 1 {
		t.Fatalf("V060 definitions = %d, want 1", count)
	}
}

func TestV060ReferenceAndExample(t *testing.T) {
	// К неизменённым фрагментам добавлены только @dsl-version 1.2 и # Lesson.
	for _, tc := range []struct {
		name, last string
		want       []v060TestWant
	}{
		{"reference", "Review 3", nil},
		{"example", "Review 4", []v060TestWant{
			{[3]int{7, 12, 13}, "Пропущено значение: 3.", [3]int{5, 14, 15}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v060TestDocument(t, v060TestHeading(t, 1, "Lesson"),
				v060TestHeading(t, 3, "Part 1. Reading"), v060TestText(t, "Read.", "Read.", 3),
				v060TestHeading(t, 3, "Practice 2"), v060TestText(t, "Write.", "Write.", 5),
				v060TestHeading(t, 3, tc.last), v060TestText(t, "Speak.", "Speak.", 7))
			v060TestRun(t, result, "current", tc.want)
		})
	}
}

func TestV060Sequence(t *testing.T) {
	for _, tc := range []struct {
		name   string
		titles []string
		want   []v060TestWant
	}{
		{"no steps", nil, nil},
		{"no digits", []string{"Reading", "Practice"}, nil},
		{"single one", []string{"1"}, nil},
		{"single three", []string{"3"}, []v060TestWant{
			{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 3. Пропущены значения: 1 … 2.", [3]int{}},
		}},
		{"start two", []string{"2"}, []v060TestWant{
			{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 2. Пропущено значение: 1.", [3]int{}},
		}},
		{"start four", []string{"4"}, []v060TestWant{
			{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 4. Пропущены значения: 1 … 3.", [3]int{}},
		}},
		{"start zero", []string{"0"}, []v060TestWant{
			{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 0.", [3]int{}},
		}},
		{"correct sequence", []string{"1", "2", "3"}, nil},
		{"single gap", []string{"1", "3"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		}},
		{"range gap", []string{"1", "5"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Пропущены значения: 2 … 4.", [3]int{3, 5, 6}},
		}},
		{"continue from actual after gap", []string{"1", "3", "4"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		}},
		{"later value does not cancel gap", []string{"1", "3", "2"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
			{[3]int{7, 5, 6}, "Нарушен порядок: после 3 следует 2.", [3]int{5, 5, 6}},
		}},
		{"repeat then advance", []string{"1", "1", "2"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Повтор значения: 1.", [3]int{3, 5, 6}},
		}},
		{"return to old value", []string{"1", "2", "1"}, []v060TestWant{
			{[3]int{7, 5, 6}, "Нарушен порядок: после 2 следует 1.", [3]int{5, 5, 6}},
		}},
		{"every repetition and immediate previous", []string{"1", "1", "1"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Повтор значения: 1.", [3]int{3, 5, 6}},
			{[3]int{7, 5, 6}, "Повтор значения: 1.", [3]int{5, 5, 6}},
		}},
		{"bad start and decrease", []string{"3", "1", "2"}, []v060TestWant{
			{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 3. Пропущены значения: 1 … 2.", [3]int{}},
			{[3]int{5, 5, 6}, "Нарушен порядок: после 3 следует 1.", [3]int{3, 5, 6}},
		}},
		{"decrease to zero then continue", []string{"1", "0", "1"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Нарушен порядок: после 1 следует 0.", [3]int{3, 5, 6}},
		}},
		{"ignore headings without numbers", []string{"1", "Reading", "2"}, nil},
		{"numbers anywhere in title", []string{"Part 1. Exercise 2", "Review 3"}, nil},
		{"gap within title", []string{"1 03"}, []v060TestWant{
			{[3]int{3, 7, 9}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		}},
		{"repeat within title", []string{"1 1"}, []v060TestWant{
			{[3]int{3, 7, 8}, "Повтор значения: 1.", [3]int{3, 5, 6}},
		}},
		{"several violations on one line", []string{"1 4 2 2"}, []v060TestWant{
			{[3]int{3, 7, 8}, "Пропущены значения: 2 … 3.", [3]int{3, 5, 6}},
			{[3]int{3, 9, 10}, "Нарушен порядок: после 4 следует 2.", [3]int{3, 7, 8}},
			{[3]int{3, 11, 12}, "Повтор значения: 2.", [3]int{3, 9, 10}},
		}},
		{"punctuation separates groups", []string{"1.2 3-4"}, nil},
		{"letters tabs and combining marks separate", []string{"1a2\t3e\u03014"}, nil},
		{"signs do not belong to numbers", []string{"-1 +2"}, nil},
		{"leading zeros", []string{"001", "0002", "03"}, nil},
		{"source groups keep zeros", []string{"001", "003"}, []v060TestWant{
			{[3]int{5, 5, 8}, "Пропущено значение: 2.", [3]int{3, 5, 8}},
		}},
		{"zero group is preserved", []string{"000"}, []v060TestWant{
			{[3]int{3, 5, 8}, "Ожидалось начало с 1; найдено 0.", [3]int{}},
		}},
		{"scripts and leading zeros", []string{"١", "०२", "𝟛"}, nil},
		{"mixed scripts in one group", []string{"1", "٠٣"}, []v060TestWant{
			{[3]int{5, 5, 7}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		}},
		{"equal values in different scripts", []string{"01", "١"}, []v060TestWant{
			{[3]int{5, 5, 6}, "Повтор значения: 1.", [3]int{3, 5, 7}},
		}},
		{"other numeric categories excluded", []string{"1", "² Ⅲ", "3"}, []v060TestWant{
			{[3]int{7, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v060TestRun(t, v060TestSteps(t, tc.titles...), "current", tc.want)
		})
	}
}

func TestV060ArbitraryPrecision(t *testing.T) {
	const huge = "1000000000000000000000000000000"
	for _, tc := range []struct {
		name   string
		titles []string
		want   []v060TestWant
	}{
		{"large gap", []string{"1", huge}, []v060TestWant{
			{[3]int{5, 5, 36}, "Пропущены значения: 2 … 999999999999999999999999999999.", [3]int{3, 5, 6}},
		}},
		{"continue with huge successor", []string{"1", huge, "1000000000000000000000000000001"}, []v060TestWant{
			{[3]int{5, 5, 36}, "Пропущены значения: 2 … 999999999999999999999999999999.", [3]int{3, 5, 6}},
		}},
		{"huge first then gap", []string{huge, "1000000000000000000000000000002"}, []v060TestWant{
			{[3]int{3, 5, 36}, "Ожидалось начало с 1; найдено 1000000000000000000000000000000. Пропущены значения: 1 … 999999999999999999999999999999.", [3]int{}},
			{[3]int{5, 5, 36}, "Пропущено значение: 1000000000000000000000000000001.", [3]int{3, 5, 36}},
		}},
		{"carry across all digits", []string{"999999999999999999999999999999", huge}, []v060TestWant{
			{[3]int{3, 5, 35}, "Ожидалось начало с 1; найдено 999999999999999999999999999999. Пропущены значения: 1 … 999999999999999999999999999998.", [3]int{}},
		}},
		{"huge repeat and decrease", []string{huge, huge, "1"}, []v060TestWant{
			{[3]int{3, 5, 36}, "Ожидалось начало с 1; найдено 1000000000000000000000000000000. Пропущены значения: 1 … 999999999999999999999999999999.", [3]int{}},
			{[3]int{5, 5, 36}, "Повтор значения: 1000000000000000000000000000000.", [3]int{3, 5, 36}},
			{[3]int{7, 5, 6}, "Нарушен порядок: после 1000000000000000000000000000000 следует 1.", [3]int{5, 5, 36}},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v060TestRun(t, v060TestSteps(t, tc.titles...), "current", tc.want)
		})
	}
}

func TestV060DecimalScripts(t *testing.T) {
	// Цифры перечислены независимо от таблиц и помощников реализации.
	// Пять математических наборов находятся в одном диапазоне Unicode Nd.
	for _, digits := range []string{
		"0123456789", "٠١٢٣٤٥٦٧٨٩", "۰۱۲۳۴۵۶۷۸۹", "०१२३४५६७८९",
		"০১২৩৪৫৬৭৮৯", "๐๑๒๓๔๕๖๗๘๙", "၀၁၂၃၄၅၆၇၈၉", "０１２３４５６７８９",
		"\u07c0\u07c1\u07c2\u07c3\u07c4\u07c5\u07c6\u07c7\u07c8\u07c9",
		"\U000104a0\U000104a1\U000104a2\U000104a3\U000104a4\U000104a5\U000104a6\U000104a7\U000104a8\U000104a9",
		"\U00011f50\U00011f51\U00011f52\U00011f53\U00011f54\U00011f55\U00011f56\U00011f57\U00011f58\U00011f59",
		"\U0001e4f0\U0001e4f1\U0001e4f2\U0001e4f3\U0001e4f4\U0001e4f5\U0001e4f6\U0001e4f7\U0001e4f8\U0001e4f9",
		"𝟎𝟏𝟐𝟑𝟒𝟓𝟔𝟕𝟖𝟗", "𝟘𝟙𝟚𝟛𝟜𝟝𝟞𝟟𝟠𝟡", "𝟢𝟣𝟤𝟥𝟦𝟧𝟨𝟩𝟪𝟫",
		"𝟬𝟭𝟮𝟯𝟰𝟱𝟲𝟳𝟴𝟵", "𝟶𝟷𝟸𝟹𝟺𝟻𝟼𝟽𝟾𝟿",
	} {
		t.Run(digits, func(t *testing.T) {
			title := strings.Join(strings.Split(digits, ""), " ")
			// Первый 0 нарушает начало; дальнейшие 1..9 идут правильно.
			v060TestRun(t, v060TestSteps(t, title), "current", []v060TestWant{
				{[3]int{3, 5, 6}, "Ожидалось начало с 1; найдено 0.", [3]int{}},
			})
		})
	}
	t.Run("long group of leading zeros", func(t *testing.T) {
		v060TestRun(t, v060TestSteps(t, strings.Repeat("٠", 1000)+"1", "2"), "current", nil)
	})
	t.Run("mixed script multi-digit value", func(t *testing.T) {
		v060TestRun(t, v060TestSteps(t, "1 2 3 4 5 6 7 8 9 ١0 1१ １２"), "current", nil)
	})
}

func TestV060SourceRanges(t *testing.T) {
	t.Run("escaped raw differs from semantic title", func(t *testing.T) {
		line := v060TestLine(t, `### \{1\} 3`, "heading", 0,
			v060TestElement(t, "heading-level", "###", "3", 1),
			v060TestElement(t, "title", `\{1\} 3`, "{1} 3", 5))
		v060TestRun(t, v060TestDocument(t, v060TestHeading(t, 1, "Lesson"), line,
			v060TestText(t, "Read.", "Read.", 3)), "current", []v060TestWant{
			{[3]int{3, 11, 12}, "Пропущено значение: 2.", [3]int{3, 7, 8}},
		})
	})
	t.Run("indent and supplementary rune", func(t *testing.T) {
		line := v060TestLine(t, " \t### 😀 3", "heading", 0,
			v060TestElement(t, "heading-level", "###", "3", 3),
			v060TestElement(t, "title", "😀 3", "😀 3", 7))
		v060TestRun(t, v060TestDocument(t, v060TestHeading(t, 1, "Lesson"),
			v060TestHeading(t, 3, "1"), v060TestText(t, "Read.", "Read.", 3),
			line, v060TestText(t, "Read.", "Read.", 5)), "current", []v060TestWant{
			{[3]int{5, 9, 10}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		})
	})
	t.Run("supplementary digits retain one-column width", func(t *testing.T) {
		v060TestRun(t, v060TestSteps(t, "𝟙", "𝟘𝟛"), "current", []v060TestWant{
			{[3]int{5, 5, 7}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
		})
	})
}

func TestV060ExcludedContexts(t *testing.T) {
	h := v060TestHeading(t, 1, "Lesson")
	one, three := v060TestHeading(t, 3, "1"), v060TestHeading(t, 3, "3")
	read := v060TestText(t, "Read.", "Read.", 3)
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v060TestWant
	}{
		// # и ## размещены до шагов, согласно правилам их расположения.
		{"first level title ignored", []model.Line{
			v060TestHeading(t, 1, "2"), one, read, three, v060TestText(t, "Read.", "Read.", 5),
		}, []v060TestWant{{[3]int{5, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"second level title ignored", []model.Line{
			h, v060TestHeading(t, 2, "2"), one, v060TestText(t, "Read.", "Read.", 4),
			three, v060TestText(t, "Read.", "Read.", 6),
		}, []v060TestWant{{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{4, 5, 6}}}},
		{"header title ignored", []model.Line{
			h, one, read, v060TestTag(t, "header", "2", "title", 3), three,
		}, []v060TestWant{{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"ordinary content ignored", []model.Line{
			h, one, read, v060TestText(t, "2", "2", 3), three,
		}, []v060TestWant{{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"escaped heading ignored", []model.Line{
			h, one, read, v060TestText(t, `\### 2`, "### 2", 3), three,
		}, []v060TestWant{{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"unparsed attempt ignored", []model.Line{
			h, one, read, v060TestInvalid(t, "###2", 3), three,
		}, []v060TestWant{{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"newpage does not reset", []model.Line{
			// DSL 2.7.1: newpage начинает новый шаг, поэтому не является
			// потомком предыдущего заголовка (как в TestV033StepBoundaries).
			h, one, read, v060TestTag(t, "newpage", "", "", 0),
			v060TestText(t, "Read.", "Read.", 5), three,
		}, []v060TestWant{{[3]int{7, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"nested pseudoheading", []model.Line{
			h, one, read, v060TestOpen(t, "text", "", 3), v060TestInvalid(t, "### 2", 5),
			v060TestText(t, "Read.", "Read.", 5), v060TestClose(t, 5), three,
		}, []v060TestWant{{[3]int{9, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"fragment pseudoheading", []model.Line{
			h, one, read, v060TestOpen(t, "fragment", "intro", 3), v060TestInvalid(t, "### 2", 5),
			v060TestText(t, "Read.", "Read.", 5), v060TestClose(t, 5), three,
		}, []v060TestWant{{[3]int{9, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
		{"deeply nested pseudoheading", []model.Line{
			h, one, read, v060TestOpen(t, "instr", "", 3), v060TestOpen(t, "hint", "", 5),
			v060TestInvalid(t, "### 2", 6), v060TestClose(t, 6), v060TestClose(t, 5), three,
		}, []v060TestWant{{[3]int{10, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Во всех случаях последний элемент — заголовок 3; добавляем
			// содержимое его шага, не меняя позиции проверяемых чисел.
			body := append(append([]model.Line{}, tc.body...),
				v060TestText(t, "Read.", "Read.", len(tc.body)+1))
			v060TestRun(t, v060TestDocument(t, body...), "current", tc.want)
		})
	}
	for _, tag := range []string{"editor", "EDITOR", "multifill", "MULTIFILL"} {
		t.Run("literal "+tag, func(t *testing.T) {
			result := v060TestDocument(t, h, one, read, v060TestOpen(t, tag, "", 3),
				v060TestText(t, "### 2", "### 2", 5), v060TestClose(t, 5), three,
				v060TestText(t, "Read.", "Read.", 8))
			v060TestRun(t, result, "current", []v060TestWant{
				{[3]int{8, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}},
			})
		})
		t.Run("only literal "+tag, func(t *testing.T) {
			v060TestRun(t, v060TestDocument(t, h, v060TestOpen(t, tag, "", 2),
				v060TestText(t, "### 4", "### 4", 3), v060TestClose(t, 3)), "current", nil)
		})
	}
}

func TestV060EndingsAndHistory(t *testing.T) {
	for _, ending := range []model.LineEnding{"\n", "\r\n", ""} {
		t.Run(strconv.Quote(string(ending)), func(t *testing.T) {
			result := v060TestSteps(t, "1", "3")
			if ending == "" {
				result.Lines = result.Lines[:5]
				count := 5
				result.Document.LineCount = &count
				result.Lines[4].LineEnding = ""
			} else {
				for i := range result.Lines {
					result.Lines[i].LineEnding = ending
				}
			}
			want := []v060TestWant{{[3]int{5, 5, 6}, "Пропущено значение: 2.", [3]int{3, 5, 6}}}
			v060TestRun(t, result, "first", want)
			v060TestRun(t, result, "second", want)
		})
	}
}

func TestV060UnavailableLinesAndVersions(t *testing.T) {
	for _, version := range []string{"1.2", "1.1", "1.3"} {
		t.Run(version, func(t *testing.T) {
			result := v060TestDocument(t)
			result.Document.DSLVersion = &version
			result.Lines = []model.Line{}
			count := 0
			result.Document.LineCount = &count
			if version == "1.2" {
				v060TestRun(t, result, "empty", nil)
				result.Document.LineCount = nil
				v060TestRun(t, result, "unavailable", nil)
				return
			}
			before := v060TestSnapshot(t, result)
			checks, err := validator.Prepare(result, v060TestRegistry(t))
			if err == nil || checks != nil || v060TestSnapshot(t, result) != before {
				t.Fatalf("unsupported version: checks=%v, err=%v", checks, err)
			}
		})
	}
}

func TestV060RecommendationDoesNotSetErrorFlags(t *testing.T) {
	// V037 требует document-id. Этот тест проверяет общие флаги, поэтому
	// исключаем её независимую ошибку корректным тегом по DSL 1.32.1.
	id := "sequence-check"
	result := v060TestDocument(t, v060TestTag(t, "document-id", id, "identifier", 0),
		v060TestHeading(t, 1, "Lesson"),
		v060TestHeading(t, 3, "1"), v060TestText(t, "Read.", "Read.", 4),
		v060TestHeading(t, 3, "3"), v060TestText(t, "Read.", "Read.", 6))
	result.Document.Metadata.DocumentID = &id
	v060TestRun(t, result, "current", []v060TestWant{
		{[3]int{6, 5, 6}, "Пропущено значение: 2.", [3]int{4, 5, 6}},
	})
	if result.Document.HasErrors {
		t.Fatalf("valid DSL with a recommendation has errors: %+v", result.Diagnostics)
	}
	for _, line := range result.Lines {
		if line.HasErrors {
			t.Errorf("line %d unexpectedly has errors", line.Number)
		}
	}
}

// v060TestLine задаёт элементы и родителя явно, не распознавая Raw.
func v060TestLine(t *testing.T, raw string, kind model.LineType, parent int, elements ...model.Element) model.Line {
	t.Helper()
	line := model.Line{Raw: raw, LineType: kind, LineEnding: "\n", Elements: append([]model.Element{}, elements...)}
	if parent != 0 {
		line.ParentLine = &parent
	}
	return line
}

// v060TestElement вычисляет длину входного элемента, но не ожидания V060.
func v060TestElement(t *testing.T, kind model.ElementType, raw, value string, start int) model.Element {
	t.Helper()
	return model.Element{ElementType: kind, Raw: raw, Value: &value, Start: start,
		End: start + utf8.RuneCountInString(raw), ErrorIDs: []string{}}
}

// v060TestHeading строит корневой заголовок с заданным уровнем и названием.
func v060TestHeading(t *testing.T, level int, title string) model.Line {
	t.Helper()
	marker := strings.Repeat("#", level)
	return v060TestLine(t, marker+" "+title, "heading", 0,
		v060TestElement(t, "heading-level", marker, strconv.Itoa(level), 1),
		v060TestElement(t, "title", title, title, level+2))
}

// v060TestText сохраняет независимые исходное и семантическое значения.
func v060TestText(t *testing.T, raw, value string, parent int) model.Line {
	t.Helper()
	return v060TestLine(t, raw, "content", parent, v060TestElement(t, "content", raw, value, 1))
}

// v060TestTag строит объявление известного тега с заданным типом параметра.
func v060TestTag(t *testing.T, tag, parameter string, kind model.ElementType, parent int) model.Line {
	t.Helper()
	raw := "@" + tag
	elements := []model.Element{v060TestElement(t, "tag", raw, strings.ToLower(tag), 1)}
	if parameter != "" {
		elements = append(elements, v060TestElement(t, kind, parameter, parameter, len(raw)+2))
		raw += " " + parameter
	}
	return v060TestLine(t, raw, "tag", parent, elements...)
}

// v060TestOpen строит объявление блока с необязательным идентификатором.
func v060TestOpen(t *testing.T, tag, identifier string, parent int) model.Line {
	t.Helper()
	line := v060TestTag(t, tag, identifier, "identifier", parent)
	line.Elements = append(line.Elements, v060TestElement(t, "block-open", "{", "{", len(line.Raw)+2))
	line.Raw += " {"
	line.LineType = "block-start"
	return line
}

// v060TestClose задаёт закрывающую строку с родителем — открытием блока.
func v060TestClose(t *testing.T, parent int) model.Line {
	t.Helper()
	return v060TestLine(t, "}", "block-end", parent, v060TestElement(t, "block-close", "}", "}", 1))
}

// v060TestInvalid задаёт ошибочную неразобранную попытку конструкции.
func v060TestInvalid(t *testing.T, raw string, parent int) model.Line {
	t.Helper()
	element := v060TestElement(t, "unparsed", raw, "", 1)
	element.Value = nil
	return v060TestLine(t, raw, "invalid", parent, element)
}

// v060TestSteps строит документ с заданными названиями шагов и содержимым Read.
// Числа из названий не извлекаются; каждый шаг имеет заголовок и дочернюю строку.
func v060TestSteps(t *testing.T, titles ...string) *model.Result {
	t.Helper()
	body := []model.Line{v060TestHeading(t, 1, "Lesson")}
	for _, title := range titles {
		parent := len(body) + 2
		body = append(body, v060TestHeading(t, 3, title), v060TestText(t, "Read.", "Read.", parent))
	}
	return v060TestDocument(t, body...)
}

// v060TestDocument добавляет версию к заданным строкам, не выполняя разбор DSL.
// Глубины следуют явным родителям. Для unparsed создаются исходные ошибки
// разбора со ссылками; метаданные берутся только из готовых элементов заголовков.
func v060TestDocument(t *testing.T, body ...model.Line) *model.Result {
	t.Helper()
	lines := append([]model.Line{v060TestTag(t, "dsl-version", "1.2", "version", 0)}, body...)
	version, count := "1.2", len(lines)
	result := &model.Result{
		Format: model.FormatVersion1,
		Document: model.Document{DSLVersion: &version, LineCount: &count,
			Metadata: model.DocumentMetadata{ResourceDirs: []string{}}},
		Processing: []model.Processing{{ID: "parser", Tool: "dsl-parser"}},
		Lines:      lines, Diagnostics: []model.Diagnostic{},
	}
	titles := map[string][]string{}
	for i := range result.Lines {
		line := &result.Lines[i]
		line.Number = i + 1
		line.Elements = append([]model.Element{}, line.Elements...)
		if line.ParentLine != nil {
			parent := *line.ParentLine
			if parent < 1 || parent >= line.Number {
				t.Fatalf("invalid test parent %d on line %d", parent, line.Number)
			}
			line.NestingLevel = result.Lines[parent-1].NestingLevel + 1
		}
		level, title := "", ""
		for j := range line.Elements {
			element := &line.Elements[j]
			if element.ElementType == "heading-level" {
				level = *element.Value
			}
			if element.ElementType == "title" {
				title = *element.Value
			}
			if element.ElementType != "unparsed" {
				continue
			}
			id := "p" + strconv.Itoa(len(result.Diagnostics)+1)
			element.ErrorIDs = []string{id}
			line.HasErrors, result.Document.HasErrors = true, true
			result.Diagnostics = append(result.Diagnostics, model.Diagnostic{
				ID: id, Source: "parser", DiagnosticCode: "P001", SeverityLevel: "error", DiagnosticScope: "element",
				Message: "Ошибка разбора строки.", RelatedLocations: []model.Location{},
				Location: &model.Location{Start: model.Position{Line: line.Number, Column: element.Start},
					End: model.Position{Line: line.Number, Column: element.End}},
			})
		}
		if line.LineType == "heading" {
			titles[level] = append(titles[level], title)
		}
	}
	if len(titles["1"]) == 1 {
		result.Document.Metadata.Title = &titles["1"][0]
	}
	if len(titles["2"]) == 1 {
		result.Document.Metadata.Subtitle = &titles["2"][0]
	}
	return result
}

// v060TestRegistry использует только полный штатный каталог регистраций.
func v060TestRegistry(t *testing.T) *diagnostics.Registry {
	t.Helper()
	registry, err := diagnostics.NewRegistry(RegistrationCheckers())
	if err != nil {
		t.Fatal(err)
	}
	return registry
}

// v060TestRun проверяет новые V060 после штатных Prepare/Run по явным ожиданиям.
func v060TestRun(t *testing.T, result *model.Result, processingID string, want []v060TestWant) {
	t.Helper()
	before := v060TestSnapshot(t, result)
	oldCount := len(result.Diagnostics)
	checks, err := validator.Prepare(result, v060TestRegistry(t))
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if v060TestSnapshot(t, result) != before {
		t.Fatal("Prepare changed the model")
	}
	result.Processing = append(result.Processing, model.Processing{ID: processingID, Tool: "dsl-validator"})
	if err := validator.Run(result, checks, processingID); err != nil {
		t.Fatalf("Run: %v", err)
	}
	var got []model.Diagnostic
	for _, entry := range result.Diagnostics[oldCount:] {
		if entry.DiagnosticCode == "V060" {
			got = append(got, entry)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("V060 count = %d, want %d; got %+v", len(got), len(want), got)
	}
	for i, expected := range want {
		entry := got[i]
		location := v060TestLocation(expected.location)
		related := []model.Location{}
		if expected.related != ([3]int{}) {
			related = append(related, v060TestLocation(expected.related))
		}
		if entry.Location == nil || *entry.Location != location || !reflect.DeepEqual(entry.RelatedLocations, related) ||
			entry.Message != v060TestMessage+"\nконкретизация: "+expected.detail ||
			entry.SeverityLevel != "recommendation" || entry.DiagnosticScope != "line" || entry.Fatal ||
			entry.ID == "" || entry.Source != processingID {
			t.Fatalf("V060 occurrence %d = %+v; want %+v, related %+v, detail %q", i, entry, location, related, expected.detail)
		}
	}
	if err := validator.CheckModel(result); err != nil {
		t.Fatalf("result integrity: %v", err)
	}
	var original, restored model.Result
	if err := json.Unmarshal([]byte(before), &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(v060TestSnapshot(t, result)), &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Lines) != len(original.Lines) || len(restored.Processing) != len(original.Processing)+1 {
		t.Fatal("source or history length changed")
	}
	// Восстанавливаются только производные флаги и ссылки; остальное
	// должно совпадать с исходной моделью и историей предыдущих запусков.
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
	if v060TestSnapshot(t, &restored) != before {
		t.Fatal("source, metadata or previous history changed")
	}
}

// v060TestLocation переводит явно заданные числа в исходный диапазон модели.
func v060TestLocation(position [3]int) model.Location {
	return model.Location{Start: model.Position{Line: position[0], Column: position[1]},
		End: model.Position{Line: position[0], Column: position[2]}}
}

// v060TestSnapshot сериализует модель для контроля побочных изменений.
func v060TestSnapshot(t *testing.T, result *model.Result) string {
	t.Helper()
	encoded, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
