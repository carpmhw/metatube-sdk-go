//go:build optional

package engine

// Register Optional Providers

import (
	_ "github.com/metatube-community/metatube-sdk-go/provider/av-league"
	_ "github.com/metatube-community/metatube-sdk-go/provider/minnanoav"
)
