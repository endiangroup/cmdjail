Describe 'cmdjail.sh'
  build() {
    make -s bin/cmdjail
    rm -f bin/.cmd.jail
  }
  cleanup() { 
    rm -f bin/.cmd.jail
    rm -f .some.log
    rm -f /tmp/cmdjail.log
    rm -f /tmp/test.jail
    rm -f /tmp/test-cmds.txt
  }
  BeforeEach build
  AfterEach cleanup

  It 'exits 1 when no .cmd.jail found and no stdin set'
    cmdjail() {
      bin/cmdjail -- cat
    }
    When run cmdjail
    The status should equal 1
    The line 1 of stderr should include "[error] finding jail file:"
  End
  It 'exits 1 when empty .cmd.jail found'
    cmdjail() {
      touch bin/.cmd.jail
      bin/cmdjail -- 'cat'
    }
    When run cmdjail
    The status should equal 1
    The line 1 of stderr should include "[error] empty jail file"
  End

  Describe 'starts shell when intent cmd is empty'
    It 'when no command options are provided'
      cmdjail() {
        echo "+ 'echo hello" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log
      }
      Data
      #|echo hello
      #|exit
      End

      When run cmdjail
      The status should equal 0
      The stdout should include "hello"
    End
  End
  
  Describe 'safety and security'
    It 'exits 77 when intent cmd isnt wrapped in single quotes as single argument'
      cmdjail() { 
        bin/cmdjail -l /tmp/cmdjail.log -- cat .cmd.jail; 
      }
      When run cmdjail
      The status should equal 77
      The line 1 of stderr should equal "[error] cmd must be wrapped in single quotes"
      The contents of file "/tmp/cmdjail.log" should include "[error] cmd must be wrapped in single quotes"
    End
    It 'exits 77 when intent cmd includes .cmd.jail'
      cmdjail() { 
        touch bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'cat .cmd.jail'; 
      }
      When run cmdjail
      The status should equal 77
      The line 1 of stderr should equal "[error] attempting to manipulate: .cmd.jail. Aborted"
      The contents of file "/tmp/cmdjail.log" should include "[error] attempting to manipulate: .cmd.jail. Aborted"
    End
    It 'exits 77 when intent cmd includes binary name'
      cmdjail() { 
        touch bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'rm cmdjail'; 
      }
      When run cmdjail
      The status should equal 77
      The line 1 of stderr should equal "[error] attempting to manipulate: cmdjail. Aborted"
      The contents of file "/tmp/cmdjail.log" should include "[error] attempting to manipulate: cmdjail. Aborted"
    End
    It 'exits 77 when intent cmd includes log file'
      cmdjail() { 
        touch bin/.cmd.jail
        bin/cmdjail -l .some.log -- 'rm .some.log'; 
      }
      When run cmdjail
      The status should equal 77
      The line 1 of stderr should equal "[error] attempting to manipulate cmdjail log. Aborted"
      The contents of file ".some.log" should include "[error] attempting to manipulate cmdjail log. Aborted"
    End
    It 'exits 1 when both jailfile and check-intent-cmds are stdin'
      cmdjail() {
        bin/cmdjail -j - --check-intent-cmds -
      }
      When run cmdjail
      The status should equal 1
      The line 1 of stderr should equal "[error] jail file and check commands cannot both be read from stdin"
    End
  End

  Describe 'whitelist only'
    It 'exits 77 and logs an attempt to run a non-whitelisted intent cmd'
      cmdjail() { 
        echo "+ 'ls" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'cat /tmp/some.file'; 
      }
      When run cmdjail
      The status should equal 77
      The contents of file "/tmp/cmdjail.log" should include "[warn] implicitly blocked intent cmd: cat /tmp/some.file"
    End
    It 'exits 0 when ls is whitelisted as literal'
      cmdjail() { 
        echo "+ 'ls -al" > bin/.cmd.jail
        bin/cmdjail -- 'ls -al'; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits 0 when ls is whitelisted as regex'
      cmdjail() { 
        echo "+ r'^ls -al$" > bin/.cmd.jail
        bin/cmdjail -- 'ls -al'; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits 0 when ls is whitelisted via external command'
      cmdjail() { 
        echo "+ grep 'ls'" > bin/.cmd.jail
        bin/cmdjail -- 'ls -al'; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'exits 1 when cat is whitelisted with invalid target'
      cmdjail() { 
        echo "+ r'^cat" > bin/.cmd.jail
        bin/cmdjail -- 'cat non-existant.file';
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "cat: non-existant.file"
    End
    It 'exits 0 when ls is whitelisted amongst other cmds'
      cmdjail() { 
        echo -e "+ r'^cat\n+ r'^ls\n+ 'find" > bin/.cmd.jail
        bin/cmdjail -- 'ls -al'; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End

  Describe 'environment variables'
    It 'loads intent cmd from CMDJAIL_CMD env var'
      cmdjail() { 
        echo "+ 'ls -al" > bin/.cmd.jail
        CMDJAIL_CMD="ls -al" bin/cmdjail; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
    It 'loads intent cmd from CMDJAIL_ENV_REFERENCE env var'
      cmdjail() { 
        echo "+ 'ls -al" > bin/.cmd.jail
        CMD="ls -al" CMDJAIL_ENV_REFERENCE=CMD bin/cmdjail; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End

  It 'reads jail file from stdin when set to "-"'
    cmdjail() { 
      bin/cmdjail -j "-" -- "ls -l"; 
    }
    Data
    #|+ 'ls -l
    End

    When run cmdjail
    The status should equal 0
    The stdout should include "total"
  End

  Describe 'balcklist only'
    It 'exits 77 and logs an attempt to run a literal blacklisted intent cmd'
      cmdjail() { 
        echo "- 'cat /tmp/some.file" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'cat /tmp/some.file'; 
      }
      When run cmdjail
      The status should equal 77
      The contents of file "/tmp/cmdjail.log" should include "[warn] blocked blacklisted intent cmd: cat /tmp/some.file"
    End
    It 'exits 77 and logs an attempt to run a regex blacklisted intent cmd'
      cmdjail() { 
        echo "- r'^cat /tmp/some.file$" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'cat /tmp/some.file'; 
      }
      When run cmdjail
      The status should equal 77
      The contents of file "/tmp/cmdjail.log" should include "[warn] blocked blacklisted intent cmd: cat /tmp/some.file"
    End
    It 'exits 77 and logs an attempt to run a external command blacklisted intent cmd'
      cmdjail() { 
        echo "- grep 'cat'" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log -- 'cat /tmp/some.file'; 
      }
      When run cmdjail
      The status should equal 77
      The contents of file "/tmp/cmdjail.log" should include "[warn] blocked blacklisted intent cmd: cat /tmp/some.file"
    End
    It 'exits 0 when ls cmd is not-blacklisted'
      cmdjail() { 
        echo "- 'cat" > bin/.cmd.jail
        bin/cmdjail -- 'ls -al'; 
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "total"
    End
  End

  Describe 'record mode'
    It 'runs the command and records it to a file'
      cmdjail() {
        bin/cmdjail --record-file /tmp/test.jail -- 'echo "hello record"'
      }
      When run cmdjail
      The status should equal 0
      The stdout should equal "hello record"
      The stderr should include "[warn] cmdjail single-command record mode, recording to: /tmp/test.jail" 
      The contents of file "/tmp/test.jail" should equal "+ 'echo \"hello record\""
    End

    It 'ignores existing jail rules'
      cmdjail() {
        echo "- 'echo \"blocked\"" > bin/.cmd.jail
        bin/cmdjail --record-file /tmp/test.jail -- 'echo "blocked"'
      }
      When run cmdjail
      The status should equal 0
      The stdout should equal "blocked"
      The stderr should include "[warn] cmdjail single-command record mode, recording to: /tmp/test.jail" 
      The contents of file "/tmp/test.jail" should equal "+ 'echo \"blocked\""
    End

    It 'records failing commands and preserves their exit code'
      cmdjail() {
        bin/cmdjail --record-file /tmp/test.jail -- 'cat no-such-file'
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "[warn] cmdjail single-command record mode, recording to: /tmp/test.jail" 
      The stderr should include "cat: no-such-file: No such file or directory"
      The contents of file "/tmp/test.jail" should equal "+ 'cat no-such-file"
    End
  End

  Describe 'shell mode'
    It 'runs allowed commands and blocks disallowed ones'
      cmdjail() {
        echo "+ 'echo hello shell" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log
      }
      Data
      #|echo hello shell
      #|ls -l
      #|exit
      End

      When run cmdjail
      The status should equal 0
      The stdout should include "hello shell"
      The contents of file "/tmp/cmdjail.log" should include "[warn] implicitly blocked intent cmd: ls -l"
    End

    It 'doesnt exit when user runs quit if not allowed'
      cmdjail() {
        echo "+ 'echo hello shell" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log
      }
      Data
      #|quit
      End

      When run cmdjail
      The stdout should include "cmdjail> "
      The contents of file "/tmp/cmdjail.log" should include "[warn] implicitly blocked intent cmd: quit"
      The status should equal 0
    End

    It 'exits when the user runs exit and its allowed'
      cmdjail() {
        echo "+ 'exit" > bin/.cmd.jail
        bin/cmdjail -l /tmp/cmdjail.log
      }
      Data
      #|quit
      End

      When run cmdjail
      The stdout should include "cmdjail> "
      The contents of file "/tmp/cmdjail.log" should not include "[warn] implicitly blocked intent cmd: exit"
      The status should equal 0
    End

    It 'records all commands when --record-file is used in shell mode'
      cmdjail() {
        # No .cmd.jail file is needed in record mode
        bin/cmdjail --record-file /tmp/test.jail
      }
      Data
      #|echo first command
      #|echo second command
      #|exit
      End

      When run cmdjail
      The status should equal 0
      The stdout should include "first command"
      The stdout should include "second command"
      The stderr should include "[warn] cmdjail shell mode recording to: /tmp/test.jail"
      The contents of file "/tmp/test.jail" should eq "+ 'echo first command
+ 'echo second command
+ 'exit"
    End

    It 'records failing commands in shell record mode'
      cmdjail() {
        # No .cmd.jail file is needed in record mode
        bin/cmdjail --record-file /tmp/test.jail
      }
      Data
      #|cat no-such-file
      #|exit
      End

      When run cmdjail
      The status should equal 0
      The stdout should include "cmdjail>"
      The stderr should include "cat: no-such-file: No such file or directory"
      The contents of file "/tmp/test.jail" should eq "+ 'cat no-such-file
+ 'exit"
    End
  End

  Describe 'check mode'
    It 'validates jailfile syntax and exits 0'
      cmdjail() {
        echo "+ 'ls -l" > bin/.cmd.jail
        bin/cmdjail --check -j bin/.cmd.jail
      }
      When run cmdjail
      The status should equal 0
      The line 1 of stdout should equal "Jail file 'bin/.cmd.jail' syntax is valid."
      The line 2 of stdout should equal "No commands provided to check. Exiting."
    End

    It 'validates jailfile and exits 1 on syntax error'
      cmdjail() {
        echo "invalid rule" > bin/.cmd.jail
        bin/cmdjail --check
      }
      When run cmdjail
      The status should equal 1
      The stderr should include "parsing jail file"
    End

    It 'checks a single allowed command and exits 0'
      cmdjail() {
        echo "+ 'ls -l" > bin/.cmd.jail
        bin/cmdjail --check -- 'ls -l'
      }
      When run cmdjail
      The status should equal 0
      The stdout should include "[ALLOWED] 'ls -l'"
      The stdout should include "Check complete. 0/1 commands would be blocked."
    End

    It 'checks a single blocked command and exits 1'
      cmdjail() {
        echo "+ 'ls -l" > bin/.cmd.jail
        bin/cmdjail --check -- 'rm -rf /'
      }
      When run cmdjail
      The status should equal 1
      The stdout should include "[BLOCKED] 'rm -rf /'"
      The stdout should include "Check complete. 1/1 commands would be blocked."
    End

    It 'checks a file of commands and exits 1'
      cmdjail() {
        echo "+ 'ls -l" > bin/.cmd.jail
        echo "whoami" > /tmp/test-cmds.txt
        echo "ls -l" >> /tmp/test-cmds.txt
        bin/cmdjail --check-intent-cmds /tmp/test-cmds.txt
      }
      When run cmdjail
      The status should equal 1
      The stdout should include "[BLOCKED] 'whoami'"
      The stdout should include "[ALLOWED] 'ls -l'"
      The stdout should include "Check complete. 1/2 commands would be blocked."
    End

    It 'checks commands from stdin and exits 1'
      cmdjail() {
        echo "+ 'ls -l" > bin/.cmd.jail
        bin/cmdjail --check-intent-cmds -
      }
      Data
      #|whoami
      #|ls -l
      End
      When run cmdjail
      The status should equal 1
      The stdout should include "[BLOCKED] 'whoami'"
      The stdout should include "[ALLOWED] 'ls -l'"
      The stdout should include "Check complete. 1/2 commands would be blocked."
    End
  End
End
