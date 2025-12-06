# tinycli

[![Go Reference](https://pkg.go.dev/badge/github.com/jonathonwebb/tinycli.svg)](https://pkg.go.dev/github.com/jonathonwebb/tinycli)
[![CI](https://github.com/jonathonwebb/tinycli/actions/workflows/ci.yaml/badge.svg)](https://github.com/jonathonwebb/tinycli/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/jonathonwebb/tinycli)](https://goreportcard.com/report/github.com/jonathonwebb/tinycli)
[![codecov](https://codecov.io/github/jonathonwebb/tinycli/graph/badge.svg?token=TD5YgMcIvw)](https://codecov.io/github/jonathonwebb/tinycli)

Package `tinycli` implements a simple command-line interface framework.

## Install

```
go get github.com/jonathonwebb/tinycli
```

## Usage

<!-- editorconfig-checker-disable -->
```go
type p struct {
	// ...
}

func main() {
	var params p
	cmd := cli.Command[*p]{ 
		// ...
	}
	env := cli.DefaultEnv(&params)
	os.Exit(int(cmd.Execute(context.Background(), env)))
}
```
<!-- editorconfig-checker-enable -->

A `tinycli` [Command](https://pkg.go.dev/github.com/jonathonwebb/tinycli#Command) parses command-line flags and environment variables, storing values in a custom parameter object that is accessible to command actions.

The execution environment of a `Command` is encapsulated in an [Env](https://pkg.go.dev/github.com/jonathonwebb/tinycli#Env). The default env looks like:

<!-- editorconfig-checker-disable -->
```go
e := Env[T]{
	Err:    os.Stderr,
	Out:    os.Stdout,
	Args:   ok.Args,
	Vars:   environ // map of var names -> values from os.Environ
	Params: params // parameter object of type T
}
```
<!-- editorconfig-checker-enable -->

The generic parameter type is usually a pointer to a struct, with fields that can be bound to command-line flags via a [flag.FlagSet](https://pkg.go.dev/flag#FlagSet). A `Command` may be configured with a `Flags` func that does the work of defining and binding command-line flags to a parameter instance:

<!-- editorconfig-checker-disable -->
```go
type p struct {
	env     string
	verbose bool
	port    uint
}

c := Command[*p]{
	Flags: func(fs *flag.FlagSet, p *p) {
		fs.StringVar(&p.env, "env", "production", "")
		fs.BoolVar(&p.verbose, "v", false, "")
		fs.UintVar(&p.port, "port", 5000, "")
	},
}
```
<!-- editorconfig-checker-enable -->

A `Command` may be configured with a `Vars` map binding flag names to environment variables:

<!-- editorconfig-checker-disable -->
```go
c := Command[*p]{
	Vars: map[string]string{
		"env":  "FOO_ENV",
		"v":    "FOO_VERBOSE",
		"port": "FOO_PORT",
	},
}
```
<!-- editorconfig-checker-enable -->

The precedence of flag sources is:

 1. User command-line flags
 2. Environment variables
 3. Flag default values

A `tinycli` command-line interface is tree, with each `Command` optionally defining a list of `Subcommands`:

<!-- editorconfig-checker-disable -->
```go
c := Command[*p]{
	Subcommands: []Command[*p]{
		&subcommandA,
		&subcommandB,
	},
}
```
<!-- editorconfig-checker-enable -->

After parsing, the `Action` func of the last visited `Command` is invoked, receiving the resulting `Env`:

<!-- editorconfig-checker-disable -->
```go
c := Command[*p]{
	Action: func(ctx context.Context, e *Env[*p]) ExitStatus {
		// e.Args contains remaining positional args
		// e.Params contains resulting params
	},
}

var params p
e := DefaultEnv[*p](&params)
status := c.Execute(context.Background(), e)
```
<!-- editorconfig-checker-enable -->

A non-goal of `tinycli` is automatically formatting `Command` usage and help text. Instead, usage and help text for a `Command` are manually configured:

<!-- editorconfig-checker-disable -->
```go
c := Command[*p]{
	Name: "foo",
	Usage: "usage: foo [flags] command",
	Help: `commands:
    bar
    baz

flags:
    -env    environment name
    -v      enable verbose output
    -port   uint port number`,
}
```
<!-- editorconfig-checker-enable -->

A `Command` may be have an `After` hook for validating and transforming
parameter values after parsing. When a pointer to a [ValueError](https://pkg.go.dev/github.com/jonathonwebb/tinycli#ValueError) is returned from the `After` hook, the error message will be formatted as if it originated from a command-line flag:

<!-- editorconfig-checker-disable -->
```go
c := Command[*p]{
	After: func(e *Env[*p]) error {
		if e.Params.port > 65535 {
			return &cli.ValueError{
				Name: "port",
				Err:  errors.New("cannot exceed 65535"),
			}
		}
		return nil
	},
}

// Results in error output like:
// invalid value "99999" for var $FOO_PORT: cannot exceed 65535
```
<!-- editorconfig-checker-enable -->

## API Documentation

The full API documentation can be found at [pkg.go.dev](https://pkg.go.dev/github.com/jonathonwebb/tinycli). Once major version `1.x.x` is released, the API for each major version will be stable -- any breaking changes to the API will require a new major version.
