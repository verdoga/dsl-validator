package console

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestPrintHelpDescribesUsageAndRules(t *testing.T) {
	var output bytes.Buffer
	if err := PrintHelp(&output); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "dsl-validator [--depth N] <path>") {
		t.Error("help does not contain the required usage line")
	}
	help := strings.ToLower(strings.Join(strings.Fields(output.String()), " "))
	tests := []struct {
		name      string
		fragments []string
	}{
		{"source", []string{"один обычный файл .json", "без учёта регистра", "или каталог"}},
		{"empty_path", []string{"пустой аргумент", "означает текущий каталог", "отсутствие пути — ошибка"}},
		{"symlinks", []string{"символические ссылки в пути не допускаются"}},
		{"depth_limit", []string{"предел уровней подкаталогов", "целое число n не меньше нуля"}},
		{"unlimited_depth", []string{"без --depth глубина не ограничена"}},
		{"zero_depth", []string{"при n=0 обрабатывается только исходный каталог"}},
		{"file_depth", []string{"для файла глубина игнорируется"}},
		{"help_flags", []string{"--help", "-h", "показать справку"}},
		{"separator", []string{"-- завершить разбор флагов", "далее передаётся путь"}},
		{"flag_order", []string{"флаги указываются до пути", "повторный --depth запрещён"}},
		{"strict_forms", []string{"-depth, --depth=n, -help и --h не поддерживаются"}},
		{"help_priority", []string{"до пути и разделителя --", "запрос справки имеет приоритет над ошибками аргументов"}},
		{"in_place_updates", []string{"json обновляются по исходному пути", "--replace не поддерживается"}},
		{"success", []string{"0 — штатное завершение или справка"}},
		{"findings_do_not_fail", []string{"любое число диагностических срабатываний", "error", "warning", "recommendation", "не меняет код 0"}},
		{"technical_failure", []string{"1 — технический отказ", "ошибка подготовки, обхода, обработки или вывода"}},
		{"usage_failure", []string{"2 — неверные аргументы или непригодный исходный путь"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, fragment := range tt.fragments {
				if !strings.Contains(help, fragment) {
					t.Errorf("help is missing %q", fragment)
				}
			}
		})
	}
}

func TestPrintHelpReturnsWriterError(t *testing.T) {
	want := errors.New("запись недоступна")
	reader, writer := io.Pipe()
	t.Cleanup(func() {
		if err := writer.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := reader.CloseWithError(want); err != nil {
		t.Fatal(err)
	}
	if err := PrintHelp(writer); !errors.Is(err, want) {
		t.Fatalf("PrintHelp returned %v, want writer error %v", err, want)
	}
}
