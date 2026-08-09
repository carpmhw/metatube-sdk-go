//go:build optional && !experimental

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestOptionalActorProviderRegistration 驗證 optional build tag 的 actor provider 集合。
func TestOptionalActorProviderRegistration(t *testing.T) {
	assert.Equal(t, []string{"AV-LEAGUE", "Gfriends", "MinnanoAV"}, actorProviderNames())
}
