package engine

import (
	"sort"

	mt "github.com/metatube-community/metatube-sdk-go/provider"
)

// actorProviderNames 取得目前 binary 編入的 actor provider 名稱。
func actorProviderNames() []string {
	var names []string
	mt.RangeActorFactory(func(name string, _ mt.ActorFactory) bool {
		names = append(names, name)
		return true
	})
	sort.Strings(names)
	return names
}
