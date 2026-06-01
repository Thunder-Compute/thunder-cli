package tui

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Thunder-Compute/thunder-cli/api"
)

func TestStatusRefreshRetryDelayUsesRetryAfter(t *testing.T) {
	InitCommonStyles(os.Stdout)
	m := newStatusModel(nil, true, []api.Instance{{ID: "1", Status: "STARTING"}}, false)

	delay, ok := m.statusRefreshRetryDelay(&api.APIError{
		StatusCode:    http.StatusTooManyRequests,
		Message:       "Rate limit exceeded",
		RetryAfter:    42 * time.Second,
		HasRetryAfter: true,
	})

	require.True(t, ok)
	assert.Equal(t, 42*time.Second, delay)
}

func TestStatusModelEmptyInstancesKeepPolling(t *testing.T) {
	InitCommonStyles(os.Stdout)
	m := newStatusModel(nil, true, nil, false)

	assert.True(t, m.monitoring)
	assert.False(t, m.emptyStartedAt.IsZero())

	model, cmd := m.Update(tickMsg(time.Now()))
	updated, ok := model.(statusModel)
	require.True(t, ok)
	require.NotNil(t, cmd)
	assert.True(t, updated.monitoring)
	assert.False(t, updated.quitting)

	model, cmd = updated.Update(instancesMsg{instances: nil})
	updated, ok = model.(statusModel)
	require.True(t, ok)
	require.NotNil(t, cmd)
	assert.True(t, updated.monitoring)
	assert.False(t, updated.quitting)
	assert.Empty(t, updated.instances)
	assert.False(t, updated.emptyStartedAt.IsZero())

	view := stripANSI(updated.View())
	assert.Contains(t, view, "No instances found")
	assert.Contains(t, view, "Last updated:")
	assert.Contains(t, view, "checking every 1m")
}

func TestNextEmptyPollDelayBacksOffAfterLongEmptyPolling(t *testing.T) {
	assert.Equal(t, emptyPollInterval, nextEmptyPollDelay(time.Now()))
	assert.Equal(t, emptyPollBackoffInterval, nextEmptyPollDelay(time.Now().Add(-emptyPollBackoffAfter-time.Second)))
}

func TestStatusModelRateLimitKeepsCachedInstances(t *testing.T) {
	InitCommonStyles(os.Stdout)
	instances := []api.Instance{{ID: "1", Status: "STARTING"}}
	m := newStatusModel(nil, true, instances, false)

	model, cmd := m.Update(instancesMsg{err: &api.APIError{
		StatusCode:    http.StatusTooManyRequests,
		Message:       "Rate limit exceeded",
		RetryAfter:    2 * time.Minute,
		HasRetryAfter: true,
	}})

	updated, ok := model.(statusModel)
	require.True(t, ok)
	require.NotNil(t, cmd)
	assert.True(t, updated.monitoring)
	assert.False(t, updated.quitting)
	assert.NoError(t, updated.err)
	assert.Error(t, updated.refreshErr)
	assert.Equal(t, instances, updated.instances)
	assert.WithinDuration(t, time.Now().Add(2*time.Minute), updated.nextRetryAt, time.Second)

	view := stripANSI(updated.View())
	assert.Contains(t, view, "STARTING")
	assert.Contains(t, view, "Refresh rate-limited. Retrying in")
	assert.Less(t, strings.Index(view, "Refresh rate-limited. Retrying in"), strings.Index(view, "Last updated:"))
}

func TestStatusModelNonRetryableAPIErrorStillExits(t *testing.T) {
	InitCommonStyles(os.Stdout)
	instances := []api.Instance{{ID: "1", Status: "STARTING"}}
	m := newStatusModel(nil, true, instances, false)

	model, cmd := m.Update(instancesMsg{err: &api.APIError{
		StatusCode: http.StatusUnauthorized,
		Message:    "authentication failed",
	}})

	updated, ok := model.(statusModel)
	require.True(t, ok)
	require.NotNil(t, cmd)
	assert.False(t, updated.monitoring)
	assert.Error(t, updated.err)
	assert.NoError(t, updated.refreshErr)
}

func stripANSI(s string) string {
	var b strings.Builder
	inEscape := false
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if inEscape {
			if ch >= '@' && ch <= '~' {
				inEscape = false
			}
			continue
		}
		if ch == 0x1b {
			inEscape = true
			continue
		}
		b.WriteByte(ch)
	}
	return b.String()
}
