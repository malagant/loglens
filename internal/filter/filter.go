// Package filter implements the v0.1 LogLens query language. A query is a
// conjunction of predicates separated by whitespace, with a small set of
// keyed predicates and a free-form substring fallback. The full DSL grammar
// (booleans, regex, ranges) is reserved for v0.2; what ships here is the
// minimum needed to drive the slash-prompt and the live row filter.
//
// Supported predicates:
//
//	level=<value>          normalized log level, exact match
//	source=<substring>     substring match against Event.Source
//	field.<key>=<value>    substring match on Event.Fields[key]
//	/needle with spaces/   substring match against Event.Raw, spaces allowed
//	bareword               substring match against Event.Raw
//	!<predicate>           negate any predicate above
//
// The empty query matches every event. Tokens are joined with logical AND.
package filter

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/loglens/loglens/internal/event"
)

// Predicate evaluates one filter term against an Event.
type Predicate interface {
	Match(ev event.Event) bool
	String() string
}

// Query is a conjunction of predicates. The zero value matches every event.
type Query struct {
	Predicates []Predicate
}

// Match reports whether the event satisfies every predicate.
func (q Query) Match(ev event.Event) bool {
	for _, p := range q.Predicates {
		if !p.Match(ev) {
			return false
		}
	}
	return true
}

// Empty reports whether the query has no predicates.
func (q Query) Empty() bool { return len(q.Predicates) == 0 }

// String renders the query in its canonical token form. It is not guaranteed
// to be byte-identical to the input but parses back to an equivalent Query.
func (q Query) String() string {
	parts := make([]string, len(q.Predicates))
	for i, p := range q.Predicates {
		parts[i] = p.String()
	}
	return strings.Join(parts, " ")
}

// Parse tokenizes and parses a query string. Empty or whitespace-only input
// returns an empty Query. Parse never returns an error today; future grammar
// revisions reserve the right to.
func Parse(s string) (Query, error) {
	var q Query
	for _, tok := range tokenize(s) {
		if p := parseTerm(tok); p != nil {
			q.Predicates = append(q.Predicates, p)
		}
	}
	return q, nil
}

// tokenize splits the query into terms. Whitespace separates tokens, but a
// pair of /…/ slashes captures everything between them so substrings can
// contain spaces. A leading ! belongs to the same token as the predicate.
func tokenize(s string) []string {
	var toks []string
	r := []rune(s)
	i := 0
	for i < len(r) {
		for i < len(r) && unicode.IsSpace(r[i]) {
			i++
		}
		if i >= len(r) {
			break
		}
		start := i
		if r[i] == '!' {
			i++
			if i >= len(r) || unicode.IsSpace(r[i]) {
				toks = append(toks, string(r[start:i]))
				continue
			}
		}
		if r[i] == '/' {
			i++
			j := i
			for j < len(r) && r[j] != '/' {
				j++
			}
			body := string(r[start:j])
			if j < len(r) {
				body += "/"
				j++
			}
			toks = append(toks, body)
			i = j
			continue
		}
		j := i
		for j < len(r) && !unicode.IsSpace(r[j]) {
			j++
		}
		toks = append(toks, string(r[start:j]))
		i = j
	}
	return toks
}

func parseTerm(tok string) Predicate {
	neg := false
	if strings.HasPrefix(tok, "!") {
		neg = true
		tok = tok[1:]
	}
	if tok == "" {
		return nil
	}
	var p Predicate
	switch {
	case strings.HasPrefix(tok, "/"):
		body := tok[1:]
		body = strings.TrimSuffix(body, "/")
		if body == "" {
			return nil
		}
		p = substrPred{needle: body}
	case strings.HasPrefix(tok, "level="):
		p = levelPred{want: event.ParseLevel(strings.TrimPrefix(tok, "level="))}
	case strings.HasPrefix(tok, "source="):
		needle := strings.TrimPrefix(tok, "source=")
		if needle == "" {
			return nil
		}
		p = sourcePred{needle: needle}
	case strings.HasPrefix(tok, "field."):
		rest := strings.TrimPrefix(tok, "field.")
		eq := strings.IndexByte(rest, '=')
		if eq <= 0 {
			p = substrPred{needle: tok}
		} else {
			p = fieldPred{key: rest[:eq], value: rest[eq+1:]}
		}
	default:
		p = substrPred{needle: tok}
	}
	if neg {
		p = notPred{inner: p}
	}
	return p
}

type substrPred struct{ needle string }

func (p substrPred) Match(ev event.Event) bool {
	return p.needle == "" || strings.Contains(ev.Raw, p.needle)
}
func (p substrPred) String() string {
	if strings.ContainsAny(p.needle, " \t") {
		return "/" + p.needle + "/"
	}
	return p.needle
}

type levelPred struct{ want event.Level }

func (p levelPred) Match(ev event.Event) bool { return ev.Level == p.want }
func (p levelPred) String() string            { return "level=" + string(p.want) }

type sourcePred struct{ needle string }

func (p sourcePred) Match(ev event.Event) bool { return strings.Contains(ev.Source, p.needle) }
func (p sourcePred) String() string            { return "source=" + p.needle }

type fieldPred struct{ key, value string }

func (p fieldPred) Match(ev event.Event) bool {
	v, ok := ev.Fields[p.key]
	if !ok {
		return false
	}
	return strings.Contains(fmt.Sprint(v), p.value)
}
func (p fieldPred) String() string { return "field." + p.key + "=" + p.value }

type notPred struct{ inner Predicate }

func (p notPred) Match(ev event.Event) bool { return !p.inner.Match(ev) }
func (p notPred) String() string            { return "!" + p.inner.String() }
