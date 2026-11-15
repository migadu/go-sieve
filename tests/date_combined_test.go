package tests

import "testing"

// These forms combine the independently loaded date extensions so parser
// normalization, field selection, variable expansion and date-part formatting
// continue to agree when used by one script.
func TestDateParsingFeaturesTogether(t *testing.T) {
	RunDovecotTestInline(t, "", `
require ["vnd.dovecot.testsuite", "date", "index", "variables", "relational", "comparator-i;ascii-numeric"];
test_set "message" text:
Received: from [2001:db8::1] (helo; x) by mail.example;
 Sat (weekday: ignore 99), (day) 31 Dec (year: 99) 116 (hour) 23 (h) : 59 (m) : 60 (s) +0000 (sent; last hop)
Received: by mail.example; Sat, 31 Dec (year) 16 23:59:60 GMT (last; note)
Subject: Combined date features

Test.
.
;
test "Received comments, obsolete years and leap seconds" {
 set "zone" "+0100";
 set "part" "ISO8601";
 if not date :zone "${zone}" "Received" "${part}" "2017-01-01T00:59:60+01:00" {test_fail "combined normalization or dynamic zone failed";}
 if not date :originalzone :index 2 "Received" "${part}" "2016-12-31T23:59:60Z" {test_fail "two-digit year or Zulu date-part failed";}
}
test "count uses just the selected field with a dynamic date-part" {
 set "part" "year";
 if not date :count "eq" "Received" "${part}" "1" {test_fail "default count was not one";}
 if not date :count "eq" :index 2 "Received" "${part}" "1" {test_fail "indexed count was not one";}
 if not date :count "eq" :index 3 "Received" "${part}" "0" {test_fail "missing indexed field counted";}
 set "part" "unknown";
 if date :count "eq" :index 3 "Received" "${part}" "0" {test_fail "unknown expanded date-part matched";}
}
test "currentdate expands its zone and date-part" {
 set "zone" "+0000";
 set "part" "iso8601";
 if not currentdate :zone "${zone}" :matches "${part}" "????-??-??T??:??:??Z" {test_fail "currentdate dynamic UTC formatting failed";}
 if not currentdate :count "eq" "${part}" "1" {test_fail "dynamic currentdate count was not one";}
 set "part" "unknown";
 if currentdate :count "ge" "${part}" "0" {test_fail "unknown currentdate part matched count";}
 set "part" "";
 if currentdate :count "ge" "${part}" "0" {test_fail "empty currentdate part matched count";}
}
`)
}
