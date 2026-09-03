package diagnostics

import (
	"testing"

	"dslparser/validatorapi"
)

// validDefinition создаёт полное тестовое определение.
func validDefinition(code string, scope validatorapi.VersionScope) validatorapi.Definition {
	return validatorapi.Definition{Passport: validatorapi.Passport{Code: code, Category: "категория", Severity: validatorapi.SeverityWarning, Title: "название", Description: "описание", Basis: "основание", Reference: "эталон", BadExample: "пример"}, Versions: scope, Check: func(validatorapi.Context, validatorapi.Reporter) error { return nil }}
}

func TestValidateRejectsInvalidRegistryContracts(t *testing.T) {
	cases := []struct {
		name        string
		definitions []validatorapi.Definition
		supported   []validatorapi.Version
	}{
		{"empty supported", nil, nil},
		{"duplicate supported", nil, []validatorapi.Version{"1.1", "1.1"}},
		{"blank passport", []validatorapi.Definition{validDefinition("CODE", validatorapi.VersionScope{All: true})}, []validatorapi.Version{"1.1"}},
		{"duplicate scope", []validatorapi.Definition{validDefinition("CODE", validatorapi.VersionScope{Versions: []validatorapi.Version{"1.1", "1.1"}})}, []validatorapi.Version{"1.1"}},
	}
	cases[2].definitions[0].Passport.Title = " "
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validate(tc.definitions, tc.supported); err == nil {
				t.Fatal("invalid registry accepted")
			}
		})
	}
}

func TestValidateAcceptsDisjointVersionScopes(t *testing.T) {
	definitions := []validatorapi.Definition{validDefinition("CODE", validatorapi.VersionScope{Versions: []validatorapi.Version{"1.0"}}), validDefinition("CODE", validatorapi.VersionScope{Versions: []validatorapi.Version{"1.1"}})}
	if err := validate(definitions, []validatorapi.Version{"1.0", "1.1"}); err != nil {
		t.Fatal(err)
	}
}
