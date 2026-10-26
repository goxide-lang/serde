package main

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	model "example.org/formal-serde-model"
	"github.com/goxide-lang/serde"
	json "github.com/goxide-lang/serde/json"
)

type unknown struct{ model.Event }

func require(ok bool, message string) {
	if !ok {
		panic(message)
	}
}

func main() {
	verify()
	fmt.Println("formal serde protocols passed")
}

func verify() {
	model.CheckStringSugar()
	wantGrouped := "2 7 local 7 DSD true\n{\"id\":2,\"First\":{\"Value\":7},\"Second\":{\"Value\":7},\"Optional\":null} true\ntrue true true \"\"\n"
	require(model.GroupedSerdeReport() == wantGrouped, "grouped declaration, defaults and field validation")
	for _, value := range []error{model.Alpha{}, model.Beta{}, model.Gamma{}, model.Delta{}, model.Epsilon{}, model.Zeta{}} {
		require(value.Error() == "failure", "combined macro-generated Error and serde derives")
	}
	require(model.MacroSerdeRoundTrip(), "combined macro-generated serde round trip")
	node := model.Node{Value: 1, Next: &model.Node{Value: 2}}
	encodedNode, err := json.Marshal(&node, model.EncodeNode)
	require(err == nil, "recursive record encode: "+fmt.Sprint(err))
	decodedNode, err := json.Unmarshal(encodedNode, model.DecodeNode)
	require(err == nil && reflect.DeepEqual(node, decodedNode), "recursive record round trip: "+fmt.Sprint(err))
	cycle := &model.Node{Value: 1}
	cycle.Next = cycle
	_, err = json.Marshal(cycle, model.EncodeNode)
	require(err != nil, "recursive cycle exceeded JSON bounds without an error")
	deep := strings.Repeat(`{"Value":1,"Next":`, json.MaxDepth+1) + "null" + strings.Repeat("}", json.MaxDepth+1)
	_, err = json.Unmarshal([]byte(deep), model.DecodeNode)
	require(err != nil, "excess recursive depth was accepted")
	input := `{"packet":{"Values":[1,2],"Pair":[3,4]},"Event":{"Count":5},"Generic":{"Payload":"text"},"Array":[6,7],"Slice":[],"Bytes":[0,255],"Map":{"b":2,"a":1},"Pointer":9,"Amount":10,"Small":127}`
	value, err := json.Unmarshal([]byte(input), model.DecodeRecord)
	require(err == nil, "record decode: "+fmt.Sprint(err))
	require(value.Packet.Values[1] == 2 && value.Packet.Pair[1] == 4 && value.Array[1] == 7 && value.Slice != nil && value.Map["a"] == 1 && *value.Pointer == 9 && value.Amount == 10 && value.Small == 127, "container values")
	encoded, err := json.Marshal(&value, model.EncodeRecord)
	want := `{"packet":{"Values":[1,2],"Pair":[3,4]},"Event":{"Count":5},"Generic":{"Payload":"text"},"Array":[6,7],"Slice":[],"Bytes":[0,255],"Map":{"a":1,"b":2},"Pointer":9,"Optional":null,"Amount":10,"Small":127}`
	require(err == nil && string(encoded) == want, "record encode: "+string(encoded)+" "+fmt.Sprint(err))
	withOptional := strings.ReplaceAll(strings.ReplaceAll(input, `"Map":{"b":2,"a":1}`, `"Map":{}`), `"Pointer":9`, `"Pointer":9,"Optional":4`)
	optional, err := json.Unmarshal([]byte(withOptional), model.DecodeRecord)
	require(err == nil && optional.Map != nil && len(optional.Map) == 0, "empty map and present Option decode: "+fmt.Sprint(err))
	encoded, err = json.Marshal(&optional, model.EncodeRecord)
	optionalWant := strings.ReplaceAll(strings.ReplaceAll(want, `"Map":{"a":1,"b":2}`, `"Map":{}`), `"Optional":null`, `"Optional":4`)
	require(err == nil && string(encoded) == optionalWant, "present Option round trip: "+string(encoded)+" "+fmt.Sprint(err))
	require(model.SourceCallbacks(), "source generic function callbacks")

	for _, input := range []string{`"Idle"`, `{"Count":42}`, `{"Object":43}`} {
		value, err := json.Unmarshal([]byte(input), model.DecodeEvent)
		require(err == nil, "event decode: "+fmt.Sprint(err))
		encoded, err := json.Marshal(&value, model.EncodeEvent)
		require(err == nil && string(encoded) == input, "event round trip: "+string(encoded)+" "+fmt.Sprint(err))
	}
	status, err := json.Unmarshal([]byte(`-1`), model.DecodeStatus)
	require(err == nil, "numeric enum decode: "+fmt.Sprint(err))
	encoded, err = json.Marshal(&status, model.EncodeStatus)
	require(err == nil && string(encoded) == "-1", "numeric enum encode")
	label, err := json.Unmarshal([]byte(`"ready"`), model.DecodeLabel)
	require(err == nil, "renamed enum decode: "+fmt.Sprint(err))
	encoded, err = json.Marshal(&label, model.EncodeLabel)
	require(err == nil && string(encoded) == `"ready"`, "renamed enum encode")
	for _, invalid := range []model.Event{nil, (*model.Idle)(nil), (*model.Count)(nil), (*model.Object)(nil), unknown{}} {
		_, err = json.Marshal(&invalid, model.EncodeEvent)
		checkError(err, serde.Type, nil)
	}
	_, err = json.Marshal((*model.Record)(nil), model.EncodeRecord)
	checkError(err, serde.Type, nil)
	var empty model.Empty
	_, err = json.Marshal(&empty, model.EncodeEmpty)
	checkError(err, serde.Type, nil)
	_, err = json.Unmarshal([]byte(`"anything"`), model.DecodeEmpty)
	checkError(err, serde.UnknownVariant, nil)
	_, err = json.Unmarshal([]byte(`3`), model.DecodeStatus)
	checkError(err, serde.UnknownVariant, nil)

	for _, test := range []struct {
		input, kind string
		path        []serde.PathSegment
	}{
		{`{}`, serde.MissingField, []serde.PathSegment{{Kind: "Field", Name: "packet"}}},
		{`{"packet":{"Values":[],"Pair":[1]}}`, serde.Range, []serde.PathSegment{{Kind: "Field", Name: "packet"}, {Kind: "Field", Name: "Pair"}}},
		{`{"packet":{"Values":["bad"],"Pair":[1,2]}}`, serde.Type, []serde.PathSegment{{Kind: "Field", Name: "packet"}, {Kind: "Field", Name: "Values"}, {Kind: "Index", Index: 0}}},
		{`{"packet":{"Values":[],"Pair":[1,2]},"Event":{"Count":"bad"}}`, serde.Type, []serde.PathSegment{{Kind: "Field", Name: "Event"}, {Kind: "Variant", Name: "Count"}}},
		{`{"Small":128}`, serde.Range, []serde.PathSegment{{Kind: "Field", Name: "Small"}}},
	} {
		_, err := json.Unmarshal([]byte(test.input), model.DecodeRecord)
		checkError(err, test.kind, test.path)
	}
	for _, bad := range []string{"128", "-129", "1.5"} {
		_, err := json.Unmarshal([]byte(bad), model.ReadSmall)
		checkError(err, serde.Range, nil)
	}
	for _, bad := range []string{"256", "-1"} {
		_, err := json.Unmarshal([]byte(bad), model.ReadUnsigned)
		checkError(err, serde.Range, nil)
	}
	for _, bad := range []string{"3.5e38", "1e-46"} {
		_, err := json.Unmarshal([]byte(bad), model.ReadFloat32)
		checkError(err, serde.Range, nil)
	}
	_, err = json.Unmarshal([]byte("1.5"), model.ReadSmall)
	var numeric *strconv.NumError
	require(errors.As(err, &numeric), "numeric cause was lost")

	for _, bad := range []string{`{}`, `{"Required":null,"First":"bad"}`, `{"Required":null,"Last":"bad"}`, `{"Required":null,"Extra":1}`, `{"Required":null,"Ignored":{"x":1,"x":2}}`, `{"Required":null,"Ignored":"\uD800"}`} {
		model.ResetDefaults()
		_, err := json.Unmarshal([]byte(bad), model.DecodeDefaults)
		require(err != nil && model.DefaultTrace() == "", "defaults ran before successful validation: "+bad+" "+fmt.Sprint(err))
	}
	model.ResetDefaults()
	defaults, err := json.Unmarshal([]byte(`{"Required":null}`), model.DecodeDefaults)
	require(err == nil && model.DefaultTrace() == "FSL" && defaults.First == 1 && defaults.Ignored.Index == 2 && defaults.Last == 3 && defaults.Required == nil, "business default order: "+fmt.Sprint(err))
	model.ResetDefaults()
	defaults, err = json.Unmarshal([]byte(`{"First":10,"Last":30,"Required":9,"Ignored":[{"unread":true}]}`), model.DecodeDefaults)
	require(err == nil && model.DefaultTrace() == "S" && defaults.First == 10 && defaults.Last == 30 && defaults.Ignored.Index == 1 && *defaults.Required == 9, "present and skipped defaults: "+fmt.Sprint(err))
	_, err = json.Unmarshal([]byte(`{"Required":null,"Extra":1}`), model.DecodeDefaults)
	checkError(err, serde.UnknownField, []serde.PathSegment{{Kind: "Field", Name: "Extra"}})

	encoded, err = json.Marshal(&model.SerializeOnly{Value: 17, Hidden: func() { panic("skip") }}, model.EncodeOnly)
	require(err == nil && string(encoded) == `{"Value":17}`, "Serialize required Deserialize or Default")
	read, err := json.Unmarshal([]byte(`{"Value":19}`), model.DecodeOnly)
	require(err == nil && read.Value == 19, "Deserialize required Serialize")
	nested := model.NestedSomeNone()
	encoded, err = json.Marshal(&nested, model.EncodeNested)
	require(err == nil && string(encoded) == "null", "nested Option semantics")
}

func checkError(err error, kind string, path []serde.PathSegment) {
	var detail *serde.Error
	require(errors.As(err, &detail) && detail.Kind == kind, "error kind: expected "+kind+", got "+fmt.Sprint(err))
	if path != nil {
		require(reflect.DeepEqual(detail.Path, path), fmt.Sprintf("error path = %#v, expected %#v", detail.Path, path))
	}
}
