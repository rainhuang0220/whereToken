//go:build !darwin && !windows

package cursor

// Linux sealed sessions stay unread. The desktop secret service is not
// called: granting a helper would widen access the way a permanent ACL
// on the platform secret tool does on macOS. A plaintext JWT still
// works. cursor connect records the user's choice and reports this
// platform limit instead of prompting in the background.
func unsealOS(token, _ string, _ bool) (string, error) {
	if _, ok := sealedBlob(token); !ok {
		return "", errSealedCredential
	}
	return "", errSealedCredential
}
