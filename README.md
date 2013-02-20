# Introduction #

Simple tray icon for detecting new email on IMAP servers using IDLE for instant
notifications.

Gives you a system tray icon showing whether you have new messages on any of the
configured IMAP accounts. When run, it will open connections to all IMAP servers
specified in the configuration file (details below) and set up an IDLE
connection to get immediately notified of changes on the server.

The system tray icon switches between three main icons:

 - an empty document, signifying no new emails
 - a single document, signifying a single new email across all accounts
 - a pile of documents, signifying multiple new emails across all accounts

When hovering the mouse over the tray icon, a tooltip will be shown which
displays the number of new messages for each configured account. Clicking the
icon executes a user-defined command.

# Configuration #

hasmail looks in `~/.config/hasmail` and expects a file in simple INI syntax.
The configuration file consists of (currently) one global option and one or more
accounts, each with multiple fields.

## Global fields ##

 - click: this command will be executed when clicking the tray icon

## Account fields ##

The value in [] can be anything, and will be what is shown in the tooltip. The
options for an account are as follows:

 - hostname: The address (+ port) to connect to. MUST currently be SSL/TLS
   enabled. Required.
 - username: Username for authentication. Required.
 - password: Password for authentication. Required.
 - click: An optional command to execute when clicking the tray icon if this,
   and only this, account has new messages. If this option is present, it
   overrides the global click option in the described scenario.

# Installation #

Assuming you've checked this out into `$GOPATH/src/hasmail`, just run
`go install hasmail`. If your path is set up correctly, you should be able to
run `hasmail` and get the message "Failed to load configuration file,
exiting...". Create your config and you're ready!

# Major dependencies #

 - https://github.com/mattn/go-gtk/
 - http://code.google.com/p/go-imap/
 - https://github.com/dlintw/goconf/

# License #

Use this code for whatever you want if you find it useful. You don't need to put
any attribution (even though I'd be happy if you did), but please let me know if
you use it for anything interesting!
