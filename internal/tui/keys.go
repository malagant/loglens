package tui

// Category groups bindings for the help overlay and the footer.
type Category string

const (
	CategoryBasic Category = "basic"
	CategoryPower Category = "power"
)

// Binding is a single keymap entry: one or more key strings (matching the
// values produced by tea.KeyMsg.String()), a short help label, and a category
// used by the footer/help renderers to decide what to surface where.
type Binding struct {
	Keys     []string
	Help     string
	Category Category
}

// Matches reports whether the given key (typically from tea.KeyMsg.String())
// is bound by this entry. Empty keys never match.
func (b Binding) Matches(key string) bool {
	if key == "" {
		return false
	}
	for _, k := range b.Keys {
		if k == key {
			return true
		}
	}
	return false
}

// Display returns the key shown in help/footer. The first key is canonical;
// the rest are aliases that we keep functional but do not advertise.
func (b Binding) Display() string {
	if len(b.Keys) == 0 {
		return ""
	}
	return b.Keys[0]
}

// Keymap is the registry of every binding LogLens understands.
type Keymap struct {
	Up        Binding
	Down      Binding
	JumpTop   Binding
	JumpBot   Binding
	PageUp    Binding
	PageDown  Binding
	Search    Binding
	Detail    Binding
	CyclePane Binding
	Pause     Binding
	Clear     Binding
	Follow    Binding
	Help      Binding
	Quit      Binding
}

// All returns every binding in display order. The order controls both the
// footer packing priority (highest first) and the help overlay section order.
// Navigation, then the essential actions (search, quit, help), then the
// structural controls (pause, clear, detail, pane cycle), then power keys.
// Quit and Help are deliberately placed before the multi-char bindings
// (enter:detail, tab:pane) so they are always visible on narrow terminals.
func (k Keymap) All() []Binding {
	return []Binding{
		k.Up, k.Down,
		k.Search,
		k.Help, k.Quit,
		k.Pause, k.Clear,
		k.Detail, k.CyclePane,
		k.JumpTop, k.JumpBot,
		k.PageUp, k.PageDown,
		k.Follow,
	}
}

// Basic returns bindings tagged CategoryBasic. The footer renders these.
func (k Keymap) Basic() []Binding {
	out := make([]Binding, 0, 9)
	for _, b := range k.All() {
		if b.Category == CategoryBasic {
			out = append(out, b)
		}
	}
	return out
}

// Power returns bindings tagged CategoryPower. The help overlay surfaces
// these in a separate section.
func (k Keymap) Power() []Binding {
	out := make([]Binding, 0, 5)
	for _, b := range k.All() {
		if b.Category == CategoryPower {
			out = append(out, b)
		}
	}
	return out
}

// DefaultKeymap is the shipped binding set. Keys mirror tea.KeyMsg.String()
// values so Update() can compare against b.Matches(msg.String()).
var DefaultKeymap = Keymap{
	Up:        Binding{Keys: []string{"k", "up"}, Help: "up", Category: CategoryBasic},
	Down:      Binding{Keys: []string{"j", "down"}, Help: "down", Category: CategoryBasic},
	Search:    Binding{Keys: []string{"/"}, Help: "search", Category: CategoryBasic},
	Detail:    Binding{Keys: []string{"enter"}, Help: "detail", Category: CategoryBasic},
	CyclePane: Binding{Keys: []string{"tab"}, Help: "pane", Category: CategoryBasic},
	Pause:     Binding{Keys: []string{"p"}, Help: "pause", Category: CategoryBasic},
	Clear:     Binding{Keys: []string{"c"}, Help: "clear", Category: CategoryBasic},
	Help:      Binding{Keys: []string{"?"}, Help: "help", Category: CategoryBasic},
	Quit:      Binding{Keys: []string{"q", "ctrl+c"}, Help: "quit", Category: CategoryBasic},

	JumpTop:  Binding{Keys: []string{"g"}, Help: "top", Category: CategoryPower},
	JumpBot:  Binding{Keys: []string{"G"}, Help: "bottom", Category: CategoryPower},
	PageUp:   Binding{Keys: []string{"ctrl+u"}, Help: "page up", Category: CategoryPower},
	PageDown: Binding{Keys: []string{"ctrl+d"}, Help: "page down", Category: CategoryPower},
	Follow:   Binding{Keys: []string{"f"}, Help: "follow tail", Category: CategoryPower},
}
