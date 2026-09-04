package models

import "time"

type SessionState string

const (
	StateCreated   SessionState = "created"
	StateStaged    SessionState = "staged"
	StateDiskReady SessionState = "disk_ready"
	StateRunning   SessionState = "running"
	StateStopped   SessionState = "stopped"
	StateFailed    SessionState = "failed"
)

type SessionRecord struct {
	SessionID        string         `json:"session_id"`
	RepoSource       string         `json:"repo_source"`
	CreatedAt        time.Time      `json:"created_at"`
	State            SessionState   `json:"state"`
	SessionDir       string         `json:"session_dir,omitempty"`
	StagedRepoDir    string         `json:"staged_repo_dir,omitempty"`
	ProjectDiskPath  string         `json:"project_disk_path,omitempty"`
	SocketPath       string         `json:"socket_path,omitempty"`
	SerialLogPath    string         `json:"serial_log_path,omitempty"`
	VmConfigPath     string         `json:"vm_config_path,omitempty"`
	FCPid            int            `json:"fc_pid,omitempty"`
	SSHKeyPath       string         `json:"ssh_key_path,omitempty"`
	SSHPublicKeyPath string         `json:"ssh_pubkey_path,omitempty"`
	NetworkConfig    *NetworkConfig `json:"network_config,omitempty"`
	HostForwardPort  int            `json:"host_forward_port,omitempty"`
	Error            string         `json:"error,omitempty"`
	AgentConfigPath  string         `json:"-"`
}

func (s *SessionRecord) ArtifactPaths(sessionDataDir string) {
	s.SessionDir = sessionDataDir + "/" + s.SessionID
	s.StagedRepoDir = s.SessionDir + "/repo"
	s.ProjectDiskPath = s.SessionDir + "/project.ext4"
	s.SocketPath = s.SessionDir + "/firecracker.socket"
	s.SerialLogPath = s.SessionDir + "/serial.log"
	s.VmConfigPath = s.SessionDir + "/vm-config.json"
	s.SSHKeyPath = s.SessionDir + "/id_ed25519"
	s.SSHPublicKeyPath = s.SessionDir + "/id_ed25519.pub"
}
