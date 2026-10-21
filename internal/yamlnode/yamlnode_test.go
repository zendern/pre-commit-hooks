package yamlnode

import (
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

func parse(t *testing.T, src string) *yaml.Node {
	t.Helper()
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &doc
}

func TestRoot(t *testing.T) {
	doc := parse(t, "a: 1\n")
	if root := Root(doc); root == nil || root.Kind != yaml.MappingNode {
		t.Fatalf("Root = %v, want the top-level mapping", root)
	}
	if root := Root(&yaml.Node{Kind: yaml.DocumentNode}); root != nil {
		t.Errorf("Root of an empty document = %v, want nil", root)
	}
	if root := Root(nil); root != nil {
		t.Errorf("Root(nil) = %v, want nil", root)
	}
}

func TestEntry(t *testing.T) {
	m := Root(parse(t, "a: 1\nb: two\n"))

	k, v := Entry(m, "b")
	if k == nil || k.Value != "b" || v == nil || v.Value != "two" {
		t.Fatalf("Entry(b) = %v, %v; want the b key and its value", k, v)
	}
	if k, v := Entry(m, "a"); k == nil || v.Value != "1" {
		t.Errorf("Entry(a) = %v, %v; want the first pair too", k, v)
	}
	if k, v := Entry(m, "missing"); k != nil || v != nil {
		t.Errorf("Entry(missing) = %v, %v; want nils", k, v)
	}
}

func TestEntryNotAMapping(t *testing.T) {
	seq := Root(parse(t, "[a, b]\n"))
	for _, n := range []*yaml.Node{nil, seq, seq.Content[0]} {
		if k, v := Entry(n, "a"); k != nil || v != nil {
			t.Errorf("Entry(%v) = %v, %v; want nils", n, k, v)
		}
	}
}

func TestEntryMatchesKeysOnly(t *testing.T) {
	// "b" is a value here, not a key, so it must not be found.
	if k, v := Entry(Root(parse(t, "a: b\n")), "b"); k != nil || v != nil {
		t.Errorf("Entry(b) = %v, %v; want nils", k, v)
	}
}

func TestEntryIgnoresAnUnpairedKey(t *testing.T) {
	// A mapping node with an odd number of children (never produced by the
	// parser) has no value for its last key.
	m := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Value: "a"}}}
	if k, v := Entry(m, "a"); k != nil || v != nil {
		t.Errorf("Entry(a) = %v, %v; want nils", k, v)
	}
}

func TestPairsInOrder(t *testing.T) {
	var got []string
	for k, v := range Pairs(Root(parse(t, "c: 3\na: 1\nb: 2\n"))) {
		got = append(got, k.Value+"="+v.Value)
	}
	if strings.Join(got, " ") != "c=3 a=1 b=2" {
		t.Errorf("Pairs = %q, want file order", got)
	}
}

func TestPairsStopsEarly(t *testing.T) {
	n := 0
	for range Pairs(Root(parse(t, "a: 1\nb: 2\nc: 3\n"))) {
		n++
		if n == 2 {
			break
		}
	}
	if n != 2 {
		t.Errorf("visited %d pairs before break, want 2", n)
	}
}

func TestPairsNotAMapping(t *testing.T) {
	seq := Root(parse(t, "[a, b]\n"))
	for _, n := range []*yaml.Node{nil, seq} {
		for k, v := range Pairs(n) {
			t.Errorf("Pairs(%v) yielded %v, %v; want nothing", n, k, v)
		}
	}
}

func TestPairsSkipsAnUnpairedKey(t *testing.T) {
	m := &yaml.Node{Kind: yaml.MappingNode, Content: []*yaml.Node{{Value: "a"}, {Value: "1"}, {Value: "b"}}}
	var got []string
	for k, v := range Pairs(m) {
		got = append(got, k.Value+"="+v.Value)
	}
	if strings.Join(got, " ") != "a=1" {
		t.Errorf("Pairs = %q, want only the complete pair", got)
	}
}
