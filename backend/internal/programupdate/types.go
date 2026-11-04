package programupdate

import (
	"context"
	"time"

	"yingce/backend/internal/buildinfo"
)

const Repository = "ws4934/open-ai-canvas"

type Config struct {
	DataDir           string
	ManagedBinaryPath string
	CurrentVersion    string
	BuildInfo         buildinfo.Info
	RequestRestart    func() error
	Drain             func(context.Context) error
	Resume            func()
}

type Release struct {
	Version     string    `json:"version"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
	Assets      []asset   `json:"-"`
}

type asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
	Size int64  `json:"size"`
}

type Phase string

const (
	PhaseIdle               Phase = "idle"
	PhaseDownloading        Phase = "downloading"
	PhaseValidating         Phase = "validating"
	PhaseDraining           Phase = "draining"
	PhaseSwitching          Phase = "switching"
	PhaseRestarting         Phase = "restarting"
	PhaseSucceeded          Phase = "succeeded"
	PhaseRolledBack         Phase = "rolled_back"
	PhaseFailed             Phase = "failed"
	PhaseManualIntervention Phase = "manual_intervention"
)

func (p Phase) Active() bool {
	return p == PhaseDownloading || p == PhaseValidating || p == PhaseDraining || p == PhaseSwitching || p == PhaseRestarting
}

type LogEntry struct {
	At      time.Time `json:"at"`
	Phase   Phase     `json:"phase"`
	Message string    `json:"message"`
}

type Operation struct {
	ID                string     `json:"id,omitempty"`
	Kind              string     `json:"kind,omitempty"`
	Phase             Phase      `json:"phase"`
	FromVersion       string     `json:"fromVersion,omitempty"`
	TargetVersion     string     `json:"targetVersion,omitempty"`
	StartedAt         *time.Time `json:"startedAt,omitempty"`
	FinishedAt        *time.Time `json:"finishedAt,omitempty"`
	Error             string     `json:"error,omitempty"`
	AutomaticRollback bool       `json:"automaticRollback"`
	Logs              []LogEntry `json:"logs"`
}

type Status struct {
	Supported         bool       `json:"supported"`
	UnsupportedReason string     `json:"unsupportedReason,omitempty"`
	Repository        string     `json:"repository"`
	CurrentVersion    string     `json:"currentVersion"`
	LatestRelease     *Release   `json:"latestRelease,omitempty"`
	UpdateAvailable   bool       `json:"updateAvailable"`
	CheckedAt         *time.Time `json:"checkedAt,omitempty"`
	CanRollback       bool       `json:"canRollback"`
	RollbackVersion   string     `json:"rollbackVersion,omitempty"`
	Operation         Operation  `json:"operation"`
}
