package parts

import (
	"fmt"
	"time"
	// http://godoc.org/code.google.com/p/go-imap/go1/imap
	"code.google.com/p/go-imap/go1/imap"
)

// errs is a map from account (as defined in the config) to an error number for
// that account's connection. 0 = no error
var Errs = map[string]int{}

// connect sets up a new IMAPS connection to the given host using the given
// credentials. The name parameter should be a human-readable name for this
// mailbox.
func Connect(notify chan bool,
	name string,
	address string,
	username string,
	password string,
	folder string,
	poll time.Duration) {

	// Connect to the server
	fmt.Printf("%s: connecting to server...\n", name)
	c, err := imap.DialTLS(address, nil)

	if err != nil {
		fmt.Printf("%s: connection failed, retrying in 3 minutes\n", name)
		fmt.Println(" ", err)
		Errs[name] = 1
		notify <- false
		time.Sleep(3 * time.Minute)
		go Connect(notify, name, address, username, password, folder, poll)
		return
	}

	// Connected successfully!
	Errs[name] = 0

	// Remember to log out and close the connection when finished
	defer c.Logout(30 * time.Second)

	// If IDLE isn't supported, we're not going to fall back on polling
	// Time to abandon ship!
	if !c.Caps["IDLE"] {
		fmt.Printf("%s: server does not support IMAP IDLE, exiting...\n", name)
		Errs[name] = 2
		notify <- false
		return
	}

	// Authenticate
	if c.State() == imap.Login {
		_, err = c.Login(username, password)
	} else {
		fmt.Printf("%s: no login presented, exiting...\n", name)
		Errs[name] = 3
		notify <- false
		return
	}

	if err != nil {
		fmt.Printf("%s: login failed, exiting...\n", name)
		Errs[name] = 4
		notify <- false
		return
	}

	c.Select(folder, true)

	// Get initial unread messages count
	fmt.Printf("%s initial state: ", name)
	UpdateTray(c, notify, name)

	// And go to IMAP IDLE mode
	cmd, err := c.Idle()

	if err != nil {
		// Fast reconnect
		fmt.Printf("%s: connection failed, reconnecting...\n", name)
		fmt.Println("  ", err)
		Errs[name] = 5
		go Connect(notify, name, address, username, password, folder, poll)
		return
	}

	// Process responses while idling
	for cmd.InProgress() {
		// Wait for server messages
		// Refresh every `poll` to avoid disconnection
		// Defaults to 29 minutes (see spec)
		c.Recv(poll)

		// We don't really care about the data, just that there *is* data
		cmd.Data = nil
		c.Data = nil

		// Update our view of the inbox
		c.IdleTerm()
		fmt.Printf("%s state: ", name)
		UpdateTray(c, notify, name)
		cmd, err = c.Idle()

		if err != nil {
			fmt.Printf("%s: connection failed, reconnecting...\n", name)
			fmt.Println("  ", err)
			Errs[name] = 5
			go Connect(notify, name, address, username, password, folder, poll)
			return
		}
	}
}
