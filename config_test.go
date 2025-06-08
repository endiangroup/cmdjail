package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// setup is a helper to reset flags and args before each test case.
func setup(args ...string) {
	pflag.CommandLine = pflag.NewFlagSet(os.Args[0], pflag.ExitOnError)
	os.Args = append([]string{os.Args[0]}, args...)
	os.Clearenv()
}

func TestParseEnvAndFlags(t *testing.T) {
	originalArgs := os.Args
	defer func() { os.Args = originalArgs }()

	t.Run("Returns config from environment variables", func(t *testing.T) {
		setup() // Reset flags and args
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

	t.Run("Returns config with command set from EnvReference", func(t *testing.T) {
		setup() // Reset flags and args
		cmd := "cmd -s --long-flag -a=123 -a 123"
		os.Setenv("CMD", cmd)
		os.Setenv(EnvPrefix+"_ENV_REFERENCE", "CMD")

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)

		assert.Equal(t, cmd, c.IntentCmd)
	})

	t.Run("Flag overrides environment variable", func(t *testing.T) {
		setup("--jail-file", "/flag/path/.cmd.jail")
		os.Setenv(EnvPrefix+"_JAILFILE", "/env/path/.cmd.jail")

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.Equal(t, "/flag/path/.cmd.jail", c.JailFile)
	})

	t.Run("Sets default jail file path relative to executable", func(t *testing.T) {
		setup()

		c, err := parseEnvAndFlags()
		assert.NoError(t, err)

		ex, _ := os.Executable()
		exPath := filepath.Dir(ex)
		expectedPath := filepath.Join(exPath, JailFilename)

		assert.Equal(t, expectedPath, c.JailFile)
	})

	t.Run("Returns error when command set after -- isn't a single argument", func(t *testing.T) {
		tests := [][]string{
			{"--", "ls", "-alh"},
			{"-v", "--", "ls", "-alh"},
		}

		for _, test := range tests {
			t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
				setup(test...)
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
			{inArgs: []string{"-v", "--", "ls -alh"}, expectedCmd: "ls -alh"},
		}

		for _, test := range tests {
			t.Run(fmt.Sprintf("%v", test), func(t *testing.T) {
				setup(test.inArgs...)
				c, err := parseEnvAndFlags()
				assert.NoError(t, err)
				assert.Equal(t, test.expectedCmd, c.IntentCmd)
			})
		}
	})

	t.Run("Returns config with Shell mode true when no command is provided", func(t *testing.T) {
		setup()
		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})

	t.Run("Returns config with CheckMode true when --check is provided", func(t *testing.T) {
		setup("--check")
		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.CheckMode)
		assert.False(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})

	t.Run("Returns config with CheckMode true when --check-intent-cmds is provided", func(t *testing.T) {
		setup("--check-intent-cmds", "/tmp/cmds")
		c, err := parseEnvAndFlags()
		assert.NoError(t, err)
		assert.True(t, c.CheckMode)
		assert.Equal(t, "/tmp/cmds", c.CheckIntentCmdsFile)
		assert.False(t, c.Shell)
		assert.Empty(t, c.IntentCmd)
	})

	t.Run("Returns error when both jailfile and check-intent-cmds are stdin", func(t *testing.T) {
		setup("-j", "-", "--check-intent-cmds", "-")
		_, err := parseEnvAndFlags()
		assert.Equal(t, ErrJailFileAndCheckCmdsFromStdin, err)
	})
}

func TestCheckCmdSafety(t *testing.T) {
	binaryName := filepath.Base(os.Args[0]) // e.g., "config.test"

	t.Run("Blocks direct manipulation of jail file", func(t *testing.T) {
		err := checkCmdSafety("cat .cmd.jail", "")
		assert.Equal(t, ErrJailFileManipulationAttempt, err)
	})

	t.Run("Blocks direct manipulation of binary", func(t *testing.T) {
		err := checkCmdSafety(fmt.Sprintf("rm %s", binaryName), "")
		assert.Equal(t, ErrJailBinaryManipulationAttempt, err)
	})

	t.Run("Blocks direct manipulation of log file", func(t *testing.T) {
		err := checkCmdSafety("rm /var/log/jail.log", "/var/log/jail.log")
		assert.Equal(t, ErrJailLogManipulationAttempt, err)
	})

	// An active security vlnrability
	//
	// t.Run("Should block jail file manipulation with quotes", func(t *testing.T) {
	// 	err := checkCmdSafety(`cat ".cmd."jail`, "")
	// 	assert.Equal(t, ErrJailFileManipulationAttempt, err, "This test should fail until the safety check is improved to handle string obfuscation.")
	// })
	//

	t.Run("Allows safe commands", func(t *testing.T) {
		err := checkCmdSafety("ls -l", "/var/log/jail.log")
		assert.NoError(t, err)
	})
}

func TestSplitAtEndOfArgs(t *testing.T) {
	tests := []struct {
		name         string
		inArgs       []string
		expectedArgs []string
		expectedCmd  []string
	}{
		{
			name:         "splits at --",
			inArgs:       []string{"cmdjail", "-v", "--", "ls", "-l"},
			expectedArgs: []string{"-v"},
			expectedCmd:  []string{"ls", "-l"},
		},
		{
			name:         "no -- returns all as args",
			inArgs:       []string{"cmdjail", "-v", "ls", "-l"},
			expectedArgs: []string{"-v", "ls", "-l"},
			expectedCmd:  nil,
		},
		{
			name:         "-- is first argument",
			inArgs:       []string{"cmdjail", "--", "ls", "-l"},
			expectedArgs: []string{},
			expectedCmd:  []string{"ls", "-l"},
		},
		{
			name:         "-- is last argument",
			inArgs:       []string{"cmdjail", "-v", "--"},
			expectedArgs: []string{"-v"},
			expectedCmd:  []string{},
		},
		{
			name:         "only --",
			inArgs:       []string{"cmdjail", "--"},
			expectedArgs: []string{},
			expectedCmd:  []string{},
		},
		{
			name:         "empty slice",
			inArgs:       []string{},
			expectedArgs: nil,
			expectedCmd:  nil,
		},
		{
			name:         "only binary name",
			inArgs:       []string{"cmdjail"},
			expectedArgs: nil,
			expectedCmd:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args, cmd := splitAtEndOfArgs(tt.inArgs)

			assert.Equal(t, tt.expectedArgs, args)
			assert.Equal(t, tt.expectedCmd, cmd)
		})
	}
}
