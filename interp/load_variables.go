package interp

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/migadu/go-sieve/parser"
)

func loadSet(script *Script, pcmd parser.Cmd) (Cmd, error) {
	if !script.RequiresExtension("variables") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'variables'")
	}
	cmd := CmdSet{}
	var mods valueModifiers

	spec := &Spec{
		Tags: map[string]SpecTag{},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Name = strings.ToLower(val[0])
				},
			},
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Value = val[0]
				},
			},
		},
	}
	mods.addSpecTags(spec.Tags)
	err := LoadSpec(script, spec, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)

	if mods.conflicting {
		return nil, parser.ErrorAt(pcmd.Position, "conflicting value modifiers")
	}

	settable, _ := script.IsVarUsable(cmd.Name)
	if !settable {
		return nil, parser.ErrorAt(pcmd.Position, "cannot set this variable")
	}

	cmd.ModifyValue = mods.apply(script)

	return cmd, err
}

// valueModifiers collects a command's RFC 5229 §4.4 value modifiers (:lower,
// :length, ...) and applies them in precedence order. set and extracttext
// share it.
type valueModifiers struct {
	byPrecedence map[int]func(string) string
	conflicting  bool // two modifiers of the same precedence were given
}

func (m *valueModifiers) addSpecTags(tags map[string]SpecTag) {
	m.byPrecedence = map[int]func(string) string{}
	set := func(prec int, f func(string) string) {
		if m.byPrecedence[prec] != nil {
			m.conflicting = true
		}
		m.byPrecedence[prec] = f
	}
	tags["length"] = SpecTag{
		MatchBool: func() {
			set(10, func(s string) string {
				// RFC mentions `characters' and not octets
				return strconv.Itoa(len([]rune(s)))
			})
		},
	}
	tags["quotewildcard"] = SpecTag{
		MatchBool: func() {
			set(20, func(s string) string {
				escaped := strings.Builder{}
				escaped.Grow(len(s))
				for _, chr := range s {
					switch chr {
					case '\\', '*', '?':
						escaped.WriteByte('\\')
						escaped.WriteRune(chr)
					default:
						escaped.WriteRune(chr)
					}
				}
				return escaped.String()
			})
		},
	}
	tags["upper"] = SpecTag{
		MatchBool: func() {
			set(40, strings.ToUpper)
		},
	}
	tags["lower"] = SpecTag{
		MatchBool: func() {
			set(40, strings.ToLower)
		},
	}
	tags["upperfirst"] = SpecTag{
		MatchBool: func() {
			set(30, func(s string) string {
				if len(s) == 0 {
					return s
				}
				first := s[0]
				if first >= 'a' && first <= 'z' {
					first -= 'a' - 'A'
				}
				return string(first) + s[1:]
			})
		},
	}
	tags["lowerfirst"] = SpecTag{
		MatchBool: func() {
			set(30, func(s string) string {
				if len(s) == 0 {
					return s
				}
				first := s[0]
				if first >= 'A' && first <= 'Z' {
					first += 'a' - 'A'
				}
				return string(first) + s[1:]
			})
		},
	}
}

// apply returns the function that runs the modifiers over a value, bounded to
// script's variable length limit.
func (m *valueModifiers) apply(script *Script) func(string) string {
	return func(s string) string {
		lastPrec := 9999
		for _, prec := range [4]int{40, 30, 20, 10} {
			fun := m.byPrecedence[prec]
			if fun != nil {
				s = fun(s)
				lastPrec = prec
			}
		}

		// If last run modifier was quotewildcard - check
		// whether created value would remain valid
		// if truncated to MaxVariableLen. If so, truncate
		// here and remove dangling backslashes (if any).
		if lastPrec == 20 {
			if len(s) > script.opts.MaxVariableLen {
				until := script.opts.MaxVariableLen

				// (Copy-pasted from RuntimeData.SetVar)
				// If this truncated an otherwise valid Unicode character,
				// remove the character altogether.
				for until > 0 && s[until] >= 128 && s[until] < 192 /* second or further octet of UTF-8 encoding */ {
					until--
				}

				if s[until-1] == '\\' {
					until--
				}

				s = s[:until]
			}
		}

		return s
	}
}

func loadStringTest(s *Script, test parser.Test) (Test, error) {
	if !s.RequiresExtension("variables") {
		return nil, fmt.Errorf("missing require 'variables'")
	}

	loaded := TestString{matcherTest: newMatcherTest()}
	var key []string
	err := LoadSpec(s, loaded.addSpecTags(&Spec{
		Pos: []SpecPosArg{
			{
				MatchStr: func(val []string) {
					loaded.Source = val
				},
				MinStrCount: 1,
			},
			{
				MatchStr: func(val []string) {
					key = val
				},
				MinStrCount: 1,
			},
		},
	}), test.Position, test.Args, test.Tests, nil)
	if err != nil {
		return nil, err
	}

	if err := loaded.setKey(s, key); err != nil {
		return nil, err
	}

	// Check if regex extension is required
	if loaded.match == MatchRegex && !s.RequiresExtension("regex") {
		return nil, fmt.Errorf("missing require 'regex'")
	}

	return loaded, nil
}
