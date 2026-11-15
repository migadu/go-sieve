package parser

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/migadu/go-sieve/lexer"
)

const exampleScript = ` #
    # Example Sieve Filter
    # Declare any optional features or extension used by the script
    #
    require ["fileinto"];

    #
    # Handle messages from known mailing lists
    # Move messages from IETF filter discussion list to filter mailbox
    #
    if header :is "Sender" "owner-ietf-mta-filters@imc.org"
            {
            fileinto "filter";  # move to "filter" mailbox
            }
    #
    # Keep all messages to or from people in my company
    #
    elsif address :DOMAIN :is ["From", "To"] "example.com"
            {
            keep;               # keep in "In" mailbox
            }

    #
    # Try and catch unsolicited email.  If a message is not to me,
    # or it contains a subject known to be spam, file it away.
    #
    elsif anyof (NOT address :all :contains
                   ["To", "Cc", "Bcc"] "me@example.com",
                 header :matches "subject"
                   ["*make*money*fast*", "*university*dipl*mas*"])
            {
            fileinto "spam";   # move to "spam" mailbox
            }
    else
            {
            # Move all other (non-company) mail to "personal"
            # mailbox.
            fileinto "personal";
            }
`

func testParse(t *testing.T, script string, cmds []Cmd) {
	toks, err := lexer.Lex(strings.NewReader(script), &lexer.Options{
		NoPosition: true,
	})
	if err != nil {
		t.Fatal("Lexer failed:", err)
	}
	s := lexer.NewStream(toks)
	actualCmds, err := parse(s, 0, &Options{})
	if err != nil {
		t.Error("parse failed:", err)
		return
	}
	if err != nil {
		if cmds == nil {
			return
		}
		t.Error("Unexpected failure:", err)
		return
	}
	if cmds == nil {
		t.Error("Unexpected success:", actualCmds)
		return
	}
	if !reflect.DeepEqual(cmds, actualCmds) {
		t.Log("Wrong parse result")
		t.Log("Expected:")
		t.Log(spew.Sdump(cmds))
		t.Log("Actual:")
		t.Log(spew.Sdump(actualCmds))
		t.Fail()
	}
}

func TestParser(t *testing.T) {
	testParse(t, exampleScript, []Cmd{
		{
			Id: "require",
			Args: []Arg{
				StringListArg{Value: []string{"fileinto"}},
			},
		},
		{
			Id: "if",
			Tests: []Test{
				{
					Id: "header",
					Args: []Arg{
						TagArg{Value: "is"},
						StringArg{Value: "Sender"},
						StringArg{Value: "owner-ietf-mta-filters@imc.org"},
					},
				},
			},
			Block: []Cmd{
				{
					Id: "fileinto",
					Args: []Arg{
						StringArg{Value: "filter"},
					},
				},
			},
		},
		{
			Id: "elsif",
			Tests: []Test{
				{
					Id: "address",
					Args: []Arg{
						TagArg{Value: "DOMAIN"},
						TagArg{Value: "is"},
						StringListArg{Value: []string{"From", "To"}},
						StringArg{Value: "example.com"},
					},
				},
			},
			Block: []Cmd{
				{
					Id: "keep",
				},
			},
		},
		{
			Id: "elsif",
			Tests: []Test{
				{
					Id: "anyof",
					Tests: []Test{
						{
							Id: "NOT",
							Tests: []Test{
								{
									Id: "address",
									Args: []Arg{
										TagArg{Value: "all"},
										TagArg{Value: "contains"},
										StringListArg{Value: []string{"To", "Cc", "Bcc"}},
										StringArg{Value: "me@example.com"},
									},
								},
							},
						},
						{
							Id: "header",
							Args: []Arg{
								TagArg{Value: "matches"},
								StringArg{Value: "subject"},
								StringListArg{Value: []string{"*make*money*fast*", "*university*dipl*mas*"}},
							},
						},
					},
				},
			},
			Block: []Cmd{
				{
					Id: "fileinto",
					Args: []Arg{
						StringArg{Value: "spam"},
					},
				},
			},
		},
		{
			Id: "else",
			Block: []Cmd{
				{
					Id: "fileinto",
					Args: []Arg{
						StringArg{Value: "personal"},
					},
				},
			},
		},
	})
}

// TestParseNumberQuantifierRange covers RFC 5228 §2.4.1: "K", "M" and "G"
// multiply the number by 2^10, 2^20 and 2^30. The largest number that still
// fits an int with its suffix is accepted with the exact product; one more is
// refused at the number's position instead of wrapping to a wrong or negative
// value, as a plain number too large for an int is refused by the lexer.
func TestParseNumberQuantifierRange(t *testing.T) {
	parseScript := func(script string) ([]Cmd, error) {
		toks, err := lexer.Lex(strings.NewReader(script), &lexer.Options{})
		if err != nil {
			t.Fatalf("lex %q: %v", script, err)
		}
		return parse(lexer.NewStream(toks), 0, &Options{})
	}

	for _, q := range []struct {
		suffix string
		mult   int
	}{
		{"K", 1 << 10}, {"k", 1 << 10},
		{"M", 1 << 20}, {"m", 1 << 20},
		{"G", 1 << 30}, {"g", 1 << 30},
	} {
		largest := math.MaxInt / q.mult

		t.Run(q.suffix+"/largest", func(t *testing.T) {
			script := "if size :over " + strconv.Itoa(largest) + q.suffix + " { keep; }"
			cmds, err := parseScript(script)
			if err != nil {
				t.Fatalf("parse %q: %v", script, err)
			}
			got := cmds[0].Tests[0].Args[1].(NumberArg).Value
			if want := largest * q.mult; got != want {
				t.Fatalf("parse %q: value %d, want %d", script, got, want)
			}
		})

		t.Run(q.suffix+"/overflow", func(t *testing.T) {
			script := "if size :over " + strconv.Itoa(largest+1) + q.suffix + " { keep; }"
			cmds, err := parseScript(script)
			if err == nil {
				t.Fatalf("parse %q: accepted with value %d, want an error",
					script, cmds[0].Tests[0].Args[1].(NumberArg).Value)
			}
			if !strings.HasPrefix(err.Error(), "1:15: ") {
				t.Fatalf("parse %q: error %q, want it at the number (1:15)", script, err)
			}
		})
	}

	// A product that wraps past zero to a small positive value.
	if math.MaxInt == math.MaxInt64 {
		script := "if size :over 18014398509481985K { keep; }" // (2^54 + 1) * 2^10 = 2^64 + 1024
		if cmds, err := parseScript(script); err == nil {
			t.Fatalf("parse %q: accepted with value %d, want an error",
				script, cmds[0].Tests[0].Args[1].(NumberArg).Value)
		}
	}
}
