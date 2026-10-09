package diagnostics

import (
	"errors"
	"testing"
)

// Присваиваемость в обе стороны проверяет совместимость типов функций
// с предусмотренными аргументами и возвращаемой ошибкой.
var (
	_ CheckFunc                   = (func(Lines, Findings) error)(nil)
	_ func(Lines, Findings) error = (CheckFunc)(nil)
	_ RegistrationFunc            = (func(*Registry) error)(nil)
	_ func(*Registry) error       = (RegistrationFunc)(nil)
)

func TestDefinitionCanRepresentAbsentCheck(t *testing.T) {
	var definition Definition
	if definition.Check != nil {
		t.Error("zero Definition.Check is not nil")
	}
}

func TestCheckFuncReturnsErrorUnchanged(t *testing.T) {
	failure := errors.New("technical check failure")
	cases := []struct {
		name string
		err  error
	}{
		{"success", nil},
		{"technical failure", failure},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var check CheckFunc = func(Lines, Findings) error {
				return tc.err
			}
			definition := Definition{Check: check}
			if got := definition.Check(nil, nil); got != tc.err {
				t.Errorf("Check returned %v, want the original error %v", got, tc.err)
			}
		})
	}
}
