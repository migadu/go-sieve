package tests

import "testing"

func TestDeleteHeaderSelectorInteractions(t *testing.T) {
	const head = `
require ["vnd.dovecot.testsuite", "editheader", "variables", "relational", "comparator-i;ascii-numeric"];
test_set "message" text:
X-Foo:
X-Foo: keep
X-Foo:
Received:
Auto-Submitted:

Test.
.
;
`
	cases := []struct{ name, script string }{
		{"expanded empty pattern", `
set "field" "X-Foo";
set "key" "";
deleteheader :is "${field}" "${key}";
if not header :count "eq" "x-foo" "1" { test_fail "expanded empty pattern selected the wrong fields"; }
if not header :is "x-foo" "keep" { test_fail "expanded empty pattern deleted a nonmatching field"; }
`},
		{"indexed empty field", `
deleteheader :index 1 :is "x-foo" "";
if not header :count "eq" "x-foo" "2" { test_fail "index selected more than one empty field"; }
if not header :is "x-foo" "" { test_fail "index deleted the other empty field"; }
`},
		{"last indexed empty field", `
deleteheader :index 1 :last :is "x-foo" "";
if not header :count "eq" "x-foo" "2" { test_fail "last index selected more than one empty field"; }
if not header :is "x-foo" "keep" { test_fail "last index selected the middle field"; }
`},
		{"last index does not select a different empty match", `
deleteheader :index 2 :last :is "x-foo" "";
if not header :count "eq" "x-foo" "3" { test_fail "index selected a matching field outside its position"; }
`},
		{"count selects all fields including empty ones", `
deleteheader :count "eq" :comparator "i;ascii-numeric" "x-foo" "3";
if exists "x-foo" { test_fail "count did not select every field"; }
`},
		{"indexed count selects only its empty field", `
deleteheader :index 1 :last :count "eq" :comparator "i;ascii-numeric" "x-foo" "1";
if not header :count "eq" "x-foo" "2" { test_fail "indexed count deleted the wrong number of fields"; }
`},
		{"wildcard includes empty values", `
deleteheader :matches "x-foo" "*";
if exists "x-foo" { test_fail "wildcard left matching empty fields"; }
`},
		{"protected empty fields remain", `
deleteheader :is "received" "";
deleteheader :is "auto-submitted" "";
if not exists ["received", "auto-submitted"] { test_fail "value selector deleted a protected empty field"; }
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			RunDovecotTestInline(t, "", head+"test \"selector interaction\" {\n"+tc.script+"}\n")
		})
	}
}
