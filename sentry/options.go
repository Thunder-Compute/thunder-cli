package sentry

import "github.com/getsentry/sentry-go"

// EventOptions holds optional settings for capturing events
type EventOptions struct {
	Tags        *Tags
	Extra       *Extra
	Level       *sentry.Level
	Fingerprint []string
}

// Tags is a builder for event tags
type Tags struct {
	tags map[string]string
}

// NewTags creates a new Tags builder
func NewTags() *Tags {
	return &Tags{
		tags: make(map[string]string),
	}
}

// Set adds a tag key-value pair
func (t *Tags) Set(key, value string) *Tags {
	t.tags[key] = value
	return t
}

// ToMap returns the tags as a map
func (t *Tags) ToMap() map[string]string {
	return t.tags
}

// Extra is a builder for extra event data
type Extra struct {
	data map[string]interface{}
}

// NewExtra creates a new Extra builder
func NewExtra() *Extra {
	return &Extra{
		data: make(map[string]interface{}),
	}
}

// Set adds an extra data key-value pair
func (e *Extra) Set(key string, value interface{}) *Extra {
	e.data[key] = value
	return e
}

// ToMap returns the extra data as a map
func (e *Extra) ToMap() map[string]interface{} {
	return e.data
}
