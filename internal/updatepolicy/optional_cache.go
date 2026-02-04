package updatepolicy

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	optionalCacheFile = "optional_update_status.json"
	optionalCacheDir  = ".thunder"
	optionalCacheSub  = "cache"
	OptionalUpdateTTL = 24 * time.Hour
)

type optionalCachePayload struct {
	LastAttempt time.Time `json:"last_attempt"`
}

// ReadOptionalUpdateAttempt returns the timestamp of the last optional update attempt.
// If no attempt has been recorded, the returned time is zero.
func ReadOptionalUpdateAttempt() (time.Time, error) {
	path, err := optionalCachePath()
	if err != nil {
		return time.Time{}, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	var payload optionalCachePayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return time.Time{}, err
	}
	return payload.LastAttempt, nil
}

// WriteOptionalUpdateAttempt records the provided timestamp as the last optional update attempt.
func WriteOptionalUpdateAttempt(ts time.Time) error {
	path, err := optionalCachePath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload := optionalCachePayload{
		LastAttempt: ts.UTC(),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// ClearOptionalUpdateAttempt removes the optional update cache entry, if it exists.
func ClearOptionalUpdateAttempt() error {
	path, err := optionalCachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func optionalCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, optionalCacheDir, optionalCacheSub)
	return filepath.Join(dir, optionalCacheFile), nil
}
