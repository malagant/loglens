// Package jsonview renders a parsed JSON object as a collapsible tree, suitable
// for the LogLens detail pane. Containers (objects, arrays) can be expanded or
// collapsed; scalars are rendered inline. Non-JSON rows fall back to a single
// raw-text line. The model exposes Bubble Tea Update/View, but the render and
// expand/collapse logic is data-driven and unit-testable without a tea.Program.
package jsonview

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Kind tags a node so renderers and tests can branch on type without
// re-inspecting the Scalar string.
type Kind int

const (
	KindRaw Kind = iota // a single non-JSON line (Scalar holds the text)
	KindObject
	KindArray
	KindString
	KindNumber
	KindBool
	KindNull
)

// Node is one position in the JSON tree. For object children Key is set;
// for array children Index is set (>= 0) and Key is empty. Containers carry
// Children and an Expanded flag; scalars carry Scalar.
type Node struct {
	Key      string
	Index    int
	Kind     Kind
	Scalar   string
	Children []*Node
	Expanded bool
}

// ClipboardWriteMsg is emitted when the user presses `y`. Host models (the
// outer TUI) translate it into an OSC52 write via OSC52Sequence. Keeping the
// I/O outside the package keeps Update pure and testable.
type ClipboardWriteMsg struct {
	Content string
}

// Model is the collapsible tree view. The zero value is not usable; construct
// with New, NewArray, or NewRaw.
type Model struct {
	Root   *Node
	Raw    string
	cursor int
}

// New builds a Model from a parsed JSON object. The raw argument is the original
// line and is what `y` copies to the clipboard. Object keys are sorted so the
// rendered order is stable across runs.
func New(fields map[string]any, raw string) Model {
	root := buildObject("", fields)
	root.Expanded = true
	return Model{Root: root, Raw: raw}
}

// NewArray builds a Model from a parsed top-level JSON array.
func NewArray(arr []any, raw string) Model {
	root := buildArray("", arr)
	root.Expanded = true
	return Model{Root: root, Raw: raw}
}

// NewRaw builds a Model for a non-JSON row. View renders the raw text on a
// single line; expand/collapse keys are no-ops in this mode.
func NewRaw(raw string) Model {
	return Model{
		Root: &Node{Kind: KindRaw, Scalar: raw, Expanded: true},
		Raw:  raw,
	}
}

// IsRaw reports whether this model is showing a non-JSON fallback row.
func (m Model) IsRaw() bool { return m.Root != nil && m.Root.Kind == KindRaw }

// Cursor returns the current selected line index in the flattened visible list.
func (m Model) Cursor() int { return m.cursor }

// Lines returns the flattened visible lines, plain text (no styling). Useful
// for tests and for snapshot comparisons.
func (m Model) Lines() []string {
	vs := m.visible()
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.text(false)
	}
	return out
}

// View renders the tree as a single string with one line per visible node.
// The selected line is highlighted. Lipgloss styles are applied at render-time
// only; the underlying tree structure stays plain.
func (m Model) View() string {
	vs := m.visible()
	var b strings.Builder
	for i, v := range vs {
		line := v.text(true)
		if i == m.cursor {
			line = selectedStyle.Render(line)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(line)
	}
	return b.String()
}

// Update handles keyboard input. Recognized keys:
//   - up/k, down/j: move the cursor
//   - right/l, enter: expand the current node (if a container)
//   - left/h: collapse the current node (if a container); on a leaf, move to parent
//   - y: copy the original raw line via ClipboardWriteMsg
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.visible())-1 {
			m.cursor++
		}
	case "right", "l", "enter":
		if n := m.currentNode(); n != nil && isContainer(n.Kind) {
			n.Expanded = true
		}
	case "left", "h":
		n := m.currentNode()
		if n == nil {
			return m, nil
		}
		if isContainer(n.Kind) && n.Expanded {
			n.Expanded = false
			return m, nil
		}
		// On a leaf or already-collapsed container, jump to parent.
		if p := m.findParent(m.Root, n); p != nil {
			for i, v := range m.visible() {
				if v.node == p {
					m.cursor = i
					break
				}
			}
		}
	case "y":
		return m, func() tea.Msg { return ClipboardWriteMsg{Content: m.Raw} }
	}
	return m, nil
}

// OSC52Sequence wraps content in the OSC52 clipboard escape. Host models
// can emit this directly to stdout once they receive a ClipboardWriteMsg.
func OSC52Sequence(content string) string {
	enc := base64.StdEncoding.EncodeToString([]byte(content))
	return "\x1b]52;c;" + enc + "\x07"
}

// internals

type visLine struct {
	node  *Node
	depth int
}

func (v visLine) text(styled bool) string {
	indent := strings.Repeat("  ", v.depth)
	label := nodeLabel(v.node, styled)
	if v.depth == 0 && v.node.Key == "" && v.node.Kind == KindObject {
		// Root object: no leading prefix, just children at depth+1.
		return indent + label
	}
	return indent + label
}

func nodeLabel(n *Node, styled bool) string {
	prefix := ""
	switch {
	case n.Kind == KindRaw:
		return n.Scalar
	case n.Kind == KindObject || n.Kind == KindArray:
		if n.Expanded {
			prefix = "▾ "
		} else {
			prefix = "▸ "
		}
	default:
		prefix = "  "
	}

	name := nodeName(n)
	switch n.Kind {
	case KindObject:
		if n.Expanded {
			return prefix + name
		}
		count := len(n.Children)
		if name == "" {
			return fmt.Sprintf("%s{%d}", prefix, count)
		}
		return fmt.Sprintf("%s%s {%d}", prefix, name, count)
	case KindArray:
		if n.Expanded {
			return prefix + name
		}
		count := len(n.Children)
		if name == "" {
			return fmt.Sprintf("%s[%d]", prefix, count)
		}
		return fmt.Sprintf("%s%s [%d]", prefix, name, count)
	default:
		val := n.Scalar
		if styled {
			val = scalarStyle(n.Kind).Render(val)
		}
		if name == "" {
			return prefix + val
		}
		return prefix + name + ": " + val
	}
}

func nodeName(n *Node) string {
	if n.Key != "" {
		return n.Key
	}
	if n.Index >= 0 {
		return "[" + strconv.Itoa(n.Index) + "]"
	}
	return ""
}

func (m Model) visible() []visLine {
	if m.Root == nil {
		return nil
	}
	var out []visLine
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		out = append(out, visLine{node: n, depth: depth})
		if isContainer(n.Kind) && n.Expanded {
			for _, c := range n.Children {
				walk(c, depth+1)
			}
		}
	}
	walk(m.Root, 0)
	return out
}

func (m Model) currentNode() *Node {
	vs := m.visible()
	if m.cursor < 0 || m.cursor >= len(vs) {
		return nil
	}
	return vs[m.cursor].node
}

func (m Model) findParent(root, target *Node) *Node {
	if root == nil {
		return nil
	}
	for _, c := range root.Children {
		if c == target {
			return root
		}
		if p := m.findParent(c, target); p != nil {
			return p
		}
	}
	return nil
}

func isContainer(k Kind) bool { return k == KindObject || k == KindArray }

func buildObject(key string, m map[string]any) *Node {
	n := &Node{Key: key, Index: -1, Kind: KindObject}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		n.Children = append(n.Children, buildNode(k, -1, m[k]))
	}
	return n
}

func buildArray(key string, a []any) *Node {
	n := &Node{Key: key, Index: -1, Kind: KindArray}
	for i, v := range a {
		n.Children = append(n.Children, buildNode("", i, v))
	}
	return n
}

func buildNode(key string, idx int, v any) *Node {
	switch t := v.(type) {
	case map[string]any:
		n := buildObject(key, t)
		n.Index = idx
		return n
	case []any:
		n := buildArray(key, t)
		n.Index = idx
		return n
	case string:
		return &Node{Key: key, Index: idx, Kind: KindString, Scalar: strconv.Quote(t)}
	case bool:
		return &Node{Key: key, Index: idx, Kind: KindBool, Scalar: strconv.FormatBool(t)}
	case nil:
		return &Node{Key: key, Index: idx, Kind: KindNull, Scalar: "null"}
	case json.Number:
		return &Node{Key: key, Index: idx, Kind: KindNumber, Scalar: string(t)}
	case float64:
		return &Node{Key: key, Index: idx, Kind: KindNumber, Scalar: formatFloat(t)}
	case int:
		return &Node{Key: key, Index: idx, Kind: KindNumber, Scalar: strconv.Itoa(t)}
	case int64:
		return &Node{Key: key, Index: idx, Kind: KindNumber, Scalar: strconv.FormatInt(t, 10)}
	default:
		b, _ := json.Marshal(v)
		return &Node{Key: key, Index: idx, Kind: KindString, Scalar: string(b)}
	}
}

func formatFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

var (
	selectedStyle = lipgloss.NewStyle().Reverse(true)

	stringStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#A6E22E"))
	numberStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#AE81FF"))
	boolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FD971F"))
	nullStyle   = lipgloss.NewStyle().Faint(true)
)

func scalarStyle(k Kind) lipgloss.Style {
	switch k {
	case KindString:
		return stringStyle
	case KindNumber:
		return numberStyle
	case KindBool:
		return boolStyle
	case KindNull:
		return nullStyle
	default:
		return lipgloss.NewStyle()
	}
}
