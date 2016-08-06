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
	"strings"
)

// Schema represents JSON schema.
type Schema struct {
	Title             string             `json:"title"`
	ID                string             `json:"id"`
	Type              SchemaType         `json:"type"`
	Description       string             `json:"description"`
	Definitions       map[string]*Schema `json:"definitions"`
	Properties        map[string]*Schema `json:"properties"`
	PatternProperties map[string]*Schema `json:"patternProperties"`
	Items             *Schema            `json:"items"`
}

type SchemaType int

const (
	ANY     SchemaType = iota
	ARRAY   SchemaType = iota
	BOOLEAN SchemaType = iota
	INTEGER SchemaType = iota
	NUMBER  SchemaType = iota
	NULL    SchemaType = iota
	OBJECT  SchemaType = iota
	STRING  SchemaType = iota
)

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

func main() {
	outputDir := flag.String("dir", ".", "output directory for go files")
	pkgName   := flag.String("pkg", "main", "package namespace for go files")
	overwrite := flag.Bool("overwrite", false, "force overwriting existing go files")
	flag.Parse()

	processor := &SchemaProcessor{
		OutputDir: *outputDir,
		PackageName: *pkgName,
		Overwrite: *overwrite,
	}

	err := processor.Process(flag.Args())
	if err != nil {
		panic(err)
	}
}

type SchemaProcessor struct {
	OutputDir string
	PackageName string
	Overwrite bool
}

func (s *SchemaProcessor) Process(files []string) error {
	for _, file := range files {
		schema := &Schema{}

		fh, err := os.OpenFile(file, os.O_RDONLY, 0644)
		defer fh.Close()
		b, err := ioutil.ReadAll(fh)
		if err != nil {
			return err
		}
		err = json.Unmarshal(b, schema)
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

func camelCase(name string) string {
	caseName := strings.Title(
		strings.Map(func(r rune) rune {
			if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				return r
			}
			return -1
		}, name),
	)
	if strings.HasSuffix(caseName, "Id") {
		return strings.TrimSuffix(caseName, "Id") + "ID"
	} else if strings.HasSuffix(caseName, "Url") {
		return strings.TrimSuffix(caseName, "Url") + "URL"
	}
	return caseName
}

func (s *SchemaProcessor) processSchema(schema *Schema) (typeName string, err error) {
	if schema.Type == OBJECT {
		typeName = camelCase(schema.Title)
		if schema.Properties != nil {
			typeData := fmt.Sprintf("type %s struct {\n", typeName)
			for k, v := range schema.Properties {
				subTypeName, err := s.processSchema(v)
				if err != nil {
					return "", err
				}
				typeData += fmt.Sprintf("    %s %s `json:\"%s,omitempty\" yaml:\"%s,omitempty\"`\n", camelCase(k), subTypeName, k, k)
			}
			typeData += "}\n"
			if err := s.writeGoCode(typeName, typeData); err != nil {
				return "", err
			}
		} else if schema.PatternProperties != nil {
			for _, v := range schema.PatternProperties {
				subTypeName, err := s.processSchema(v)
				if err != nil {
					return "", err
				}

				// verify subTypeName is not a simple type
				if strings.ToTitle(subTypeName) == subTypeName {
					typeName = fmt.Sprintf("%sMap", subTypeName)
					typeData := fmt.Sprintf("type %s map[string]%s\n", typeName, subTypeName)
					if err := s.writeGoCode(typeName, typeData); err != nil {
						return "", err
					}
				}
			}
		}
	} else if schema.Type == ARRAY {
		subTypeName, err := s.processSchema(schema.Items)
		if err != nil {
			return "", err
		}
		if strings.ToTitle(subTypeName) == subTypeName {
			if strings.HasSuffix(subTypeName, "s") {
				typeName = fmt.Sprintf("%ses", subTypeName)
			} else {
				typeName = fmt.Sprintf("%ss", subTypeName)
			}
			typeData := fmt.Sprintf("type %s []%s\n", typeName, subTypeName)
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
///
// This Code is Generated by Schemy Project:
// https://github.com/coryb/schemy
// 
// Generated with command: %s
//
// DO NOT EDIT
//

`, strings.Join(os.Args, " "))
		
	if _, err := fh.Write([]byte(preamble)); err != nil {
		return err
	}
	if _, err := fh.Write([]byte(code)); err != nil {
		return err
	}

	cmd := exec.Command("gofmt", "-s", "-w", file);
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
