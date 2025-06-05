package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/kelseyhightower/envconfig"
	"github.com/spf13/pflag"
)

type envConfig struct {
	Cmd          string
	Log          string
	EnvReference string `envconfig:"ENV_REFERENCE"`
	JailFile     string `default:".cmd.jail"`
	Verbose      bool
}

type Config struct {
	Cmd      string
	Log      string
	JailFile string
	Verbose  bool
}

const (
	EnvPrefix = "CMDJAIL"
)

var (
	flagLog          string
	flagEnvReference string
	flagJailFile     string
	flagVerbose      bool
	flagVersion      bool
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

var ErrCmdNotWrappedInQuotes = errors.New("command not wrapped in single quotes to prevent subshell spawning (')")

func parseEnvAndFlags() (Config, error) {
	var conf envConfig
	err := envconfig.Process(EnvPrefix, &conf)
	if err != nil {
		return Config{}, err
	}

	pflag.BoolVarP(&flagVerbose, "verbose", "v", conf.Verbose, "enable verbose mode")
	pflag.StringVarP(&flagLog, "log-file", "l", conf.Log, "log file location e.g. /var/log/cmdjail.log. If unset logging is disabled.")
	pflag.StringVarP(&flagEnvReference, "env-reference", "r", conf.EnvReference, "name of an environment variable that holds the cmd to execute e.g. SSH_ORIGINAL_COMMAND")
	pflag.StringVarP(&flagJailFile, "jail-file", "j", conf.JailFile, "jail file location. By default it searches for a .cmd.jail file")

	args, cmdOptions := splitArgs(os.Args)
	pflag.CommandLine.Parse(args)

	if conf.Cmd != "" && conf.EnvReference != "" {
		// TODO: log warning
	}

	cmd := conf.Cmd
	if cmd == "" && conf.EnvReference != "" {
		cmd = os.Getenv(conf.EnvReference)
	}

	if len(cmdOptions) > 0 {
		if !strings.HasPrefix(cmdOptions[0], "'") && !strings.HasSuffix(cmdOptions[0], "'") {
			return Config{}, ErrCmdNotWrappedInQuotes
		}

		cmd = strings.Trim(cmdOptions[0], "'")
	}

	return Config{
		Cmd:      cmd,
		Log:      flagLog,
		JailFile: flagJailFile,
		Verbose:  flagVerbose,
	}, nil
}

var ErrEmptyJailFile = errors.New("empty jail file")

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

func main() {
	conf, err := parseEnvAndFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[error] %s\n", err.Error())
		os.Exit(1)
	}

	_, err = os.Open(conf.JailFile)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "[error] jail file not found: %s\n", conf.JailFile)
		} else {
			fmt.Fprintf(os.Stderr, "[error] opening jail file: %s: %s\n", conf.JailFile, err.Error())
		}
		os.Exit(1)
	}

	// jf, err := parseJailFile()
}
