package sync

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/ssh"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var unsafeDests = map[string]bool{
	"/": true, "/usr": true, "/bin": true, "/etc": true, "/var": true, "/sbin": true,
}

func PullRsyncExcludes() []string {
	return []string{"--exclude=.filter-staged", "--exclude=.filter"}
}

func ValidateDest(dest string) error {
	clean := strings.TrimRight(dest, "/")
	if clean == "" {
		clean = "/"
	}
	if unsafeDests[clean] {
		return fmt.Errorf("unsafe pull destination: %s", dest)
	}
	if !filepath.IsAbs(clean) {
		return fmt.Errorf("relative pull destination: %s", dest)
	}
	return nil
}

func DestOwner(path string) (uid, gid int, err error) {
	fi, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return 0, 0, err
		}
		fi, err = os.Lstat(filepath.Dir(path))
		if err != nil {
			return 0, 0, err
		}
	}
	st, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, 0, fmt.Errorf("no unix stat for %s", path)
	}
	return int(st.Uid), int(st.Gid), nil
}

func PullWorkspace(session *models.SessionRecord, cfg *config.CellConfig, opts models.PullOptions) (*models.PullResult, error) {
	dest := opts.Dest
	if dest == "" {
		dest = session.RepoSource
	}
	if err := ValidateDest(dest); err != nil {
		return nil, err
	}
	if session.NetworkConfig == nil {
		return nil, fmt.Errorf("session has no network config")
	}
	if session.SSHKeyPath == "" {
		return nil, fmt.Errorf("session has no ssh key")
	}

	uid, gid, err := DestOwner(dest)
	if err != nil {
		return nil, fmt.Errorf("dest owner: %w", err)
	}

	args := []string{"-av", "--no-owner", "--no-group", fmt.Sprintf("--chown=%d:%d", uid, gid)}
	if opts.DryRun {
		args = append(args, "-n")
	}
	if opts.Delete {
		args = append(args, "--delete")
	}
	args = append(args, PullRsyncExcludes()...)

	rsh := ssh.RemoteShell(session.SSHKeyPath)
	guest := fmt.Sprintf("%s@%s:%s/", cfg.SSHUser, session.NetworkConfig.GuestIP, cfg.GuestRepoDir)
	args = append(args, "-e", rsh, guest, dest+"/")

	cmd := exec.Command("rsync", args...)
	if verbose.Enabled() && !opts.Quiet {
		verbose.V("pull: rsync %s", strings.Join(args, " "))
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("rsync: %w", err)
		}
	} else if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("rsync: %w\n%s", err, string(out))
	}
	return &models.PullResult{Destination: dest}, nil
}
