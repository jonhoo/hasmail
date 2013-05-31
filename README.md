# Introduction #

Using mutt (or pine), but annoyed that it doesn't give you any notifications
when you've received new emails? hasmail is a simple tray application that
detects new emails on IMAP servers using IDLE (push rather than pull). When it
detects unseen messages, it shows a OSD style notification and changes the tray
icon to indicate that you have new mail.

When hovering the mouse over the tray icon, hasmail shows which of the
configured accounts have unseen messages. Clicking the icon executes a
user-defined command.

# Configuration #

hasmail looks for `~/.hasmailrc` and expects a file in simple INI syntax. The
configuration file consists of (currently) one global option and one or more
accounts, each with multiple fields.

## Global fields ##

 - click: this command will be executed when clicking the tray icon

## Account fields ##

The value in [] can be anything, and will be what is shown in the tooltip. The
options for an account are as follows:

 - hostname: The address (+ port) to connect to. MUST currently be SSL/TLS
   enabled. Required.
 - username: Username for authentication. Required.
 - password: Command to execute to get password for authentication. Required.
 - click: An optional command to execute when clicking the tray icon if this,
   and only this, account has new messages. If this option is present, it
   overrides the global click option in the described scenario.
 - folder: What folder to check for new messages. INBOX is the default.
 - poll: How often (in minutes) to force a recheck of the account. By default it
	 is set to 29 (as suggested in the spec) to avoid the server disconnecting us
	 due to inactivity.

# Installation #

Assuming you've checked this out into `$GOPATH/src/hasmail`, just run
`go install hasmail`. If your path is set up correctly, you should be able to
run `hasmail` and get the message "Failed to load configuration file,
exiting...". Create your config and you're ready!

If you're lucky enough to be running [Arch Linux](https://www.archlinux.org/),
you can just install the package from [the
AUR](https://aur.archlinux.org/packages/hasmail/) or run `makepkg` in `pkg/`.

# Major dependencies #

 - https://github.com/mattn/go-gtk/
 - http://code.google.com/p/go-imap/
 - https://github.com/dlintw/goconf/

# License #

Use this code for whatever you want if you find it useful. You don't need to put
any attribution (even though I'd be happy if you did), but please let me know if
you use it for anything interesting!
