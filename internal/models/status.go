package models

import "time"

type SessionStatus struct {
	SessionID    string `json:"session_id"`
	VMRunning    bool   `json:"vm_running"`
	SSHReachable bool   `json:"ssh_reachable"`
	TmuxReady    bool   `json:"tmux_ready"`
	RuntimeReady bool   `json:"runtime_ready"`
	GuestIP      string `json:"guest_ip,omitempty"`
	TapName      string `json:"tap_name,omitempty"`
}

type SessionListEntry struct {
	SessionID    string       `json:"session_id"`
	VMRunning    bool         `json:"vm_running"`
	SSHReachable bool         `json:"ssh_reachable"`
	TmuxReady    bool         `json:"tmux_ready"`
	RuntimeReady bool         `json:"runtime_ready"`
	GuestIP      string       `json:"guest_ip,omitempty"`
	TapName      string       `json:"tap_name,omitempty"`
	State        SessionState `json:"state"`
	RepoSource   string       `json:"repo_source"`
	CreatedAt    time.Time    `json:"created_at"`
}

type PullResult struct {
	Destination      string `json:"destination"`
	FilesTransferred *int   `json:"files_transferred,omitempty"`
}

type PullOptions struct {
	Dest   string `json:"dest,omitempty"`
	DryRun bool   `json:"dry_run,omitempty"`
	Delete bool   `json:"delete,omitempty"`
	Quiet  bool   `json:"quiet,omitempty"`
}
