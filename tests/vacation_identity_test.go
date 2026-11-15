package tests

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/migadu/go-sieve"
	"github.com/migadu/go-sieve/interp"
)

// TestVacationResponseIdentity exercises the public runtime result, so consumers
// can track the response without reparsing scripts or using expanded text.
func TestVacationResponseIdentity(t *testing.T) {
	evaluate := func(t *testing.T, script string) (string, interp.VacationResponse) {
		t.Helper()
		opts := sieve.DefaultOptions()
		opts.EnabledExtensions = []string{"vacation", "variables"}
		s, err := sieve.Load(strings.NewReader(`require ["vacation", "variables"]; `+script), opts)
		if err != nil {
			t.Fatal(err)
		}
		d := sieve.NewRuntimeData(s, interp.DummyPolicy{}, interp.EnvelopeStatic{From: "sender@example.com", To: "owner@example.com"}, interp.MessageStatic{})
		if err := s.Execute(context.Background(), d); err != nil {
			t.Fatal(err)
		}
		r, ok := d.VacationResponses["sender@example.com"]
		if !ok {
			t.Fatal("no vacation response")
		}
		// Reflection keeps this regression runnable on the version that did not
		// expose an identity. An absent field is a test failure, not a build failure.
		field := reflect.ValueOf(r).FieldByName("ResponseIdentity")
		if !field.IsValid() || field.Kind() != reflect.String || field.String() == "" {
			t.Fatal("vacation result has no response identity")
		}
		return field.String(), r
	}

	for _, tc := range []struct {
		name, a, b string
		same       bool
	}{
		{"same implicit response", `vacation "Away";`, `vacation "Away";`, true},
		{"different reason", `vacation "Away";`, `vacation "Contact support";`, false},
		{"different subject", `vacation :subject "A" "Away";`, `vacation :subject "B" "Away";`, false},
		{"different from", `vacation :from "a@example.com" "Away";`, `vacation :from "b@example.com" "Away";`, false},
		{"different MIME", `vacation "Away";`, `vacation :mime "Away";`, false},
		{"tuple boundary", `vacation :subject "ab" :from "c" "Away";`, `vacation :subject "a" :from "bc" "Away";`, false},
		{"period and addresses do not change identity", `vacation :days 1 "Away";`, `vacation :days 30 :addresses "alias@example.com" "Away";`, true},
		{"same explicit handle", `vacation :handle "away" "A";`, `vacation :handle "away" :mime :subject "B" "C";`, true},
		{"different explicit handles", `vacation :handle "one" "Away";`, `vacation :handle "two" "Away";`, false},
		{"empty explicit handle", `vacation :handle "" "A";`, `vacation :handle "" "B";`, true},
		{"empty handle distinct from omitted", `vacation :handle "" "Away";`, `vacation "Away";`, false},
		// An omitted :subject gets the default subject, an explicit empty one is
		// kept (RFC 5230 section 5.3): the arguments and the replies differ, so
		// they are separate responses (section 4.2).
		{"empty subject distinct from omitted", `vacation :subject "" "Away";`, `vacation "Away";`, false},
		{"empty subjects", `vacation :subject "" "Away";`, `vacation :subject "" "Away";`, true},
		{"unexpanded implicit tuple", `set "name" "Alice"; vacation :subject "${name}" :from "${name}@example.com" "Away ${name}";`, `set "name" "Bob"; vacation :subject "${name}" :from "${name}@example.com" "Away ${name}";`, true},
		{"unexpanded explicit handle", `set "name" "Alice"; vacation :handle "${name}" "Away";`, `set "name" "Bob"; vacation :handle "${name}" "Away";`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a, _ := evaluate(t, tc.a)
			b, _ := evaluate(t, tc.b)
			if (a == b) != tc.same {
				t.Fatalf("identity equality=%v, want %v (%q, %q)", a == b, tc.same, a, b)
			}
		})
	}
	_, reply := evaluate(t, `set "name" "Alice"; vacation :subject "Hello ${name}" :handle "${name}" "Away ${name}";`)
	if reply.Subject != "Hello Alice" || reply.Handle != "Alice" || reply.Body != "Away Alice" {
		t.Fatalf("rendered fields no longer expand: %+v", reply)
	}
}

// TestVacationResponseIdentityIsStable pins identities that consumers have stored
// for RFC 5230 section 4.2 tracking: a changed key sends one more response to every
// sender still inside its :days window.
func TestVacationResponseIdentityIsStable(t *testing.T) {
	for script, want := range map[string]string{
		`vacation "Away";`:                 "v1:r:a39585b336bbc917d9e3255bb6d7a98f0954405c17c42d745a1937fd67bb4beb",
		`vacation :subject "Away" "Away";`: "v1:r:ede262faa761eeb2930255dbed5d6b5beef895b7dc4e719f9f983360c7e6ceb4",
		`vacation :handle "away" "Away";`:  "v1:h:2478d85065bcf563c04fcac130cbedb96924875c6db20bde13edeb8de18be9bc",
	} {
		opts := sieve.DefaultOptions()
		opts.EnabledExtensions = []string{"vacation"}
		s, err := sieve.Load(strings.NewReader(`require "vacation"; `+script), opts)
		if err != nil {
			t.Fatal(err)
		}
		d := sieve.NewRuntimeData(s, interp.DummyPolicy{}, interp.EnvelopeStatic{From: "sender@example.com", To: "owner@example.com"}, interp.MessageStatic{})
		if err := s.Execute(context.Background(), d); err != nil {
			t.Fatal(err)
		}
		if got := d.VacationResponses["sender@example.com"].ResponseIdentity; got != want {
			t.Errorf("%s: identity %q, want %q", script, got, want)
		}
	}
}
