package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"sort"
	"strings"
)

// Schema represents JSON schema.
type Schema struct {
	Title             string             `json:"title,omitempty"`
	ID                string             `json:"id,omitempty"`
	Type              SchemaType         `json:"type,omitempty"`
	Description       string             `json:"description,omitempty"`
	Definitions       map[string]*Schema `json:"definitions,omitempty"`
	Properties        map[string]*Schema `json:"properties,omitempty"`
	PatternProperties map[string]*Schema `json:"patternProperties,omitempty"`
	Ref               string             `json:"$ref,omitempty"`
	Items             *Schema            `json:"items,omitempty"`
	Root              *Schema            `json:"-"`
}

// Name will attempt to determine the name of the Schema element using
// the Title or ID or Description (in that order)
func (s *Schema) Name() string {
	name := s.Title
	if name == "" {
		name = s.ID
	}
	if name == "" {
		return s.Description
	}
	return name
}

// SchemaType is an ENUM that is set on parsing the schema
type SchemaType int

const (
	// ANY is a schema element that has no defined type
	ANY SchemaType = iota
	// ARRAY is a schema type "array"
	ARRAY SchemaType = iota
	// BOOLEAN is a schema type "boolean"
	BOOLEAN SchemaType = iota
	// INTEGER is a schema type "integer"
	INTEGER SchemaType = iota
	// NUMBER is a schema type "number"
	NUMBER SchemaType = iota
	// NULL is a schema type "null"
	NULL SchemaType = iota
	// OBJECT is a schema type "object"
	OBJECT SchemaType = iota
	// STRING is a schema type "string"
	STRING SchemaType = iota
)

// UnmarshalJSON for SchemaType so we can parse the schema
// type string and set the ENUM
func (s *SchemaType) UnmarshalJSON(b []byte) error {
	var schemaType string
	err := json.Unmarshal(b, &schemaType)
	if err != nil {
		return err
	}
	types := map[string]SchemaType{
		"array":   ARRAY,
		"boolean": BOOLEAN,
		"integer": INTEGER,
		"number":  NUMBER,
		"null":    NULL,
		"object":  OBJECT,
		"string":  STRING,
	}
	if val, ok := types[schemaType]; ok {
		*s = val
		return nil
	}
	return fmt.Errorf("Unknown schema type \"%s\"", schemaType)
}

func (s *SchemaType) MarshalJSON() ([]byte, error) {
	switch *s {
	case ANY:
		return []byte("\"object\""), nil
	case ARRAY:
		return []byte("\"array\""), nil
	case BOOLEAN:
		return []byte("\"boolean\""), nil
	case INTEGER:
		return []byte("\"integer\""), nil
	case NUMBER:
		return []byte("\"number\""), nil
	case NULL:
		return []byte("\"null\""), nil
	case OBJECT:
		return []byte("\"object\""), nil
	case STRING:
		return []byte("\"string\""), nil
	}
	return nil, fmt.Errorf("Unknown Schema Type: %#v", s)
}

func main() {
	outputDir := flag.String("dir", ".", "output directory for go files")
	pkgName := flag.String("pkg", "main", "package namespace for go files")
	overwrite := flag.Bool("overwrite", false, "force overwriting existing go files")
	stdout := flag.Bool("stdout", false, "print go code to stdout rather than files")
	format := flag.Bool("fmt", true, "pass code through gofmt")
	flag.Parse()

	processor := &SchemaProcessor{
		OutputDir:   *outputDir,
		PackageName: *pkgName,
		Overwrite:   *overwrite,
		Stdout:      *stdout,
		Fmt:         *format,
	}

	err := processor.Process(flag.Args())
	if err != nil {
		panic(err)
	}
}

// SchemaProcessor object used to convert json schemas to golang structs
type SchemaProcessor struct {
	OutputDir   string
	PackageName string
	Overwrite   bool
	Stdout      bool
	Fmt         bool
	processed   map[string]bool
}

// Process will read a list of json schema files, parse them
// and write them to the OutputDir
func (s *SchemaProcessor) Process(files []string) error {
	for _, file := range files {
		fh, err := os.OpenFile(file, os.O_RDONLY, 0644)
		defer fh.Close()
		b, err := ioutil.ReadAll(fh)
		if err != nil {
			return err
		}

		schema, err := s.ParseSchema(b)
		if err != nil {
			return err
		}

		_, err = s.processSchema(schema)
		if err != nil {
			return err
		}
	}
	return nil
}

// ParseSchema simply parses the schema and post-processes the objects
// so each knows the Root object and also resolve/flatten any $ref objects
// found in the document.
func (s *SchemaProcessor) ParseSchema(data []byte) (*Schema, error) {
	schema := &Schema{}
	err := json.Unmarshal(data, schema)
	if err != nil {
		return nil, err
	}
	setRoot(schema, schema)
	return schema, nil
}

func setRoot(root, schema *Schema) {
	schema.Root = root
	if schema.Properties != nil {
		for k, v := range schema.Properties {
			setRoot(root, v)
			if v.Name() == "" {
				v.Title = k
			}
		}
	}
	if schema.PatternProperties != nil {
		for _, v := range schema.PatternProperties {
			setRoot(root, v)
		}
	}
	if schema.Items != nil {
		setRoot(root, schema.Items)
	}

	if schema.Ref != "" {
		schemaPath := strings.Split(schema.Ref, "/")
		var ctx interface{}
		ctx = schema
		for _, part := range schemaPath {
			if part == "#" {
				ctx = root
			} else if part == "definitions" {
				ctx = ctx.(*Schema).Definitions
			} else if part == "properties" {
				ctx = ctx.(*Schema).Properties
			} else if part == "patternProperties" {
				ctx = ctx.(*Schema).PatternProperties
			} else if part == "items" {
				ctx = ctx.(*Schema).Items
			} else {
				if cast, ok := ctx.(map[string]*Schema); ok {
					ctx = cast[part]
				}
			}
		}
		if cast, ok := ctx.(*Schema); ok {
			*schema = *cast
		}
	}
}

func camelCase(name string) string {
	caseName := strings.Title(
		strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				return r
			}
			return ' '
		}, name),
	)
	caseName = strings.Replace(caseName, " ", "", -1)

	for _, suffix := range []string{"Id", "Url", "Json", "Xml"} {
		if strings.HasSuffix(caseName, suffix) {
			return strings.TrimSuffix(caseName, suffix) + strings.ToUpper(suffix)
		}
	}

	for _, prefix := range []string{"Url", "Json", "Xml"} {
		if strings.HasPrefix(caseName, prefix) {
			return strings.ToUpper(prefix) + strings.TrimPrefix(caseName, prefix)
		}
	}

	return caseName
}

func (s *SchemaProcessor) processSchema(schema *Schema) (typeName string, err error) {
	prettyPrint, err := json.MarshalIndent(schema, "// ", "  ")
	if err != nil {
		return "", err
	}
	if schema.Type == OBJECT {
		typeName = camelCase(schema.Name())
		if schema.Properties != nil {
			typeData := fmt.Sprintf("// %s defined from schema:\n// %s\ntype %s struct {\n", typeName, prettyPrint, typeName)
			keys := []string{}
			for k := range schema.Properties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := schema.Properties[k]
				subTypeName, err := s.processSchema(v)
				if err != nil {
					return "", err
				}
				typeData += fmt.Sprintf("    %s %s `json:\"%s,omitempty\" yaml:\"%s,omitempty\"`\n", camelCase(k), subTypeName, k, k)
			}
			typeData += "}\n\n"
			if err := s.writeGoCode(typeName, typeData); err != nil {
				return "", err
			}
			typeName = fmt.Sprintf("*%s", typeName)
		} else if schema.PatternProperties != nil {
			keys := []string{}
			for k := range schema.PatternProperties {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				v := schema.PatternProperties[k]
				subTypeName, err := s.processSchema(v)
				if err != nil {
					return "", err
				}

				// verify subTypeName is not a simple type
				if strings.Title(subTypeName) == subTypeName {
					typeName = strings.TrimPrefix(fmt.Sprintf("%sMap", subTypeName), "*")
					typeData := fmt.Sprintf("// %s defined from schema:\n// %s\ntype %s map[string]%s\n\n", typeName, prettyPrint, typeName, subTypeName)
					if err := s.writeGoCode(typeName, typeData); err != nil {
						return "", err
					}
				} else {
					typeName = fmt.Sprintf("map[string]%s", subTypeName)
				}
			}
		}
	} else if schema.Type == ARRAY {
		subTypeName, err := s.processSchema(schema.Items)
		if err != nil {
			return "", err
		}
		if strings.Title(subTypeName) == subTypeName {
			if strings.HasSuffix(subTypeName, "s") {
				typeName = fmt.Sprintf("%ses", subTypeName)
			} else {
				typeName = fmt.Sprintf("%ss", subTypeName)
			}
			typeName = strings.TrimPrefix(typeName, "*")
			typeData := fmt.Sprintf("// %s defined from schema:\n// %s\ntype %s []%s\n\n", typeName, prettyPrint, typeName, subTypeName)
			if err := s.writeGoCode(typeName, typeData); err != nil {
				return "", err
			}
		} else {
			typeName = fmt.Sprintf("[]%s", subTypeName)
		}
	} else if schema.Type == BOOLEAN {
		typeName = "bool"
	} else if schema.Type == INTEGER {
		typeName = "int"
	} else if schema.Type == NUMBER {
		typeName = "float"
	} else if schema.Type == NULL || schema.Type == ANY {
		typeName = "interface{}"
	} else if schema.Type == STRING {
		typeName = "string"
	}
	return
}

func (s *SchemaProcessor) writeGoCode(typeName, code string) error {
	if seen, ok := s.processed[typeName]; ok && seen {
		return nil
	}
	// mark schemas as processed so we dont print/write it out again
	if s.processed == nil {
		s.processed = map[string]bool{
			typeName: true,
		}
	} else {
		s.processed[typeName] = true
	}

	if s.Stdout {
		if s.Fmt {
			cmd := exec.Command("gofmt", "-s")
			inPipe, _ := cmd.StdinPipe()
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Start()
			inPipe.Write([]byte(code))
			inPipe.Close()
			return cmd.Wait()
		}
		fmt.Print(code)
		return nil
	}
	file := path.Join(s.OutputDir, fmt.Sprintf("%s.go", typeName))
	if !s.Overwrite {
		if _, err := os.Stat(file); err == nil {
			log.Printf("File %s already exists, skipping without -overwrite", file)
			return nil
		}
	}
	fh, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	//defer fh.Close()
	preamble := fmt.Sprintf("package %s\n", s.PackageName)
	preamble += fmt.Sprintf(`
/////////////////////////////////////////////////////////////////////////
// This Code is Generated by SlipScheme Project:
// https://github.com/coryb/slipscheme
// 
// Generated with command:
// %s
/////////////////////////////////////////////////////////////////////////
//                            DO NOT EDIT                              //
/////////////////////////////////////////////////////////////////////////

`, strings.Join(os.Args, " "))

	if _, err := fh.Write([]byte(preamble)); err != nil {
		return err
	}
	if _, err := fh.Write([]byte(code)); err != nil {
		return err
	}

	if s.Fmt {
		cmd := exec.Command("gofmt", "-s", "-w", file)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	return nil
}
