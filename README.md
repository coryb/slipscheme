[![](https://images.microbadger.com/badges/image/coryb/slipscheme.svg)](http://microbadger.com/images/coryb/slipscheme)
[![](https://images.microbadger.com/badges/version/coryb/slipscheme.svg)](http://microbadger.com/images/coryb/slipscheme)
[![Build Status](https://github.com/coryb/slipscheme/workflows/Build/badge.svg)](https://github.com/coryb/slipscheme/actions)

## slipscheme

Simple tool to convert JSON schemas to Go types

## Download

Download the binaries from the [latest release](https://github.com/coryb/slipscheme/releases/latest).

You can also run it from super-minimal docker image as well (only 6M).  It would be run like:
```bash
docker run -i --rm -v $(pwd):/work coryb/slipscheme:latest
```

## Usage

```bash
Usage of slipscheme:
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
```

By default `slipscheme` will print go code to a file, one per type.  

## Example

See the [sample.json](./sample.json) file checked into git and this is what `slipscheme` will generate:

```bash
$ ./slipscheme -stdout sample.json
type Address struct {
        City    string `json:"city,omitempty" yaml:"city,omitempty"`
        State   string `json:"state,omitempty" yaml:"state,omitempty"`
        Street  string `json:"street,omitempty" yaml:"street,omitempty"`
        Zipcode int    `json:"zipcode,omitempty" yaml:"zipcode,omitempty"`
}

type Phone struct {
        AreaCode    string `json:"area-code,omitempty" yaml:"area-code,omitempty"`
        CountryCode string `json:"country-code,omitempty" yaml:"country-code,omitempty"`
        Number      string `json:"number,omitempty" yaml:"number,omitempty"`
}

type Phones []*Phone

type Contact struct {
        EmailAddress []string `json:"email-address,omitempty" yaml:"email-address,omitempty"`
        HomeAddress  *Address `json:"home-address,omitempty" yaml:"home-address,omitempty"`
        Name         string   `json:"name,omitempty" yaml:"name,omitempty"`
        Phone        Phones   `json:"phone,omitempty" yaml:"phone,omitempty"`
        WorkAddress  *Address `json:"work-address,omitempty" yaml:"work-address,omitempty"`
}
```

Otherwise, you can run `slipscheme` to generate the type files:
```bash
$ ./slipscheme -dir out sample.json

$ ls -1 out
Address.go
Contact.go
Phone.go
Phones.go

$ cat out/Phone.go
package test

/////////////////////////////////////////////////////////////////////////
// This Code is Generated by SlipScheme Project:
// https://github.com/coryb/slipscheme
//
// Generated with command: ./slipscheme -dir out -pkg test sample.json
/////////////////////////////////////////////////////////////////////////
//                            DO NOT EDIT                              //
/////////////////////////////////////////////////////////////////////////

type Phone struct {
        AreaCode    string `json:"area-code,omitempty" yaml:"area-code,omitempty"`
        CountryCode string `json:"country-code,omitempty" yaml:"country-code,omitempty"`
        Number      string `json:"number,omitempty" yaml:"number,omitempty"`
}
```