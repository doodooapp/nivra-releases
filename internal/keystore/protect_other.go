//go:build !windows

package keystore

// Non-Windows builds exist for testing the release tooling. The production
// Windows build uses DPAPI and never stores the private key in plaintext.
func protect(data []byte) ([]byte, error) {
	return append([]byte("NIVRA-TEST-KEY\x00"), data...), nil
}

func unprotect(data []byte) ([]byte, error) {
	const prefix = "NIVRA-TEST-KEY\x00"
	if len(data) < len(prefix) || string(data[:len(prefix)]) != prefix {
		return nil, errInvalidTestKey
	}
	return append([]byte(nil), data[len(prefix):]...), nil
}
