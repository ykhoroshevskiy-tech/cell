package guestinit

import "embed"

//go:embed guest-entry.sh tmux-attach-opencode.sh
var Scripts embed.FS
