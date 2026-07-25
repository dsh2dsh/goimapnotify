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

This application is mostly compatible with the configuration of [imapnotify made
with Python](https://github.com/a-sk/python-imapnotify) (be sure to change
`password_eval` to `passwordCMD`, see [issue
#3](https://gitlab.com/shackra/goimapnotify/issues/3)), the following are all
options available for the configuration:

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
        onNewMail: 'mbsync examplecom:INBOX'
        onChangedMail: 'mbsync examplenet:INBOX'
        onChangedMailPost: SKIP
        onNewMailPost: SKIP

  - hostCMD: COMMAND_TO_RETRIEVE_HOST
    port: 993
    tls: true
    tlsOptions:
      rejectUnauthorized: true
      starttls: true
    username: ''
    usernameCMD: ''
    password: ''
    passwordCMD: ''
    xoAuth2: false
    onNewMail: ''
    onNewMailPost: ''
    onChangedMail: ''
    onChangedMailPost: ''
    onDeletedMail: ''
    onDeletedMailPost: ''
    boxes:
      - mailbox: INBOX
        onNewMail: 'mbsync examplenet:INBOX'
        onNewMailPost: SKIP
        onChangedMail: 'mbsync examplenet:INBOX'

      - mailbox: Junk
        onNewMail: 'mbsync examplenet:Junk'
        onNewMailPost: SKIP
```

On first start, the application will run `onNewMail` and `onNewMailPost` and
then wait for events from your IMAP server.

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
capability. If you use self-signed certificates or something, be sure to set
`rejectUnauthorized` as `false`. To enable TLS connection, set `tls` as `true`
and `starttls` as `false`

If your host do not offer IDLE, a sane default of checking every 15 minutes will
take place instead.

You can also use xoAuth2 instead of password based authentication by setting the
`xoAuth2` option to `true` and the output of a tool which can provide xoAuth2
encoded tokens in `passwordCMD`. Examples:

- [Google oauth2l](https://github.com/google/oauth2l)

- [xoauth2 fetcher for O365](https://github.com/harishkrupo/oauth2ms).

## Install

```
go install github.com/dsh2dsh/goimapnotify/cmd/goimapnotify@latest
```

## Usage

```
Usage of goimapnotify:
  -conf string
        Configuration file, supported formats: json, yaml/yml, toml (default "$XDG_CONFIG_HOME/goimapnotify/goimapnotify.yaml")
  -dial-retry-attempts int
        Number of attempts when connecting to an IMAP server, using exponential backoff (default 5)
  -list
        List all mailboxes and exit
  -log-level string
        Change the logging level; possible values are: error, warn, info, debug (default "info")
  -wait int
        Delay in seconds between the IDLE event and the execution of the scripts (default 1)
```

As you can notice, `-list` can help you figure out the mailbox hierarchy of your
mail server.
