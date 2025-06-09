# Rsync Time Backup with cmdjail

This example demonstrates how to use `cmdjail` to securely allow remote rsync backups via SSH while strictly limiting the commands that can be executed.

> **⚠️ Security Warning**
> Note this example is illustrative, the rules in the jail file may not all be present and they may allow an attacker to wipe prior backups and fill up the disk, amongst other potential issues. `cmdjail` should be another layer in your security, not the sole provider of it.

## Overview

The configuration in this directory enables secure remote backups using the [rsync-time-backup](https://github.com/laurent22/rsync-time-backup) script (or similar time-based backup solutions) by:

1. Restricting SSH access to only run specific commands from the script
2. Limiting file operations to a specific backup directory
3. Preventing any command execution outside the defined ruleset

## Files

- `authorized_keys` - Example SSH authorized_keys entry that forces all SSH connections to use cmdjail
- `rsync-time-backup.jail` - Jail file containing the allowed commands for the backup process

## Security Considerations

- The jail file only allows specific commands needed for rsync backups script
- All commands are restricted to operate within the specified backup directory
- Regular expressions are carefully crafted to prevent command injection
- Additional precautions like `chroot` should be taken!

## Testing

You can test your configuration using cmdjail's check mode:

```bash
cmdjail --check -j /etc/cmdjail/rsync-time-backup.jail -- 'find /your/backup/path/backup.marker'
```
