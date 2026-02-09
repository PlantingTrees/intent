package intentengine

import (
	"time"
)

// CommandType represents the type of command
type CommandType string

const (
	CommandSearch CommandType = "search"
	CommandListen CommandType = "listen"
)

// DateRange represents a time range for filtering
type DateRange struct {
	Start time.Time
	End   time.Time
}

// Intent represents a parsed user intent
type Intent struct {
	Command       CommandType
	Keywords      []string   // What to search for (e.g., "updates", "invite", "assessment")
	Sender        string     // Email sender to filter by
	DateRange     *DateRange // Optional date range
	AllFromSender bool       // True if user wants ALL emails from sender (*)
}

// NewIntent creates a new Intent
func NewIntent(cmd CommandType) *Intent {
	return &Intent{
		Command:  cmd,
		Keywords: make([]string, 0),
	}
}

// AddKeyword adds a keyword to search for
func (i *Intent) AddKeyword(keyword string) {
	i.Keywords = append(i.Keywords, keyword)
}

// SetSender sets the sender filter
func (i *Intent) SetSender(sender string, all bool) {
	i.Sender = sender
	i.AllFromSender = all
}

// SetDateRange sets the date range filter
func (i *Intent) SetDateRange(start, end time.Time) {
	i.DateRange = &DateRange{
		Start: start,
		End:   end,
	}
}

// SetRecentRange sets the date range to "recent" (yesterday to today)
func (i *Intent) SetRecentRange() {
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	// Set to start of yesterday and end of today
	start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
	end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())

	i.SetDateRange(start, end)
}
