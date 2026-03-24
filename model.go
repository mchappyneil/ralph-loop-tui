package main

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/uuid"
)

// Screen types
type screenType int

const (
	screenHomebase screenType = iota
	screenOutput
	screenAnalytics
)

func (s screenType) String() string {
	switch s {
	case screenHomebase:
		return "Homebase"
	case screenOutput:
		return "Output"
	case screenAnalytics:
		return "Analytics"
	default:
		return "Unknown"
	}
}

// Iteration status
type iterationStatus string

const (
	statusIdle      iterationStatus = "idle"
	statusPreflight iterationStatus = "preflight"
	statusRunning   iterationStatus = "running"
	statusCompleted iterationStatus = "completed"
	statusError     iterationStatus = "error"
	statusFinished  iterationStatus = "finished"
)

// iterationPhase tracks which phase of the pipeline the current iteration is in
type iterationPhase int

const (
	phaseContextGatherer iterationPhase = iota
	phaseDev
	phaseReviewer
	phaseFixer
)

func (p iterationPhase) String() string {
	switch p {
	case phaseContextGatherer:
		return "context-gatherer"
	case phaseDev:
		return "dev"
	case phaseReviewer:
		return "reviewer"
	case phaseFixer:
		return "fixer"
	default:
		return "unknown"
	}
}

// Messages
type startIterationMsg struct{}
type claudeOutputLineMsg struct {
	line string
}
type claudeDoneMsg struct {
	output string
	err    error
}
type tickMsg time.Time

// bdReadyCheckMsg carries the result of verifying whether ready work remains.
// Used to prevent premature COMPLETE when closing a task unblocks new ones.
type bdReadyCheckMsg struct {
	readyCount int
	err        error
}

// preflightDoneMsg carries census results from bd ready/blocked/list queries
// run before starting a Claude iteration.
type preflightDoneMsg struct {
	readyCount      int
	blockedCount    int
	inProgressCount int
	totalOpenCount  int
	graphOutput     string
	err             error
}

// Model holds all application state
type model struct {
	// Iteration tracking
	iteration  int
	maxIter    int
	startTime  time.Time
	endTime    time.Time
	status     iterationStatus
	statusText string
	lastError  string
	rawOutput  string

	// Screen management
	activeScreen screenType
	width        int
	height       int

	// Viewports for each screen
	homebaseVP viewport.Model
	outputVP   viewport.Model

	// Content for each screen
	homebaseContent string
	outputContent   string // Parsed output
	rawOutputLog    string // Raw JSON output
	showRawOutput   bool   // Toggle between raw/parsed on Output screen
	followOutput    bool   // Auto-scroll on Output screen

	// Analytics data
	analytics    *analyticsData
	sessionStart time.Time

	// Claude execution
	claudePath string
	sleep      time.Duration
	loopDone   bool
	epic       string // Filter to tasks within a specific epic

	// Phase pipeline state
	currentPhase      iterationPhase
	reviewCycle       int    // current review cycle (1-based)
	maxReviewCycles   int    // from -max-review-cycles flag
	consecutiveErrors int    // consecutive Claude errors; reset on success
	gathererOutput    string // stored between context-gatherer → dev/reviewer/fixer
	reviewerFeedback  string // stored between reviewer → fixer
	currentTaskID     string
	currentTaskTitle  string

	// Reporting
	reporter      Reporter
	hubURL        string
	hubInstanceID string
	sessionEnded  bool   // prevents duplicate SessionEnded calls
	sessionID     string // unique ID for this session
	repo          string // repository name for event reporting
	instanceID    string // instance ID for event reporting

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Program reference for sending messages from goroutines
	program *tea.Program

	// Demo mode
	demoMode        bool
	demoScenarioIdx int
}

func initialModel(reporter Reporter) model {
	ctx, cancel := context.WithCancel(context.Background())

	homebaseVP := viewport.New(0, 0)
	outputVP := viewport.New(0, 0)

	initialText := "Press q to quit.\nRalph loop will start automatically..."
	homebaseVP.SetContent(initialText)
	outputVP.SetContent("Raw Claude output will appear here...")

	return model{
		iteration:       0,
		maxIter:         *maxIterations,
		status:          statusPreflight,
		statusText:      "Gathering issue census...",
		activeScreen:    screenHomebase,
		homebaseVP:      homebaseVP,
		outputVP:        outputVP,
		homebaseContent: initialText,
		outputContent:   "",
		followOutput:    true,
		analytics:       newAnalyticsData(),
		sessionStart:    time.Now(),
		sessionID:       uuid.New().String(),
		claudePath:      *claudeBin,
		sleep:           time.Duration(*sleepSeconds) * time.Second,
		epic:            *epicFilter,
		maxReviewCycles: *maxReviewCycles,
		reporter:        reporter,
		ctx:             ctx,
		cancel:          cancel,
	}
}

// endSession sends a SessionEnded event exactly once, preventing duplicates.
func (m *model) endSession(reason string) {
	if m.sessionEnded {
		return
	}
	m.sendEvent(EventSessionEnded, map[string]any{"reason": reason})
	m.sessionEnded = true
}

// buildEventContext creates an EventContext snapshot from the model's current state.
func (m *model) buildEventContext() EventContext {
	status := "running"
	if m.status == statusFinished {
		status = "ended"
	}
	return EventContext{
		SessionID:        m.sessionID,
		SessionStart:     m.sessionStart,
		MaxIterations:    m.maxIter,
		CurrentIteration: m.iteration,
		Status:           status,
		CurrentPhase:     m.currentPhase.String(),
		Analytics:        m.analytics.toEventAnalytics(),
	}
}

// sendEvent builds and dispatches an event through the reporter.
func (m *model) sendEvent(eventType EventType, data map[string]any) {
	m.reporter.Send(NewEvent(eventType, m.instanceID, m.repo, m.epic,
		m.buildEventContext(), data))
}

// appendHomebase adds a line to the homebase content
func (m *model) appendHomebase(s string) {
	if m.homebaseContent == "" ||
		m.homebaseContent == "Press q to quit.\nRalph loop will start automatically..." {
		m.homebaseContent = s
	} else {
		m.homebaseContent = m.homebaseContent + "\n" + s
	}
	m.homebaseVP.SetContent(m.homebaseContent)
	m.homebaseVP.GotoBottom()
}

// appendOutput adds a line to the output content
func (m *model) appendOutput(s string) {
	if m.outputContent == "" {
		m.outputContent = s
	} else {
		m.outputContent = m.outputContent + "\n" + s
	}
	// Only update viewport if not in raw mode
	if !m.showRawOutput {
		m.outputVP.SetContent(m.outputContent)
		if m.followOutput {
			m.outputVP.GotoBottom()
		}
	}
}

// SetProgram sets the tea.Program reference for sending messages from goroutines
func (m *model) SetProgram(p *tea.Program) {
	m.program = p
}
