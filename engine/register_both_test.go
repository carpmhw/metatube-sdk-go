//go:build experimental && optional

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCombinedActorProviderRegistration 驗證同時指定兩個 build tag 時的 actor provider 集合。
func TestCombinedActorProviderRegistration(t *testing.T) {
	assert.Equal(t, []string{"AV-LEAGUE", "Gfriends", "MinnanoAV", "ThePornDBActor"}, actorProviderNames())
}
