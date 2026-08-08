package sync

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/ykhoroshevskiy-tech/cell/internal/config"
	"github.com/ykhoroshevskiy-tech/cell/internal/models"
	"github.com/ykhoroshevskiy-tech/cell/internal/verbose"
)

var unsafeDests = map[string]bool{
	"/": true, "/usr": true, "/bin": true, "/etc": true, "/var": true, "/sbin": true,
}

func ValidateDest(dest string) error {
	clean := strings.TrimRight(dest, "/")
	if clean == "" {
		clean = "/"
	}
	if unsafeDests[clean] {
		return fmt.Errorf("unsafe pull destination: %s", dest)
	}
	return nil
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

	args := []string{"-av"}
	if opts.DryRun {
		args = append(args, "-n")
	}
	if opts.Delete {
		args = append(args, "--delete")
	}
	args = append(args, "--exclude=.filter-staged")

	rsh := fmt.Sprintf(
		"ssh -i %s -o IdentitiesOnly=yes -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null",
		session.SSHKeyPath,
	)
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
