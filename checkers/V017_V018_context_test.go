package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/model"
	"github.com/verdoga/dsl-validator/validator"
)

func TestV017V018Context(t *testing.T) {
	for _, tc := range []struct {
		name, code, raw string
		want            []string
	}{
		{"before space", "V017", "Вначале слово , дальше конец", []string{"слово , дальше"}},
		{"after sign", "V018", "Вначале написано слово,дальше в конце", []string{"написано слово,дальше в"}},
		{"multiple before", "V017", "One , two three four ! five", []string{"One , two", "four ! five"}},
		{"multiple after", "V018", "One,two three four five!six seven", []string{"One,two three four", "four five!six seven"}},
		{"start", "V018", ",А", []string{",А"}},
		{"end", "V017", "Wait !", []string{"Wait !"}},
		{"spaces", "V017", "Word\t\u00a0!\u202fnext rest", []string{"Word\t\u00a0!\u202fnext"}},
		{"Unicode", "V018", "до Я😀,Б после конца", []string{"до Я😀,Б после"}},
		{"quotes", "V018", "before «Hi!»Next after end", []string{"before «Hi!»Next after"}},
		{"HTML before", "V017", "before <em>word </em><strong>,</strong> next end", []string{"word , next"}},
		{"HTML after", "V018", "before <em>Hello,</em><strong>world</strong> next end", []string{"before Hello,world next"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v020TextBlock(t, tc.raw)
			v017V018CheckMessages(t, result, tc.code, tc.want)
		})
	}
}

func TestV017V018AnswerContext(t *testing.T) {
	for _, separate := range []bool{false, true} {
		v017V018CheckMessages(t, v015AnswerDocument(t, "outside ", "Word , next", " after", separate),
			"V017", []string{"Word , next"})
		v017V018CheckMessages(t, v015AnswerDocument(t, "outside ", "Hello,world", " after", separate),
			"V018", []string{"Hello,world"})
		// HTML внутри ответа является буквальным текстом.
		v017V018CheckMessages(t, v015AnswerDocument(t, "outside ", "<em>Hi,Next</em>", " after", separate),
			"V018", []string{"<em>Hi,Next</em>"})
	}
}

// v017V018CheckMessages сравнивает полные сообщения с явно заданными примерами.
func v017V018CheckMessages(t *testing.T, result *model.Result, code string, snippets []string) {
	t.Helper()
	checks, err := validator.Prepare(result, v018Registry(t))
	if err != nil {
		t.Fatal(err)
	}
	result.Processing = append(result.Processing, model.Processing{ID: "context", Tool: "dsl-validator"})
	if err := validator.Run(result, checks, "context"); err != nil {
		t.Fatal(err)
	}
	base := "Перед знаком препинания находится пробел."
	if code == "V018" {
		base = "После знака препинания отсутствует пробел."
	}
	var messages []string
	for _, entry := range result.Diagnostics {
		if string(entry.DiagnosticCode) == code && entry.Source == "context" {
			messages = append(messages, entry.Message)
		}
	}
	if len(messages) != len(snippets) {
		t.Fatalf("%s messages = %q, want snippets %q", code, messages, snippets)
	}
	for i, snippet := range snippets {
		want := base + "\nконкретизация: Обратите внимание на знак препинания: «" + snippet + "»"
		if messages[i] != want {
			t.Errorf("message %d = %q, want %q", i, messages[i], want)
		}
	}
}
