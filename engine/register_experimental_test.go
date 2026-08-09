//go:build experimental && !optional

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestExperimentalActorProviderRegistration 驗證 experimental build tag 的 actor provider 集合。
func TestExperimentalActorProviderRegistration(t *testing.T) {
	assert.Equal(t, []string{"Gfriends", "ThePornDBActor"}, actorProviderNames())
}
