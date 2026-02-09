package fetcher

import (
	"fmt"
	"log"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
	"github.com/jhillyerd/enmime"
)

type Email struct {
	ID      uint32
	From    string
	Subject string
	Body    string
	Sender  string
	Date    time.Time
}

func FetchHeaders(c *client.Client, count int) {
	// 1. Get the status of the INBOX (we need the total message count)
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}

	if mbox.Messages == 0 {
		fmt.Println("Inbox is empty!")
		return
	}

	// 2. Calculate the range (e.g., "Give me the last 10 emails")
	from := uint32(1)
	to := mbox.Messages
	if mbox.Messages > uint32(count) {
		from = mbox.Messages - uint32(count) + 1
	}

	// 3. Create a Sequence Set (the list of IDs to fetch)
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, to)

	// 4. Define WHAT to fetch (We want the "Envelope" = Metadata)
	// Fetching the Body is slow; Envelope is fast.
	section := imap.FetchEnvelope
	items := []imap.FetchItem{section}

	messages := make(chan *imap.Message, 10)
	done := make(chan error, 1)

	// 5. Trigger the fetch in a goroutine
	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	// 6. Loop through results and PRINT them
	fmt.Println("\n--- INBOX HEADERS ---")
	for msg := range messages {
		fmt.Printf("* [%d] %s\n", msg.SeqNum, msg.Envelope.Subject)
		fmt.Printf("  From: %v\n", msg.Envelope.From[0].Address())
		fmt.Printf("  Date: %v\n", msg.Envelope.Date)
		fmt.Println("-------------------------")
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}
}

// FetchRecentHeaders gets the metadata for the last N messages
func FetchRecentHeaders(c *client.Client, count int) ([]Email, error) {
	// 1. Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		return nil, err
	}

	if mbox.Messages == 0 {
		return []Email{}, nil
	}

	// 2. Calculate Range (Last N messages)
	from := uint32(1)
	if mbox.Messages > uint32(count) {
		from = mbox.Messages - uint32(count) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	// 3. Fetch
	messages := make(chan *imap.Message, count)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, []imap.FetchItem{imap.FetchEnvelope}, messages)
	}()

	// 4. Convert to Domain Model
	var results []Email
	for msg := range messages {
		sender := "Unknown"
		if len(msg.Envelope.From) > 0 {
			sender = msg.Envelope.From[0].Address()
		}

		results = append(results, Email{
			ID:      msg.SeqNum,
			Subject: msg.Envelope.Subject,
			Sender:  sender,
			Date:    msg.Envelope.Date,
		})
	}

	return results, <-done
}

func FetchAndPrintBodies(c *client.Client, count int) {
	// 1. Select INBOX
	mbox, err := c.Select("INBOX", false)
	if err != nil {
		log.Fatal(err)
	}

	// 2. Calculate Range
	from := uint32(1)
	if mbox.Messages > uint32(count) {
		from = mbox.Messages - uint32(count) + 1
	}
	seqset := new(imap.SeqSet)
	seqset.AddRange(from, mbox.Messages)

	// 3. Define the Body Section
	// Peek: true ensures we don't accidentally mark emails as "Read"
	section := &imap.BodySectionName{Peek: true}
	items := []imap.FetchItem{imap.FetchEnvelope, section.FetchItem()}

	messages := make(chan *imap.Message, count)
	done := make(chan error, 1)

	go func() {
		done <- c.Fetch(seqset, items, messages)
	}()

	fmt.Println("--- FETCHING & PARSING ---")

	for msg := range messages {
		fmt.Printf("\n📧 [%d] %s\n", msg.SeqNum, msg.Envelope.Subject)

		// 4. Get the Raw IO Reader
		r := msg.GetBody(section)
		if r == nil {
			log.Println("   [!] Server returned no body content.")
			continue
		}

		// 5. Parse with enmime (Stable & Simple)
		// This handles the complex recursive MIME structure for you.
		envelope, err := enmime.ReadEnvelope(r)
		if err != nil {
			log.Printf("   [!] Parsing error: %v\n", err)
			continue
		}

		// 6. Display Content
		// enmime automatically finds the best readable text (Text or HTML stripped)
		// It stores it in 'envelope.Text'
		if len(envelope.Text) > 0 {
			// Print first 100 chars just for preview
			preview := envelope.Text
			if len(preview) > 100 {
				preview = preview[:200] + "..."
			}
			fmt.Printf("   BODY: %s\n", preview)
		} else {
			fmt.Println("   [!] Body was empty or only contained attachments.")
		}
	}

	if err := <-done; err != nil {
		log.Fatal(err)
	}
}
