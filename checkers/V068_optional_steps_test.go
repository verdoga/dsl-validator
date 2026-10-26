package checkers

import (
	"testing"

	"github.com/verdoga/dsl-validator/model"
)

func TestV068OptionalSteps(t *testing.T) {
	for _, tc := range []struct {
		name string
		body []model.Line
		want []v068Expected
	}{
		{"ordinary task without steps", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "instr", "content", "Answer the question.", 3, 2),
			v068TestTag(t, "question", "content", "What is your name?", 3, 2),
		}, nil},
		{"literal step does not enable minimum", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestOpen(t, "multifill", 3, 2),
			v068TestText(t, "@step Write", "@step Write", 4, 3),
			v068TestClose(t, 4, 3),
		}, nil},
		{"step outside task does not enable minimum", []model.Line{
			v068TestTag(t, "task", "identifier", "practice", 2, 1),
			v068TestTag(t, "endtask", "", "", 3, 2),
			v068TestTag(t, "step", "title", "Outside", 2, 1),
		}, nil},
		{"empty task between single step tasks", []model.Line{
			v068TestTag(t, "task", "identifier", "first", 2, 1),
			v068TestTag(t, "step", "title", "Read", 3, 2),
			v068TestTag(t, "task", "identifier", "second", 2, 1),
			v068TestTag(t, "instr", "content", "Read.", 5, 2),
			v068TestTag(t, "task", "identifier", "third", 2, 1),
			v068TestTag(t, "step", "title", "Write", 7, 2),
		}, []v068Expected{{3, 1, 6, "Найдено этапов: 1."}, {7, 1, 6, "Найдено этапов: 1."}}},
		{"step in next branch does not enable previous task", []model.Line{
			v068TestOpen(t, "variants", 2, 1),
			v068TestTag(t, "variant", "name", "A", 3, 2),
			v068TestTag(t, "task", "identifier", "first", 3, 2),
			v068TestTag(t, "variant", "name", "B", 3, 2),
			v068TestTag(t, "task", "identifier", "second", 3, 2),
			v068TestTag(t, "step", "title", "Read", 7, 3),
			v068TestClose(t, 3, 2),
		}, []v068Expected{{7, 1, 6, "Найдено этапов: 1."}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v068TestRun(t, v068TestDocument(t, tc.body...), "current", tc.want)
		})
	}
}
