Describe 'cmdjail.sh'
  cleanup() { 
    rm -f bin/.cmd.jail
    rm -f /tmp/cmdjail.log
  }
  AfterEach cleanup
  It 'exits with code 126 when no .cmd.jail found'
    When run bin/cmdjail.sh
    The status should equal 126
    The line 1 of stderr should equal "[error]: no .cmd.jail file found"
  End
  It 'exits with code 126 and logs when intent cmd includes .cmd.jail'
    Path log-file=/tmp/cmdjail.log
    cmdjail() { 
      touch bin/.cmd.jail
      CMDJAIL_LOG=/tmp/cmdjail.log bin/cmdjail.sh -- cat .cmd.jail; 
    }
    When run cmdjail
    The status should equal 126
    The line 1 of stderr should equal "[error]: attempting to manipulate .cmd.jail. Aborting."
    The contents of file "/tmp/cmdjail.log" should include "[error]: attempting to manipulate .cmd.jail. Aborting."
  End
  It 'exits with code 126 and logs when .cmd.jail found and intent cmd is empty'
    Path log-file=/tmp/cmdjail.log
    cmdjail() { 
      touch bin/.cmd.jail
      CMDJAIL_LOG=/tmp/cmdjail.log bin/cmdjail.sh; 
    }
    When run cmdjail
    The status should equal 126
    The line 1 of stderr should equal "[error]: no command"
    The contents of file "/tmp/cmdjail.log" should include "[error]: no command"
  End
  It 'exits with code 126 and logs when intent cmd includes cmdjail.sh'
    Path log-file=/tmp/cmdjail.log
    cmdjail() { 
      touch bin/.cmd.jail
      CMDJAIL_LOG=/tmp/cmdjail.log bin/cmdjail.sh -- cat cmdjail.sh; 
    }
    When run cmdjail
    The status should equal 126
    The line 1 of stderr should equal "[error]: attempting to manipulate cmdjail.sh. Aborting."
    The contents of file "/tmp/cmdjail.log" should include "[error]: attempting to manipulate cmdjail.sh. Aborting."
  End
  It 'logs an attempt to run a blacklisted command'
    Path log-file=/tmp/cmdjail.log
    cmdjail() { 
      touch bin/.cmd.jail
      CMDJAIL_LOG=/tmp/cmdjail.log bin/cmdjail.sh -- cat /tmp/cmdjail.log; 
    }
    When run cmdjail
    The status should equal 2
    The contents of file "/tmp/cmdjail.log" should include "[warn]: blocked blacklisted command: cat /tmp/cmdjail.log"
  End
  Describe 'exits with cli flag subcommand exit code when its whitelisted'
    It 'exits with 0 when ls is whitelisted'
      cmdjail() { 
        echo "ls" > bin/.cmd.jail
        bin/cmdjail.sh -- ls -al; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits with 1 when cat is whitelisted with invalid target'
      cmdjail() { 
        echo "cat" > bin/.cmd.jail
        bin/cmdjail.sh -- cat non-existant.file; 
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "cat: non-existant.file"
    End
    It 'exits with 0 when ls is whitelisted with other cmds'
      cmdjail() { 
        echo -e "cat\nls\nfind" > bin/.cmd.jail
        bin/cmdjail.sh -- ls -al; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End
  Describe 'exits with env var subcommand exit code when its whitelisted'
    It 'exits with 0 when ls is whitelisted'
      cmdjail() { 
        echo "ls" > bin/.cmd.jail
        CMDJAIL_CMD='ls -al' bin/cmdjail.sh
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits with 1 when cat is whitelisted with invalid target'
      cmdjail() { 
        echo "cat" > bin/.cmd.jail
        CMDJAIL_CMD='cat non-existant.file' bin/cmdjail.sh
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "cat: non-existant.file"
    End
    It 'exits with 0 when ls is whitelisted with other cmds'
      cmdjail() { 
        echo -e "cat\nls\nfind" > bin/.cmd.jail
        CMDJAIL_CMD='ls -al' bin/cmdjail.sh
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End
  Describe 'exits with env var reference subcommand exit code when its whitelisted'
    It 'exits with 0 when ls is whitelisted'
      cmdjail() { 
        echo "ls" > bin/.cmd.jail
        CMD='ls -al' CMDJAIL_ENV_REFERENCE=CMD bin/cmdjail.sh
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits with 1 when cat is whitelisted with invalid target'
      cmdjail() { 
        echo "cat" > bin/.cmd.jail
        CMD='cat non-existant.file' CMDJAIL_ENV_REFERENCE=CMD bin/cmdjail.sh
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "cat: non-existant.file"
    End
    It 'exits with 0 when ls is whitelisted with other cmds'
      cmdjail() { 
        echo -e "cat\nls\nfind" > bin/.cmd.jail
        CMD='ls -al' CMDJAIL_ENV_REFERENCE=CMD bin/cmdjail.sh;
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End
  Describe 'handles multiple -- occurrences correctly'
    It 'uses the command after the last -- when multiple -- are present'
      cmdjail() { 
        echo -e "ls\ncat" > bin/.cmd.jail
        bin/cmdjail.sh --flag1 --flag2 -- ls --help -- cat .gitignore
      }
      When run cmdjail
      The status should equal 0
      The stdout should include ".env"
    End
    It 'handles commands with -- in their arguments'
      cmdjail() {
        echo "grep" > bin/.cmd.jail
        DEBUG=true bin/cmdjail.sh -- grep --version --exclude
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "grep"
    End
    It 'handles empty args after the last --'
      cmdjail() {
        echo "ls" > bin/.cmd.jail
        bin/cmdjail.sh -- ls -- --
      }
      When run cmdjail
      The status should equal 126
      The stderr should include "[error]: no command"
    End
  End
  # Describe 'security vulnerabilities'
  # cleanup() {
  #   rm -f bin/.cmd.jail
  #   rm -f /tmp/cmdjail.log
  #   rm -f /tmp/evil.sh
  # }
  # AfterEach cleanup
  #   It 'is vulnerable to command injection via backticks'
  #   Path triggered=.subshell
  #     cmdjail() {
  #       # Create a whitelist with only 'echo' allowed
  #       echo "echo" > bin/.cmd.jail
  #
  #       # Run a command that should be blocked, but using backtick execution
  #       # The whitelist check only looks at the first word (echo), but the
  #       # backticks will be evaluated when the command is executed
  #       DEBUG=true bin/cmdjail.sh -- "echo `touch .subshell && id`"
  #     }
  #     When run cmdjail
  #     The status should equal 126
  #     # If the test passes with exit code 0 and shows uid output, it means 'id' was executed
  #     # despite not being in the whitelist
  #     The path triggered should not be file
  #     The stdout should not include "uid="
  #   End
    # It 'is vulnerable to command injection via subshell expression'
    #   cmdjail() {
    #     # Create a whitelist with only 'echo' allowed
    #     echo "echo" > bin/.cmd.jail
    #
    #     # Create a file that will execute the 'id' command when sourced
    #     echo "id" > /tmp/evil.sh
    #     chmod +x /tmp/evil.sh
    #
    #     # Run a command that should be blocked, but using command substitution
    #     # The whitelist check only looks at the first word (echo), but the
    #     # command substitution will be expanded when executed
    #     bin/cmdjail.sh -- "echo $(sh /tmp/evil.sh)"
    #   }
    #   When run cmdjail
    #   The status should equal 0
    #   The stdout should not include "uid="
    # End
  # End
End
