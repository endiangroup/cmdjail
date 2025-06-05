package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"log/syslog"
	"os"
	"path/filepath"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/pflag"
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
	Cmd      string
	Log      string
	JailFile string
	Verbose  bool
}

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
	ErrEmptyJailFile         = errors.New("empty jail file")
	ErrNoIntentCmd           = errors.New("no intent cmd provided")
	// TODO: promote to type and capture cmd to log
	ErrJailFileManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", JailFilename)
	// TODO: promote to type and capture cmd to log
	ErrJailBinaryManipulationAttempt = fmt.Errorf("attempting to manipulate: %s. Aborted", filepath.Base(os.Args[0]))
	// TODO: promote to type and capture cmd to log
	ErrJailLogManipulationAttempt = errors.New("attempting to manipulate cmdjail log. Aborted")
)

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
		return Config{}, err
	}

	err = envconfig.Process(EnvPrefix, &conf)
	if err != nil {
		return Config{}, err
	}

	pflag.BoolVarP(&flagVerbose, "verbose", "v", conf.Verbose, "enable verbose mode")
	pflag.StringVarP(&flagLog, "log-file", "l", conf.Log, "log file location e.g. /var/log/cmdjail.log. If unset defaults to syslog.")
	pflag.StringVarP(&flagEnvReference, "env-reference", "r", conf.EnvReference, "name of an environment variable that holds the cmd to execute e.g. SSH_ORIGINAL_COMMAND")
	pflag.StringVarP(&flagJailFile, "jail-file", "j", conf.JailFile, "jail file location. By default it searches for a .cmd.jail file")

	args, cmdOptions := splitArgs(os.Args)
	pflag.CommandLine.Parse(args)

	if flagLog != "" {
		logFd, err := os.Create(flagLog)
		if err != nil {
			return Config{}, err
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
			return Config{}, ErrCmdNotWrappedInQuotes
		}

		cmd = cmdOptions[0]
	}

	// TODO: These are all very simplistic checks, likely need to make them more sophisticated
	if cmd == "" {
		return Config{}, ErrNoIntentCmd
	} else if strings.Contains(cmd, JailFilename) {
		return Config{}, ErrJailFileManipulationAttempt
	} else if strings.Contains(cmd, filepath.Base(os.Args[0])) {
		return Config{}, ErrJailBinaryManipulationAttempt
	} else if flagLog != "" && strings.Contains(cmd, flagLog) {
		return Config{}, ErrJailLogManipulationAttempt
	}

	return Config{
		Cmd:      cmd,
		Log:      flagLog,
		JailFile: flagJailFile,
		Verbose:  flagVerbose,
	}, nil
}

type JailFile struct {
	Allowed []string
}

func parseJailFile(f io.Reader) (JailFile, error) {
	scanner := bufio.NewScanner(f)

	var jf JailFile
	jf.Allowed = []string{}
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			jf.Allowed = append(jf.Allowed, strings.TrimSpace(line))
		}
	}

	if len(jf.Allowed) == 0 {
		return jf, ErrEmptyJailFile
	}

	if err := scanner.Err(); err != nil {
		return JailFile{}, err
	}

	return jf, nil
}

func errIsAny(target error, errs ...error) bool {
	for _, err := range errs {
		if errors.Is(target, err) {
			return true
		}
	}

	return false
}

func main() {
	logWriter, err := syslog.New(syslog.LOG_SYSLOG, "cmdjail")
	if err != nil {
		printErr(os.Stderr, "unable to set logfile: %s", err.Error())
		os.Exit(1)
	}
	log.SetOutput(logWriter)
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	conf, err := parseEnvAndFlags()
	if err != nil {
		printLogErr(os.Stderr, "%s", err.Error())
		if errIsAny(err,
			ErrCmdNotWrappedInQuotes,
			ErrJailFileManipulationAttempt,
			ErrJailBinaryManipulationAttempt,
			ErrJailLogManipulationAttempt) {
			os.Exit(77)
		}
		os.Exit(1)
	}

	jailFileFd, err := os.Open(conf.JailFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			printLogErr(os.Stderr, "finding jail file: %s", conf.JailFile)
		} else {
			printLogErr(os.Stderr, "opening jail file: %s: %s", conf.JailFile, err.Error())
		}
		os.Exit(1)
	}

	_, err = parseJailFile(jailFileFd)
	if err != nil {
		if errors.Is(err, ErrEmptyJailFile) {
			printLogErr(os.Stderr, "empty jail file: %s", conf.JailFile)
		} else {
			printLogErr(os.Stderr, "parsing jail file: %s: %s", conf.JailFile, err.Error())
		}
		os.Exit(1)
	}
}

func printLog(printTo io.Writer, msg string, args ...any) {
	fmt.Fprintf(printTo, msg, args...)
	log.Printf(msg, args...)
}

func printErr(printTo io.Writer, msg string, args ...any) {
	fmt.Fprintf(printTo, "[error] "+msg, args...)
}

func logErr(msg string, args ...any) {
	log.Printf(msg, args...)
}

func printLogErr(printTo io.Writer, msg string, args ...any) {
	printLog(printTo, "[error] "+msg, args...)
}
