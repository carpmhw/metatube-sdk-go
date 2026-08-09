package minnanoav

import (
	"testing"

	"github.com/metatube-community/metatube-sdk-go/provider/internal/testkit"
)

// TestMinnanoAV_GetActorInfoByID 驗證 Minnano-AV 線上演員詳情查詢。
func TestMinnanoAV_GetActorInfoByID(t *testing.T) {
	testkit.Test(t, New, []string{"665064"})
}

// TestMinnanoAV_SearchActor 驗證 Minnano-AV 線上演員搜尋。
func TestMinnanoAV_SearchActor(t *testing.T) {
	testkit.Test(t, New, []string{"宮西ひかる"})
}
