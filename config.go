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

type envConfig struct {
	Cmd          string
	Log          string
	EnvReference string `envconfig:"ENV_REFERENCE"`
	JailFile     string
	Verbose      bool
}

func defaultEnvConfig() (envConfig, error) {
	ex, err := os.Executable()
	if err != nil {
		return envConfig{}, err
	}
	exPath := filepath.Dir(ex)

	return envConfig{
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

func splitArgs(args []string) ([]string, []string) {
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

func parseEnvAndFlags() (Config, error) {
	conf, err := defaultEnvConfig()
	if err != nil {
		return NoConfig, err
	}

	err = envconfig.Process(EnvPrefix, &conf)
	if err != nil {
		return NoConfig, err
	}

	pflag.BoolVarP(&flagVerbose, "verbose", "v", conf.Verbose, "enable verbose mode")
	pflag.StringVarP(&flagLog, "log-file", "l", conf.Log, "log file location e.g. /var/log/cmdjail.log. If unset defaults to syslog.")
	pflag.StringVarP(&flagEnvReference, "env-reference", "r", conf.EnvReference, "name of an environment variable that holds the cmd to execute e.g. SSH_ORIGINAL_COMMAND")
	pflag.StringVarP(&flagJailFile, "jail-file", "j", conf.JailFile, "jail file location, if not set uses stdin. By default it searches for a .cmd.jail file")

	args, cmdOptions := splitArgs(os.Args)
	pflag.CommandLine.Parse(args)

	if flagLog != "" {
		logFd, err := os.Create(flagLog)
		if err != nil {
			return NoConfig, err
		}
		log.SetOutput(logFd)
	}

	if conf.Cmd != "" && conf.EnvReference != "" {
		// TODO: log warning
	}

	cmd := conf.Cmd
	if cmd == "" && conf.EnvReference != "" {
		cmd = os.Getenv(conf.EnvReference)
	}

	if len(cmdOptions) > 0 {
		if len(cmdOptions) > 1 {
			return NoConfig, ErrCmdNotWrappedInQuotes
		}

		cmd = cmdOptions[0]
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
