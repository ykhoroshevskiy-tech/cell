package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/hypervisor"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
	"github.com/ykhoroshevskiy-tech/cell/internal/stage"
	"github.com/ykhoroshevskiy-tech/cell/internal/sync"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

type SessionManager struct {
	cfg        *config.CellConfig
	hypervisor hypervisor.Hypervisor
}

func NewSessionManager(cfg *config.CellConfig) (*SessionManager, error) {
	hv, err := hypervisor.NewHypervisor(cfg)
	if err != nil {
		return nil, err
	}
	return &SessionManager{cfg: cfg, hypervisor: hv}, nil
}

func (sm *SessionManager) Launch(ctx context.Context, repoPath, agentConfigPath string, attach bool) (*models.SessionRecord, error) {
	verbose.V("launch: preparing session for %s", repoPath)
	session, err := sm.prepareSession(repoPath)
	if err != nil {
		return nil, err
	}
	session.AgentConfigPath = agentConfigPath
	if _, err := WriteServerPassword(session.SessionDir); err != nil {
		return nil, err
	}
	verbose.V("launch: staging repo → %s", session.StagedRepoDir)
	if err := sm.stageRepo(session); err != nil {
		return nil, err
	}
	verbose.V("launch: building project disk (%d MB)", sm.cfg.ProjectDiskSizeMB)
	if err := sm.buildDisk(session); err != nil {
		return nil, err
	}
	verbose.V("launch: starting VM (vcpu=%d mem=%dMiB)", sm.cfg.VCPUCount, sm.cfg.MemSizeMiB)
	if err := sm.startVM(ctx, session); err != nil {
		return nil, err
	}
	verbose.V("launch: waiting for runtime ready (timeout %v)", sm.cfg.SSHReadyTimeoutSec)
	if err := sm.waitReady(session); err != nil {
		return nil, err
	}
	verbose.V("launch: runtime ready at %s", session.NetworkConfig.GuestIP)

	if attach {
		pullCtx, cancelPull := context.WithCancel(ctx)
		defer cancelPull()
		if sm.cfg.AutoPull {
			verbose.V("launch: auto-pull enabled (interval %ds)", sm.cfg.AutoPullIntervalSec)
			go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
		}
		verbose.V("launch: attaching host TUI")
		if err := sm.attachTUI(session); err != nil {
			cancelPull()
			return session, err
		}
		cancelPull()
	}
	return session, nil
}

func (sm *SessionManager) pullAdapter(ctx context.Context, session *models.SessionRecord, opts models.PullOptions) (*models.PullResult, error) {
	return sm.Pull(ctx, session.SessionID, opts)
}

func (sm *SessionManager) prepareSession(repoSource string) (*models.SessionRecord, error) {
	if _, err := os.Stat(sm.cfg.KernelPath); err != nil {
		return nil, fmt.Errorf("kernel missing at %s: run cell bootstrap", sm.cfg.KernelPath)
	}
	if _, err := os.Stat(sm.cfg.RootfsPath); err != nil {
		return nil, fmt.Errorf("rootfs missing at %s: run cell bootstrap", sm.cfg.RootfsPath)
	}
	if _, err := os.Stat(sm.cfg.FirecrackerBin); err != nil {
		return nil, fmt.Errorf("firecracker missing at %s: run cell bootstrap", sm.cfg.FirecrackerBin)
	}

	resolved, err := ResolveRepoSource(repoSource)
	if err != nil {
		return nil, err
	}

	id, err := newSessionID()
	if err != nil {
		return nil, err
	}
	session := &models.SessionRecord{
		SessionID:  id,
		RepoSource: resolved,
		CreatedAt:  time.Now(),
		State:      models.StateCreated,
	}
	session.ArtifactPaths(sm.cfg.SessionDataDir)
	if err := os.MkdirAll(session.SessionDir, 0755); err != nil {
		return nil, err
	}

	netCfg, err := sm.allocateNetwork(id)
	if err != nil {
		return nil, err
	}
	session.NetworkConfig = netCfg
	verbose.V("session %s: tap=%s guest=%s host=%s", id, netCfg.TapName, netCfg.GuestIP, netCfg.HostIP)
	if err := writeNetworkJSON(session.SessionDir, netCfg); err != nil {
		return nil, err
	}

	kp, err := ssh.WriteKeyPair(session.SessionDir)
	if err != nil {
		return nil, err
	}
	session.SSHKeyPath = kp.PrivateKeyPath
	session.SSHPublicKeyPath = kp.PublicKeyPath

	return session, sm.saveSession(session)
}

func newSessionID() (string, error) {
	b := make([]byte, 6)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (sm *SessionManager) stageRepo(session *models.SessionRecord) error {
	if err := stage.StageRepository(session.RepoSource, session.StagedRepoDir, sm.cfg.IncludeGit, sm.cfg.ExcludePatterns); err != nil {
		session.State = models.StateFailed
		session.Error = err.Error()
		_ = sm.saveSession(session)
		return err
	}
	session.State = models.StateStaged
	return sm.saveSession(session)
}

func (sm *SessionManager) buildDisk(session *models.SessionRecord) error {
	rootDir, err := os.MkdirTemp("", "cell-disk-root-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(rootDir)

	filterDir := filepath.Join(rootDir, ".filter")
	if err := stage.StageRepository(session.StagedRepoDir, rootDir, true, nil); err != nil {
		return err
	}
	if err := os.MkdirAll(filterDir, 0700); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rootDir, ".filter-staged"), []byte("ok\n"), 0644); err != nil {
		return err
	}

	pub, err := os.ReadFile(session.SSHPublicKeyPath)
	if err != nil {
		return err
	}
	if len(pub) == 0 {
		return fmt.Errorf("authorized_keys would be empty")
	}
	if err := os.WriteFile(filepath.Join(filterDir, "authorized_keys"), pub, 0600); err != nil {
		return err
	}
	if err := CopyPasswordToDiskRoot(session.SessionDir, rootDir); err != nil {
		return err
	}
	if err := WriteServePortToDiskRoot(rootDir, sm.cfg.AgentServePort); err != nil {
		return err
	}
	if err := CopyAgentConfigToDiskRoot(session.AgentConfigPath, rootDir); err != nil {
		return err
	}

	_ = os.Remove(session.ProjectDiskPath)
	verbose.V("mkfs: project disk %s (%d MB)", session.ProjectDiskPath, sm.cfg.ProjectDiskSizeMB)
	truncate := exec.Command("truncate", "-s", fmt.Sprintf("%dM", sm.cfg.ProjectDiskSizeMB), session.ProjectDiskPath)
	if verbose.Enabled() {
		truncate.Stdout = os.Stdout
		truncate.Stderr = os.Stderr
		if err := truncate.Run(); err != nil {
			return fmt.Errorf("truncate project disk: %w", err)
		}
	} else if out, err := truncate.CombinedOutput(); err != nil {
		return fmt.Errorf("truncate project disk: %w\n%s", err, out)
	}
	mkfs := exec.Command("mkfs.ext4", "-F", "-d", rootDir, session.ProjectDiskPath)
	if verbose.Enabled() {
		mkfs.Stdout = os.Stdout
		mkfs.Stderr = os.Stderr
		if err := mkfs.Run(); err != nil {
			return fmt.Errorf("mkfs.ext4: %w", err)
		}
	} else if out, err := mkfs.CombinedOutput(); err != nil {
		return fmt.Errorf("mkfs.ext4: %w\n%s", err, out)
	}

	session.State = models.StateDiskReady
	return sm.saveSession(session)
}

func (sm *SessionManager) startVM(ctx context.Context, session *models.SessionRecord) error {
	if ssh.VMRunningForSession(session) {
		return fmt.Errorf("VM already running for session %s", session.SessionID)
	}
	if err := models.ValidateNetworkConfig(session.NetworkConfig); err != nil {
		return err
	}
	verbose.V("launch: reconciling network tap=%s", session.NetworkConfig.TapName)
	if err := sm.reconcileNetwork(session.NetworkConfig); err != nil {
		session.State = models.StateFailed
		session.Error = err.Error()
		_ = sm.saveSession(session)
		return err
	}

	vmConfig := &models.VmConfigDocument{
		BootSource: models.BootSourceSpec{
			KernelImagePath: sm.cfg.KernelPath,
			BootArgs:        session.NetworkConfig.KernelBootArgs(),
		},
		Drives: []models.DriveSpec{
			{DriveID: "rootfs", PathOnHost: sm.cfg.RootfsPath, IsRootDevice: true, IsReadOnly: true},
			{DriveID: "project", PathOnHost: session.ProjectDiskPath, IsRootDevice: false, IsReadOnly: false},
		},
		MachineConfig: models.MachineConfigSpec{
			VCPUCount:   sm.cfg.VCPUCount,
			MemSizeMiB:  sm.cfg.MemSizeMiB,
			SMT:         false,
			CPUTemplate: "None",
		},
		NetworkInterfaces: []models.NetworkInterfaceSpec{
			{IfaceID: "eth0", HostDevName: session.NetworkConfig.TapName, GuestMac: session.NetworkConfig.GuestMac},
		},
	}
	vmData, _ := json.MarshalIndent(vmConfig, "", "  ")
	if err := os.WriteFile(session.VmConfigPath, vmData, 0644); err != nil {
		return err
	}
	if verbose.Enabled() {
		fmt.Printf("--- vm config ---\n%s-----------------\n", vmData)
	}

	// Do NOT wrap Firecracker in WithTimeout/CommandContext here: cancel on return would SIGKILL the VM.
	verbose.V("launch: spawning firecracker %s --api-sock %s", sm.cfg.FirecrackerBin, session.SocketPath)
	pid, err := sm.hypervisor.Start(ctx, vmConfig, session.SerialLogPath, session.SocketPath)
	if err != nil {
		_ = sm.reconcileNetwork(nil)
		session.State = models.StateFailed
		session.Error = err.Error()
		_ = sm.saveSession(session)
		return err
	}
	session.FCPid = pid
	session.State = models.StateRunning
	verbose.V("launch: firecracker pid=%d serial=%s", pid, session.SerialLogPath)
	return sm.saveSession(session)
}

func (sm *SessionManager) waitReady(session *models.SessionRecord) error {
	return ssh.WaitRuntimeReady(session, sm.cfg)
}

func (sm *SessionManager) attachTUI(session *models.SessionRecord) error {
	pw, err := ReadServerPassword(session.SessionDir)
	if err != nil {
		return err
	}
	err = ssh.AttachTUI(session, sm.cfg, pw)
	_ = sm.saveSession(session) // persist HostForwardPort
	return err
}

func (sm *SessionManager) Stop(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if session.FCPid > 0 {
		if err := sm.hypervisor.Stop(session.FCPid); err != nil {
			return err
		}
		session.FCPid = 0
	}
	session.State = models.StateStopped
	if err := sm.saveSession(session); err != nil {
		return err
	}
	return sm.reconcileNetwork(nil)
}

func (sm *SessionManager) StopAll(ctx context.Context) (int, []error) {
	list, err := sm.List(ctx, true)
	if err != nil {
		return 0, []error{err}
	}
	var errs []error
	stopped := 0
	for _, entry := range list {
		if !entry.VMRunning {
			continue
		}
		if err := sm.Stop(ctx, entry.SessionID); err != nil {
			errs = append(errs, err)
		} else {
			stopped++
		}
	}
	return stopped, errs
}

func (sm *SessionManager) Attach(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if !ssh.VMRunningForSession(session) {
		return fmt.Errorf("VM not running; cell start --session %s", sessionID)
	}
	pullCtx, cancelPull := context.WithCancel(ctx)
	defer cancelPull()
	if sm.cfg.AutoPull {
		go sync.StartAutoPull(pullCtx, session, sm.cfg, sm.pullAdapter)
	}
	return sm.attachTUI(session)
}

func (sm *SessionManager) AttachShell(ctx context.Context, sessionID string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	return ssh.AttachShell(session, sm.cfg)
}

func (sm *SessionManager) Pull(ctx context.Context, sessionID string, opts models.PullOptions) (*models.PullResult, error) {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return nil, err
	}
	if !ssh.VMRunningForSession(session) {
		return nil, fmt.Errorf("VM not running; start session or pull while VM is up")
	}
	if session.NetworkConfig == nil {
		return nil, fmt.Errorf("session has no network config")
	}
	if err := ssh.WaitForSSH(session.NetworkConfig.GuestIP, 22, 2*time.Second); err != nil {
		return nil, fmt.Errorf("SSH not reachable: %w", err)
	}
	return sync.PullWorkspace(session, sm.cfg, opts)
}

func (sm *SessionManager) Status(ctx context.Context, sessionID string) (*models.SessionStatus, error) {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return nil, err
	}
	return ssh.SessionStatus(session, sm.cfg), nil
}

func (sm *SessionManager) List(ctx context.Context, runningOnly bool) ([]*models.SessionListEntry, error) {
	_ = ctx
	entries, err := os.ReadDir(sm.cfg.SessionDataDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var list []*models.SessionListEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		session, err := sm.loadSession(entry.Name())
		if err != nil {
			continue
		}
		st := ssh.SessionStatus(session, sm.cfg)
		item := &models.SessionListEntry{
			SessionID:    session.SessionID,
			VMRunning:    st.VMRunning,
			SSHReachable: st.SSHReachable,
			ServerReady:  st.ServerReady,
			RuntimeReady: st.RuntimeReady,
			GuestIP:      st.GuestIP,
			TapName:      st.TapName,
			State:        session.State,
			RepoSource:   session.RepoSource,
			CreatedAt:    session.CreatedAt,
		}
		if runningOnly && !item.VMRunning {
			continue
		}
		list = append(list, item)
	}
	return list, nil
}

func (sm *SessionManager) Rescue(ctx context.Context, sessionID, destPath string) error {
	_ = ctx
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return err
	}
	if _, err := os.Stat(session.ProjectDiskPath); err != nil {
		return fmt.Errorf("project disk missing: %w", err)
	}
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return err
	}
	mntDir, err := os.MkdirTemp("", "cell-rescue-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(mntDir)

	mount := exec.Command("mount", "-t", "ext4", "-o", "loop,ro", session.ProjectDiskPath, mntDir)
	if out, err := mount.CombinedOutput(); err != nil {
		return fmt.Errorf("mount project disk: %w\n%s", err, out)
	}
	defer func() { _ = exec.Command("umount", mntDir).Run() }()

	cp := exec.Command("cp", "-a", mntDir+"/.", destPath+"/")
	if out, err := cp.CombinedOutput(); err != nil {
		return fmt.Errorf("copy rescued files: %w\n%s", err, out)
	}
	return nil
}

func (sm *SessionManager) loadSession(sessionID string) (*models.SessionRecord, error) {
	path := filepath.Join(sm.cfg.SessionDataDir, sessionID, "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read session %s: %w", sessionID, err)
	}
	var session models.SessionRecord
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	session.ArtifactPaths(sm.cfg.SessionDataDir)
	return &session, nil
}

func (sm *SessionManager) saveSession(session *models.SessionRecord) error {
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(session.SessionDir, "session.json"), data, 0644)
}

func (sm *SessionManager) SerialLog(sessionID string) ([]byte, error) {
	session, err := sm.loadSession(sessionID)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(session.SerialLogPath)
}

// ponytail: stop with bounded wait lives in hypervisor.Stop; kept for tests
func stopProcess(pid int) error {
	if pid <= 0 {
		return nil
	}
	proc, _ := os.FindProcess(pid)
	if proc == nil {
		return nil
	}
	_ = proc.Signal(syscall.SIGTERM)
	done := make(chan struct{})
	go func() {
		_, _ = proc.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = proc.Kill()
		<-done
	}
	return nil
}
