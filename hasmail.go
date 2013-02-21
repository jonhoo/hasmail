// Welcome!
// main() is at the bottom.

package main

import (
	"fmt"
	"os"
	// for parsing mail
	"bytes"
	"net/mail"
	// for signals
	"syscall"
	"os/signal"
	// http://godoc.org/code.google.com/p/go-imap/go1/imap
	"code.google.com/p/go-imap/go1/imap"
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
	// for logout delay
	"time"
)

// unseen is a map from account (as defined in the config) to the UIDs of unseen
// messages for that account
var unseen = map[string] []uint32 {}

// errs is a map from account (as defined in the config) to an error number for
// that account's connection. 0 = no error
var errs = map[string] int {}

// updateTray is called whenever a client detects that the number of unseen
// messages *may* have changed. It will search the selected folder for unseen
// messages, count them and store the result. Then it will use the notify
// channel to let our main process update the status icon.
func updateTray(c *imap.Client, notify chan bool, name string) {
	cmd, _ := c.Search("UNSEEN")

	if _,ok := unseen[name]; !ok {
		unseen[name] = []uint32 {}
	}

	unseenMessages := []uint32 {}
	for cmd.InProgress() {
		// Wait for the next response (no timeout)
		c.Recv(-1)

		// Process command data
		for _, rsp := range cmd.Data {
			result := rsp.SearchResults()
			unseenMessages = append(unseenMessages, result...)
		}

		// Reset for next run
		cmd.Data = nil
		c.Data = nil
	}

	fmt.Printf("%d unseen\n", len(unseenMessages))

	// Find messages that the user hasn't been notified of
	// TODO: Make this optional/configurable
	newUnseen, _ := imap.NewSeqSet("")
	numNewUnseen := 0
	for _, uid := range unseenMessages {
		seen := false
		for _, olduid := range unseen[name] {
			if olduid == uid {
				seen = true
				break
			}
		}

		if !seen {
			newUnseen.AddNum(uid)
			numNewUnseen++
		}
	}

	// If we do have new unseen messages, fetch and display them
	if numNewUnseen > 0 {
		messages := make([]string, numNewUnseen)
		i := 0

		// Fetch headers...
		cmd, _ = c.Fetch(newUnseen, "RFC822.HEADER")
		for cmd.InProgress() {
			c.Recv(-1)
			for _, rsp := range cmd.Data {
					header := imap.AsBytes(rsp.MessageInfo().Attrs["RFC822.HEADER"])
					if msg, _ := mail.ReadMessage(bytes.NewReader(header)); msg != nil {
						messages[i] = msg.Header.Get("Subject")
						i++
					}
			}

			cmd.Data = nil
			c.Data = nil
		}

		// Print them in reverse order to get newest first
		notification := ""
		for ; i > 0; i-- {
			notification += "> " + messages[i-1] + "\n"
		}
		notification = strings.TrimRight(notification, "\n")
		fmt.Println(notification)

		// And send them with notify-send!
		title := fmt.Sprintf("%s has new mail (%d unseen)", name, len(unseenMessages))
		sh := exec.Command("/usr/bin/notify-send",
											 "-i", "notification-message-email",
											 "-c", "email",
											 title, notification)

		err := sh.Start()
		if err != nil {
			fmt.Println("Failed to notify user...")
			fmt.Println(err)
		}
	}

	unseen[name] = unseenMessages

	// Let main process know something has changed
	notify <- true
}

// connect sets up a new IMAPS connection to the given host using the given
// credentials. The name parameter should be a human-readable name for this
// mailbox.
func connect(notify chan bool,
						 name string,
						 address string,
						 username string,
						 password string) {

	// Connect to the server
	fmt.Printf("%s: Connecting to server...\n", name)
	c, err := imap.DialTLS(address, nil)

	if err != nil {
		fmt.Printf("%s: Connection failed\n", name)
		fmt.Println(" ", err)
		errs[name] = 1
		notify <- false
		return
	}

	// Remember to log out and close the connection when finished
	defer c.Logout(30 * time.Second)

	// If IDLE isn't supported, we're not going to fall back on polling
	// Time to abandon ship!
	if !c.Caps["IDLE"] {
		fmt.Printf("%s: Server does not support IMAP IDLE, exiting...\n", name)
		errs[name] = 2
		notify <- false
		return
	}

	// Authenticate
	if c.State() == imap.Login {
		_, err = c.Login(username, password)
	} else {
		fmt.Printf("%s: No login presented, exiting...\n", name)
		errs[name] = 3
		notify <- false
		return
	}

	if err != nil {
		fmt.Printf("%s: Login failed, exiting...\n", name)
		errs[name] = 4
		notify <- false
		return
	}

	// TODO: Add support for other folders than INBOX
	c.Select("INBOX", true)

	// Get initial unread messages count
	fmt.Printf("%s: Initial inbox state: ", name)
	updateTray(c, notify, name)

	// And go to IMAP IDLE mode
	cmd, err := c.Idle()

	// Process responses while idling
	for cmd.InProgress() {
		// Wait for the next response (no timeout)
		c.Recv(-1)

		// Because the example tells us to do it...
		cmd.Data = nil

		// Process unilateral server data
		for _, _ = range c.Data {
			c.IdleTerm()
			fmt.Printf("%s inbox state changed to: ", name)
			updateTray(c, notify, name)
			cmd, _ = c.Idle()
			break
		}

		// Don't want to read the same data again next round
		c.Data = nil
	}
}

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
	gtk.Init(&os.Args);
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

		for account, unseenList := range unseen {
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
			sh := exec.Command("/bin/sh", "-c", command)
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

		hostname, _ := conf.GetString(account, "hostname")
		username, _ := conf.GetString(account, "username")
		password, _ := conf.GetString(account, "password")
		go connect(notify, account, hostname, username, password)
	}

	// Let the user know that we've now initiated all the connections
	si.SetFromStock(gtk.STOCK_CONNECT)
	si.SetTooltipText("Connecting")

	// Keep updating the status icon (or quit if the user wants us to)
	for {
		select {
		case <- quit:
			return
		case <-notify:
			totUnseen := 0
			s := ""
			for account, e := range errs {
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
				}

				s += "\n"
			}

			for account, unseenList := range unseen {
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
