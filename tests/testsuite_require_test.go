package tests

import (
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
)

// Every vnd.dovecot.testsuite command and test, in Pigeonhole's syntax.
var testsuiteCommands = []struct {
	name   string
	script string
}{
	{"test", `test "x" { stop; }`},
	{"test_set", `test_set "message" "Subject: x\r\n\r\nx\r\n";`},
	{"test_fail", `test_fail "x";`},
	{"test_binary_load", `test_binary_load "x";`},
	{"test_binary_save", `test_binary_save "x";`},
	{"test_config_reload", `test_config_reload;`},
	{"test_config_reload :extension", `test_config_reload :extension "variables";`},
	{"test_config_set", `test_config_set "sieve_variables_max_variable_size" "100000000";`},
	{"test_config_unset", `test_config_unset "sieve_variables_max_variable_size";`},
	{"test_script_compile", `if test_script_compile "x" { stop; }`},
	{"test_script_run", `if test_script_run { stop; }`},
	{"test_error", `if test_error :index 1 :contains "x" { stop; }`},
}

func loadTestsuiteScript(t *testing.T, script string, testEnv bool) error {
	opts := sieve.DefaultOptions()
	opts.EnabledExtensions = []string{"variables", "fileinto"}
	if testEnv {
		opts.Interp.T = t
	}
	_, err := sieve.Load(strings.NewReader(script), opts)
	return err
}

// RFC 5228 §2.10.5: "If an extension is not enabled with "require",
// implementations MUST treat it as if they did not support it at all."
// vnd.dovecot.testsuite is not a real extension at all outside a Go test
// (require refuses it without Options.T), so none of its commands or tests
// may load without it: test_config_set would otherwise raise the variable
// size limit of the compiled script for every run of it.
func TestTestsuiteNeedsRequire(t *testing.T) {
	for _, tc := range testsuiteCommands {
		t.Run(tc.name, func(t *testing.T) {
			if err := loadTestsuiteScript(t, tc.script, false); err == nil {
				t.Errorf("loaded without require %q", "vnd.dovecot.testsuite")
			}
			if err := loadTestsuiteScript(t, tc.script, true); err == nil {
				t.Errorf("loaded in a testing environment without require %q", "vnd.dovecot.testsuite")
			}
		})
	}
}

func TestTestsuiteRequireNeedsTestingEnvironment(t *testing.T) {
	if err := loadTestsuiteScript(t, `require "vnd.dovecot.testsuite";`, false); err == nil {
		t.Fatal("require \"vnd.dovecot.testsuite\" loaded outside a testing environment")
	}
}

func TestTestsuiteLoadsWhenRequired(t *testing.T) {
	for _, tc := range testsuiteCommands {
		t.Run(tc.name, func(t *testing.T) {
			script := `require "vnd.dovecot.testsuite"; ` + tc.script
			if err := loadTestsuiteScript(t, script, true); err != nil {
				t.Errorf("load failed: %v", err)
			}
		})
	}
}
