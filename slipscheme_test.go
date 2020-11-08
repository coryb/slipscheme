package main

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ExampleUsage() {
	runMain([]string{"slipscheme"}, Stdio{
		Stdout: os.Stdout,
	})
	// Output:
	// Usage: slipscheme <schema file> [<schema file> ...]
	//   -comments
	//     	enable/disable print comments (default true)
	//   -dir string
	//     	output directory for go files (default ".")
	//   -fmt
	//     	pass code through gofmt (default true)
	//   -overwrite
	//     	force overwriting existing go files
	//   -pkg string
	//     	package namespace for go files (default "main")
	//   -stdout
	//     	print go code to stdout rather than files
}

func TestOutputFiles(t *testing.T) {
	tdir, err := ioutil.TempDir("", "slipscheme")
	noError(t, err)
	runMain([]string{"slipscheme", "-dir", tdir, "sample.json"}, Stdio{
		Stdout: os.Stdout,
	})
	exists(t, filepath.Join(tdir, "Contact.go"))
	exists(t, filepath.Join(tdir, "Address.go"))
	exists(t, filepath.Join(tdir, "Phone.go"))
	exists(t, filepath.Join(tdir, "Phones.go"))
}

func Example_stdoutSlice() {
	input := `{
	"title": "stuff",
	"type": "array",
	"items": {
		"type": "string"
	}
}`
	runMain([]string{"slipscheme", "-stdout=true", "-"}, Stdio{
		Stdin:  strings.NewReader(input),
		Stdout: os.Stdout,
	})

	// Output:
	// // Stuff defined from schema:
	// // {
	// //   "title": "stuff",
	// //   "type": "array",
	// //   "items": {
	// //     "type": "string"
	// //   }
	// // }
	// type Stuff []string
}

func Example_noComments() {
	input := `{
	"title": "stuff",
	"type": "array",
	"items": {
		"type": "string"
	}
}`
	runMain([]string{"slipscheme", "-stdout=true", "-comments=false", "-"}, Stdio{
		Stdin:  strings.NewReader(input),
		Stdout: os.Stdout,
	})

	// Output:
	// type Stuff []string
}

func Example_struct() {
	input := `{
	"title": "thing",
	"type": "object",
	"properties": {
		"this": {
			"type": "integer"
		},
		"that": {
			"type": "string"
		}
	}
}`
	runMain([]string{"slipscheme", "-stdout=true", "-comments=false", "-"}, Stdio{
		Stdin:  strings.NewReader(input),
		Stdout: os.Stdout,
	})

	// Output:
	// type Thing struct {
	// 	That string `json:"that,omitempty" yaml:"that,omitempty"`
	// 	This int    `json:"this,omitempty" yaml:"this,omitempty"`
	// }
}

func Example_noFmt() {
	input := `{
	"title": "thing",
	"type": "object",
	"properties": {
		"this": {
			"type": "integer"
		},
		"that": {
			"type": "string"
		}
	}
}`
	runMain([]string{"slipscheme", "-stdout=true", "-comments=false", "-fmt=false", "-"}, Stdio{
		Stdin:  strings.NewReader(input),
		Stdout: os.Stdout,
	})

	// Output:
	// type Thing struct {
	//     That string `json:"that,omitempty" yaml:"that,omitempty"`
	//     This int `json:"this,omitempty" yaml:"this,omitempty"`
	// }
}

func Example_structSlice() {
	input := `{
	"title": "stuff",
	"type": "array",
	"items": {
		"title": "thing",
		"type": "object",
		"properties": {
			"this": {
				"type": "integer"
			},
			"that": {
				"type": "string"
			}
		}
	}
}`
	runMain([]string{"slipscheme", "-stdout=true", "-comments=false", "-"}, Stdio{
		Stdin:  strings.NewReader(input),
		Stdout: os.Stdout,
	})

	// Output:
	// type Thing struct {
	// 	That string `json:"that,omitempty" yaml:"that,omitempty"`
	// 	This int    `json:"this,omitempty" yaml:"this,omitempty"`
	// }

	// type Stuff []*Thing
}

func noError(t *testing.T, err error) {
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
}

func exists(t *testing.T, file string) {
	_, err := os.Stat(file)
	if err != nil {
		t.Fatalf("Failed to test for file existence on %s: %s", file, err)
	}
}
