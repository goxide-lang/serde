package serde_macros

import (
	"strings"
	"testing"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/token"
)

func deriveForTest(t *testing.T, source string, write bool) pm.Output {
	t.Helper()
	scope := token.NewScope(t.Name())
	definition, call := scope.NewContext(), scope.NewContext()
	parse := func(source string) token.TokenStream {
		stream, err := token.ParseText(source, call)
		if err != nil {
			t.Fatal(err)
		}
		return stream
	}
	span, err := scope.NewSpan(0, uint32(len(source)))
	if err != nil {
		t.Fatal(err)
	}
	request, err := pm.NewRequest(scope, pm.RequestConfig{
		Entry: "derive", Kind: pm.Derive, Position: pm.ItemsPosition,
		DefSite: definition, CallSite: call, DefSpan: span, CallSpan: span,
		Target: pm.Target{OS: "linux", Arch: "amd64"}, Item: parse(source),
		RuntimeRoots: map[string]token.TokenStream{
			"github.com/goxide-lang/serde": parse("serde"),
			"github.com/goxide-lang/std":   parse("standard"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return deriveSerde(request.Context(), request.Item(), write)
}

func TestDeriveRejectsInvalidWireContracts(t *testing.T) {
	for _, test := range []struct{ name, source, diagnostic string }{
		{"duplicate-wire", `struct Record{#[serde(rename="id")]First int;#[serde(rename="id")]Second int}`, "duplicate serde field name"},
		{"duplicate-option", `struct Record{#[serde(skip,skip)]Value int}`, "duplicate serde skip"},
		{"mapped-payload", `enum Event{#[serde(rename="n")]Number(int)}`, "requires unit variants"},
		{"partial-number", `enum Event{#[serde(value=1)]One,Two}`, "value on every variant"},
		{"mixed-number", `enum Event{#[serde(value=1)]One,#[serde(rename="two")]Two}`, "cannot mix rename"},
		{"duplicate-number", `enum Event{#[serde(value=1)]One,#[serde(value=1)]Two}`, "duplicate enum serde mapping"},
		{"out-of-range-number", `enum Event{#[serde(value=9223372036854775808)]One}`, "signed int64"},
		{"tuple-payload", `enum Event{Pair(int,int)}`, "single payload"},
		{"variant-field", `enum Event{Item{#[serde(skip)]Value int}}`, "invalid attribute position"},
		{"container-option", `#[serde(skip)]struct Record{Value int}`, "not supported at this position"},
		{"field-option", `struct Record{#[serde(deny_unknown_fields)]Value int}`, "not supported at this position"},
		{"alias", `type Alias = struct{Value int}`, "defined struct or enum"},
	} {
		t.Run(test.name, func(t *testing.T) {
			for _, write := range []bool{false, true} {
				output := deriveForTest(t, test.source, write)
				if !output.Tokens.IsEmpty() || len(output.Diagnostics) != 1 || !strings.Contains(output.Diagnostics[0].Message, test.diagnostic) {
					t.Fatalf("write=%v: output=%s diagnostics=%+v, want %q", write, output.Tokens.Display(), output.Diagnostics, test.diagnostic)
				}
			}
		})
	}
}
