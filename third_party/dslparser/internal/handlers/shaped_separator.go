package handlers

import (
	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/scan"
)

// separatorIndex находит допустимые локальные разделители и возвращает первый из них.
func separatorIndex(lines []model.Line) (int, int) {
	first, count := -1, 0
	for i, line := range lines {
		if line.Text == "---" && !scan.Escaped(line.Text, 0) {
			if first < 0 {
				first = i
			}
			count++
		}
	}
	return first, count
}

// applyMatching разбирает две группы сопоставления вокруг обязательного разделителя.
func applyMatching(n *model.Node, lines []model.Line) {
	body := bodyLines(n, lines)
	separator, count := separatorIndex(body)
	if count != 1 {
		code, message := "MISSING_SEPARATOR", "обязательный разделитель отсутствует"
		if count > 1 {
			code, message = "MULTIPLE_SEPARATORS", "разделитель указан несколько раз"
		}
		separatorFailure(n, body, code, message)
		return
	}
	n.Elements = append(n.Elements, nonemptyGroup("upper", body[:separator]), separatorElement(body[separator]), nonemptyGroup("lower", body[separator+1:]))
}

// applyOrdering разбирает сокращённую или расширенную форму упорядочивания.
func applyOrdering(n *model.Node, lines []model.Line) {
	body := bodyLines(n, lines)
	separator, count := separatorIndex(body)
	if count == 0 {
		n.Elements = append(n.Elements, nonemptyGroup("items", body))
		return
	}
	if count > 1 {
		separatorFailure(n, body, "MULTIPLE_SEPARATORS", "разделитель указан несколько раз")
		return
	}
	n.Elements = append(n.Elements, nonemptyGroup("items", body[:separator]), separatorElement(body[separator]), nonemptyGroup("positions", body[separator+1:]))
}

// separatorFailure сохраняет неклассифицированное тело и одну диагностику разделителя.
func separatorFailure(n *model.Node, body []model.Line, code, message string) {
	n.Parsed = false
	n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: code, Message: message, Span: n.Span})
	n.Elements = append(n.Elements, group("body", body))
}
