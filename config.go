package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/pflag"
)

const (
	EnvPrefix    = "CMDJAIL"
	JailFilename = ".cmd.jail"
)

var (
	flagLog          string
	flagEnvReference string
	flagJailFile     string
	flagVerbose      bool
	flagVersion      bool
)

var (
	// TODO: promote to type and capture cmd to log
	ErrCmdNotWrappedInQuotes = errors.New("cmd must be wrapped in single quotes")
	ErrNoIntentCmd           = errors.New("no intent cmd provided")
	// TODO: promote to type and capture cmd to log
	ErrJailFileManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", JailFilename)
	// TODO: promote to type and capture cmd to log
	ErrJailBinaryManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", filepath.Base(os.Args[0]))
	// TODO: promote to type and capture cmd to log
	ErrJailLogManipulationAttempt = errors.New("attempting to manipulate cmdjail log. Aborted")
)

type envVars struct {
	IntentCmd    string `envconfig:"CMD"`
	Log          string
	EnvReference string `envconfig:"ENV_REFERENCE"`
	JailFile     string
	Verbose      bool
}

func defaultEnvVars() (envVars, error) {
	ex, err := os.Executable()
	if err != nil {
		return envVars{}, err
	}
	exPath := filepath.Dir(ex)

	return envVars{
		JailFile: filepath.Join(exPath, JailFilename),
	}, nil
}

type Config struct {
	IntentCmd string
	Log       string
	JailFile  string
	Verbose   bool
}

var NoConfig = Config{}

func init() {
	pflag.ErrHelp = errors.New("")
}

func parseEnvVars() (envVars, error) {
	envvars, err := defaultEnvVars()
	if err != nil {
		return envVars{}, err
	}

	err = envconfig.Process(EnvPrefix, &envvars)
	if err != nil {
		return envVars{}, err
	}

	return envvars, nil
}

func parseEnvAndFlags() (Config, error) {
	conf, err := parseEnvVars()
	if err != nil {
		return NoConfig, err
	}

	cmdOptions := parseFlags(conf)

	if flagVerbose {
		debug = true
	}

	if flagLog != "" {
		logFd, err := os.Create(flagLog)
		if err != nil {
			return NoConfig, err
		}
		log.SetOutput(logFd)
	}

	if conf.IntentCmd != "" && conf.EnvReference != "" {
		printLogWarn(os.Stderr, "both %s and %s environment variables are set", EnvPrefix+"_CMD")
	}

	cmd := conf.IntentCmd
	if cmd != "" {
		printLogDebug(os.Stderr, "intent command loaded from $%s_CMD\n", EnvPrefix)
	}

	if cmd == "" && conf.EnvReference != "" {
		cmd = os.Getenv(conf.EnvReference)
		printLogDebug(os.Stderr, "intent command loaded from $%S_%s\n", EnvPrefix, conf.EnvReference)
	}

	if len(cmdOptions) > 0 {
		if len(cmdOptions) > 1 {
			return NoConfig, ErrCmdNotWrappedInQuotes
		}

		cmd = cmdOptions[0]
		printLogDebug(os.Stderr, "intent command loaded from arguments after --\n")
	}

	// TODO: These are all very simplistic checks, likely need to make them more sophisticated
	if cmd == "" {
		return NoConfig, ErrNoIntentCmd
	} else if strings.Contains(cmd, JailFilename) {
		return NoConfig, ErrJailFileManipulationAttempt
	} else if strings.Contains(cmd, filepath.Base(os.Args[0])) {
		return NoConfig, ErrJailBinaryManipulationAttempt
	} else if flagLog != "" && strings.Contains(cmd, flagLog) {
		return NoConfig, ErrJailLogManipulationAttempt
	}

	return Config{
		IntentCmd: cmd,
		Log:       flagLog,
		JailFile:  flagJailFile,
		Verbose:   flagVerbose,
	}, nil
}

func splitAtEndOfArgs(args []string) ([]string, []string) {
	if len(args) == 0 || len(args) == 1 {
		return nil, nil
	}

	args = args[1:]

	for i, arg := range args {
		if arg == "--" {
			return args[:i], args[i+1:]
		}
	}
	return args, nil
}

func parseFlags(conf envVars) []string {
	// pflag.BoolVar(&flagVersion, "version", conf.Verbose, "print version info")
	pflag.BoolVarP(&flagVerbose, "verbose", "v", conf.Verbose, "enable verbose mode")
	pflag.StringVarP(&flagLog, "log-file", "l", conf.Log, "log file location e.g. /var/log/cmdjail.log. If unset defaults to syslog.")
	pflag.StringVarP(&flagEnvReference, "env-reference", "r", conf.EnvReference, "name of an environment variable that holds the cmd to execute e.g. SSH_ORIGINAL_COMMAND")
	pflag.StringVarP(&flagJailFile, "jail-file", "j", conf.JailFile, "jail file location, if not set checks stdin")

	args, cmdOptions := splitAtEndOfArgs(os.Args)
	pflag.CommandLine.Parse(args)
	return cmdOptions
}
