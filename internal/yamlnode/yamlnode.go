// Package yamlnode holds small helpers for walking a parsed yaml.Node tree.
package yamlnode

import (
	"slices"

	"go.yaml.in/yaml/v3"
)

// Root returns a parsed document's top-level node, or nil when there is none.
func Root(doc *yaml.Node) *yaml.Node {
	if doc == nil || len(doc.Content) == 0 {
		return nil
	}
	return doc.Content[0]
}

// Entry returns the key and value nodes for key in a mapping node, or nils.
func Entry(m *yaml.Node, key string) (*yaml.Node, *yaml.Node) {
	for k, v := range Pairs(m) {
		if k.Value == key {
			return k, v
		}
	}
	return nil, nil
}

// Pairs yields the key/value node pairs of a mapping node in file order.
func Pairs(m *yaml.Node) func(yield func(*yaml.Node, *yaml.Node) bool) {
	return func(yield func(*yaml.Node, *yaml.Node) bool) {
		if m == nil || m.Kind != yaml.MappingNode {
			return
		}
		// Chunks rather than an index stepped by 2: a mutant that drops the
		// step can only hang, which hides the result.
		for pair := range slices.Chunk(m.Content, 2) {
			if len(pair) < 2 || !yield(pair[0], pair[1]) {
				return
			}
		}
	}
}
