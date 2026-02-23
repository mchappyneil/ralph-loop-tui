package main

import (
	"context"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
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
	statusRunning   iterationStatus = "running"
	statusCompleted iterationStatus = "completed"
	statusError     iterationStatus = "error"
	statusFinished  iterationStatus = "finished"
)

// iterationPhase tracks which phase of the pipeline the current iteration is in
type iterationPhase int

const (
	phasePlanner  iterationPhase = iota
	phaseDev
	phaseReviewer
	phaseFixer
)

func (p iterationPhase) String() string {
	switch p {
	case phasePlanner:
		return "planner"
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
	analytics    analyticsData
	sessionStart time.Time

	// Claude execution
	claudePath string
	sleep      time.Duration
	loopDone   bool
	epic       string // Filter to tasks within a specific epic

	// Phase pipeline state
	currentPhase     iterationPhase
	reviewCycle      int    // current review cycle (1-based)
	maxReviewCycles  int    // from -max-review-cycles flag
	plannerOutput    string // stored between planner → dev/reviewer/fixer
	reviewerFeedback string // stored between reviewer → fixer

	// Context for cancellation
	ctx    context.Context
	cancel context.CancelFunc

	// Program reference for sending messages from goroutines
	program *tea.Program
}

func initialModel() model {
	ctx, cancel := context.WithCancel(context.Background())

	homebaseVP := viewport.New(0, 0)
	outputVP := viewport.New(0, 0)

	initialText := "Press q to quit.\nRalph loop will start automatically..."
	homebaseVP.SetContent(initialText)
	outputVP.SetContent("Raw Claude output will appear here...")

	return model{
		iteration:       0,
		maxIter:         *maxIterations,
		status:          statusIdle,
		statusText:      "Idle",
		activeScreen:    screenHomebase,
		homebaseVP:      homebaseVP,
		outputVP:        outputVP,
		homebaseContent: initialText,
		outputContent:   "",
		followOutput:    true,
		analytics:       newAnalyticsData(),
		sessionStart:    time.Now(),
		claudePath:      *claudeBin,
		sleep:           time.Duration(*sleepSeconds) * time.Second,
		epic:            *epicFilter,
		maxReviewCycles: *maxReviewCycles,
		ctx:             ctx,
		cancel:          cancel,
	}
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

// currentViewport returns the active screen's viewport
func (m *model) currentViewport() *viewport.Model {
	switch m.activeScreen {
	case screenHomebase:
		return &m.homebaseVP
	case screenOutput:
		return &m.outputVP
	default:
		return &m.homebaseVP
	}
}

// SetProgram sets the tea.Program reference for sending messages from goroutines
func (m *model) SetProgram(p *tea.Program) {
	m.program = p
}
