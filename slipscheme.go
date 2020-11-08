package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
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
	Title                string             `json:"title,omitempty"`
	ID                   string             `json:"id,omitempty"`
	Type                 SchemaType         `json:"type,omitempty"`
	Description          string             `json:"description,omitempty"`
	Definitions          map[string]*Schema `json:"definitions,omitempty"`
	Properties           map[string]*Schema `json:"properties,omitempty"`
	AdditionalProperties bool               `json:"additionalProperties,omitempty"`
	PatternProperties    map[string]*Schema `json:"patternProperties,omitempty"`
	Ref                  string             `json:"$ref,omitempty"`
	Items                *Schema            `json:"items,omitempty"`
	OneOf                []*Schema          `json:"oneOf,omitempty"`
	Const                string             `json:"const,omitempty"`
	Enum                 []string           `json:"enum,omitempty"`
	Root                 *Schema            `json:"-"`
	// only populated on Root node
	raw map[string]interface{}
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

var schemaTypes = map[string]SchemaType{
	"array":   ARRAY,
	"boolean": BOOLEAN,
	"integer": INTEGER,
	"number":  NUMBER,
	"null":    NULL,
	"object":  OBJECT,
	"string":  STRING,
}

// UnmarshalJSON for SchemaType so we can parse the schema
// type string and set the ENUM
func (s *SchemaType) UnmarshalJSON(b []byte) error {
	var schemaType string
	err := json.Unmarshal(b, &schemaType)
	if err != nil {
		return err
	}
	if val, ok := schemaTypes[schemaType]; ok {
		*s = val
		return nil
	}
	return fmt.Errorf("Unknown schema type \"%s\"", schemaType)
}

// MarshalJSON for SchemaType so we serialized the schema back
// to json for debugging
func (s *SchemaType) MarshalJSON() ([]byte, error) {
	schemaType := s.String()
	if schemaType == "unknown" {
		return nil, fmt.Errorf("Unknown Schema Type: %#v", s)
	}
	return []byte(fmt.Sprintf("%q", schemaType)), nil
}

func (s SchemaType) String() string {
	switch s {
	case ANY:
		return "any"
	case ARRAY:
		return "array"
	case BOOLEAN:
		return "boolean"
	case INTEGER:
		return "integer"
	case NUMBER:
		return "number"
	case NULL:
		return "null"
	case OBJECT:
		return "object"
	case STRING:
		return "string"
	}
	return "unknown"
}

func main() {
	exitCode := runMain(os.Args, Stdio{
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	})
	os.Exit(exitCode)
}

// Stdio holds common io readers/writers
type Stdio struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

func runMain(arguments []string, io Stdio) int {
	flags := flag.NewFlagSet(arguments[0], flag.ExitOnError)
	outputDir := flags.String("dir", ".", "output directory for go files")
	pkgName := flags.String("pkg", "main", "package namespace for go files")
	overwrite := flags.Bool("overwrite", false, "force overwriting existing go files")
	stdout := flags.Bool("stdout", false, "print go code to stdout rather than files")
	format := flags.Bool("fmt", true, "pass code through gofmt")
	comments := flags.Bool("comments", true, "enable/disable print comments")

	flags.SetOutput(io.Stderr)
	flags.Parse(arguments[1:])

	processor := &SchemaProcessor{
		OutputDir:   *outputDir,
		PackageName: *pkgName,
		Overwrite:   *overwrite,
		Stdout:      *stdout,
		Fmt:         *format,
		Comment:     *comments,
		IO:          io,
	}

	args := flags.Args()
	if len(args) == 0 {
		flags.SetOutput(io.Stdout)
		fmt.Fprintf(io.Stdout, "Usage: %s <schema file> [<schema file> ...]\n", arguments[0])
		flags.PrintDefaults()
		return 0
	}
	err := processor.Process(args)
	if err != nil {
		fmt.Fprintf(io.Stderr, "Error: %s\n", err)
		return 1
	}
	return 0
}

// SchemaProcessor object used to convert json schemas to golang structs
type SchemaProcessor struct {
	OutputDir   string
	PackageName string
	Overwrite   bool
	Stdout      bool
	Fmt         bool
	Comment     bool
	processed   map[string]bool
	IO          Stdio
}

// Process will read a list of json schema files, parse them
// and write them to the OutputDir
func (s *SchemaProcessor) Process(files []string) error {
	for _, file := range files {
		var r io.Reader
		var b []byte
		if file == "-" {
			r = s.IO.Stdin
		} else {
			fh, err := os.OpenFile(file, os.O_RDONLY, 0644)
			defer fh.Close()
			if err != nil {
				return err
			}
			r = fh
		}
		b, err := ioutil.ReadAll(r)
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

	raw := map[string]interface{}{}
	err = json.Unmarshal(data, &raw)
	if err != nil {
		return nil, err
	}
	schema.raw = raw

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

	for _, one := range schema.OneOf {
		setRoot(root, one)
	}

	if schema.Ref != "" {
		schemaPath := strings.Split(schema.Ref, "/")
		var ctx interface{}
		ctx = schema
		for _, part := range schemaPath {
			switch part {
			case "#":
				ctx = root
			case "definitions":
				ctx = ctx.(*Schema).Definitions
			case "properties":
				ctx = ctx.(*Schema).Properties
			case "patternProperties":
				ctx = ctx.(*Schema).PatternProperties
			case "items":
				ctx = ctx.(*Schema).Items
			default:
				if cast, ok := ctx.(map[string]*Schema); ok {
					if def, ok := cast[part]; ok {
						ctx = def
						continue
					}
				}
				// no match in the structure, so loop through the raw document
				// in case they are using out-of-spec paths ie #/$special/thing
				var cursor interface{} = root.raw
				for _, part := range schemaPath {
					if part == "#" {
						continue
					}
					cast, ok := cursor.(map[string]interface{})
					if !ok {
						panic(fmt.Sprintf("Expected map[string]interface{}, got: %T at path %q in $ref %q", cursor, part, schema.Ref))
					}
					value, ok := cast[part]
					if !ok {
						panic(fmt.Sprintf("path %q for $ref %q not found in document %#v", part, schema.Ref, cast))
					}
					cursor = value
				}

				// turn it back into json
				document, err := json.Marshal(cursor)
				if err != nil {
					panic(err)
				}
				// now try to parse the new extracted sub document as a schema
				refSchema := &Schema{}
				err = json.Unmarshal(document, refSchema)
				if err != nil {
					panic(err)
				}
				setRoot(root, refSchema)

				if refSchema.Name() == "" {
					// just guess on the name from the json document path
					refSchema.Description = part
				}
				ctx = refSchema
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
	caseName = strings.ReplaceAll(caseName, " ", "")

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

func (s *SchemaProcessor) structComment(schema *Schema, typeName string) string {
	if !s.Comment {
		return ""
	}
	prettySchema, _ := json.MarshalIndent(schema, "// ", "  ")
	return fmt.Sprintf("// %s defined from schema:\n// %s\n", typeName, prettySchema)
}

func (s *SchemaProcessor) processSchema(schema *Schema) (typeName string, err error) {
	switch schema.Type {
	case OBJECT:
		typeName = camelCase(schema.Name())
		switch {
		case schema.Properties != nil:
			typeData := fmt.Sprintf("%stype %s struct {\n", s.structComment(schema, typeName), typeName)
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
		case schema.PatternProperties != nil:
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
					typeData := fmt.Sprintf("%stype %s map[string]%s\n\n", s.structComment(schema, typeName), typeName, subTypeName)
					if err := s.writeGoCode(typeName, typeData); err != nil {
						return "", err
					}
				} else {
					typeName = fmt.Sprintf("map[string]%s", subTypeName)
				}
			}
		case schema.AdditionalProperties:
			// TODO we can probably do better, but this is a catch-all for now
			typeName = "map[string]interface{}"
		}
	case ARRAY:
		subTypeName, err := s.processSchema(schema.Items)
		if err != nil {
			return "", err
		}

		typeName = camelCase(schema.Name())
		if typeName == "" {
			if strings.Title(subTypeName) == subTypeName {
				if strings.HasSuffix(subTypeName, "s") {
					typeName = fmt.Sprintf("%ses", subTypeName)
				} else {
					typeName = fmt.Sprintf("%ss", subTypeName)
				}
			}
		}
		if typeName != "" {
			typeName = strings.TrimPrefix(typeName, "*")
			typeData := fmt.Sprintf("%stype %s []%s\n\n", s.structComment(schema, typeName), typeName, subTypeName)
			if err := s.writeGoCode(typeName, typeData); err != nil {
				return "", err
			}
		} else {
			typeName = fmt.Sprintf("[]%s", subTypeName)
		}
	case ANY:
		switch {
		case len(schema.OneOf) > 0:
			return s.mergeSchemas(schema, schema.OneOf...)
		case schema.Const != "":
			// Const is a special case of Enum
			return "string", nil
		case len(schema.Enum) > 0:
			// TODO this is bogus, but assuming Enums are string types for now
			return "string", nil
		}
		typeName = "interface{}"
	case BOOLEAN:
		typeName = "bool"
	case INTEGER:
		typeName = "int"
	case NUMBER:
		typeName = "float64"
	case NULL:
		typeName = "interface{}"
	case STRING:
		typeName = "string"
	}
	return
}

func (s *SchemaProcessor) mergeSchemas(parent *Schema, schemas ...*Schema) (typeName string, err error) {
	switch len(schemas) {
	case 0:
		return "", fmt.Errorf("Merging zero schemas")
	case 1:
		// TODO: Not sure this is correct, should the name come from the oneOf
		// schema or the only constraint schema?
		return s.processSchema(schemas[0])
	}

	mergedParent := &Schema{
		Description: parent.Name(),
		Root:        parent.Root,
		Properties:  map[string]*Schema{},
		Type:        OBJECT,
	}

	uncommonSchemas := map[string]*Schema{}
	for _, schema := range schemas {
		// TODO we need a Schema.Copy() function
		uncommonSchemas[schema.Name()] = &Schema{
			Description: schema.Name(),
			Root:        parent.Root,
			Properties:  map[string]*Schema{},
			Type:        schema.Type,
		}
	}

	// find any common properties, and assign them to mergeParent
	// else create subtype with uncommon properties with `json:",inline"`

	allProperties := map[string]int{}
	for _, schema := range schemas {
		for p := range schema.Properties {
			allProperties[p]++
		}
	}

	for _, schema := range schemas {
		for p, v := range schema.Properties {
			if allProperties[p] > 1 {
				mergedParent.Properties[p] = v
			} else {
				uncommonSchemas[schema.Name()].Properties[p] = v
			}
		}
	}

	typeName = camelCase(mergedParent.Name())
	typeData := fmt.Sprintf("%stype %s struct {\n", s.structComment(mergedParent, typeName), typeName)

	keys := []string{}
	for k := range mergedParent.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := mergedParent.Properties[k]
		subTypeName, err := s.processSchema(v)
		if err != nil {
			return "", err
		}
		typeData += fmt.Sprintf("    %s %s `json:\"%s,omitempty\" yaml:\"%s,omitempty\"`\n", camelCase(k), subTypeName, k, k)
	}

	oneOfKeys := []string{}
	for name, schema := range uncommonSchemas {
		if len(schema.Properties) > 0 {
			oneOfKeys = append(oneOfKeys, name)
		}
	}
	sort.Strings(oneOfKeys)

	for _, k := range oneOfKeys {
		oneOfTypeName, err := s.processSchema(uncommonSchemas[k])
		if err != nil {
			return "", err
		}
		typeData += fmt.Sprintf("    %s %s `json:\",inline\" yaml:\",inline\"`\n", camelCase(k), oneOfTypeName)
	}

	typeData += "}\n\n"
	if err := s.writeGoCode(typeName, typeData); err != nil {
		return "", err
	}
	return typeName, nil
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
			cmd.Stdout = s.IO.Stdout
			cmd.Stderr = s.IO.Stderr
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
	defer fh.Close()
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
		cmd.Stdin = s.IO.Stdin
		cmd.Stdout = s.IO.Stdout
		cmd.Stderr = s.IO.Stderr
		return cmd.Run()
	}
	return nil
}
