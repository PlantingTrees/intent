package intentengine

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"
)

// Email represents a simplified email structure for filtering
type Email struct {
	ID      string
	From    string
	Subject string
	Date    time.Time
	Body    string
}

// Executor executes parsed intents
type Executor struct {
	imapClient *client.Client
}

// NewExecutor creates a new executor instance with IMAP client
func NewExecutor(c *client.Client) *Executor {
	return &Executor{
		imapClient: c,
	}
}

// Execute executes the given intent
func (e *Executor) Execute(intent *Intent) (interface{}, error) {
	switch intent.Command {
	case CommandSearch:
		return e.executeSearch(intent)
	case CommandListen:
		return e.executeListen(intent)
	default:
		return nil, fmt.Errorf("unknown command type: %s", intent.Command)
	}
}

// executeSearch performs a search based on the intent using Gmail IMAP
func (e *Executor) executeSearch(intent *Intent) (interface{}, error) {
	fmt.Println("\n=== Executing SEARCH ===")
	fmt.Println("Keywords:", strings.Join(intent.Keywords, ", "))
	fmt.Println("Sender:", intent.Sender)
	if intent.AllFromSender {
		fmt.Println("Mode: ALL emails from sender domain")
	}
	if intent.DateRange != nil {
		fmt.Printf("Date Range: %s to %s\n",
			intent.DateRange.Start.Format("2006-01-02"),
			intent.DateRange.End.Format("2006-01-02"))
	}

	// Select INBOX
	mbox, err := e.imapClient.Select("INBOX", false)
	if err != nil {
		return nil, fmt.Errorf("failed to select INBOX: %w", err)
	}

	fmt.Printf("\nMailbox: %s (%d messages)\n", mbox.Name, mbox.Messages)

	// Build IMAP search criteria
	criteria := e.buildSearchCriteria(intent)

	fmt.Println("\nSearching...")

	// Perform search
	seqNums, err := e.imapClient.Search(criteria)
	if err != nil {
		return nil, fmt.Errorf("search failed: %w", err)
	}

	fmt.Printf("✓ Found %d matching messages\n\n", len(seqNums))

	if len(seqNums) == 0 {
		fmt.Println("No messages found matching your criteria.")
		return map[string]interface{}{
			"command": "search",
			"count":   0,
			"results": []map[string]string{},
		}, nil
	}

	// Fetch message details
	messages := e.fetchMessages(seqNums)

	// Display results
	fmt.Println("=== Search Results ===\n")
	results := []map[string]string{}

	for i, msg := range messages {
		fmt.Printf("[%d] From: %s\n", i+1, msg.From)
		fmt.Printf("    Subject: %s\n", msg.Subject)
		fmt.Printf("    Date: %s\n", msg.Date.Format("2006-01-02 15:04:05"))
		fmt.Println()

		results = append(results, map[string]string{
			"from":    msg.From,
			"subject": msg.Subject,
			"date":    msg.Date.Format("2006-01-02 15:04:05"),
		})
	}

	return map[string]interface{}{
		"command": "search",
		"count":   len(messages),
		"results": results,
	}, nil
}

// executeListen sets up a listener/watcher for emails

func (e *Executor) executeListen(intent *Intent) (interface{}, error) {
	fmt.Println("\n=== LISTENING  ===")
	fmt.Println("Watching for emails from:", intent.Sender)

	mbox, err := e.imapClient.Select("[Gmail]/All Mail", false)
	if err != nil {
		return nil, err
	}

	lastUID := mbox.UidNext - 1

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		<-ticker.C

		mbox, err := e.imapClient.Select("[Gmail]/All Mail", false)
		if err != nil {
			log.Println("Select error:", err)
			continue
		}

		if mbox.UidNext <= lastUID+1 {
			continue // nothing new
		}

		// Fetch ONLY new messages
		set := new(imap.SeqSet)
		set.AddRange(lastUID+1, mbox.UidNext-1)

		messages := make(chan *imap.Message, 10)
		go func() {
			_ = e.imapClient.UidFetch(set, []imap.FetchItem{
				imap.FetchUid,
				imap.FetchEnvelope,
				imap.FetchInternalDate,
			}, messages)
		}()

		for msg := range messages {
			lastUID = msg.Uid

			if msg.Envelope == nil || len(msg.Envelope.From) == 0 {
				continue
			}

			addr := msg.Envelope.From[0]
			from := fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)

			if strings.Contains(strings.ToLower(from), strings.ToLower(intent.Sender)) {
				fmt.Println("📧 NEW EMAIL RECEIVED!")
				fmt.Printf("   From: %s\n", from)
				fmt.Printf("   Subject: %s\n", msg.Envelope.Subject)
				fmt.Printf("   Date: %s\n\n", msg.InternalDate)
			}
		}
	}
}

// func (e *Executor) executeListen(intent *Intent) (interface{}, error) {
// 	fmt.Println("\n=== Executing LISTEN ===")
// 	fmt.Println("Watching for emails from:", intent.Sender)
// 	if intent.AllFromSender {
// 		fmt.Println("Mode: ALL emails from sender domain")
// 	}
//
// 	// Select INBOX
// 	_, err := e.imapClient.Select("INBOX", false)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to select INBOX: %w", err)
// 	}
//
// 	fmt.Println("\n✓ Listening... (Press Ctrl+C to stop)")
// 	fmt.Println("Checking for new messages periodically...\n")
//
// 	// Keep track of seen messages
// 	seenMsgs := make(map[uint32]bool)
//
// 	// Build criteria for the sender
// 	criteria := imap.NewSearchCriteria()
// 	if intent.AllFromSender {
// 		// For wildcard, we'll check all messages and filter in code
// 		criteria.WithoutFlags = []string{imap.SeenFlag}
// 	} else {
// 		criteria.Header.Set("From", intent.Sender)
// 		criteria.WithoutFlags = []string{imap.SeenFlag}
// 	}
//
// 	// Polling loop
// 	ticker := time.NewTicker(5 * time.Second)
// 	defer ticker.Stop()
//
// 	for {
// 		select {
// 		case <-ticker.C:
// 			// Search for new messages
// 			seqNums, err := e.imapClient.Search(criteria)
// 			if err != nil {
// 				log.Printf("Search error: %v", err)
// 				continue
// 			}
//
// 			// Filter out already seen messages
// 			var newMsgs []uint32
// 			for _, num := range seqNums {
// 				if !seenMsgs[num] {
// 					newMsgs = append(newMsgs, num)
// 					seenMsgs[num] = true
// 				}
// 			}
//
// 			if len(newMsgs) > 0 {
// 				messages := e.fetchMessages(newMsgs)
//
// 				for _, msg := range messages {
// 					// If wildcard, filter by domain
// 					if intent.AllFromSender {
// 						if !strings.Contains(strings.ToLower(msg.From), strings.ToLower(intent.Sender)) {
// 							continue
// 						}
// 					}
//
// 					fmt.Println("📧 NEW EMAIL RECEIVED!")
// 					fmt.Printf("   From: %s\n", msg.From)
// 					fmt.Printf("   Subject: %s\n", msg.Subject)
// 					fmt.Printf("   Date: %s\n\n", msg.Date.Format("2006-01-02 15:04:05"))
// 				}
// 			}
// 		}
// 	}
// } // executeListen sets up a listener/watcher for emails
//
// executeListen sets up a listener/watcher for ALL emails

// buildSearchCriteria builds IMAP search criteria from intent
func (e *Executor) buildSearchCriteria(intent *Intent) *imap.SearchCriteria {
	criteria := imap.NewSearchCriteria()

	// Add sender filter
	if intent.Sender != "" {
		if intent.AllFromSender {
			// For wildcard, we search all and filter afterwards
			// IMAP doesn't support wildcard domains directly
		} else {
			criteria.Header.Set("From", intent.Sender)
		}
	}

	// Add date range filter
	if intent.DateRange != nil {
		criteria.Since = intent.DateRange.Start
		criteria.Before = intent.DateRange.End
	}

	// Note: IMAP SEARCH doesn't support OR for text terms easily
	// For keywords, we'll do multiple searches or filter in code
	// For now, if keywords exist, we'll search for them in subject/body
	if len(intent.Keywords) > 0 {
		// Search for first keyword in text
		// Gmail supports X-GM-RAW for more complex queries
		criteria.Text = []string{intent.Keywords[0]}
	}

	return criteria
}

// fetchMessages retrieves full message details for given sequence numbers
func (e *Executor) fetchMessages(seqNums []uint32) []Email {
	if len(seqNums) == 0 {
		return []Email{}
	}

	// Create sequence set
	seqSet := new(imap.SeqSet)
	seqSet.AddNum(seqNums...)

	// Fetch envelope and date
	messages := make(chan *imap.Message, len(seqNums))
	section := &imap.BodySectionName{}

	done := make(chan error, 1)
	go func() {
		done <- e.imapClient.Fetch(seqSet, []imap.FetchItem{
			imap.FetchEnvelope,
			imap.FetchInternalDate,
			section.FetchItem(),
		}, messages)
	}()

	var emails []Email
	for msg := range messages {
		if msg.Envelope == nil {
			continue
		}

		// Extract sender
		from := "Unknown"
		if len(msg.Envelope.From) > 0 {
			addr := msg.Envelope.From[0]
			if addr.PersonalName != "" {
				from = fmt.Sprintf("%s <%s@%s>", addr.PersonalName, addr.MailboxName, addr.HostName)
			} else {
				from = fmt.Sprintf("%s@%s", addr.MailboxName, addr.HostName)
			}
		}

		emails = append(emails, Email{
			ID:      fmt.Sprintf("%d", msg.SeqNum),
			From:    from,
			Subject: msg.Envelope.Subject,
			Date:    msg.InternalDate,
			Body:    "", // Body fetching can be added if needed
		})
	}

	if err := <-done; err != nil {
		log.Printf("Fetch error: %v", err)
	}

	return emails
}

// FilterEmails filters a slice of emails based on the intent
// This is a helper method you can use with your email data
func (e *Executor) FilterEmails(emails []Email, intent *Intent) []Email {
	var filtered []Email

	for _, email := range emails {
		if e.matchesIntent(email, intent) {
			filtered = append(filtered, email)
		}
	}

	return filtered
}

// matchesIntent checks if an email matches the intent criteria
func (e *Executor) matchesIntent(email Email, intent *Intent) bool {
	// Check sender filter
	if intent.Sender != "" {
		if intent.AllFromSender {
			// Check if email is from the domain
			if !strings.HasSuffix(strings.ToLower(email.From), "@"+strings.ToLower(intent.Sender)) &&
				!strings.Contains(strings.ToLower(email.From), strings.ToLower(intent.Sender)) {
				return false
			}
		} else {
			// Exact sender match
			if !strings.Contains(strings.ToLower(email.From), strings.ToLower(intent.Sender)) {
				return false
			}
		}
	}

	// Check keywords (match in subject or body)
	if len(intent.Keywords) > 0 {
		matched := false
		searchText := strings.ToLower(email.Subject + " " + email.Body)

		for _, keyword := range intent.Keywords {
			if strings.Contains(searchText, strings.ToLower(keyword)) {
				matched = true
				break
			}
		}

		if !matched {
			return false
		}
	}

	// Check date range
	if intent.DateRange != nil {
		if email.Date.Before(intent.DateRange.Start) || email.Date.After(intent.DateRange.End) {
			return false
		}
	}

	return true
}

// Validate validates an intent before execution
func (e *Executor) Validate(intent *Intent) error {
	if intent == nil {
		return fmt.Errorf("intent cannot be nil")
	}

	switch intent.Command {
	case CommandSearch:
		// Search requires either keywords or sender
		if len(intent.Keywords) == 0 && intent.Sender == "" {
			return fmt.Errorf("search requires at least keywords or sender")
		}
	case CommandListen:
		// Listen requires a sender
		if intent.Sender == "" {
			return fmt.Errorf("listen requires a sender")
		}
		// Listen should not have keywords
		if len(intent.Keywords) > 0 {
			return fmt.Errorf("listen command does not support keywords")
		}
	default:
		return fmt.Errorf("unknown command type: %s", intent.Command)
	}

	return nil
}
