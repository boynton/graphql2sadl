package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/boynton/sadl"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/parser"
	"github.com/graphql-go/graphql/language/source"
)

var _ = ioutil.ReadFile
var _ = sadl.Pretty

var verbose bool = false

func main() {
	pJSON := flag.Bool("j", false, "set to true to format the file as the JSON SADL model, rather than SADL source")
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Println("usage: graphql2sadl [options] file.graphql")
		os.Exit(1)
	}
	path := args[0]
	src, err := ioutil.ReadFile(path)
	doc, err := parser.Parse(parser.ParseParams{
		Source: &source.Source{
			Body: src,
			Name: "GraphQL",
		},
		Options: parser.ParseOptions{
			NoLocation: true,
		},
	})
	if err != nil {
		log.Fatalf("Cannot parse: %v\n", err)
	}
	schema, err := gqlSchema(doc)
	if err != nil {
		log.Fatalf("*** Cannot convert: %v\n", err)
	}
	model, err := sadl.NewModel(schema)
	if err != nil {
		log.Fatalf("*** Cannot validate: %v\n", err)
	}
	if *pJSON {
		fmt.Println(sadl.Pretty(model))
	} else {
		fmt.Println(sadl.Decompile(model))
	}
}

func gqlSchema(doc *ast.Document) (*sadl.Schema, error) {
	schema := &sadl.Schema{
		Name: "generatedFromGraphQL",
	}
	ignore := make(map[string]bool, 0)
	var err error
	for _, def := range doc.Definitions {
		switch tdef := def.(type) {
		case *ast.ObjectDefinition:
			if _, ok := ignore[tdef.Name.Value]; !ok {
				err = gqlStruct(schema, tdef)
			}
		case *ast.SchemaDefinition:
			for _, opt := range tdef.OperationTypes {
				iname := (*ast.Named)(opt.Type).Name.Value
				ignore[iname] = true
			}
		case *ast.EnumDefinition:
			err = gqlEnum(schema, tdef)
		case *ast.UnionDefinition:
			err = gqlUnion(schema, tdef)
		case *ast.InterfaceDefinition:
			//ignore for now fmt.Println("fix me: interfaces")
		case *ast.InputObjectDefinition:
			//ignore for now fmt.Println("fix me: input objects")
		case *ast.ScalarDefinition:
			sname := tdef.Name.Value
			switch sname {
			case "Timestamp":
			case "UUID":
				//Allow the name through, a native SADL type
			default:
				err = fmt.Errorf("Unsupported custom scalar: %s\n", sadl.Pretty(def))
			}
		default:
			err = fmt.Errorf("Unsupported definition: %v\n", def.GetKind())
		}
		if err != nil {
			return nil, err
		}
	}
	return schema, nil
}

func typeName(t ast.Type) string {
	switch tt := t.(type) {
	case *ast.Named:
		return tt.Name.Value
	case *ast.List:
		return "Array"
	default:
		panic("FixMe")
	}
}

func convertTypeName(n string) string {
	switch n {
	case "Int":
		return "Int32"
	case "Float":
		return "Float64"
	case "Boolean":
		return "Bool"
	case "ID":
		return "String" //anything else for this?!
	default:
		//Assume a reference to a user-defined type for now.
		return n
	}
}

func stringValue(sv *ast.StringValue) string {
	if sv == nil {
		return ""
	}
	return sv.Value
}

func commentValue(descr string) string {
	//SADL comments do not contain unescaped newlines
	return strings.Replace(descr, "\n", " ", -1)
}

func gqlEnum(schema *sadl.Schema, def *ast.EnumDefinition) error {
	td := &sadl.TypeDef{
		Name: def.Name.Value,
		TypeSpec: sadl.TypeSpec{
			Type: "Enum",
		},
	}
	if def.Description != nil {
		td.Comment = commentValue(def.Description.Value)
	}
	for _, symdef := range def.Values {
		el := &sadl.EnumElementDef{
			Symbol: symdef.Name.Value,
		}
		if symdef.Description != nil {
			el.Comment = commentValue(symdef.Description.Value)
		}
		td.Elements = append(td.Elements, el)
	}
	schema.Types = append(schema.Types, td)
	return nil
}

func gqlUnion(schema *sadl.Schema, def *ast.UnionDefinition) error {
	td := &sadl.TypeDef{
		Name: def.Name.Value,
		TypeSpec: sadl.TypeSpec{
			Type: "Union",
		},
	}
	if def.Description != nil {
		td.Comment = commentValue(def.Description.Value)
	}
	for _, vardef := range def.Types {
		td.Variants = append(td.Variants, vardef.Name.Value)
	}
	schema.Types = append(schema.Types, td)
	return nil
}

func gqlStruct(schema *sadl.Schema, structDef *ast.ObjectDefinition) error {
	td := &sadl.TypeDef{
		Name:    structDef.Name.Value,
		Comment: commentValue(stringValue(structDef.Description)),
		TypeSpec: sadl.TypeSpec{
			Type: "Struct",
		},
	}
	for _, fnode := range structDef.Fields {
		f := (*ast.FieldDefinition)(fnode)
		fd := &sadl.StructFieldDef{
			Name:    f.Name.Value,
			Comment: commentValue(stringValue(f.Description)),
		}
		switch t := (*ast.FieldDefinition)(fnode).Type.(type) {
		case *ast.Named:
			fd.Type = convertTypeName(t.Name.Value)
		case *ast.List:
			fd.Type = "Array"
			switch it := t.Type.(type) {
			case *ast.Named:
				fd.Items = convertTypeName(it.Name.Value)
			case *ast.NonNull:
				switch it := it.Type.(type) {
				case *ast.Named:
					fd.Items = convertTypeName(it.Name.Value)
				default:
					panic("inline list type not supported")
				}
			default:
				panic("list type not supported")
			}
		case *ast.NonNull:
			fd.Required = true
			switch t := t.Type.(type) {
			case *ast.Named:
				fd.Type = convertTypeName(t.Name.Value)
			case *ast.List:
				fd.Type = "Array"
				switch it := t.Type.(type) {
				case *ast.Named:
					fd.Items = convertTypeName(it.Name.Value)
				case *ast.NonNull:
					fd.Items = convertTypeName(typeName(it.Type))
				default:
					panic("inline list type not supported")
				}
			default:
				fd.Type = convertTypeName(typeName(t))
			}
		}
		td.Fields = append(td.Fields, fd)
	}
	schema.Types = append(schema.Types, td)
	return nil
}
