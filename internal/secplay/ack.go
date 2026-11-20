package secplay

import (
	"os"
	"path/filepath"
)

// AckFile is the per-repo marker recording that the authorized-use notice for
// the offensive (red) side was shown. It is local, not versioned.
const AckFile = "secplay-red-ack"

// AuthorizedUseNotice is printed once per repo on the first `tb red` use.
const AuthorizedUseNotice = "tb red provides offensive AppSec methodology for AUTHORIZED testing of software you own or are permitted to assess. Use it only within an engagement's scope and the law."

// AckPath returns the ack marker path inside a toolbelt dir.
func AckPath(toolbeltDir string) string {
	return filepath.Join(toolbeltDir, AckFile)
}

// NeedsAck reports whether the red authorized-use notice still needs showing.
// It errs toward showing: only a successful stat (the marker exists) suppresses
// it, so a transient stat error does not silence the notice forever.
func NeedsAck(toolbeltDir string) bool {
	_, err := os.Stat(AckPath(toolbeltDir))
	return err != nil
}

// RecordAck writes the ack marker, creating the toolbelt dir if needed.
func RecordAck(toolbeltDir string) error {
	if err := os.MkdirAll(toolbeltDir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(AckPath(toolbeltDir), []byte("acknowledged\n"), 0o600)
}
