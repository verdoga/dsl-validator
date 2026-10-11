package validator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/verdoga/dsl-validator/diagnostics"
	dslversions "github.com/verdoga/dsl-validator/dsl-versions"
	"github.com/verdoga/dsl-validator/model"
)

func TestPrepareRejectsNilArgumentsAndUnpreparedRegistry(t *testing.T) {
	registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionAll}, "V001")
	for _, tc := range []struct {
		name     string
		result   *model.Result
		registry *diagnostics.Registry
	}{
		{"nil result", nil, registry},
		{"nil registry", integrityModel(t), nil},
		{"both nil", nil, nil},
		{"unprepared registry", integrityModel(t), &diagnostics.Registry{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			checks, err := prepareWithoutChanges(t, tc.result, tc.registry)
			if err == nil || checks != nil {
				t.Fatalf("Prepare() = %v, %v; want nil and error", checks, err)
			}
		})
	}
	if *calls != 0 {
		t.Errorf("Prepare called Check %d times", *calls)
	}
}

func TestPrepareVersionRefusalPrecedesIntegrityAndIgnoresRaw(t *testing.T) {
	text := func(s string) *string { return &s }
	for _, tc := range []struct {
		name    string
		version *string
		display string
	}{
		{"absent", nil, "не задана"},
		{"empty", text(""), "не задана"},
		{"leading zero", text("01.2"), "01.2"},
		{"space", text(" 1.2"), " 1.2"},
		{"prefix", text("v1.2"), "v1.2"},
		{"unsupported", text("2.0"), "2.0"},
		{"unsupported minor", text("1.10"), "1.10"},
	} {
		for _, broken := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/broken=%v", tc.name, broken), func(t *testing.T) {
				registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionAll}, "V001")
				r := integrityModel(t)
				r.Document.DSLVersion = tc.version
				if broken {
					r.Format = "invalid"
					r.Lines[0].Raw = "@dsl-version 1.2"
				}
				checks, err := prepareWithoutChanges(t, r, registry)
				if checks != nil || err == nil {
					t.Fatalf("Prepare() = %v, %v; want nil and version error", checks, err)
				}
				want := "версия DSL " + tc.display + " не поддерживается"
				if !strings.Contains(err.Error(), want) {
					t.Errorf("version refusal lost priority or substituted a version: %v", err)
				}
				if *calls != 0 {
					t.Errorf("Prepare called Check %d times", *calls)
				}
			})
		}
	}
}

func TestPrepareRejectsEmptySelectionBeforeIntegrity(t *testing.T) {
	for _, emptyRegistry := range []bool{false, true} {
		for _, broken := range []bool{false, true} {
			t.Run(fmt.Sprintf("emptyRegistry=%v/broken=%v", emptyRegistry, broken), func(t *testing.T) {
				codes := []diagnostics.Code{"V001"}
				if emptyRegistry {
					codes = nil
				}
				registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: "2.0"}, codes...)
				r := integrityModel(t)
				if broken {
					r.Format = "invalid"
				}
				checks, err := prepareWithoutChanges(t, r, registry)
				if checks != nil || err == nil {
					t.Fatalf("Prepare() = %v, %v; want nil and empty-selection error", checks, err)
				}
				if !strings.Contains(err.Error(), "1.2") || !strings.Contains(err.Error(), "нет подходящих диагностик") {
					t.Errorf("empty-selection error lacks version or lost priority: %v", err)
				}
				if *calls != 0 {
					t.Errorf("Prepare called Check %d times", *calls)
				}
			})
		}
	}
}

func TestPrepareRejectsBrokenModelAfterSelection(t *testing.T) {
	registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionAll}, "V001")
	r := integrityModel(t)
	r.Lines[1].Elements[0].ErrorIDs[0] = "missing"
	checks, err := prepareWithoutChanges(t, r, registry)
	if checks != nil || err == nil {
		t.Fatalf("Prepare() = %v, %v; want nil and integrity error", checks, err)
	}
	if !strings.Contains(err.Error(), "целостность модели") || !strings.Contains(err.Error(), "missing") {
		t.Errorf("expected model integrity failure, got %v", err)
	}
	if *calls != 0 {
		t.Errorf("Prepare called Check %d times", *calls)
	}
}

func TestPrepareReturnsNumericCodeOrderAndIndependentSelection(t *testing.T) {
	registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: dslversions.V1_2}, "V1000", "V010", "V999", "V002", "V001")
	r := integrityModel(t)
	want := []diagnostics.Code{"V001", "V002", "V010", "V999", "V1000"}
	for attempt := 0; attempt < 2; attempt++ {
		checks, err := prepareWithoutChanges(t, r, registry)
		if err != nil || len(checks) != len(want) {
			t.Fatalf("Prepare() = %v, %v", checks, err)
		}
		for i, code := range want {
			definition := checks[i]
			if definition.Code != code || definition.Message != "message "+string(code) || definition.Check == nil ||
				definition.Severity != diagnostics.SeverityWarning || definition.Scope != diagnostics.ScopeDocument ||
				definition.Versions != (diagnostics.VersionRule{Mode: diagnostics.VersionExact, Version: dslversions.V1_2}) {
				t.Errorf("definition %d changed or out of numeric order: %+v", i, definition)
			}
		}
		// Изменение полученной выборки не должно менять следующий результат
		// или опубликованный реестр.
		checks[0] = diagnostics.Definition{}
	}
	if *calls != 0 {
		t.Errorf("Prepare called Check %d times", *calls)
	}
}

func TestPrepareUsesStoredVersionWithoutParsingLines(t *testing.T) {
	registry, calls := prepareRegistry(t, diagnostics.VersionRule{Mode: diagnostics.VersionAll}, "V001")
	r := integrityModel(t)
	// Согласованная физическая строка содержит другую версию; допуск должен
	// по-прежнему использовать только сохранённый Document.DSLVersion.
	text := "@dsl-version 2.0"
	r.Lines[3].LineType, r.Lines[3].Raw = model.LineTypeContent, text
	r.Lines[3].Elements = []model.Element{{ElementType: model.ElementTypeContent, Raw: text, Value: &text, Start: 1, End: len(text) + 1, ErrorIDs: []string{}}}
	checks, err := prepareWithoutChanges(t, r, registry)
	if err != nil || len(checks) != 1 || *calls != 0 {
		t.Fatalf("Prepare() = %v, %v; Check calls = %d", checks, err, *calls)
	}
}

func prepareRegistry(t *testing.T, rule diagnostics.VersionRule, codes ...diagnostics.Code) (*diagnostics.Registry, *int) {
	t.Helper()
	calls := 0
	registry, err := diagnostics.NewRegistry(map[string]diagnostics.RegistrationFunc{
		"checks": func(registry *diagnostics.Registry) error {
			for _, code := range codes {
				if err := registry.Register(diagnostics.Definition{
					Code: code, Severity: diagnostics.SeverityWarning, Scope: diagnostics.ScopeDocument,
					Message: "message " + string(code), Versions: rule,
					Check: func(diagnostics.Lines, diagnostics.Findings) error { calls++; return nil },
				}); err != nil {
					return err
				}
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return registry, &calls
}

func prepareWithoutChanges(t *testing.T, r *model.Result, registry *diagnostics.Registry) ([]diagnostics.Definition, error) {
	t.Helper()
	modelBefore := integritySnapshot(t, r)
	registryBefore := prepareRegistrySnapshot(t, registry)
	checks, err := Prepare(r, registry)
	if integritySnapshot(t, r) != modelBefore {
		t.Error("Prepare changed model data or pointers")
	}
	if prepareRegistrySnapshot(t, registry) != registryBefore {
		t.Error("Prepare changed registry selection or registration count")
	}
	return checks, err
}

func prepareRegistrySnapshot(t *testing.T, registry *diagnostics.Registry) string {
	t.Helper()
	if registry == nil {
		return "nil"
	}
	definitions, err := registry.ForVersion(dslversions.V1_2)
	return fmt.Sprintf("%d %#v %v", registry.Len(), definitions, err)
}
