package setup

import (
	"errors"
	"os"
)

// Uninstall stops and removes the service and binary. Containers and stacks
// keep running. With purge it also deletes history, activity, credentials and
// the service user, which can't be undone.
func Uninstall(ui *UI, purge bool) error {
	if os.Geteuid() != 0 {
		return errors.New("run with sudo")
	}
	// --yes skips the questions, which here default to no.
	if !ui.Yes && !ui.Confirm("Remove the StunSails service and binary? Your containers keep running.", false) {
		return errors.New("stopped; nothing was removed")
	}
	ui.Run("systemctl", "disable", "--now", Service)
	for _, p := range []string{UnitPath, UnitPath + ".prev", BinPath + ".prev"} {
		os.Remove(p)
	}
	ui.Run("systemctl", "daemon-reload")
	if purge {
		ui.Say("\nPurging deletes %s (history, activity) and %s (password, authenticator key).", StateDir, ConfDir)
		if !ui.Yes && !ui.Confirm("Delete them for good?", false) {
			ui.Say("Kept them. The service is removed.")
		} else {
			os.RemoveAll(StateDir)
			os.RemoveAll(ConfDir)
			ui.Run("userdel", User)
		}
	} else {
		ui.Say("Kept %s and %s; `sudo stunsails init` picks them up again.", StateDir, ConfDir)
	}
	// Last: this may be the running binary, which keeps working until it exits.
	os.Remove(BinPath)
	ui.Say("StunSails is removed.")
	return nil
}
