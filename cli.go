/*
Package tinycli implements a simple command-line interface framework.

# Usage

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

A tinycli [Command] parses command-line flags and environment variables, storing
values in a custom parameter object that is accessible to command actions.

The execution environment of a Command is encapsulated in an [Env]. The default
env looks like:

	e := Env[T]{
	  Err:    os.Stderr,
	  Out:    os.Stdout,
	  Args:   ok.Args,
	  Vars:   environ // map of var names -> values from os.Environ
	  Params: params // parameter object of type T
	}

The generic parameter type is usually a pointer to a struct, with fields that
can be bound to command-line flags via a [flag.FlagSet]. A Command may be
configured with a Flags func that does the work of defining and binding
command-line flags to a parameter instance:

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

A Command may be configured with a Vars map binding flag names to environment
variables:

	c := Command[*p]{
		Vars: map[string]string{
			"env":  "FOO_ENV",
			"v":    "FOO_VERBOSE",
			"port": "FOO_PORT",
		},
	}

The precedence of flag sources is:

 1. User command-line flags
 2. Environment variables
 3. Flag default values

A tinycli command-line interface is tree, with each Command optionally defining
a list of Subcommands:

	c := Command[*p]{
		Subcommands: []Command[*p]{
			&subcommandA,
			&subcommandB,
		},
	}

After parsing, the Action func of the last visited Command is invoked, receiving
the execution Env with the resulting parameter object and remaining positional
arguments:

	c := Command[*p]{
		Action: func(ctx context.Context, e *Env[*p]) ExitStatus {
			// e.Args contains remaining positional args
			// e.Params contains resulting params
		},
	}

	var params p
	e := DefaultEnv[*p](&params)
	status := c.Execute(context.Background(), e)

A non-goal of tinycli is automatically formatting Command usage and help text.
Instead, usage and help text for a Command are manually configured:

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

A Command may be have an After hook for validating and transforming
parameter values after parsing. When a pointer to a [ValueError] is returned
from the After hook, the error message will be formatted as if it originated
from a command-line flag:

	c := Command[*p]{
		After: func(params *p) error {
			if p.port > 65535 {
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
*/
package tinycli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
)

type valueSource int

const (
	sourceDefault valueSource = iota
	sourceFlag
	sourceVar
)

// An Env represents the execution environment for a [Command].
//
// P is the type of custom parameter data available to Commands executed with
// the Env.
type Env[P any] struct {
	Err    io.Writer         // standard output stream
	Out    io.Writer         // error output stream
	Args   []string          // command-line arguments
	Vars   map[string]string // env var names -> values
	Params P                 // custom data available to Command actions
}

// DefaultEnv returns an [Env] using the process environment.
//
// The resulting Env will use the [os.Stderr] and [os.Stdout] streams,
// [os.Args], and environment variables from [os.Environ].
func DefaultEnv[P any](params P) *Env[P] {
	environ := os.Environ()
	vars := make(map[string]string, len(environ))
	for _, v := range environ {
		key, value, _ := strings.Cut(v, "=")
		vars[key] = value
	}
	return &Env[P]{
		Err:    os.Stderr,
		Out:    os.Stdout,
		Args:   os.Args,
		Vars:   vars,
		Params: params,
	}
}

// Printf formats and writes a message to the Env standard output stream.
func (e Env[P]) Printf(format string, args ...any) (int, error) {
	if e.Out != nil {
		return fmt.Fprintf(e.Out, format, args...)
	}
	return 0, nil
}

// Errorf formats and writes an error message to the Env error output stream.
func (e Env[P]) Errorf(format string, args ...any) (int, error) {
	if e.Err != nil {
		return fmt.Fprintf(e.Err, format, args...)
	}
	return 0, nil
}

func (e Env[P]) getVar(name string) (value string, isSet bool) {
	if e.Vars == nil {
		return "", false
	}
	value, isSet = e.Vars[name]
	return value, isSet
}

// An ExitStatus is the result of command execution.
type ExitStatus int

const (
	ExitSuccess ExitStatus = 0 // execution succeeded
	ExitFailure ExitStatus = 1 // execution failed due to an error
	ExitUsage   ExitStatus = 2 // execution failed due to invalid user input
)

var (
	errMissingCommand = errors.New("missing command")
	errUnknownCommand = errors.New("unknown command")
)

// A FlagsFunc is a hook for defining flags and binding them to parameter values.
type FlagsFunc[P any] = func(*flag.FlagSet, P)

// An AfterFunc is a hook for validating or transforming parameter values.
type AfterFunc[P any] = func(P) error

// An ActionFunc is a function called when a Command is invoked.
type ActionFunc[P any] = func(context.Context, *Env[P]) ExitStatus

// A Command represents a CLI command.
//
// P is the type of custom parameter data available to Command actions.
type Command[P any] struct {
	Name        string            // name used to invoke the command
	Usage       string            // short usage text
	Help        string            // log help text
	Flags       FlagsFunc[P]      // flag setup hook
	Vars        map[string]string // flag names -> env var names
	After       AfterFunc[P]      // post-parse hook
	Action      ActionFunc[P]     // command action function
	Subcommands []*Command[P]     // child commands

	fs   *flag.FlagSet
	meta map[string]*flagMeta
}

// A Value error is an error associated with a Command flag.
type ValueError struct {
	Name string // flag name
	Err  error  // wrapped error
}

func (e *ValueError) Error() string {
	return e.Err.Error()
}

type decoratedValueError struct {
	rawValue string
	flagName string
	varName  string
	source   valueSource
	isBool   bool
	err      error
}

func (e *decoratedValueError) Error() string {
	var (
		valuePrefix  string
		sourcePrefix string
		sourceName   string = "UNKNOWN"
	)

	if e.isBool {
		valuePrefix = "boolean "
	}

	switch e.source {
	case sourceFlag:
		if !e.isBool {
			sourcePrefix = "flag "
		}
		sourceName = e.flagName
	case sourceVar:
		if !e.isBool {
			sourcePrefix = "var "
		}
		sourceName = "$" + e.varName
	}

	return fmt.Sprintf("invalid %svalue %q for %s%s: %v", valuePrefix, e.rawValue, sourcePrefix, sourceName, e.err)
}

func (c *Command[P]) decorateValueError(ve *ValueError) error {
	meta, ok := c.getMeta(ve.Name)
	if !ok {
		return ve
	}
	return &decoratedValueError{
		rawValue: meta.value,
		flagName: meta.flagName,
		source:   meta.valueSource,
		varName:  meta.varName,
		isBool:   meta.isBool,
		err:      ve.Err,
	}
}

func (c *Command[P]) onHelp(e *Env[P]) {
	e.Printf("%s\n\n%s\n", c.Usage, c.Help)
}

func (c *Command[P]) onErr(e *Env[P], err error) {
	e.Errorf("%s\n%v\n", c.Usage, err)
}

func (c *Command[P]) flagSet() *flag.FlagSet {
	if c.fs == nil {
		c.fs = flag.NewFlagSet(c.Name, flag.ContinueOnError)
		c.fs.Usage = func() { /* no-op */ }
		c.fs.SetOutput(io.Discard)
	}
	return c.fs
}

func (c *Command[P]) lookupVarName(flagName string) (varName string, exists bool) {
	if c.Vars == nil {
		return "", false
	}
	varName, exists = c.Vars[flagName]
	return varName, exists
}

func (c *Command[P]) getVar(flagName string, env *Env[P]) (varName string, value string, isSet bool) {
	varName, exists := c.lookupVarName(flagName)
	if !exists {
		return "", "", false
	}
	value, isSet = env.getVar(varName)
	return varName, value, isSet
}

func (c *Command[P]) getMeta(flagName string) (*flagMeta, bool) {
	// c.meta must not be nil
	meta, exists := c.meta[flagName]
	if !exists {
		return nil, false
	}
	return meta, true
}

func (c *Command[P]) lookupSubcommand(name string) *Command[P] {
	if c.Subcommands == nil {
		return nil
	}
	for i := range c.Subcommands {
		if c.Subcommands[i].Name == name {
			return c.Subcommands[i]
		}
	}
	return nil
}

type flagMeta struct {
	flagName    string
	varName     string
	value       string
	valueSource valueSource
	isBool      bool
}

type boolFlag interface {
	flag.Value
	IsBoolFlag() bool
}

// Execute parses command-line arguments and vars from the environment, calls
// hook functions, then calls the command's action or defers to the specified
// subcommand's own Execute method.
func (c *Command[P]) Execute(ctx context.Context, e *Env[P]) ExitStatus {
	if c.Flags != nil {
		c.Flags(c.flagSet(), e.Params)
	}

	if len(e.Args) < 1 {
		c.onErr(e, errors.New("no arguments provided"))
		return ExitFailure
	}

	if err := c.flagSet().Parse(e.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			c.onHelp(e)
			return ExitSuccess
		}
		c.onErr(e, err)
		return ExitUsage
	}

	c.meta = make(map[string]*flagMeta, c.flagSet().NFlag())
	c.flagSet().VisitAll(func(f *flag.Flag) {
		_, isBool := f.Value.(boolFlag)
		c.meta[f.Name] = &flagMeta{
			flagName:    f.Name,
			value:       f.Value.String(),
			valueSource: sourceDefault,
			isBool:      isBool,
		}
	})

	c.flagSet().Visit(func(f *flag.Flag) {
		m := c.meta[f.Name]
		m.valueSource = sourceFlag
	})

	keys := make([]string, 0, len(c.meta))
	for k := range c.meta {
		keys = append(keys, k)
	}
	slices.Sort(keys)

	for _, k := range keys {
		m := c.meta[k]
		if m.valueSource != sourceDefault {
			continue
		}
		varName, envValue, isSet := c.getVar(m.flagName, e)
		if isSet {
			if setErr := c.flagSet().Set(m.flagName, envValue); setErr != nil {
				valErr := decoratedValueError{
					rawValue: envValue,
					source:   sourceVar,
					varName:  varName,
					isBool:   m.isBool,
					err:      setErr,
				}

				c.onErr(e, &valErr)
				return ExitUsage
			}
			m.varName = varName
			m.value = envValue
			m.valueSource = sourceVar
		}
	}

	if c.After != nil {
		if err := c.After(e.Params); err != nil {
			if valErr, isValErr := err.(*ValueError); isValErr {
				err = c.decorateValueError(valErr)
			}
			c.onErr(e, err)
			return ExitUsage
		}
	}

	e.Args = c.flagSet().Args()

	if len(e.Args) > 0 {
		subCmd := c.lookupSubcommand(e.Args[0])
		if subCmd != nil {
			return subCmd.Execute(ctx, e)
		}
	}

	if c.Action != nil {
		return c.Action(ctx, e)
	}

	if len(e.Args) == 0 {
		c.onErr(e, errMissingCommand)
		return ExitFailure
	}

	c.onErr(e, errUnknownCommand)
	return ExitFailure
}
