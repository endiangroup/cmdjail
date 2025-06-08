package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestParseEnvAndFlags(t *testing.T) {
	t.Run("Returns config from environment variables", func(t *testing.T) {
		os.Clearenv()
		defer os.Clearenv()
		cmd := "cmd -s --long-flag -a=123 -a 123"
		os.Setenv(EnvPrefix+"_CMD", cmd)
		log := "/tmp/cmdjail.log"
		os.Setenv(EnvPrefix+"_LOGFILE", log)
		jailfile := "/tmp/.cmd.jail"
		os.Setenv(EnvPrefix+"_JAILFILE", jailfile)
		os.Setenv(EnvPrefix+"_VERBOSE", "true")

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)

		assert.Equal(t, cmd, c.IntentCmd)
		assert.Equal(t, log, c.Log)
		assert.Equal(t, jailfile, c.JailFile)
		assert.Equal(t, true, c.Verbose)
	})

	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	t.Run("Returns config with command set from EnvReference", func(t *testing.T) {
		os.Clearenv()
		defer os.Clearenv()
		cmd := "cmd -s --long-flag -a=123 -a 123"
		os.Setenv("CMD", cmd)
		os.Setenv(EnvPrefix+"_ENV_REFERENCE", "CMD")

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)

		assert.Equal(t, cmd, c.IntentCmd)
	})

	t.Run("Returns error when command set after -- isn't a single argument", func(t *testing.T) {
		tests := [][]string{
			// Note: if you wrap an argument in single quotes it will appear
			// as a single item in the args array
			{"--", "ls", "-alh"},
			{"-v", "--", "ls", "-alh"},
			{"-v", "--", "ls", "-alh"},
		}

		for _, test := range tests {
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
			t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
				os.Args = append([]string{os.Args[0]}, test...)

				_, err := parseEnvAndFlags()

				assert.Equal(t, ErrCmdNotWrappedInQuotes, err)
			})
		}
	})
	t.Run("Returns config with command set from after -- arg", func(t *testing.T) {
		tests := []struct {
			inArgs      []string
			expectedCmd string
		}{
			{inArgs: []string{"--", "ls"}, expectedCmd: "ls"},
			{inArgs: []string{"-v", "--", "ls"}, expectedCmd: "ls"},
			// Note: if you wrap an argument in single quotes it will appear
			// as a single item in the args array
			{inArgs: []string{"-v", "--", "ls -alh"}, expectedCmd: "ls -alh"},
		}

		for _, test := range tests {
			pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
			t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
				os.Args = append([]string{os.Args[0]}, test.inArgs...)

				c, err := parseEnvAndFlags()
				assert.NoError(t, err)

				assert.Equal(t, test.expectedCmd, c.IntentCmd)
			})
		}
	})
	t.Run("Returns config with Shell mode true when no command is provided", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
		os.Args = []string{os.Args[0]}
		os.Clearenv()

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})
	t.Run("Returns config with CheckMode true when --check is provided", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
		os.Args = []string{os.Args[0], "--check"}
		os.Clearenv()

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.CheckMode)
		assert.False(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})

	t.Run("Returns config with CheckMode true when --check-intent-cmds is provided", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
		os.Args = []string{os.Args[0], "--check-intent-cmds", "/tmp/cmds"}
		os.Clearenv()

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.CheckMode)
		assert.Equal(t, "/tmp/cmds", c.CheckIntentCmdsFile)
		assert.False(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})

	t.Run("Returns error when both jailfile and check-intent-cmds are stdin", func(t *testing.T) {
		pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
		os.Args = []string{os.Args[0], "-j", "-", "--check-intent-cmds", "-"}
		os.Clearenv()

		_, err := parseEnvAndFlags()
		assert.Equal(t, ErrJailFileAndCheckCmdsFromStdin, err)
	})
}
