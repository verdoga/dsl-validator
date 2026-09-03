package pipeline

import (
	"errors"
	"fmt"
	"os"
	"time"

	"dslparser/Diagnostics"
	"dslparser/internal/parseradapter"
	"dslparser/internal/report"
	"dslparser/validatorapi"
)

// ErrVersionExcluded обозначает штатное исключение файла выбранной версией.
var ErrVersionExcluded = errors.New("файл исключён выбором версии")

// Runner обрабатывает один файл без параллельных диагностик.
type Runner struct {
	parser parseradapter.Adapter
	now    func() time.Time
}

// New создаёт production runner.
func New(parser parseradapter.Adapter) Runner { return Runner{parser: parser, now: time.Now} }

// Version открывает файл и определяет версию единственным парсером.
func (r Runner) Version(path string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()
	result := r.parser.ParseReader(f)
	if result.Document != nil {
		return string(result.Document.Version()), true, nil
	}
	return "", false, nil
}

// Run обрабатывает файл в режиме auto либо explicit и записывает готовый отчёт.
func (r Runner) Run(path, mode string, selected *string) (report.Report, error) {
	now := r.now
	if now == nil {
		now = time.Now
	}
	started := now()
	out := report.New(path, mode, selected, started)
	if _, err := os.Stat(path); err != nil {
		addValidator(&out, "FILE_INFO_FAILURE", err.Error())
		stage := "opening"
		out.Analysis.Status, out.Analysis.FailedStage = "incomplete", &stage
		out.Finish(now())
		return out, report.WriteAtomic(out)
	}
	file, err := os.Open(path)
	if err != nil {
		addValidator(&out, "FILE_OPEN_FAILURE", err.Error())
		stage := "opening"
		out.Analysis.Status = "incomplete"
		out.Analysis.FailedStage = &stage
		out.Finish(now())
		return out, report.WriteAtomic(out)
	}
	result := r.parser.ParseReader(file)
	closeErr := file.Close()
	if closeErr != nil {
		addValidator(&out, "FILE_CLOSE_FAILURE", closeErr.Error())
	}
	if result.Fatal != nil {
		addParser(&out, string(result.Fatal.Code), result.Fatal.Message, nil, nil)
		stage := "parsing"
		out.Analysis.Status = "incomplete"
		out.Analysis.FailedStage = &stage
	} else if result.Failure != nil {
		addValidator(&out, result.Failure.Code, result.Failure.Error())
		stage := "parsing"
		out.Analysis.Status = "incomplete"
		out.Analysis.FailedStage = &stage
	} else {
		raw, canonical := result.Document.VersionRaw(), string(result.Document.Version())
		out.DSLVersion = report.DSLVersion{Status: "determined", Raw: &raw, Canonical: &canonical}
		if mode == "explicit" && selected != nil && canonical != *selected {
			return out, fmt.Errorf("%w: версия %s, выбрана %s", ErrVersionExcluded, canonical, *selected)
		}
		for _, issue := range result.Document.ParserIssues() {
			s := issue.Span()
			var related *validatorapi.Span
			if value, ok := issue.RelatedSpan(); ok {
				related = &value
			}
			addParser(&out, string(issue.Code()), issue.Message(), &s, related)
		}
		runDefinitions(&out, result.Context, diagnostics.ForVersion(result.Document.Version()))
	}
	out.Finish(now())
	if err := report.WriteAtomic(out); err != nil {
		return out, err
	}
	return out, nil
}

// addParser добавляет parser entry.
func addParser(out *report.Report, code, message string, s, related *validatorapi.Span) {
	entry := report.Entry{Code: code, Level: validatorapi.SeverityError, Source: validatorapi.SourceParser, Message: message}
	if s != nil {
		entry.Location = &report.Location{Start: s.Start, End: s.End, EndExclusive: true}
	}
	if related != nil {
		entry.RelatedLocation = &report.Location{Start: related.Start, End: related.End, EndExclusive: true}
	}
	out.Entries = append(out.Entries, entry)
}

// addValidator добавляет validator entry.
func addValidator(out *report.Report, code, message string) {
	out.Entries = append(out.Entries, report.Entry{Code: code, Level: validatorapi.SeverityError, Source: validatorapi.SourceValidator, Message: message})
}

// findingReporter проверяет, дедуплицирует и добавляет findings.
// resultError обозначает отклонённое некорректное срабатывание.
type resultError struct{ reason string }

// Error возвращает причину отклонения.
func (e resultError) Error() string { return e.reason }

type findingReporter struct {
	out        *report.Report
	definition validatorapi.Definition
	seen       []validatorapi.Finding
}

func (r *findingReporter) Report(f validatorapi.Finding) error {
	if f.Message == "" || f.Span != nil && !f.Span.Valid() || f.RelatedSpan != nil && !f.RelatedSpan.Valid() {
		addValidator(r.out, "DIAGNOSTIC_RESULT_INVALID", r.definition.Passport.Code+": некорректное срабатывание")
		return resultError{reason: "некорректное срабатывание"}
	}
	for _, v := range r.seen {
		if sameFinding(v, f) {
			return nil
		}
	}
	r.seen = append(r.seen, f)
	p := r.definition.Passport
	entry := report.Entry{Code: p.Code, Level: p.Severity, Source: validatorapi.SourceDiagnostic, Category: &p.Category, Title: &p.Title, Message: f.Message, Description: &p.Description, Basis: &p.Basis, Reference: &p.Reference, BadExample: &p.BadExample}
	if f.Span != nil {
		entry.Location = &report.Location{Start: f.Span.Start, End: f.Span.End, EndExclusive: true}
	}
	if f.RelatedSpan != nil {
		entry.RelatedLocation = &report.Location{Start: f.RelatedSpan.Start, End: f.RelatedSpan.End, EndExclusive: true}
	}
	r.out.Entries = append(r.out.Entries, entry)
	return nil
}

// sameFinding сообщает равенство: true — findings совпадают, false — различаются.
func sameFinding(a, b validatorapi.Finding) bool {
	return a.Message == b.Message && sameSpan(a.Span, b.Span) && sameSpan(a.RelatedSpan, b.RelatedSpan)
}

// sameSpan сообщает равенство необязательных диапазонов.
func sameSpan(a, b *validatorapi.Span) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

// runDefinitions изолирует error и panic одной функции и продолжает остальные.
func runDefinitions(out *report.Report, context validatorapi.Context, definitions []validatorapi.Definition) {
	for _, definition := range definitions {
		func() {
			defer func() {
				if value := recover(); value != nil {
					addValidator(out, "DIAGNOSTIC_PANIC", fmt.Sprintf("%s: %v", definition.Passport.Code, value))
				}
			}()
			reporter := &findingReporter{out: out, definition: definition}
			if err := definition.Check(context, reporter); err != nil {
				var invalid resultError
				if !errors.As(err, &invalid) {
					addValidator(out, "DIAGNOSTIC_EXECUTION_FAILURE", definition.Passport.Code+": "+err.Error())
				}
			}
		}()
	}
}
