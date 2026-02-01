package main

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// analyticsData tracks session statistics
type analyticsData struct {
	sessionStart     time.Time
	iterationHistory []iterationRecord
	passedCount      int
	failedCount      int
	initialReady     int
	currentReady     int
	tasksClosed      int
}

// iterationRecord stores data for a single iteration
type iterationRecord struct {
	iteration int
	duration  time.Duration
	passed    bool
	taskID    string
	notes     string
}

func newAnalyticsData() analyticsData {
	return analyticsData{
		sessionStart:     time.Now(),
		iterationHistory: make([]iterationRecord, 0),
	}
}

// addIteration records a completed iteration
func (a *analyticsData) addIteration(iteration int, duration time.Duration, passed bool, taskID, notes string) {
	record := iterationRecord{
		iteration: iteration,
		duration:  duration,
		passed:    passed,
		taskID:    taskID,
		notes:     notes,
	}
	a.iterationHistory = append(a.iterationHistory, record)
	if passed {
		a.passedCount++
		a.tasksClosed++
	} else {
		a.failedCount++
	}
}

// totalDuration returns the sum of all iteration durations
func (a *analyticsData) totalDuration() time.Duration {
	var total time.Duration
	for _, r := range a.iterationHistory {
		total += r.duration
	}
	return total
}

// avgDuration returns the average iteration duration
func (a *analyticsData) avgDuration() time.Duration {
	if len(a.iterationHistory) == 0 {
		return 0
	}
	return a.totalDuration() / time.Duration(len(a.iterationHistory))
}

// fastestDuration returns the shortest iteration duration
func (a *analyticsData) fastestDuration() time.Duration {
	if len(a.iterationHistory) == 0 {
		return 0
	}
	fastest := a.iterationHistory[0].duration
	for _, r := range a.iterationHistory[1:] {
		if r.duration < fastest {
			fastest = r.duration
		}
	}
	return fastest
}

// slowestDuration returns the longest iteration duration
func (a *analyticsData) slowestDuration() time.Duration {
	if len(a.iterationHistory) == 0 {
		return 0
	}
	slowest := a.iterationHistory[0].duration
	for _, r := range a.iterationHistory[1:] {
		if r.duration > slowest {
			slowest = r.duration
		}
	}
	return slowest
}

// lastIterations returns the last n iteration records
func (a *analyticsData) lastIterations(n int) []iterationRecord {
	if len(a.iterationHistory) <= n {
		return a.iterationHistory
	}
	return a.iterationHistory[len(a.iterationHistory)-n:]
}

// lastTask returns the most recent task ID, or empty string if none
func (a *analyticsData) lastTask() string {
	if len(a.iterationHistory) == 0 {
		return ""
	}
	return a.iterationHistory[len(a.iterationHistory)-1].taskID
}

// estimatedRemaining estimates time remaining based on avg duration and remaining iterations
func (a *analyticsData) estimatedRemaining(currentIter, maxIter int) time.Duration {
	if len(a.iterationHistory) == 0 {
		return 0
	}
	remaining := maxIter - currentIter
	if remaining <= 0 {
		return 0
	}
	return a.avgDuration() * time.Duration(remaining)
}

// RalphStatus holds parsed status from Ralph's output
type RalphStatus struct {
	ReadyBefore int
	ReadyAfter  int
	Task        string
	Passed      bool
	Notes       string
}

// parseRalphStatus extracts the [Ralph status] block from Claude output
// The output is in stream-json format, so we need to extract text content first
func parseRalphStatus(output string) *RalphStatus {
	// First, extract all text content from the JSON output
	textContent := extractTextFromStreamJSON(output)

	// Look for the status block in the extracted text
	statusBlockRegex := regexp.MustCompile(`(?s)\[Ralph status\]\s*\n(.*?)(?:\n\n|$)`)
	match := statusBlockRegex.FindStringSubmatch(textContent)
	if match == nil {
		return nil
	}

	block := match[1]
	status := &RalphStatus{}

	// Parse each field
	lines := strings.Split(block, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "ready_before":
			if n, err := strconv.Atoi(value); err == nil {
				status.ReadyBefore = n
			}
		case "ready_after":
			if n, err := strconv.Atoi(value); err == nil {
				status.ReadyAfter = n
			}
		case "task":
			status.Task = value
		case "tests":
			status.Passed = strings.ToUpper(value) == "PASSED"
		case "notes":
			status.Notes = value
		}
	}

	return status
}

// extractTextFromStreamJSON extracts all text content from stream-json output
// Uses ExtractFullText from jsonparser to properly extract full text without truncation
func extractTextFromStreamJSON(output string) string {
	return ExtractFullText(output)
}
