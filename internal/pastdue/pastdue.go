// Package pastdue performs a non-blocking subscription-status check against
// the Thunder API and surfaces a warning when the caller's subscription is
// past_due (or otherwise non-current). It is best-effort: failures are
// silenced, results are cached on disk so a single command invocation never
// blocks on a network round-trip.
package pastdue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Thunder-Compute/thunder-cli/api"
	"github.com/Thunder-Compute/thunder-cli/utils"
)

// CacheTTL is how long a status result is trusted before re-fetching.
const CacheTTL = 10 * time.Minute

// FetchTimeout caps the background subscription request.
const FetchTimeout = 4 * time.Second

const cacheFileName = "subscription_cache.json"

type cacheEntry struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
}

func cachePath() (string, error) {
	dir, err := utils.ThunderDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, cacheFileName), nil
}

func readCache() (*cacheEntry, error) {
	p, err := cachePath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func writeCache(entry cacheEntry) error {
	p, err := cachePath()
	if err != nil {
		return err
	}
	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(p), filepath.Base(p)+".*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, p)
}

// CachedWarning returns a user-facing warning string if the cached
// subscription status indicates a billing issue and the cache is fresh.
// Returns "" when there is no fresh cache entry, or the status is healthy.
func CachedWarning() string {
	entry, err := readCache()
	if err != nil {
		return ""
	}
	if time.Since(entry.CheckedAt) > CacheTTL {
		return ""
	}
	return WarningFor(entry.Status)
}

// WarningFor maps a subscription status to a user-facing warning, or "" when
// the status is healthy / unknown.
func WarningFor(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "past_due", "unpaid":
		return "A recent payment failed and your Thunder Compute account will soon be deactivated. All instances and snapshots will be permanently deleted. Pay off the outstanding balance at https://console.thundercompute.com/settings/billing to continue using Thunder Compute."
	case "incomplete_expired", "canceled":
		return "Your Thunder Compute account is deactivated. Update your payment method at https://console.thundercompute.com/settings/billing to continue using Thunder Compute."
	}
	return ""
}

// Refresh fetches the current subscription status via the API client and
// writes it to the cache. Safe to call from a goroutine — all errors are
// swallowed (best-effort). A nil client is a no-op.
func Refresh(ctx context.Context, client *api.Client) {
	if client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, FetchTimeout)
	defer cancel()

	status, err := client.GetSubscriptionStatusCtx(ctx)
	if err != nil {
		return
	}
	_ = writeCache(cacheEntry{
		Status:    status,
		CheckedAt: time.Now(),
	})
}

