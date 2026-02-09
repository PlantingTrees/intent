package intentengine

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// Parser handles parsing of user commands
type Parser struct {
	commandPattern *regexp.Regexp
}

// NewParser creates a new parser instance
func NewParser() *Parser {
	// Pattern to match: CMD (for|on) "keywords" FROM "sender" [date]
	// Examples:
	// - search for "updates" from "noreply"
	// - listen from "*@exonMobileHr.com"
	// - search for "invite" from "hr@company.com" [recent]
	// - search on "assessment" from "noreply" [2024-01-01 to 2024-01-31]

	return &Parser{
		commandPattern: regexp.MustCompile(`(?i)^\s*(search|listen)\s+(?:for|on)?\s*"([^"]+)"\s+from\s+"([^"]+)"(?:\s+\[([^\]]+)\])?\s*$`),
	}
}

// Parse parses a user command string into an Intent
func (p *Parser) Parse(input string) (*Intent, error) {
	input = strings.TrimSpace(input)

	if input == "" {
		return nil, fmt.Errorf("empty command")
	}

	// Try full pattern match first
	matches := p.commandPattern.FindStringSubmatch(input)
	if matches != nil {
		return p.parseFullCommand(matches)
	}

	// Try simpler patterns
	return p.parseSimpleCommand(input)
}

// parseFullCommand parses a command matching the full pattern
func (p *Parser) parseFullCommand(matches []string) (*Intent, error) {
	if len(matches) < 4 {
		return nil, fmt.Errorf("invalid command format")
	}

	// Extract command type
	cmdStr := strings.ToLower(matches[1])
	var cmd CommandType
	switch cmdStr {
	case "search":
		cmd = CommandSearch
	case "listen":
		cmd = CommandListen
	default:
		return nil, fmt.Errorf("unknown command: %s", cmdStr)
	}

	intent := NewIntent(cmd)

	// Extract keywords
	keywords := matches[2]
	if keywords != "" {
		// Split by common delimiters
		parts := strings.FieldsFunc(keywords, func(r rune) bool {
			return r == ',' || r == '|' || r == '&'
		})
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				intent.AddKeyword(trimmed)
			}
		}
	}

	// Extract sender
	sender := strings.TrimSpace(matches[3])
	if sender != "" {
		// Check if it's a wildcard (all emails from sender)
		allFromSender := strings.HasPrefix(sender, "*@") || strings.HasPrefix(sender, "*")
		if allFromSender {
			sender = strings.TrimPrefix(sender, "*")
			sender = strings.TrimPrefix(sender, "@")
		}
		intent.SetSender(sender, allFromSender)
	}

	// Extract date range if present
	if len(matches) > 4 && matches[4] != "" {
		dateStr := strings.TrimSpace(matches[4])
		if err := p.parseDateRange(intent, dateStr); err != nil {
			return nil, fmt.Errorf("invalid date range: %w", err)
		}
	}

	// Validate LISTEN command
	if cmd == CommandListen && len(intent.Keywords) > 0 {
		return nil, fmt.Errorf("LISTEN command does not support keywords, it only watches for emails from sender")
	}

	return intent, nil
}

// parseSimpleCommand handles simpler command formats
func (p *Parser) parseSimpleCommand(input string) (*Intent, error) {
	// Handle: listen from "sender"
	listenPattern := regexp.MustCompile(`(?i)^\s*listen\s+from\s+"([^"]+)"\s*$`)
	if matches := listenPattern.FindStringSubmatch(input); matches != nil {
		intent := NewIntent(CommandListen)
		sender := strings.TrimSpace(matches[1])
		allFromSender := strings.HasPrefix(sender, "*@") || strings.HasPrefix(sender, "*")
		if allFromSender {
			sender = strings.TrimPrefix(sender, "*")
			sender = strings.TrimPrefix(sender, "@")
		}
		intent.SetSender(sender, allFromSender)
		return intent, nil
	}

	return nil, fmt.Errorf("unable to parse command. Expected format:\n" +
		"  SEARCH for \"keywords\" from \"sender\" [date_range]\n" +
		"  LISTEN from \"sender\"")
}

// parseDateRange parses date range expressions
func (p *Parser) parseDateRange(intent *Intent, dateStr string) error {
	dateStr = strings.ToLower(strings.TrimSpace(dateStr))

	// Handle "recent" shorthand
	if dateStr == "recent" {
		intent.SetRecentRange()
		return nil
	}

	// Handle "today"
	if dateStr == "today" {
		now := time.Now()
		start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		intent.SetDateRange(start, end)
		return nil
	}

	// Handle "yesterday"
	if dateStr == "yesterday" {
		now := time.Now()
		yesterday := now.AddDate(0, 0, -1)
		start := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		end := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 23, 59, 59, 0, yesterday.Location())
		intent.SetDateRange(start, end)
		return nil
	}

	// Handle "last N days"
	lastDaysPattern := regexp.MustCompile(`last\s+(\d+)\s+days?`)
	if matches := lastDaysPattern.FindStringSubmatch(dateStr); matches != nil {
		days := 0
		fmt.Sscanf(matches[1], "%d", &days)
		now := time.Now()
		start := now.AddDate(0, 0, -days)
		start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location())
		end := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
		intent.SetDateRange(start, end)
		return nil
	}

	// Handle range: "YYYY-MM-DD to YYYY-MM-DD"
	rangePattern := regexp.MustCompile(`(\d{4}-\d{2}-\d{2})\s+to\s+(\d{4}-\d{2}-\d{2})`)
	if matches := rangePattern.FindStringSubmatch(dateStr); matches != nil {
		start, err := time.Parse("2006-01-02", matches[1])
		if err != nil {
			return fmt.Errorf("invalid start date: %w", err)
		}
		end, err := time.Parse("2006-01-02", matches[2])
		if err != nil {
			return fmt.Errorf("invalid end date: %w", err)
		}
		// Set to end of end day
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 0, end.Location())
		intent.SetDateRange(start, end)
		return nil
	}

	// Handle single date
	if singleDate, err := time.Parse("2006-01-02", dateStr); err == nil {
		start := time.Date(singleDate.Year(), singleDate.Month(), singleDate.Day(), 0, 0, 0, 0, singleDate.Location())
		end := time.Date(singleDate.Year(), singleDate.Month(), singleDate.Day(), 23, 59, 59, 0, singleDate.Location())
		intent.SetDateRange(start, end)
		return nil
	}

	return fmt.Errorf("unrecognized date format: %s", dateStr)
}

// ParseExamples returns example commands for user reference
func ParseExamples() []string {
	return []string{
		`search for "updates" from "noreply"`,
		`search for "invite" from "hr@company.com" [recent]`,
		`search for "assessment" from "noreply" [yesterday]`,
		`search for "updates" from "noreply" [last 7 days]`,
		`search for "job" from "careers@company.com" [2024-01-01 to 2024-01-31]`,
		`listen from "hr@exonMobile.com"`,
		`listen from "*@exonMobileHr.com"`,
		`search for "interview, assessment" from "*@recruiters.com" [recent]`,
	}
}
