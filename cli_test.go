package tinycli_test

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	cli "github.com/jonathonwebb/tinycli"
)

func TestEnv_Printf(t *testing.T) {
	t.Run("with_writer", func(t *testing.T) {
		var buf bytes.Buffer
		env := cli.Env[any]{Out: &buf}
		env.Printf("hello %s", "world")

		want := "hello world"
		if got := buf.String(); want != got {
			t.Errorf("env.Printf(%q, %q) wrote %q, want %q", "hello %s", "world", got, want)
		}
	})

	t.Run("nil_writer", func(t *testing.T) {
		env := cli.Env[any]{Out: nil}
		env.Printf("hello %s", "world") // don't panic!
	})
}

func TestEnv_Errorf(t *testing.T) {
	t.Run("with_writer", func(t *testing.T) {
		var buf bytes.Buffer
		env := cli.Env[any]{Err: &buf}
		env.Errorf("hello %s", "world")

		want := "hello world"
		if got := buf.String(); want != got {
			t.Errorf("env.Errorf(%q, %q) wrote %q, want %q", "hello %s", "world", got, want)
		}
	})

	t.Run("nil_writer", func(t *testing.T) {
		env := cli.Env[any]{Err: nil}
		env.Errorf("hello %s", "world") // don't panic!
	})
}

func TestDefaultEnv(t *testing.T) {
	const testEnvVar = "TEST_ENV_VAR"
	const testEnvValue = "test_value"

	t.Setenv(testEnvVar, testEnvValue)

	type testData struct {
		version string
	}
	data := testData{version: "x.x.x"}

	env := cli.DefaultEnv(data)

	if want, got := os.Stderr, env.Err; want != got {
		t.Errorf("DefaultEnv(%+v).Err = %v, want %v", data, got, want)
	}
	if want, got := os.Stdout, env.Out; want != got {
		t.Errorf("DefaultEnv(%+v).Out = %v, want %v", data, got, want)
	}
	if env.Args == nil {
		t.Errorf("DefaultEnv(%+v).Args = %v, want non-nil", data, env.Args)
	}

	value, exists := env.Vars[testEnvVar]
	if !exists {
		t.Errorf("DefaultEnv(%+v).Vars[%q] does not exist", data, testEnvVar)
	}
	if want, got := testEnvValue, value; want != got {
		t.Errorf("DefaultEnv(%+v).Vars[%q] = %v, want %v", data, testEnvVar, got, want)
	}

	if want, got := data, env.Params; want != got {
		t.Errorf("DefaultEnv(%+v).Data = %v, want %v", data, got, want)
	}
}

type tc[T any] struct {
	name string
	args []string
	vars map[string]string

	wantParams T
	wantArgs   []string
	wantOutbuf string
	wantErrbuf string
	wantStatus cli.ExitStatus
}

func execTestCommand[T any](t *testing.T, cmd *cli.Command[T], params T, tt tc[T]) (gotParams T, gotArgs []string, gotOutbuf string, gotErrbuf string, gotStatus cli.ExitStatus) {
	t.Helper()

	var (
		outbuf, errbuf bytes.Buffer
	)
	e := cli.Env[T]{
		Out:    &outbuf,
		Err:    &errbuf,
		Args:   tt.args,
		Vars:   tt.vars,
		Params: params,
	}
	status := cmd.Execute(t.Context(), &e)
	return e.Params, e.Args, outbuf.String(), errbuf.String(), status
}

var (
	errCustomTest = errors.New("custom test error")
)

func TestCommand_Execute(t *testing.T) {
	type p struct {
		RootStr      string
		RootInt      int
		RootBool     bool
		RootFlagOnly string

		SubStr      string
		SubInt      int
		SubBool     bool
		SubFlagOnly string

		NilVarMapStr string
		NilFlagsStr  string
		NilAfterStr  string
	}

	cmdFactory := func() *cli.Command[*p] {
		c := &cli.Command[*p]{
			Name:  "root",
			Usage: "root usage",
			Help:  "root help",
			Flags: func(fs *flag.FlagSet, p *p) {
				fs.StringVar(&p.RootStr, "rootStr", "", "")
				fs.IntVar(&p.RootInt, "rootInt", 0, "")
				fs.BoolVar(&p.RootBool, "rootBool", false, "")
				fs.StringVar(&p.RootFlagOnly, "rootFlagOnly", "", "")
			},
			Vars: map[string]string{
				"rootStr":  "ROOT_STR",
				"rootInt":  "ROOT_INT",
				"rootBool": "ROOT_BOOL",
			},
			After: func(p *p) error {
				if p.RootStr == "value_err" {
					return &cli.ValueError{
						Name: "rootStr",
						Err:  errCustomTest,
					}
				}
				if p.RootStr == "generic_err" {
					return errCustomTest
				}
				if p.RootStr == "unknown_flag_err" {
					return &cli.ValueError{
						Name: "rootUnknown",
						Err:  errCustomTest,
					}
				}
				return nil
			},
			Subcommands: []*cli.Command[*p]{
				{
					Name:  "sub",
					Usage: "sub usage",
					Help:  "sub help",
					Flags: func(fs *flag.FlagSet, p *p) {
						fs.StringVar(&p.SubStr, "subStr", "", "")
						fs.IntVar(&p.SubInt, "subInt", 0, "")
						fs.BoolVar(&p.SubBool, "subBool", false, "")
						fs.StringVar(&p.SubFlagOnly, "subFlagOnly", "", "")
					},
					Vars: map[string]string{
						"subStr":  "SUB_STR",
						"subInt":  "SUB_INT",
						"subBool": "SUB_BOOL"},
					Action: func(ctx context.Context, e *cli.Env[*p]) cli.ExitStatus {
						e.Printf("sub out\n")
						return cli.ExitSuccess
					},
					After: func(p *p) error {
						if p.SubStr == "value_err" {
							return &cli.ValueError{
								Name: "subStr",
								Err:  errCustomTest,
							}
						}
						if p.SubStr == "generic_err" {
							return errCustomTest
						}
						if p.SubStr == "unknown_flag_err" {
							return &cli.ValueError{
								Name: "subUnknown",
								Err:  errCustomTest,
							}
						}
						return nil
					},
				},
				{
					Name:  "nil_varmap",
					Usage: "nil_varmap usage",
					Help:  "nil_varmap help",
					Flags: func(fs *flag.FlagSet, p *p) {
						fs.StringVar(&p.NilVarMapStr, "nilVarMapStr", "", "")
					},
					Vars: nil,
					Action: func(ctx context.Context, e *cli.Env[*p]) cli.ExitStatus {
						e.Printf("nil_varmap out\n")
						return cli.ExitSuccess
					},
					After: func(p *p) error {
						return nil
					},
				},
				{
					Name:  "nil_flags",
					Usage: "nil_flags usage",
					Help:  "nil_flags help",
					Flags: nil,
					Vars: map[string]string{
						"nilFlagsStr": "NIL_FLAGS_STR",
					},
					Action: func(ctx context.Context, e *cli.Env[*p]) cli.ExitStatus {
						e.Printf("nil_flags out\n")
						return cli.ExitSuccess
					},
					After: func(p *p) error {
						return nil
					},
				},
				{
					Name:  "nil_after",
					Usage: "nil_after usage",
					Help:  "nil_after help",
					Flags: nil,
					Vars: map[string]string{
						"nilAfterStr": "NIL_AFTER_STR",
					},
					Action: func(ctx context.Context, e *cli.Env[*p]) cli.ExitStatus {
						e.Printf("nil_after out\n")
						return cli.ExitSuccess
					},
					After: nil,
				},
			},
		}

		return c
	}

	tests := []tc[*p]{
		{
			name: "nil_args",
			args: nil,
			vars: map[string]string{},

			wantErrbuf: "root usage\nno arguments provided\n",
			wantStatus: cli.ExitFailure,
		},
		{
			name: "no_args",
			args: []string{},
			vars: map[string]string{},

			wantErrbuf: "root usage\nno arguments provided\n",
			wantStatus: cli.ExitFailure,
		},
		{
			name: "nil_vars",
			args: []string{"root", "sub"},
			vars: nil,

			wantOutbuf: "sub out\n",
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "h_arg",
			args: []string{"root", "-h"},
			vars: map[string]string{},

			wantOutbuf: "root usage\n\nroot help\n",
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "help_arg",
			args: []string{"root", "-help"},
			vars: map[string]string{},

			wantOutbuf: "root usage\n\nroot help\n",
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "h_arg_sub",
			args: []string{"root", "sub", "-h"},
			vars: map[string]string{},

			wantOutbuf: "sub usage\n\nsub help\n",
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "help_arg_sub",
			args: []string{"root", "sub", "-help"},
			vars: map[string]string{},

			wantOutbuf: "sub usage\n\nsub help\n",
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "valid_flag",
			args: []string{"root", "-rootStr=testVal", "sub"},
			vars: map[string]string{},

			wantOutbuf: "sub out\n",
			wantParams: &p{RootStr: "testVal"},
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "invalid_flag",
			args: []string{"root", "-rootInt=invalid", "sub"},
			vars: map[string]string{},

			wantErrbuf: "root usage\ninvalid value \"invalid\" for flag -rootInt: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "valid_env_var",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"ROOT_STR":  "valid",
				"ROOT_INT":  "1",
				"ROOT_BOOL": "true",
			},

			wantOutbuf: "sub out\n",
			wantParams: &p{RootStr: "valid", RootInt: 1, RootBool: true},
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "invalid_env_var",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"ROOT_INT": "invalid",
			},

			wantErrbuf: "root usage\ninvalid value \"invalid\" for var $ROOT_INT: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "invalid_env_var_bool",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"ROOT_BOOL": "invalid",
			},

			wantErrbuf: "root usage\ninvalid boolean value \"invalid\" for $ROOT_BOOL: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "valid_flag_sub",
			args: []string{"root", "sub", "-subStr=testVal"},
			vars: map[string]string{},

			wantOutbuf: "sub out\n",
			wantParams: &p{SubStr: "testVal"},
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "invalid_flag_sub",
			args: []string{"root", "sub", "-subInt=invalid"},
			vars: map[string]string{},

			wantErrbuf: "sub usage\ninvalid value \"invalid\" for flag -subInt: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "valid_env_var_sub",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"SUB_STR":  "valid",
				"SUB_INT":  "1",
				"SUB_BOOL": "true",
			},

			wantOutbuf: "sub out\n",
			wantParams: &p{SubStr: "valid", SubInt: 1, SubBool: true},
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "invalid_env_var_sub",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"SUB_INT": "invalid",
			},

			wantErrbuf: "sub usage\ninvalid value \"invalid\" for var $SUB_INT: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "invalid_env_var_bool_sub",
			args: []string{"root", "sub"},
			vars: map[string]string{
				"SUB_BOOL": "invalid",
			},

			wantErrbuf: "sub usage\ninvalid boolean value \"invalid\" for $SUB_BOOL: parse error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "positional_args",
			args: []string{"root", "sub", "pos1", "pos2"},
			vars: map[string]string{},

			wantOutbuf: "sub out\n",
			wantArgs:   []string{"pos1", "pos2"},
			wantStatus: cli.ExitSuccess,
		},
		{
			name: "missing_cmd",
			args: []string{"root"},

			wantErrbuf: "root usage\nmissing command\n",
			wantStatus: cli.ExitFailure,
		},
		{
			name: "unknown_cmd",
			args: []string{"root", "foo"},
			vars: map[string]string{},

			wantErrbuf: "root usage\nunknown command\n",
			wantStatus: cli.ExitFailure,
		},
		{
			name: "after_value_err",
			args: []string{"root", "-rootStr=value_err", "sub"},
			vars: map[string]string{},

			wantErrbuf: "root usage\ninvalid value \"value_err\" for flag rootStr: custom test error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "after_value_unknown_flag",
			args: []string{"root", "-rootStr=unknown_flag_err", "sub"},
			vars: map[string]string{},

			wantErrbuf: "root usage\ncustom test error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "after_generic_err",
			args: []string{"root", "-rootStr=generic_err", "sub"},

			wantErrbuf: "root usage\ncustom test error\n",
			wantStatus: cli.ExitUsage,
		},
		{
			name: "nil_cmd_flags_func",
			args: []string{"root", "nil_flags"},

			wantOutbuf: "nil_flags out\n",
		},
		{
			name: "nil_cmd_varmap",
			args: []string{"root", "nil_varmap"},

			wantOutbuf: "nil_varmap out\n",
		},
		{
			name: "nil_cmd_after",
			args: []string{"root", "nil_after"},

			wantOutbuf: "nil_after out\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var params p
			gotParams, gotArgs, gotOutbuf, gotErrbuf, gotStatus := execTestCommand(t, cmdFactory(), &params, tt)

			if want, got := tt.wantStatus, gotStatus; want != got {
				t.Errorf("%s: cmd.Execute()=%v, want %v", tt.name, got, want)
			}
			if diff := cmp.Diff(tt.wantOutbuf, gotOutbuf); diff != "" {
				t.Errorf("%s: cmd.Execute out buffer mismatch (-want +got):\n%s", tt.name, diff)
			}
			if diff := cmp.Diff(tt.wantErrbuf, gotErrbuf); diff != "" {
				t.Errorf("%s: cmd.Execute err buffer mismatch (-want +got):\n%s", tt.name, diff)
			}
			if tt.wantArgs != nil {
				if diff := cmp.Diff(tt.wantArgs, gotArgs); diff != "" {
					t.Errorf("%s: env.Args mismatch (-want +got):\n%s", tt.name, diff)
				}
			}
			if tt.wantParams != nil {
				if diff := cmp.Diff(tt.wantParams, gotParams); diff != "" {
					t.Errorf("%s: cmd.Execute() params mismatch (-want +got):\n%s", tt.name, diff)
				}
			}
		})
	}
}

func ExampleCommand() {
	type p struct {
		env     string
		verbose bool
		port    uint
	}

	newRootCmd := func() *cli.Command[*p] {
		serveCmd := &cli.Command[*p]{
			Name:  "serve",
			Usage: "usage: foo [foo flags] serve [flags]",
			Help: `flags:
  -port`,
			Flags: func(fs *flag.FlagSet, p *p) {
				fs.UintVar(&p.port, "port", 5000, "")
			},
			Vars: map[string]string{
				"port": "FOO_PORT",
			},
			After: func(p *p) error {
				if p.port > 65535 {
					return &cli.ValueError{
						Name: "port",
						Err:  errors.New("cannot exceed 65535"),
					}
				}
				return nil
			},
			Action: func(ctx context.Context, env *cli.Env[*p]) cli.ExitStatus {
				env.Printf("env=%s\n", env.Params.env)
				env.Printf("verbose=%t\n", env.Params.verbose)
				env.Printf("port=%d\n", env.Params.port)
				return cli.ExitSuccess
			},
		}

		return &cli.Command[*p]{
			Name:  "foo",
			Usage: "usage: foo [flags] command",
			Help: `commands:
  serve

flags:
  -env
  -verbose`,
			Flags: func(fs *flag.FlagSet, p *p) {
				fs.StringVar(&p.env, "env", "production", "")
				fs.BoolVar(&p.verbose, "v", false, "")
			},
			After: func(p *p) error {
				if p.env == "dev" && !p.verbose {
					p.verbose = true
				}
				return nil
			},
			Subcommands: []*cli.Command[*p]{
				serveCmd,
			},
		}

	}

	var p1 p
	status := newRootCmd().Execute(context.Background(), &cli.Env[*p]{
		Args: []string{"foo", "-env=dev", "serve"},
		Vars: map[string]string{
			"FOO_PORT": "8999",
		},
		Out:    os.Stdout,
		Err:    os.Stdout, // for example output
		Params: &p1,
	})
	fmt.Printf("status=%d\n\n", status)

	var p2 p
	status = newRootCmd().Execute(context.Background(), &cli.Env[*p]{
		Args: []string{"foo", "-env=dev", "serve"},
		Vars: map[string]string{
			"FOO_PORT": "99999",
		},
		Out:    os.Stdout,
		Err:    os.Stdout, // for example output
		Params: &p2,
	})
	fmt.Printf("status=%d\n\n", status)

	var p3 p
	status = newRootCmd().Execute(context.Background(), &cli.Env[*p]{
		Args:   []string{"foo", "-h"},
		Out:    os.Stdout,
		Err:    os.Stdout, // for example output
		Params: &p3,
	})
	fmt.Printf("status=%d\n", status)

	// Output: env=dev
	// verbose=true
	// port=8999
	// status=0
	//
	// usage: foo [foo flags] serve [flags]
	// invalid value "99999" for var $FOO_PORT: cannot exceed 65535
	// status=2
	//
	// usage: foo [flags] command
	//
	// commands:
	//   serve
	//
	// flags:
	//   -env
	//   -verbose
	// status=0
}
