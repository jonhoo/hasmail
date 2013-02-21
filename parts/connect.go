package parts

import (
	"fmt"
	// http://godoc.org/code.google.com/p/go-imap/go1/imap
	"code.google.com/p/go-imap/go1/imap"
	// for logout delay
	"time"
)

// errs is a map from account (as defined in the config) to an error number for
// that account's connection. 0 = no error
var Errs = map[string] int {}

// connect sets up a new IMAPS connection to the given host using the given
// credentials. The name parameter should be a human-readable name for this
// mailbox.
func Connect(notify chan bool,
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
		Errs[name] = 1
		notify <- false
		return
	}

	// Remember to log out and close the connection when finished
	defer c.Logout(30 * time.Second)

	// If IDLE isn't supported, we're not going to fall back on polling
	// Time to abandon ship!
	if !c.Caps["IDLE"] {
		fmt.Printf("%s: Server does not support IMAP IDLE, exiting...\n", name)
		Errs[name] = 2
		notify <- false
		return
	}

	// Authenticate
	if c.State() == imap.Login {
		_, err = c.Login(username, password)
	} else {
		fmt.Printf("%s: No login presented, exiting...\n", name)
		Errs[name] = 3
		notify <- false
		return
	}

	if err != nil {
		fmt.Printf("%s: Login failed, exiting...\n", name)
		Errs[name] = 4
		notify <- false
		return
	}

	// TODO: Add support for other folders than INBOX
	c.Select("INBOX", true)

	// Get initial unread messages count
	fmt.Printf("%s: Initial inbox state: ", name)
	UpdateTray(c, notify, name)

	// And go to IMAP IDLE mode
	cmd, err := c.Idle()

	// Process responses while idling
	for cmd.InProgress() {
		// Wait for server messages
		// Refresh every 29 minutes to avoid disconnection (see spec)
		c.Recv(29 * time.Minute)

		// We don't really care about the data, just that there *is* data
		cmd.Data = nil
		c.Data = nil

		// Update our view of the inbox
		c.IdleTerm()
		fmt.Printf("%s inbox state: ", name)
		UpdateTray(c, notify, name)
		cmd, _ = c.Idle()
	}
}
