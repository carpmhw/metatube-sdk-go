//go:build !optional && !experimental

package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDefaultActorProviderRegistration 驗證無 build tag 時的 actor provider 集合。
func TestDefaultActorProviderRegistration(t *testing.T) {
	assert.Equal(t, []string{"Gfriends"}, actorProviderNames())
}
