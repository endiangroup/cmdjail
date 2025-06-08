package main

import (
	"errors"
	"fmt"
	"io"
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
	flagRecordFile   string
	flagVersion      bool
)

var (
	// TODO: promote to type and capture cmd to log
	ErrCmdNotWrappedInQuotes = errors.New("cmd must be wrapped in single quotes")
	// TODO: promote to type and capture cmd to log
	ErrJailFileManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", JailFilename)
	// TODO: promote to type and capture cmd to log
	ErrJailBinaryManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", filepath.Base(os.Args[0]))
	// TODO: promote to type and capture cmd to log
	ErrJailLogManipulationAttempt = errors.New("attempting to manipulate cmdjail log. Aborted")
)

type envVars struct {
	IntentCmd    string `envconfig:"CMDJAIL_CMD"`
	Log          string
	EnvReference string `envconfig:"CMDJAIL_ENV_REFERENCE"`
	JailFile     string
	RecordFile   string
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
	IntentCmd  string
	Log        string
	JailFile   string
	Verbose    bool
	RecordFile string
	Shell      bool
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
	envvars, err := parseEnvVars()
	if err != nil {
		return NoConfig, err
	}

	cmdOptions := parseFlags(envvars)

	if flagVerbose {
		debug = true
	}

	// Configure logging based on flag and environment variable precedence.
	logFileIsSetByFlag := pflag.CommandLine.Changed("log-file")
	_, logFileIsSetByEnv := os.LookupEnv(EnvPrefix + "_LOG")

	var logVal string
	if !logFileIsSetByEnv && !logFileIsSetByFlag {
		log.SetOutput(io.Discard)
	} else {
		logVal = envvars.Log
		if logFileIsSetByFlag {
			logVal = flagLog
		}

		if logVal == "" {
			// An empty value means use syslog.
			if err := setLoggerToSyslog(); err != nil {
				return NoConfig, fmt.Errorf("configuring syslog logger: %w", err)
			}
			printLogDebug(os.Stdout, "logging to syslog\n")
		} else {
			// A non-empty value is a file path.
			if err := setLoggerToFile(logVal); err != nil {
				return NoConfig, fmt.Errorf("configuring file logger: %w", err)
			}
			printLogDebug(os.Stdout, "logging to: %s\n", logVal)
		}
	}

	printLogDebug(os.Stderr, "loaded env vars: %+v", envvars)
	pflag.Visit(func(f *pflag.Flag) {
		printLogDebug(os.Stderr, "flag set: %+v", f)
	})

	if envvars.IntentCmd != "" && envvars.EnvReference != "" {
		printLogWarn(os.Stderr, "both %s and %s environment variables are set", EnvPrefix+"_CMD")
	}

	cmd := envvars.IntentCmd
	if cmd != "" {
		printLogDebug(os.Stderr, "intent command loaded from $%s_CMD\n", EnvPrefix)
	}

	if cmd == "" && flagEnvReference != "" {
		cmd = os.Getenv(flagEnvReference)
		printLogDebug(os.Stderr, "intent command loaded from: $%s", flagEnvReference)
	}

	if len(cmdOptions) > 0 {
		if len(cmdOptions) > 1 {
			return NoConfig, ErrCmdNotWrappedInQuotes
		}

		cmd = cmdOptions[0]
		printLogDebug(os.Stderr, "intent command loaded from arguments\n")
	}

	shellMode := cmd == ""

	if cmd != "" {
		if err := checkCmdSafety(cmd, logVal); err != nil {
			return NoConfig, err
		}
	}

	return Config{
		IntentCmd:  cmd,
		Log:        flagLog,
		JailFile:   flagJailFile,
		Verbose:    flagVerbose,
		RecordFile: flagRecordFile,
		Shell:      shellMode,
	}, nil
}

func checkCmdSafety(cmd, logPath string) error {
	if strings.Contains(cmd, JailFilename) {
		return ErrJailFileManipulationAttempt
	} else if strings.Contains(cmd, filepath.Base(os.Args[0])) {
		return ErrJailBinaryManipulationAttempt
	} else if logPath != "" && strings.Contains(cmd, logPath) {
		return ErrJailLogManipulationAttempt
	}
	return nil
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

func parseFlags(envvars envVars) []string {
	// pflag.BoolVar(&flagVersion, "version", conf.Verbose, "print version info")
	pflag.BoolVarP(&flagVerbose, "verbose", "v", envvars.Verbose, "enable verbose mode")
	pflag.StringVarP(&flagLog, "log-file", "l", envvars.Log, "log file location e.g. /var/log/cmdjail.log. If set to \"\" logs to syslog. If unset logging is disabled.")
	pflag.StringVarP(&flagEnvReference, "env-reference", "e", envvars.EnvReference, "name of an environment variable that holds the cmd to execute e.g. SSH_ORIGINAL_COMMAND")
	pflag.StringVarP(&flagJailFile, "jail-file", "j", envvars.JailFile, "jail file location, if not set checks stdin")
	pflag.StringVarP(&flagRecordFile, "record", "r", envvars.RecordFile, "transparently run the intent cmd and append it to the specified file as a literal allow rule")

	args, cmdOptions := splitAtEndOfArgs(os.Args)
	pflag.CommandLine.Parse(args)

	return cmdOptions
}
