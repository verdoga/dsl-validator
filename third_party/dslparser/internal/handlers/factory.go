package handlers

import (
	"github.com/verdoga/dslparser/internal/model"
	"github.com/verdoga/dslparser/internal/registry"
)

// Apply детализирует все известные теги физического дерева.
func Apply(nodes []*model.Node, lines []model.Line, rules registry.Registry) {
	for _, n := range nodes {
		if n.Kind != 3 {
			Apply(n.Children, lines, rules)
			continue
		}
		entry, known := rules.Lookup(n.CanonicalName)
		if !known {
			applyUnknown(n, lines)
			continue
		}
		blockForm := n.Block != nil
		if blockForm && entry.Forms&registry.FormBlock == 0 || !blockForm && entry.Forms&registry.FormInline == 0 {
			n.Parsed = false
			n.Diagnostics = append(n.Diagnostics, model.Diagnostic{Code: "UNSUPPORTED_TAG_FORM", Message: "форма тега не поддерживается", Span: n.Span})
		}
		applyArguments(n, entry)
		if blockForm {
			switch entry.BodyHandler {
			case registry.HandlerRawBody:
				applyRaw(n, lines, entry.PreserveEmpty)
			case registry.HandlerExample:
				applyExample(n, lines)
			case registry.HandlerWordlist:
				applyWordlistBlock(n, lines)
			case registry.HandlerItems:
				applyItems(n, lines)
			case registry.HandlerMatching:
				applyMatching(n, lines)
			case registry.HandlerOrdering:
				applyOrdering(n, lines)
			}
		} else if entry.BodyHandler == registry.HandlerWordlist {
			applyWordlistInline(n)
		}
		Apply(n.Children, lines, rules)
	}
}

// applyUnknown сохраняет универсальные токены и непрозрачное тело неизвестного тега.
func applyUnknown(n *model.Node, lines []model.Line) {
	addUniversalTokens(n, argumentTail(n))
	if n.Block != nil {
		applyRaw(n, lines, true)
	}
}

// addUniversalTokens сохраняет доступные токены неизвестного тега.
func addUniversalTokens(n *model.Node, raw string) {
	if raw == "" {
		return
	}
	start := n.Candidate.NameEnd + 1
	// Неизвестная схема использует общую токенизацию, включая частичный результат.
	addTokens(n, raw, start)
}
