package parts

import (
	"fmt"
	// for parsing mail
	"bytes"
	"net/mail"
	// for notify-send
	"os/exec"
	// for strings.TrimRight
	"strings"

	"github.com/mxk/go-imap/imap"
)

// unseen is a map from account (as defined in the config) to the UIDs of unseen
// messages for that account
var Unseen = map[string][]uint32{}

// updateTray is called whenever a client detects that the number of unseen
// messages *may* have changed. It will search the selected folder for unseen
// messages, count them and store the result. Then it will use the notify
// channel to let our main process update the status icon.
func UpdateTray(c *imap.Client, notify chan bool, name string) {
	// Send > Search since Search adds CHARSET UTF-8 which might not be supported
	cmd, err := c.Send("SEARCH", "UNSEEN")

	if err != nil {
		fmt.Printf("%s failed to look for new messages\n", name)
		fmt.Println("  ", err)
		return
	}

	if _, ok := Unseen[name]; !ok {
		Unseen[name] = []uint32{}
	}

	unseenMessages := []uint32{}
	for cmd.InProgress() {
		// Wait for the next response (no timeout)
		err = c.Recv(-1)

		// Process command data
		for _, rsp := range cmd.Data {
			result := rsp.SearchResults()
			unseenMessages = append(unseenMessages, result...)
		}

		// Reset for next run
		cmd.Data = nil
		c.Data = nil
	}

	// Check command completion status
	if rsp, err := cmd.Result(imap.OK); err != nil {
		if err == imap.ErrAborted {
			fmt.Println("fetch command aborted")
		} else {
			fmt.Println("fetch error:", rsp.Info)
		}

		return
	}

	fmt.Printf("%d unseen\n", len(unseenMessages))

	// Find messages that the user hasn't been notified of
	// TODO: Make this optional/configurable
	newUnseen, _ := imap.NewSeqSet("")
	numNewUnseen := 0
	for _, uid := range unseenMessages {
		seen := false
		for _, olduid := range Unseen[name] {
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

	Unseen[name] = unseenMessages

	// Let main process know something has changed
	notify <- true
}
