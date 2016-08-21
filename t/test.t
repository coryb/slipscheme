#!/bin/bash
#eval "$(curl -q -s https://raw.githubusercontent.com/coryb/osht/master/osht.sh)"
. $HOME/oss/osht/osht.sh
cd $(dirname $0)
slipscheme="../slipscheme"

PLAN 17

NRUNS $slipscheme
DIFF <<EOF
Usage: ../slipscheme <schema file> [<schema file> ...]
  -comments
    	enable/disable print comments (default true)
  -dir string
    	output directory for go files (default ".")
  -fmt
    	pass code through gofmt (default true)
  -overwrite
    	force overwriting existing go files
  -pkg string
    	package namespace for go files (default "main")
  -stdout
    	print go code to stdout rather than files
EOF

outdir=$(mktemp -d)

RUNS $slipscheme -dir $outdir ../sample.json
OK -f $outdir/Contact.go
OK -f $outdir/Address.go
OK -f $outdir/Phone.go
OK -f $outdir/Phones.go

rm -rf $outdir

RUNS $slipscheme -stdout=true - <<EOF
{
  "title": "stuff",
  "type": "array",
  "items": {
    "type": "string"
   }
}
EOF
DIFF <<EOF
// Stuff defined from schema:
// {
//   "title": "stuff",
//   "type": "array",
//   "items": {
//     "type": "string"
//   }
// }
type Stuff []string

EOF

RUNS $slipscheme -stdout=true -comments=false - <<EOF
{
  "title": "stuff",
  "type": "array",
  "items": {
    "type": "string"
   }
}
EOF
DIFF <<EOF
type Stuff []string

EOF

RUNS $slipscheme -stdout=true -comments=false - <<EOF
{
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
EOF
DIFF <<'EOF'
type Thing struct {
	That string `json:"that,omitempty" yaml:"that,omitempty"`
	This int    `json:"this,omitempty" yaml:"this,omitempty"`
}

EOF

RUNS $slipscheme -stdout=true -comments=false -fmt=false - <<EOF
{
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
EOF
DIFF <<'EOF'
type Thing struct {
    That string `json:"that,omitempty" yaml:"that,omitempty"`
    This int `json:"this,omitempty" yaml:"this,omitempty"`
}

EOF

RUNS $slipscheme -stdout=true -comments=false - <<EOF
{
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
}
EOF
DIFF <<'EOF'
type Thing struct {
	That string `json:"that,omitempty" yaml:"that,omitempty"`
	This int    `json:"this,omitempty" yaml:"this,omitempty"`
}

type Stuff []*Thing

EOF
