package interp

import (
	"strings"

	"github.com/migadu/go-sieve/lexer"
	"github.com/migadu/go-sieve/parser"
)

// maxForEveryPartNesting is Pigeonhole's SIEVE_MAX_LOOP_DEPTH. RFC 5703 §3
// only asks for one level of nesting.
const maxForEveryPartNesting = 4

// mimeOpt is a header test's MIMEOPTS choice (RFC 5703 §4.1).
type mimeOpt int

const (
	mimeOptNone mimeOpt = iota
	mimeOptType
	mimeOptSubtype
	mimeOptContentType
	mimeOptParam
)

// mimeTestOpts holds the :mime, :anychild and MIMEOPTS tagged arguments the
// mime extension adds to the header, address and exists tests.
type mimeTestOpts struct {
	enabled  bool // :mime
	anychild bool
	opt      mimeOpt
	optCnt   int
	params   []string // :param names
}

// addSpecTags registers the tags. MIMEOPTS exist on the header test only.
func (o *mimeTestOpts) addSpecTags(tags map[string]SpecTag, withOpts bool) {
	tags["mime"] = SpecTag{MatchBool: func() { o.enabled = true }}
	tags["anychild"] = SpecTag{MatchBool: func() { o.anychild = true }}
	if !withOpts {
		return
	}
	tags["type"] = SpecTag{MatchBool: func() { o.opt = mimeOptType; o.optCnt++ }}
	tags["subtype"] = SpecTag{MatchBool: func() { o.opt = mimeOptSubtype; o.optCnt++ }}
	tags["contenttype"] = SpecTag{MatchBool: func() { o.opt = mimeOptContentType; o.optCnt++ }}
	tags["param"] = SpecTag{
		NeedsValue:  true,
		MinStrCount: 1,
		MatchStr: func(val []string) {
			o.opt = mimeOptParam
			o.optCnt++
			o.params = val
		},
	}
}

func (o *mimeTestOpts) validate(s *Script, pos lexer.Position) error {
	if o.optCnt > 1 {
		return parser.ErrorAt(pos, "only one of :type, :subtype, :contenttype or :param is allowed")
	}
	if (o.anychild || o.optCnt > 0) && !o.enabled {
		return parser.ErrorAt(pos, ":anychild, :type, :subtype, :contenttype and :param require :mime")
	}
	if o.enabled && !s.RequiresExtension("mime") {
		return parser.ErrorAt(pos, "missing require 'mime'")
	}
	return nil
}

// scope returns the parts a :mime test examines (RFC 5703 §4.1): the running
// foreverypart's current part, or the whole message outside a loop; with
// :anychild also every part nested in it.
func (o *mimeTestOpts) scope(d *RuntimeData) ([]*mimePart, error) {
	root, err := d.mimeTree()
	if err != nil {
		return nil, err
	}
	cur := d.currentPart()
	if cur == nil {
		cur = root
	}
	if !o.anychild {
		return []*mimePart{cur}, nil
	}
	return cur.subtree(), nil
}

// values applies the MIMEOPTS to the raw values of header name. It returns the
// strings to match and what they add to a :count: with :type, :subtype or
// :contenttype the values that parsed, with :param the parameters found.
func (o *mimeTestOpts) values(name string, raw []string, params []string) ([]string, uint64) {
	switch o.opt {
	case mimeOptType, mimeOptSubtype, mimeOptContentType:
		var out []string
		for _, v := range raw {
			mediaType, _ := parseMediaTypeLenient(v)
			if mediaType == "" {
				continue
			}
			typ, subtype, _ := strings.Cut(mediaType, "/")
			var s string
			switch strings.ToLower(name) {
			case "content-type":
				switch o.opt {
				case mimeOptType:
					s = typ
				case mimeOptSubtype:
					s = subtype
				default:
					s = mediaType
				}
			case "content-disposition":
				// A disposition has no subtype; :contenttype is the
				// disposition itself.
				if o.opt != mimeOptSubtype {
					s = typ
				}
			}
			out = append(out, s)
		}
		return out, uint64(len(out))
	case mimeOptParam:
		var out []string
		for _, v := range raw {
			_, have := parseMediaTypeLenient(v)
			for _, want := range params {
				if val, ok := have[strings.ToLower(want)]; ok {
					out = append(out, decodeHeaderValue(val))
				}
			}
		}
		return out, uint64(len(out))
	default:
		out := make([]string, len(raw))
		for i, v := range raw {
			out[i] = decodeHeaderValue(v)
		}
		return out, uint64(len(raw))
	}
}

func loadForEveryPart(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("foreverypart") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'foreverypart'")
	}
	if len(s.loops) >= maxForEveryPartNesting {
		return nil, parser.ErrorAt(pcmd.Position, "foreverypart nested deeper than %d levels", maxForEveryPartNesting)
	}

	cmd := &CmdForEveryPart{}
	// Tagged arguments load before the block, so the entry pushed here carries
	// the loop's name by the time a break inside the block looks it up.
	s.loops = append(s.loops, "")
	defer func() { s.loops = s.loops[:len(s.loops)-1] }()

	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"name": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					cmd.Name = val[0]
					s.loops[len(s.loops)-1] = val[0]
				},
			},
		},
		AddBlock: func(cmds []Cmd) {
			cmd.Block = cmds
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}
	return cmd, nil
}

func loadBreak(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("foreverypart") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'foreverypart'")
	}
	cmd := CmdBreak{}
	err := LoadSpec(s, &Spec{
		Tags: map[string]SpecTag{
			"name": {
				NeedsValue:  true,
				MinStrCount: 1,
				MaxStrCount: 1,
				NoVariables: true,
				MatchStr: func(val []string) {
					cmd.Name = val[0]
				},
			},
		},
	}, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block)
	if err != nil {
		return nil, err
	}

	if len(s.loops) == 0 {
		return nil, parser.ErrorAt(pcmd.Position, "break outside foreverypart")
	}
	if cmd.Name != "" {
		found := false
		for _, name := range s.loops {
			if name == cmd.Name {
				found = true
				break
			}
		}
		if !found {
			return nil, parser.ErrorAt(pcmd.Position, "break: no enclosing foreverypart named %q", cmd.Name)
		}
	}
	return cmd, nil
}

func loadExtractText(s *Script, pcmd parser.Cmd) (Cmd, error) {
	if !s.RequiresExtension("extracttext") {
		return nil, parser.ErrorAt(pcmd.Position, "missing require 'extracttext'")
	}
	if len(s.loops) == 0 {
		// RFC 5703 §7: "SHOULD be flagged as a compilation error".
		return nil, parser.ErrorAt(pcmd.Position, "extracttext outside foreverypart")
	}

	cmd := CmdExtractText{First: -1}
	var mods valueModifiers
	spec := &Spec{
		Tags: map[string]SpecTag{
			"first": {
				NeedsValue: true,
				MatchNum: func(val int) {
					cmd.First = val
				},
			},
		},
		Pos: []SpecPosArg{
			{
				MinStrCount: 1,
				MaxStrCount: 1,
				MatchStr: func(val []string) {
					cmd.Name = strings.ToLower(val[0])
				},
			},
		},
	}
	mods.addSpecTags(spec.Tags)
	if err := LoadSpec(s, spec, pcmd.Position, pcmd.Args, pcmd.Tests, pcmd.Block); err != nil {
		return nil, err
	}
	if mods.conflicting {
		return nil, parser.ErrorAt(pcmd.Position, "conflicting value modifiers")
	}
	if settable, _ := s.IsVarUsable(cmd.Name); !settable {
		return nil, parser.ErrorAt(pcmd.Position, "cannot set this variable")
	}
	cmd.Modify = mods.apply(s)
	return cmd, nil
}
