package serde_macros

import (
	"fmt"
	"strconv"
	"strings"

	pm "github.com/goxide-lang/syntax/proc_macro"
	"github.com/goxide-lang/syntax/syn"
	"github.com/goxide-lang/syntax/token"
)

func serdeFragment(ctx pm.Context, source string) token.TokenStream {
	stream, err := token.ParseText(source, ctx.DefSite())
	if err != nil {
		panic(err)
	}
	nodes := stream.Tokens()
	for len(nodes) > 0 && nodes[len(nodes)-1].Kind() == token.SemicolonKind {
		nodes = nodes[:len(nodes)-1]
	}
	return token.New(nodes...)
}
func serdeGroup(ctx pm.Context, delimiter token.Delimiter, stream token.TokenStream) token.Token {
	result, err := token.NewGroup(delimiter, stream, ctx.DefSite())
	if err != nil {
		panic(err)
	}
	return result
}
func serdeSplice(ctx pm.Context, stream token.TokenStream, bindings map[string]token.TokenStream) token.TokenStream {
	result := token.New()
	for _, node := range stream.Tokens() {
		if value, ok := bindings[node.Text()]; ok && node.Kind() == token.IdentKind {
			result = result.Append(value.Tokens()...)
			continue
		}
		if node.Kind() == token.GroupKind {
			node = serdeGroup(ctx, node.Delimiter(), serdeSplice(ctx, node.Children(), bindings))
		}
		result = result.Append(node)
	}
	return result
}

type serdeField struct {
	wire         string
	renamed      bool
	defaultValue bool
	skip         bool
	deny         bool
	number       *int64
}

func serdeOptions(attributes []syn.Attribute, name, allowed string) (serdeField, error) {
	options := serdeField{wire: name}
	seen := map[string]bool{}
	for _, attribute := range attributes {
		path := attribute.Path().Tokens()
		if len(path) != 1 || path[0].Text() != "serde" {
			continue
		}
		arguments := attribute.Arguments().Tokens()
		if len(arguments) != 1 || arguments[0].Kind() != token.GroupKind || arguments[0].Delimiter() != token.Parenthesis {
			return options, fmt.Errorf("invalid serde attribute")
		}
		parts := arguments[0].Children().Tokens()
		if len(parts) == 0 {
			return options, fmt.Errorf("serde requires an attribute argument")
		}
		for index := 0; index < len(parts); {
			option := parts[index].Text()
			index++
			if !strings.Contains(" "+allowed+" ", " "+option+" ") {
				return options, fmt.Errorf("serde %s is not supported at this position", option)
			}
			if seen[option] {
				return options, fmt.Errorf("duplicate serde %s attribute", option)
			}
			seen[option] = true
			switch option {
			case "default":
				options.defaultValue = true
			case "skip":
				options.skip = true
			case "deny_unknown_fields":
				options.deny = true
			case "rename", "value":
				if index+1 >= len(parts) || parts[index].Text() != "=" {
					return options, fmt.Errorf("serde %s requires a literal value", option)
				}
				index++
				literal := parts[index].Text()
				index++
				if option == "rename" {
					var err error
					options.wire, err = strconv.Unquote(literal)
					if err != nil || !strings.HasPrefix(literal, "\"") && !strings.HasPrefix(literal, "`") {
						return options, fmt.Errorf("serde rename requires a string literal")
					}
					options.renamed = true
				} else {
					if literal == "-" && index < len(parts) {
						literal += parts[index].Text()
						index++
					}
					number, err := strconv.ParseInt(literal, 10, 64)
					if err != nil {
						return options, fmt.Errorf("serde value requires a signed int64 decimal literal")
					}
					options.number = &number
				}
			}
			if index < len(parts) {
				if parts[index].Text() != "," {
					return options, fmt.Errorf("serde attribute arguments must be comma-separated")
				}
				index++
			}
		}
	}
	return options, nil
}

func itemTarget(ctx pm.Context, item syn.Item) token.TokenStream {
	// A declaration name can originate in a declarative ident fragment. Refer
	// to the declared type with its binding context, not that opaque fragment.
	name := item.Name()
	identifier, err := token.NewIdent(name.Text(), name.Raw(), name.Context())
	if err != nil {
		panic(err)
	}
	target := token.New(identifier.WithSpan(name.Span()))
	parameters := item.Generics().Tokens()
	if len(parameters) == 0 {
		return target
	}
	var names []token.Token
	first := true
	for _, node := range parameters[0].Children().Tokens() {
		if node.Kind() == token.PunctKind && node.Text() == "," {
			first = true
			continue
		}
		if !first || node.Text() == "const" {
			continue
		}
		if len(names) != 0 {
			names = append(names, serdeFragment(ctx, ",").Tokens()...)
		}
		names = append(names, node)
		first = false
	}
	return target.Append(serdeGroup(ctx, token.Bracket, token.New(names...)))
}
func deriveSerde(ctx pm.Context, input token.TokenStream, write bool) pm.Output {
	item, err := syn.Parse(input)
	if err != nil {
		return pm.Failure(err.Error(), ctx.CallSpan())
	}
	if item.Alias() {
		return pm.Failure("serde derives require a defined struct or enum declaration", item.Span())
	}
	root, err := ctx.RuntimeRoot("github.com/goxide-lang/serde")
	if err != nil {
		return pm.Failure(err.Error(), ctx.CallSpan())
	}
	bindings := map[string]token.TokenStream{"__runtime": root, "__generics": item.Generics(), "__target": itemTarget(ctx, item)}
	protocol := "Deserialize"
	if write {
		protocol = "Serialize"
	}
	var body strings.Builder
	bounds := []string{}
	fields, record := item.Type().StructFields()
	if record {
		container, err := serdeOptions(item.Attributes(), "", "deny_unknown_fields")
		if err != nil {
			return pm.Failure(err.Error(), item.Span())
		}
		names, wires, types := []string{}, []string{}, []string{}
		options := []serdeField{}
		seen := map[string]bool{}
		for _, field := range fields {
			if len(field.Names()) == 0 {
				return pm.Failure("embedded fields are not supported", field.Span())
			}
			for _, name := range field.Names() {
				if name.Text() == "_" {
					return pm.Failure("blank fields are not supported", field.Span())
				}
				option, err := serdeOptions(field.Attributes(), name.Text(), "rename default skip")
				if err != nil {
					return pm.Failure(err.Error(), field.Span())
				}
				if seen[option.wire] {
					return pm.Failure("duplicate serde field name "+strconv.Quote(option.wire), field.Span())
				}
				seen[option.wire] = true
				n := len(names)
				fieldName, fieldType := fmt.Sprintf("__field%d", n), fmt.Sprintf("__type%d", n)
				names = append(names, fieldName)
				types = append(types, fieldType)
				wires = append(wires, strconv.Quote(option.wire))
				options = append(options, option)
				bindings[fieldName], bindings[fieldType] = name.ToTokens(), field.Type().ToTokens()
				if !option.skip {
					bounds = append(bounds, fieldType+":__runtime::"+protocol)
				}
				if !write && (option.defaultValue || option.skip) {
					standard, err := ctx.RuntimeRoot("github.com/goxide-lang/std")
					if err != nil {
						return pm.Failure(err.Error(), field.Span())
					}
					bindings["__standard"] = standard
					bounds = append(bounds, fieldType+":__standard::Default")
				}
			}
		}
		var activeWires []string
		for i, wire := range wires {
			if !options[i].skip {
				activeWires = append(activeWires, wire)
			}
		}
		if write {
			fmt.Fprintf(&body, "func serialize(self &Self,serializer __runtime::Serializer)error { writer,err:=serializer.Struct(%q,%d);if err!=nil{return err};", item.Name().Text(), len(activeWires))
			for i, name := range names {
				if options[i].skip {
					continue
				}
				fmt.Fprintf(&body, "if err:=writer.Field(%s,func(child __runtime::Serializer)error{return __runtime::Write(child,&self.%s)});err!=nil{return err};", wires[i], name)
			}
			body.WriteString("return writer.End() }")
		} else {
			body.WriteString("func deserialize(deserializer __runtime::Deserializer)(Self,error){var result Self;")
			for i := range names {
				if !options[i].skip {
					fmt.Fprintf(&body, "seen%d:=false;", i)
				}
			}
			fmt.Fprintf(&body, "err:=deserializer.Struct(%q,[]string{%s},__runtime::VisitorFuncs{OnMap:func(fields __runtime::MapAccess)error{for{var name string;more,err:=fields.NextKey(func(child __runtime::Deserializer)error{value,err:=__runtime::ReadString(child);name=value;return err});if err!=nil{return err};if !more{break};switch name{", item.Name().Text(), strings.Join(activeWires, ","))
			for i, name := range names {
				if options[i].skip {
					fmt.Fprintf(&body, "case %s:if err:=fields.NextValue(func(child __runtime::Deserializer)error{return child.Skip()});err!=nil{return err};", wires[i])
					continue
				}
				fmt.Fprintf(&body, "case %s:if seen%d{return __runtime::AtField(__runtime::NewError(__runtime::DuplicateField,\"duplicate field\"),name)};seen%d=true;if err:=fields.NextValue(func(child __runtime::Deserializer)error{value,err:=__runtime::Read[%s](child);result.%s=value;return err});err!=nil{return err};", wires[i], i, i, types[i], name)
			}
			if container.deny {
				body.WriteString("default:return __runtime::AtField(__runtime::NewError(__runtime::UnknownField,\"unknown field\"),name)")
			} else {
				body.WriteString("default:if err:=fields.NextValue(func(child __runtime::Deserializer)error{return child.Skip()});err!=nil{return err}")
			}
			body.WriteString("}};return nil}});if err!=nil{return result,err};")
			for i, name := range names {
				if options[i].skip || options[i].defaultValue {
					continue
				}
				fmt.Fprintf(&body, "if !seen%d{value,optional:=__runtime::Missing[%s]();if !optional{return result,__runtime::AtField(__runtime::NewError(__runtime::MissingField,\"required field missing\"),%s)};result.%s=value};", i, types[i], wires[i], name)
			}
			// Validate every required field before any business default runs.
			for i, name := range names {
				if options[i].skip {
					fmt.Fprintf(&body, "result.%s=__standard::default_value[%s]();", name, types[i])
				} else if options[i].defaultValue {
					fmt.Fprintf(&body, "if !seen%d{result.%s=__standard::default_value[%s]()};", i, name, types[i])
				}
			}
			body.WriteString("return result,nil};func missing()(Self,bool){var zero Self;return zero,false}")
		}
	} else if item.Kind() == syn.EnumItem {
		if _, err := serdeOptions(item.Attributes(), "", ""); err != nil {
			return pm.Failure(err.Error(), item.Span())
		}
		variants := item.Variants()
		wires, patterns, constructors := []string{}, []string{}, []string{}
		options := []serdeField{}
		payloads := []bool{}
		numeric, mapped := false, false
		for i, variant := range variants {
			name := fmt.Sprintf("__variant%d", i)
			bindings[name] = variant.Name().ToTokens()
			option, err := serdeOptions(variant.Attributes(), variant.Name().Text(), "rename value")
			if err != nil {
				return pm.Failure(err.Error(), variant.Span())
			}
			options = append(options, option)
			numeric = numeric || option.number != nil
			mapped = mapped || option.number != nil || option.renamed
			wires = append(wires, strconv.Quote(option.wire))
			fields := variant.Fields()
			count := 0
			for _, field := range fields {
				if _, err := serdeOptions(field.Attributes(), "", ""); err != nil {
					return pm.Failure("invalid attribute position: "+err.Error(), field.Span())
				}
				if variant.Kind() == syn.TupleVariant {
					count++
				} else {
					count += len(field.Names())
				}
			}
			if count > 1 {
				return pm.Failure("serde enum variant "+variant.Name().Text()+" requires a unit or single payload", variant.Span())
			}
			payloads = append(payloads, count != 0)
			path := fmt.Sprintf("__target::__variant%d", i)
			pattern, constructor := path, path
			if count == 1 {
				typ := fmt.Sprintf("__payload%d", i)
				bindings[typ] = fields[0].Type().ToTokens()
				bounds = append(bounds, typ+":__runtime::"+protocol)
				pattern, constructor = path+"(value)", path+"(value)"
				if variant.Kind() == syn.StructVariant {
					field := fields[0].Names()[0]
					if field.Text() == "_" {
						return pm.Failure("blank fields are not supported", field.Span())
					}
					key := fmt.Sprintf("__payloadName%d", i)
					bindings[key] = field.ToTokens()
					pattern, constructor = path+"{"+key+":value}", path+"{"+key+":value}"
				}
			}
			patterns, constructors = append(patterns, pattern), append(constructors, constructor)
		}
		seen := map[string]bool{}
		for i, variant := range variants {
			if mapped && payloads[i] {
				return pm.Failure("explicit enum serde mapping requires unit variants", variant.Span())
			}
			if numeric {
				if options[i].number == nil || options[i].renamed {
					return pm.Failure("numeric enum serde mapping requires value on every variant and cannot mix rename", variant.Span())
				}
				wires[i] = strconv.FormatInt(*options[i].number, 10)
			}
			if seen[wires[i]] {
				return pm.Failure("duplicate enum serde mapping "+wires[i], variant.Span())
			}
			seen[wires[i]] = true
		}
		if write {
			body.WriteString("func serialize(self &Self,serializer __runtime::Serializer)error{")
			if len(variants) == 0 {
				body.WriteString("return __runtime::NewError(__runtime::Type,\"invalid empty enum\")}")
			} else {
				body.WriteString("if !enum_valid(*self){return __runtime::NewError(__runtime::Type,\"invalid enum\")};return match &*self{")
				for i := range variants {
					if numeric {
						fmt.Fprintf(&body, "%s=>serializer.Int64(%s),", patterns[i], wires[i])
					} else if !payloads[i] {
						fmt.Fprintf(&body, "%s=>serializer.UnitVariant(%q,%s),", patterns[i], item.Name().Text(), wires[i])
					} else {
						fmt.Fprintf(&body, "%s=>serializer.NewtypeVariant(%q,%s,func(child __runtime::Serializer)error{return __runtime::Write(child,value)}),", patterns[i], item.Name().Text(), wires[i])
					}
				}
				body.WriteString("}}")
			}
		} else if numeric {
			body.WriteString("func deserialize(deserializer __runtime::Deserializer)(Self,error){var result Self;value,err:=__runtime::ReadInt64(deserializer);if err!=nil{return result,err};switch value{")
			for i := range variants {
				fmt.Fprintf(&body, "case %s:return %s,nil;", wires[i], constructors[i])
			}
			body.WriteString("default:return result,__runtime::NewError(__runtime::UnknownVariant,\"unknown numeric variant\")}};func missing()(Self,bool){var zero Self;return zero,false}")
		} else {
			accessName := "variant"
			if len(variants) == 0 {
				accessName = "_"
			}
			fmt.Fprintf(&body, "func deserialize(deserializer __runtime::Deserializer)(Self,error){var result Self;err:=deserializer.Enum(%q,[]string{%s},__runtime::VisitorFuncs{OnEnum:func(access __runtime::EnumAccess)error{name,%s,err:=access.Variant();if err!=nil{return err};switch name{", item.Name().Text(), strings.Join(wires, ","), accessName)
			for i := range variants {
				if !payloads[i] {
					fmt.Fprintf(&body, "case %s:if err:=variant.Unit();err!=nil{return err};result=%s;return nil;", wires[i], constructors[i])
				} else {
					fmt.Fprintf(&body, "case %s:return variant.Newtype(func(child __runtime::Deserializer)error{value,err:=__runtime::Read[__payload%d](child);if err!=nil{return err};result=%s;return nil});", wires[i], i, constructors[i])
				}
			}
			body.WriteString("default:return __runtime::NewError(__runtime::UnknownVariant,name)}}});return result,err};func missing()(Self,bool){var zero Self;return zero,false}")
		}
	} else {
		return pm.Failure("serde derives require defined structs and enums", item.Span())
	}
	where := ""
	// Concrete implementations prove field protocols in their ordinary bodies.
	// Making them prerequisites would introduce a spurious cycle for Node ->
	// *Node (and mutually recursive records) instead of ordinary recursion.
	if item.Generics().IsEmpty() {
		bounds = nil
	}
	if len(bounds) > 0 {
		where = "where " + strings.Join(bounds, ",")
	}
	if !item.Where().IsEmpty() {
		bindings["__where"] = item.Where()
		where = "__where"
		if len(bounds) != 0 {
			where += "," + strings.Join(bounds, ",")
		}
	}
	source := "impl __generics __runtime::" + protocol + " for __target " + where + " {" + body.String() + "};"
	return pm.Success(serdeSplice(ctx, serdeFragment(ctx, source), bindings))
}
