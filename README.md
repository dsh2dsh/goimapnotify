goimapnotify
============

Execute scripts on IMAP mailbox changes (new/deleted/updated messages) using
IDLE, Golang version.

This project is a fork of
[goimapnotify](https://gitlab.com/shackra/goimapnotify).

Current FreeBSD port is
[here](https://github.com/dsh2dsh/freebsd-ports/tree/master/mail/goimapnotify).
Keep in mind, sometimes it is a development version of this project, for
testing.

## Changes from [upstream](https://gitlab.com/shackra/goimapnotify):

* Delete weird undocumented network change detector

  It was added by upstream in commit 6264fa1b0471fc50a1f19567e24b97b8b090e591
  and enabled by default. It really looks like a DDOS. By default it connects to
  `archlinux.org:80` and `ubuntu.com:80` every 30 seconds.

* Support only YAML configuration file

  Support of `JSON` or `TOML` removed. Default configuration file is
  `$XDG_CONFIG_HOME/goimapnotify/goimapnotify.yaml`.

* Option to disable startup sync of every mailbox

  By default it runs `onNewMail` command for every mailbox. With top level
  option

  ```yaml
  startupSync: false # don't run onNewMail right after connection
  ```

  it skipped.

* Configurable max delay

  Every next IDLE event delays execution of commands for this mailbox. With
  stream of many events this delay is increasing and increasing, until it
  reaches `maxDelay`, which is 5 minutes by default. After that all commands
  executed. The limit can be configured by top level option

  ```yaml
  maxDelay: "10m"
  ```

  Here we changed it from default 5 minutes to 10 minutes.

* Desktop notifications

  Output of executed commands can be sent as desktop notifications. It can be
  configured by top level option:

  ```yaml
  desktopNotify:
    enable: true
    appName: "goimapnotify" # default: goimapnotify
    appIcon: "mail-unread"
    category: "email.arrived"
    desktopEntry: "firefox"
    actionTimeout: "10s"    # default: 10s
  ```

  Every option is optional.

  `desktopEntry` defines a `.desktop` file, associated with notification.
  Notification will have an icon defined in that file and will be stored in
  notification history. Without `desktopEntry` it will not be stored in
  notification history after disappearing.

  `actionTimeout` defines how long to wait before kill action's command.

  Any mailbox can add actions to its notifications for executing commands.
  Example:

  ```yaml
  configurations:
    - host: "imap.fastmail.com"
      boxes:
        - mailbox: "INBOX"
          notificationActions:
            - key: "default"
              label: "View"
              exec: [ "xdg-open", "https://app.fastmail.com/mail/Inbox/" ]
              closeAll: false
              closeSame: false
              close: false
  ```

  With this configuration it runs `xdg-open` on click, because `default` is a
  magic key for defining on click actions. Anything else can be used as value of
  `key` and it must be uniq. As an example one possible rendering of actions
  would be as buttons in the notification popup.

  `closeAll` configures goimapnotify to close all notications on click.
  `closeSame` - closes all notifications about this mailbox. `close` closes just
  this notification. All 3 options are `false` by default, but an action with
  `key: default` may be auto closed by DE. At least KDE closes it on click.

  Test notification can be sent using `goimapotify test-notify`.

* All excutable commands now configured using YAML lists, instead of strings

  So instead of

  ```yaml
  onNewMail: "mbsync examplecom:INBOX"
  ```

  it should be

  ```yaml
  onNewMail: [ "mbsync", "examplecom:INBOX" ]
  ```

  In this case `mbsync` executed directly, without shell.

  > ⚠️ For compatibility old way still works, but it isn't safe, because
  > everything is executed through a shell and template variables are not shell
  > escaped.

## Configuration

This application is mostly compatible with the configuration of
[imapnotify made with Python](https://github.com/a-sk/python-imapnotify)
(be sure to change `password_eval` to `passwordCMD`, see
[issue #3](https://gitlab.com/shackra/goimapnotify/issues/3)),
the following are all options available for the configuration:

```yaml
configurations:
  - host: example.com
    port: 143
    tls: true
    tlsOptions:
      rejectUnauthorized: false
      starttls: true
    idleLogoutTimeout: 15
    username: USERNAME
    alias: ExampleCOM
    password: PASSWORD
    xoAuth2: false
    boxes:
      - mailbox: INBOX
        onNewMail: [ "mbsync", "examplecom:INBOX" ]
        onChangedMail: [ "mbsync", "examplenet:INBOX" ]
        onChangedMailPost: [ "SKIP" ]
        onNewMailPost: [ "SKIP" ]

  - hostCMD: [ "COMMAND_TO_RETRIEVE_HOST", "args" ]
    port: 993
    tls: true
    tlsOptions:
      rejectUnauthorized: true
      starttls: true
    username: ''
    usernameCMD: []
    password: ''
    passwordCMD: []
    xoAuth2: false
    onNewMail: []
    onNewMailPost: []
    onChangedMail: []
    onChangedMailPost: []
    onDeletedMail: []
    onDeletedMailPost: []
    boxes:
      - mailbox: INBOX
        onNewMail: [ "mbsync", "examplenet:INBOX" ]
        onNewMailPost: [ "SKIP" ]
        onChangedMail: [ "mbsync", "examplenet:INBOX" ]

      - mailbox: Junk
        onNewMail: [ "mbsync", "examplenet:Junk" ]
        onNewMailPost: [ "SKIP" ]
```

On first start, the application will run `onNewMail` and then wait for events
from your IMAP server.

- `onNewMail`: is an executable or script to run when new mail has arrived.

- `onNewMailPost`: is an executable or script to run after `onNewMail` has ran.

- `onChangedMail`: is an executable or script to run when a flag changed on an
  email (Seen, Flagged, ...).

- `onChangedMailPost`: is an executable or script to run after `onChangedMail`
  has ran.

- `onDeletedMail`: is an executable or script to run when mail has been delete.

- `onDeletedMailPost`: is an executable or script to run after `onDeletedMail`
  has ran.

- `hostCMD`: is an executable or script that retrieves your host from somewhere,
  we cannot pass arguments to this command from `Stdin`.

- `usernameCMD`: is an executable or script that retrieves your username from
  somewhere, we cannot pass arguments to this command from `Stdin`.

- `passwordCMD`: is an executable or script that retrieves your password from
  somewhere, we cannot pass arguments to this command from `Stdin`.

  > ⚠️ **Security**: Avoid embedding secrets literally in the command - use an
  > external secret manager (e.g., `pass`, `gopass`, `oauth2l`) so credentials
  > are not visible in the process list (`/proc/PID/cmdline`).

- `xoAuth2`: is an option that allow us to login on your IMAP using OAuth2, **be
  aware**: the token is retrieve from `passwordCMD` (see
  shackra/goimapnotify#9).

- `wait`: is the delay in seconds before the mail syncing is trigger (see
  shackra/goimapnotify#10).

- `boxes`: List of mailboxes. If none is defined, all will be monitored.

- `idleLogoutTimeout`: Change the time between restarts of the IDLE command (see
  shackra/goimapnotify#49)

- `enableIDCommand`: Tell goimapotify that your server needs (and supports!) the
  ID command (see shackra/goimapnotify#58 shackra/goimapnotify#57; the servers
  in those tickets did not support ID and they responded with a non-standard
  error message, causing goimapnotify to fail)

The application will use TLS as long as the IMAP server advertises this
capability. **Certificate verification is enabled by default** - set
`rejectUnauthorized` to `false` only if you must connect to a server with a
self-signed or untrusted certificate.

To enable TLS connection, set `tls` as `true` and `starttls` as `false`

If your host do not offer IDLE, a sane default of checking every 15 minutes will
take place instead.

You can also use xoAuth2 instead of password based authentication by setting the
`xoAuth2` option to `true` and the output of a tool which can provide xoAuth2
encoded tokens in `passwordCMD`. Examples:

- [Google oauth2l](https://github.com/google/oauth2l)

- [xoauth2 fetcher for O365](https://github.com/harishkrupo/oauth2ms).

## Install

```
go install github.com/dsh2dsh/goimapnotify@latest
```

## Usage

```
goimapnotify executes scripts on IMAP mailbox changes (new/deleted/updated messages) using IDLE.

Usage:
  goimapnotify [flags]
  goimapnotify [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  list        List all mailboxes and exit
  test-notify Show test desktop notification

Flags:
  -c, --conf string               Configuration file (default "/home/dsh/.config/goimapnotify/goimapnotify.yaml")
  -r, --dial-retry-attempts int   number of attempts when connecting to an IMAP server, using exponential backoff (default 5)
  -h, --help                      help for goimapnotify
      --list                      List all mailboxes and exit
  -l, --log-level string          change the logging level (error|warn|info|debug) (default "info")
  -s, --syslog                    send log output to syslog instead of stderr
  -w, --wait int                  delay in seconds between the IDLE event and the execution of the scripts (default 1)

Use "goimapnotify [command] --help" for more information about a command.
```

As you can notice, `list` can help you figure out the mailbox hierarchy of your
mail server.
