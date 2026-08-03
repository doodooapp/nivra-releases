//go:build !windows

package keystore

import "errors"

var errInvalidTestKey = errors.New("invalid non-Windows test key")
