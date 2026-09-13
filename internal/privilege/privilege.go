package privilege

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

const CellGroupName = "cell"

// InCellGroup reports whether the process credentials carry the cell group —
// what group file permissions are actually checked against.
func InCellGroup() bool {
	if os.Geteuid() == 0 {
		return true
	}
	gid, err := CellGroupGID()
	if err != nil {
		return false
	}
	if os.Getgid() == gid {
		return true
	}
	gids, err := os.Getgroups()
	if err != nil {
		return false
	}
	for _, g := range gids {
		if g == gid {
			return true
		}
	}
	return false
}

// cellGroupFileMember reports /etc/group membership, which applies only after
// the credentials are refreshed by re-login or newgrp.
func cellGroupFileMember() bool {
	u, err := user.Current()
	if err != nil {
		return false
	}
	gids, err := u.GroupIds()
	if err != nil {
		return false
	}
	g, err := user.LookupGroup(CellGroupName)
	if err != nil {
		return false
	}
	for _, gid := range gids {
		if gid == g.Gid {
			return true
		}
	}
	return false
}

// CellGroupPending reports cell group membership recorded in /etc/group that
// the current process credentials do not carry yet.
func CellGroupPending() bool {
	return cellGroupFileMember() && !InCellGroup()
}

// CellGroupGID returns the numeric gid of the cell group.
func CellGroupGID() (int, error) {
	g, err := user.LookupGroup(CellGroupName)
	if err != nil {
		return 0, fmt.Errorf("lookup group %s: %w", CellGroupName, err)
	}
	gid, err := strconv.Atoi(g.Gid)
	if err != nil {
		return 0, fmt.Errorf("parse group gid: %w", err)
	}
	return gid, nil
}

// NeedsRuntimeAccess returns true when argv invokes a command that needs cell group membership.
func NeedsRuntimeAccess(args []string) bool {
	if len(args) < 2 {
		return false
	}
	switch args[1] {
	case "version", "help", "--help", "-h", "bootstrap", "rescue":
		return false
	default:
		return true
	}
}

// RequireRuntimeAccess returns an error when the process cannot run runtime commands.
func RequireRuntimeAccess() error {
	if !NeedsRuntimeAccess(os.Args) {
		return nil
	}
	if InCellGroup() {
		return nil
	}
	if CellGroupPending() {
		return fmt.Errorf("cell: %s group updated by bootstrap — re-login (or newgrp %s) to apply it", CellGroupName, CellGroupName)
	}
	return fmt.Errorf("cell: runtime requires the %s group — run: cell bootstrap", CellGroupName)
}

// RequireBootstrapRoot returns an error when bootstrap is not run as root.
func RequireBootstrapRoot() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("cell: bootstrap requires root")
}
