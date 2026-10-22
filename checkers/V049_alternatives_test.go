package checkers

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestV049MultifillAlternativeAnswers(t *testing.T) {
	for _, tc := range []struct {
		name, raw string
		want      []v049Expected
	}{
		{"two alternatives", "_____{A}{B}", nil},
		{"three alternatives", "_____{A}{B}{C}", nil},
		{"escaped closing in alternative", `_____{A}{B\}C}{D}`, nil},
		{"escaped opening in alternative", `_____{A}{B\{C}{D}`, nil},
		{"escaped backslash in alternative", `_____{A}{B\\}{C}`, nil},
		{"odd backslashes before closing", `_____{A}{B\\\}C}{D}`, nil},
		{"empty first answer", "_____{}{B}", nil},
		{"empty alternative", "_____{A}{}{B}", nil},
		{"unterminated alternative", "_____{A}{B", nil},
		{"unterminated escaped alternative", `_____{A}{B\}`, nil},
		{"unterminated trailing backslash", `_____{A}{B\`, nil},
		{"separate placeholders", "_____{A}{B} _____{C}{D}", nil},
		{"adjacent placeholders", "_____{A}{B}_____{C}{D}", nil},
		{"literal backslash before placeholder", `\\_____{A}{B}`, nil},
		{"extra closing", "_____{A}{B}}", []v049Expected{{4, 12, 13}}},
		{"closing in following text", "_____{A}{B} x}", []v049Expected{{4, 14, 15}}},
		{"space breaks alternatives", "_____{A} {B}", []v049Expected{{4, 12, 13}}},
		{"text breaks alternatives", "_____{A}x{B}", []v049Expected{{4, 12, 13}}},
		{"escaped opening breaks alternatives", `_____{A}\{B}{C}`, []v049Expected{{4, 12, 13}, {4, 15, 16}}},
		{"no nesting in alternative", "_____{A}{B{C}}", []v049Expected{{4, 14, 15}}},
		{"four underscores", "____{A}{B}", []v049Expected{{4, 7, 8}, {4, 10, 11}}},
		{"six underscores", "______{A}{B}", []v049Expected{{4, 9, 10}, {4, 12, 13}}},
		{"escaped underscores", `\_____{A}{B}`, []v049Expected{{4, 9, 10}, {4, 12, 13}}},
		{"escaped initial opening", `_____\{A}{B}`, []v049Expected{{4, 9, 10}, {4, 12, 13}}},
		{"Unicode columns after alternatives", "Я😀_____{А}{Б}}", []v049Expected{{4, 14, 15}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := v049Document(t, nil, []model.Line{
				v049Block(t, "multifill", 2, 1), v049Text(t, tc.raw, 3, 2), v049Close(t, 3, 2),
			})
			v049Run(t, result, "current", tc.want)
		})
	}
}

func TestV049AlternativeAnswersRequireMultifillBlock(t *testing.T) {
	for _, tag := range []string{"editor", "note", "text"} {
		t.Run(tag, func(t *testing.T) {
			result := v049Document(t, nil, []model.Line{
				v049Block(t, tag, 2, 1), v049Text(t, "_____{A}{B}", 3, 2), v049Close(t, 3, 2),
			})
			v049Run(t, result, "current", []v049Expected{{4, 8, 9}, {4, 11, 12}})
		})
	}
}

func TestV049AlternativesDoNotContinueOnNextLine(t *testing.T) {
	for _, first := range []string{"_____{A}", "_____{A}{B"} {
		t.Run(first, func(t *testing.T) {
			result := v049Document(t, nil, []model.Line{
				v049Block(t, "multifill", 2, 1), v049Text(t, first, 3, 2),
				v049Text(t, "{C}", 3, 2), v049Close(t, 3, 2),
			})
			v049Run(t, result, "current", []v049Expected{{5, 3, 4}})
		})
	}
}

func TestV049ReviewRegression(t *testing.T) {
	data, err := os.ReadFile("testdata/V049_review.json")
	if err != nil {
		t.Fatal(err)
	}
	var result model.Result
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	// Путь из Windows не участвует в проверке и может быть невалиден на Unix.
	result.Document.FilePath = nil
	// Исходный отчёт содержит шесть ложных V049. История сохраняется,
	// повторные запуски не должны добавлять новые срабатывания.
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.DiagnosticCode == "V049" {
			count++
		}
	}
	if count != 6 {
		t.Fatalf("fixture V049 count=%d, want 6", count)
	}
	v049Run(t, &result, "regression-first", nil)
	v049Run(t, &result, "regression-second", nil)
}
