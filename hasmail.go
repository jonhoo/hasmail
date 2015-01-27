// Welcome!
// main() is at the bottom.

package main

import (
	"fmt"
	"os"
	"time"

	"hasmail/parts"
	// for signals
	"os/signal"
	"syscall"
	// for configuration
	"github.com/dlintw/goconf"
	// for tray icon
	// https://github.com/mattn/go-gtk/blob/master/example/statusicon/statusicon.go
	"github.com/mattn/go-gtk/glib"
	"github.com/mattn/go-gtk/gtk"
	// for click command
	"os/exec"
	// for strings.TrimRight
	"strings"
)

// main reads the config file, connects to each IMAP server in a separate thread
// and watches the notify channel for messages telling it to update the status
// icon.
func main() {
	conf, err := goconf.ReadConfigFile(os.Getenv("HOME") + "/.hasmailrc")
	if err != nil {
		fmt.Println("Failed to load configuration file, exiting...\n", err)
		return
	}

	// This channel will be used to tell us if we should exit the program
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)

	// GTK engage!
	gtk.Init(&os.Args)
	glib.SetApplicationName("hasmail")
	defer gtk.MainQuit()

	// Set up the tray icon
	si := gtk.NewStatusIconFromStock(gtk.STOCK_DISCONNECT)
	si.SetTitle("hasmail")
	si.SetTooltipMarkup("Not connected")

	// Set up the tray icon right click menu
	mi := gtk.NewMenuItemWithLabel("Quit")
	mi.Connect("activate", func() {
		quit <- syscall.SIGINT
	})
	nm := gtk.NewMenu()
	nm.Append(mi)
	nm.ShowAll()
	si.Connect("popup-menu", func(cbx *glib.CallbackContext) {
		nm.Popup(nil, nil, gtk.StatusIconPositionMenu, si, uint(cbx.Args(0)), uint32(cbx.Args(1)))
	})

	/* If the user clicks the tray icon, here's what happens:
	 *   - if only a single account has new emails, and a click command has been
	 *     specified for that account, execute it
	 *   - if the user has specified a default click handler, execute that
	 *   - otherwise, do nothing
	 */
	si.Connect("activate", func(cbx *glib.CallbackContext) {
		command, geterr := conf.GetString("default", "click")
		nonZero := 0
		nonZeroAccount := ""

		for account, unseenList := range parts.Unseen {
			if len(unseenList) > 0 {
				nonZero++
				nonZeroAccount = account
			}
		}

		// Can't just use HasOption here because that also checks "default" section,
		// even though Get* doesn't...
		if nonZero == 1 {
			acommand, ageterr := conf.GetString(nonZeroAccount, "click")
			if ageterr == nil {
				command = acommand
				geterr = ageterr
			}
		}

		if geterr == nil {
			fmt.Printf("Executing user command: %s\n", command)
			shell := os.Getenv("SHELL")
			if shell == "" {
				shell = "/bin/sh"
			}

			sh := exec.Command(shell, "-c", command)
			sh.Env = os.Environ()
			err = sh.Start()
			if err != nil {
				fmt.Printf("Failed to run command '%s' on click\n", command)
				fmt.Println(err)
			}
		} else {
			fmt.Println("No action defined for click\n", geterr)
		}
	})

	go gtk.Main()

	// Connect to all accounts
	sections := conf.GetSections()
	notify := make(chan bool, len(sections))
	for _, account := range sections {
		// default isn't really an account
		if account == "default" {
			continue
		}

		initConnection(notify, conf, account)
	}

	// Let the user know that we've now initiated all the connections
	si.SetFromStock(gtk.STOCK_CONNECT)
	si.SetTooltipText("Connecting")

	// Keep updating the status icon (or quit if the user wants us to)
	for {
		select {
		case <-quit:
			return
		case <-notify:
			totUnseen := 0
			s := ""
			for account, e := range parts.Errs {
				if e == 0 {
					continue
				}

				s += account + ": "

				switch e {
				case 1:
					s += "Connection failed!"
				case 2:
					s += "IDLE not supported!"
				case 3:
					s += "No login credentials given!"
				case 4:
					s += "Login failed!"
				case 5:
					s += "Connection dropped!"
				}

				s += "\n"
			}

			for account, unseenList := range parts.Unseen {
				if parts.Errs[account] != 0 {
					continue
				}

				numUnseen := len(unseenList)

				if numUnseen >= 0 {
					totUnseen += numUnseen
				}
				s += account + ": "

				switch numUnseen {
				case 0:
					s += "No new messages"
				case 1:
					s += "One new message"
				default:
					s += fmt.Sprintf("%d new messages", numUnseen)
				}

				s += "\n"
			}

			// get rid of trailing newline
			s = strings.TrimRight(s, "\n")
			si.SetTooltipText(s)

			// http://developer.gnome.org/gtk3/3.0/gtk3-Stock-Items.html
			switch totUnseen {
			case 0:
				si.SetFromStock(gtk.STOCK_NEW)
			case 1:
				si.SetFromStock(gtk.STOCK_DND)
			default:
				si.SetFromStock(gtk.STOCK_DND_MULTIPLE)
			}
		}
	}
}

// Initializes a new connection to the given account in a separate goroutine
func initConnection(notify chan bool, conf *goconf.ConfigFile, account string) {
	hostname, _ := conf.GetString(account, "hostname")
	username, _ := conf.GetString(account, "username")
	passexec, _ := conf.GetString(account, "password")

	pw := exec.Command("/bin/sh", "-c", passexec)
	pw.Env = os.Environ()
	pwbytes, err := pw.Output()
	if err != nil {
		fmt.Printf("%s: password command failed: %s\n", account, err)
		return
	}

	password := strings.TrimRight(string(pwbytes), "\n")
	if password == "" {
		fmt.Printf("%s: password command returned an empty string\n", account)
		return
	}

	folder, e := conf.GetString(account, "folder")
	if e != nil {
		folder = "INBOX"
	}

	poll, e := conf.GetInt(account, "poll")
	if e != nil {
		poll = 29
	}
	polli := time.Duration(poll) * time.Minute

	go parts.Connect(notify, account, hostname, username, password, folder, polli)
}
