Describe 'cmdjail.sh'
  build() {
    make -s bin/cmdjail
    rm -f bin/.cmd.jail
  }
  cleanup() { 
    rm -f bin/.cmd.jail
    rm -f .some.log
    rm -f /tmp/cmdjail.log
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

  Describe 'exits 1 when intent cmd is empty'
    It 'command options'
      cmdjail() { 
        bin/cmdjail -l /tmp/cmdjail.log --; 
      }
      When run cmdjail
      The status should equal 1
      The line 1 of stderr should equal "[error] no intent cmd provided"
      The contents of file "/tmp/cmdjail.log" should include "[error] no intent cmd provided"
    End
    It 'env var'
      cmdjail() { 
        CMDJAIL_CMD="" bin/cmdjail -l /tmp/cmdjail.log;
      }
      When run cmdjail
      The status should equal 1
      The line 1 of stderr should equal "[error] no intent cmd provided"
      The contents of file "/tmp/cmdjail.log" should include "[error] no intent cmd provided"
    End
    It 'env var reference'
      cmdjail() { 
        CMD="" CMDJAIL_ENV_REFERENCE="CMD" bin/cmdjail -l /tmp/cmdjail.log;
      }
      When run cmdjail
      The status should equal 1
      The line 1 of stderr should equal "[error] no intent cmd provided"
      The contents of file "/tmp/cmdjail.log" should include "[error] no intent cmd provided"
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
End
