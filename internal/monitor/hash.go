package monitor

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

func HashDesired(desired DesiredExternalMonitor, length int) (string, error) {
	if length < 1 || length > sha256.Size*2 {
		return "", fmt.Errorf("hash length must be between 1 and %d: %d", sha256.Size*2, length)
	}

	desired.Hash = ""
	data, err := json.Marshal(desired)
	if err != nil {
		return "", fmt.Errorf("marshal desired monitor: %w", err)
	}

	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:length], nil
}
