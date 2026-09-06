package privilege

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

const CellGroupName = "cell"

// InCellGroup reports whether the current user is in the cell group.
func InCellGroup() bool {
	if os.Geteuid() == 0 {
		return true
	}
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
	return fmt.Errorf("cell: runtime requires group '%s' — run: sudo cell bootstrap", CellGroupName)
}

// RequireBootstrapRoot returns an error when bootstrap is not run as root.
func RequireBootstrapRoot() error {
	if os.Geteuid() == 0 {
		return nil
	}
	return fmt.Errorf("cell: bootstrap requires root")
}
