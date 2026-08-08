package guestinit

import "embed"

//go:embed guest-entry.sh tmux-attach.sh
var Scripts embed.FS
