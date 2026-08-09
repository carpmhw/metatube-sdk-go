package minnanoav

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"

	mt "github.com/metatube-community/metatube-sdk-go/provider"
)

// TestNewProvider 驗證 MinnanoAV provider 的基本設定。
func TestNewProvider(t *testing.T) {
	provider := New()

	assert.Equal(t, Name, provider.Name())
	assert.Equal(t, "https://www.minnano-av.com", provider.URL().String())
	assert.Equal(t, float64(Priority), provider.Priority())
	assert.Equal(t, language.Japanese, provider.Language())
}

// TestNormalizeActorID 驗證演員 ID 的正規化規則。
func TestNormalizeActorID(t *testing.T) {
	provider := New()

	assert.Equal(t, "665064", provider.NormalizeActorID("665064"))
	assert.Empty(t, provider.NormalizeActorID(""))
	assert.Empty(t, provider.NormalizeActorID("665064.html"))
	assert.Empty(t, provider.NormalizeActorID("actor-665064"))
}

// TestParseActorIDFromURL 驗證演員 URL 的 ID 解析與格式檢查。
func TestParseActorIDFromURL(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	id, err := provider.ParseActorIDFromURL(server.URL + "/actress665064.html?name=%E5%AE%AE%E8%A5%BF")
	require.NoError(t, err)
	assert.Equal(t, "665064", id)

	_, err = provider.ParseActorIDFromURL("https://other.example/actress665064.html")
	assert.Same(t, mt.ErrInvalidURL, err)

	_, err = provider.ParseActorIDFromURL(server.URL + "/actress/665064.html")
	assert.Same(t, mt.ErrInvalidURL, err)
}

// TestGetActorInfoByID 驗證完整演員詳情的欄位解析。
func TestGetActorInfoByID(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/actress665064.html":
			writeFixture(w, actressDetailHTML)
		case "/":
			writeFixture(w, homepageHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	info, err := provider.GetActorInfoByID("665064")
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, "665064", info.ID)
	assert.Equal(t, "宮西ひかる", info.Name)
	assert.Equal(t, "MinnanoAV", info.Provider)
	assert.Equal(t, server.URL+"/actress665064.html", info.Homepage)
	assert.Equal(t, []string{"宮西あゆみ（舞ワイフ）", "藤井知花(熟女JAPAN)"}, []string(info.Aliases))
	assert.Equal(t, []string{server.URL + "/p_actress_125_125/021/665064.jpg"}, []string(info.Images))
	assert.Equal(t, 161, info.Height)
	assert.Equal(t, "B:88 / W:61 / H:86", info.Measurements)
	assert.Equal(t, "E", info.CupSize)
	assert.Equal(t, "A", info.BloodType)
	assert.Equal(t, "東京都", info.Nationality)
	assert.Equal(t, "ダンス", info.Hobby)
	assert.Equal(t, "1999-08-22", time.Time(info.Birthday).Format("2006-01-02"))
	assert.Equal(t, "2022-01-11", time.Time(info.DebutDate).Format("2006-01-02"))
}

// TestGetActorInfoByIDErrors 驗證演員 ID 查詢的錯誤契約。
func TestGetActorInfoByIDErrors(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/actress999999.html":
			http.Redirect(w, r, "/", http.StatusFound)
		case "/actress500.html":
			w.WriteHeader(http.StatusInternalServerError)
		case "/":
			writeFixture(w, homepageHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	_, err := provider.GetActorInfoByID("not-an-id")
	assert.Same(t, mt.ErrInvalidID, err)

	_, err = provider.GetActorInfoByID("999999")
	assert.Same(t, mt.ErrInfoNotFound, err)

	_, err = provider.GetActorInfoByID("500")
	assert.NotErrorIs(t, err, mt.ErrInfoNotFound)
}

// TestGetActorInfoByURL 驗證依 URL 查詢演員詳情。
func TestGetActorInfoByURL(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/actress665064.html" {
			writeFixture(w, actressDetailHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	info, err := provider.GetActorInfoByURL(server.URL + "/actress665064.html")
	require.NoError(t, err)
	assert.Equal(t, "665064", info.ID)
}

// TestGetActorInfoByIDIncompleteMetadata 驗證必要欄位缺失時的錯誤。
func TestGetActorInfoByIDIncompleteMetadata(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/actress665065.html" {
			writeFixture(w, incompleteActorHTML)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	_, err := provider.GetActorInfoByID("665065")
	assert.Same(t, mt.ErrIncompleteMetadata, err)
}

// TestSearchActor 驗證搜尋清單的解析與分頁限制。
func TestSearchActor(t *testing.T) {
	var requests int
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		assert.Equal(t, "actress", r.URL.Query().Get("search_scope"))
		assert.Equal(t, "ひかる", r.URL.Query().Get("search_word"))
		writeFixture(w, actressSearchHTML)
	}))
	defer server.Close()

	results, err := provider.SearchActor("ひかる")
	require.NoError(t, err)
	require.Len(t, results, 2)
	assert.Equal(t, 1, requests)
	assert.Equal(t, "665064", results[0].ID)
	assert.Equal(t, "宮西ひかる", results[0].Name)
	assert.Equal(t, "MinnanoAV", results[0].Provider)
	assert.Equal(t, server.URL+"/actress665064.html", results[0].Homepage)
	assert.Equal(t, []string{server.URL + "/p_actress_125_125/021/665064.jpg"}, []string(results[0].Images))
	assert.Equal(t, "665066", results[1].ID)
}

// TestSearchActorRedirectAndEmpty 驗證精確搜尋轉址與空結果。
func TestSearchActorRedirectAndEmpty(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/actress665064.html" {
			writeFixture(w, actressDetailHTML)
			return
		}
		switch r.URL.Query().Get("search_word") {
		case "宮西ひかる":
			http.Redirect(w, r, "/actress665064.html", http.StatusFound)
		case "不存在":
			writeFixture(w, emptySearchHTML)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	results, err := provider.SearchActor("宮西ひかる")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, "665064", results[0].ID)

	_, err = provider.SearchActor("不存在")
	assert.Same(t, mt.ErrInfoNotFound, err)
}

// TestSearchActorPreservesUpstreamError 驗證上游錯誤不會被誤判為找不到資料。
func TestSearchActorPreservesUpstreamError(t *testing.T) {
	provider, server := newFixtureProvider(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	_, err := provider.SearchActor("ひかる")
	assert.Error(t, err)
	assert.False(t, errors.Is(err, mt.ErrInfoNotFound))
}

// TestParseMeasurements 驗證尺寸與罩杯的 table-driven 解析。
func TestParseMeasurements(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantHeight int
		wantBust   int
		wantCup    string
		wantWaist  int
		wantHips   int
	}{
		{
			name:       "complete size",
			input:      "T161 / B88(Eカップ) / W61 / H86 / S",
			wantHeight: 161,
			wantBust:   88,
			wantCup:    "E",
			wantWaist:  61,
			wantHips:   86,
		},
		{
			name:       "missing measurements",
			input:      "T150 / B80 / W55",
			wantHeight: 150,
			wantBust:   80,
			wantWaist:  55,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			height, bust, cup, waist, hips := parseMeasurements(tt.input)
			assert.Equal(t, tt.wantHeight, height)
			assert.Equal(t, tt.wantBust, bust)
			assert.Equal(t, tt.wantCup, cup)
			assert.Equal(t, tt.wantWaist, waist)
			assert.Equal(t, tt.wantHips, hips)
		})
	}
}

// TestParseDebutDate 驗證出道作品日期的 table-driven 解析。
func TestParseDebutDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "full Japanese date", input: "作品名（2022年01月 11日）", want: "2022-01-11"},
		{name: "date absent", input: "作品名", want: "0001-01-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, time.Time(parseDebutDate(tt.input)).Format("2006-01-02"))
		})
	}
}

// newFixtureProvider 建立使用 httptest server 的 MinnanoAV provider。
func newFixtureProvider(t *testing.T, handler http.Handler) (*MinnanoAV, *httptest.Server) {
	t.Helper()
	server := httptest.NewServer(handler)
	return newWithBaseURL(server.URL), server
}

// writeFixture 將 HTML fixture 寫入測試 HTTP 回應。
func writeFixture(w http.ResponseWriter, fixture string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(fixture))
}

const actressDetailHTML = `<!doctype html>
<html><head><link rel="canonical" href="/actress665064.html"></head><body>
<section id="main-section">
<h1>宮西ひかる<span>みやにしひかる / Miyanisi Hikaru</span></h1>
<div class="actress-header"><div class="act-area"><div class="thumb"><img src="/p_actress_125_125/021/665064.jpg"></div></div></div>
<div class="act-profile"><table>
<tr><td><span>別名</span></td><td><p>宮西あゆみ（舞ワイフ）</p><p>藤井知花(熟女JAPAN)</p></td></tr>
<tr><td><span>生年月日</span></td><td><p>1999年08月22日</p></td></tr>
<tr><td><span>サイズ</span></td><td><p>T161 / B88(Eカップ) / W61 / H86 / S</p></td></tr>
<tr><td><span>血液型</span></td><td><p>A型</p></td></tr>
<tr><td><span>出身地</span></td><td><p>東京都</p></td></tr>
<tr><td><span>趣味・特技</span></td><td><p>ダンス</p></td></tr>
<tr><td><span>デビュー作品</span></td><td><p>作品名（2022年01月 11日）</p></td></tr>
</table></div>
</section></body></html>`

const incompleteActorHTML = `<!doctype html><html><body><section id="main-section"><h1></h1></section></body></html>`

const homepageHTML = `<!doctype html><html><body><section class="site_body front"></section></body></html>`

const actressSearchHTML = `<!doctype html><html><body>
<main id="main-area"><main class="main-column list-table"><table class="tbllist actress"><tbody>
<tr><td><a href="actress665064.html?宮西ひかる"><img src="p_actress_125_125/021/665064.jpg"></a></td><td class="details"><h2 class="ttl"><a href="actress665064.html?宮西ひかる">宮西ひかる</a></h2><a href="actress665064.html" class="detail-link">詳情</a></td></tr>
<tr><td><a href="actress665066.html"><img src="p_actress_125_125/021/665066.jpg"></a></td><td class="details"><h2 class="ttl"><a href="actress665066.html">別的ひかる</a></h2><a href="actress665066.html" class="detail-link">詳情</a></td></tr>
<tr><td><a href="?page=2">2</a></td></tr>
</tbody></table></main></main></body></html>`

const emptySearchHTML = `<!doctype html><html><body><main id="main-area"><main class="main-column list-table"><table class="tbllist actress"><tbody></tbody></table></main></main></body></html>`
