package minnanoav

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
	"golang.org/x/net/html"
	"golang.org/x/text/language"
	dt "gorm.io/datatypes"

	"github.com/metatube-community/metatube-sdk-go/common/parser"
	"github.com/metatube-community/metatube-sdk-go/model"
	"github.com/metatube-community/metatube-sdk-go/provider"
	"github.com/metatube-community/metatube-sdk-go/provider/internal/scraper"
)

var (
	_ provider.ActorProvider = (*MinnanoAV)(nil)
	_ provider.ActorSearcher = (*MinnanoAV)(nil)
)

const (
	Name     = "MinnanoAV"
	Priority = 1001
)

const (
	baseURL    = "https://www.minnano-av.com"
	actorPath  = "/actress%s.html"
	searchPath = "/search_result.php"
)

var (
	actorIDPattern   = regexp.MustCompile(`^\d+$`)
	actorURLPattern  = regexp.MustCompile(`^/actress(\d+)\.html$`)
	debutDatePattern = regexp.MustCompile(`（\s*(\d{4})年\s*(\d{1,2})月\s*(\d{1,2})日\s*）`)
	heightPattern    = regexp.MustCompile(`(?:^|/)\s*T(\d+)`)
	bustPattern      = regexp.MustCompile(`(?:^|/)\s*B(\d+)`)
	waistPattern     = regexp.MustCompile(`(?:^|/)\s*W(\d+)`)
	hipsPattern      = regexp.MustCompile(`(?:^|/)\s*H(\d+)`)
	cupPattern       = regexp.MustCompile(`(?i)([A-Z])カップ`)
)

// MinnanoAV 提供 minnano-av.com 的演員資料與搜尋功能。
type MinnanoAV struct {
	*scraper.Scraper
}

// New 建立使用公開網站的 MinnanoAV provider。
func New() *MinnanoAV {
	return newWithBaseURL(baseURL)
}

// newWithBaseURL 建立指定 base URL 的 provider，供隔離測試使用。
func newWithBaseURL(rawURL string) *MinnanoAV {
	return &MinnanoAV{Scraper: scraper.NewDefaultScraper(
		Name, rawURL, Priority, language.Japanese,
		scraper.WithDisableCookies(),
	)}
}

// NormalizeActorID 驗證並正規化 MinnanoAV 演員 ID。
func (m *MinnanoAV) NormalizeActorID(id string) string {
	if !actorIDPattern.MatchString(id) {
		return ""
	}
	return id
}

// ParseActorIDFromURL 從 MinnanoAV URL 解析演員 ID。
func (m *MinnanoAV) ParseActorIDFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() != m.URL().Hostname() {
		return "", provider.ErrInvalidURL
	}
	match := actorURLPattern.FindStringSubmatch(u.EscapedPath())
	if len(match) != 2 {
		return "", provider.ErrInvalidURL
	}
	if id := m.NormalizeActorID(match[1]); id != "" {
		return id, nil
	}
	return "", provider.ErrInvalidURL
}

// GetActorInfoByID 依演員 ID 取得演員資料。
func (m *MinnanoAV) GetActorInfoByID(id string) (*model.ActorInfo, error) {
	id = m.NormalizeActorID(id)
	if id == "" {
		return nil, provider.ErrInvalidID
	}
	return m.getActorInfoByURL(m.actorURL(id), id)
}

// GetActorInfoByURL 依演員 URL 取得演員資料。
func (m *MinnanoAV) GetActorInfoByURL(rawURL string) (*model.ActorInfo, error) {
	id, err := m.ParseActorIDFromURL(rawURL)
	if err != nil {
		return nil, err
	}
	return m.getActorInfoByURL(rawURL, id)
}

// SearchActor 依演員姓名搜尋演員。
func (m *MinnanoAV) SearchActor(keyword string) (results []*model.ActorSearchResult, err error) {
	if strings.TrimSpace(keyword) == "" {
		return nil, provider.ErrInvalidKeyword
	}

	c := m.ClonedCollector()
	c.ParseHTTPErrorResponse = true
	c.OnError(func(_ *colly.Response, requestErr error) {
		err = requestErr
	})
	c.OnResponse(func(response *colly.Response) {
		if response.StatusCode != http.StatusOK {
			err = fmt.Errorf("minnanoav returned HTTP status %d", response.StatusCode)
			return
		}
		if m.isActorURL(response.Request.URL) {
			id, parseErr := m.ParseActorIDFromURL(response.Request.URL.String())
			if parseErr != nil {
				err = parseErr
				return
			}
			info := m.newActorInfo(id, response.Request.URL.String())
			if err = m.parseActorInfo(info, response.Body); err != nil {
				return
			}
			if !info.IsValid() {
				err = provider.ErrIncompleteMetadata
				return
			}
			results = append(results, info.ToSearchResult())
			return
		}
		if !m.isSearchURL(response.Request.URL) {
			err = provider.ErrInfoNotFound
			return
		}
		err = m.parseSearchResults(response, &results)
	})

	visitErr := c.Visit(m.searchURL(keyword))
	if err != nil {
		return nil, err
	}
	if visitErr != nil {
		return nil, visitErr
	}
	if len(results) == 0 {
		return nil, provider.ErrInfoNotFound
	}
	return results, nil
}

// getActorInfoByURL 讀取演員頁面並驗證完整必要欄位。
func (m *MinnanoAV) getActorInfoByURL(rawURL, id string) (*model.ActorInfo, error) {
	info := m.newActorInfo(id, rawURL)
	var callbackErr error
	c := m.ClonedCollector()
	c.ParseHTTPErrorResponse = true
	c.OnError(func(_ *colly.Response, requestErr error) {
		callbackErr = requestErr
	})
	c.OnResponse(func(response *colly.Response) {
		if response.StatusCode != http.StatusOK {
			callbackErr = fmt.Errorf("minnanoav returned HTTP status %d", response.StatusCode)
			return
		}
		if !m.isActorURL(response.Request.URL) {
			callbackErr = provider.ErrInfoNotFound
			return
		}
		callbackErr = m.parseActorInfo(info, response.Body)
	})

	visitErr := c.Visit(rawURL)
	if callbackErr != nil {
		return nil, callbackErr
	}
	if visitErr != nil {
		return nil, visitErr
	}
	if !info.IsValid() {
		return nil, provider.ErrIncompleteMetadata
	}
	return info, nil
}

// newActorInfo 建立 MinnanoAV 演員資料的初始結構。
func (m *MinnanoAV) newActorInfo(id, homepage string) *model.ActorInfo {
	return &model.ActorInfo{
		ID:       id,
		Provider: m.Name(),
		Homepage: homepage,
		Aliases:  []string{},
		Images:   []string{},
	}
}

// actorURL 建立指定演員 ID 的詳情頁 URL。
func (m *MinnanoAV) actorURL(id string) string {
	u := *m.URL()
	u.Path = strings.TrimRight(u.Path, "/") + fmt.Sprintf(actorPath, id)
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

// searchURL 建立帶有演員搜尋條件的 URL。
func (m *MinnanoAV) searchURL(keyword string) string {
	u := *m.URL()
	u.Path = strings.TrimRight(u.Path, "/") + searchPath
	query := u.Query()
	query.Set("search_scope", "actress")
	query.Set("search_word", keyword)
	query.Set("search", "Go")
	u.RawQuery = query.Encode()
	u.Fragment = ""
	return u.String()
}

// isActorURL 判斷 URL 是否為目前 provider 的演員詳情頁。
func (m *MinnanoAV) isActorURL(u *url.URL) bool {
	return u != nil && u.Hostname() == m.URL().Hostname() && actorURLPattern.MatchString(u.EscapedPath())
}

// isSearchURL 判斷 URL 是否為目前 provider 的演員搜尋頁。
func (m *MinnanoAV) isSearchURL(u *url.URL) bool {
	return u != nil && u.Hostname() == m.URL().Hostname() && u.EscapedPath() == searchPath
}

// parseActorInfo 將演員詳情 HTML 解析至 ActorInfo。
func (m *MinnanoAV) parseActorInfo(info *model.ActorInfo, body []byte) error {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(body))
	if err != nil {
		return err
	}

	info.Name = directText(document.Find("#main-section > h1").First())
	document.Find("#main-section .actress-header .act-area .thumb > img").First().Each(func(_ int, selection *goquery.Selection) {
		if src, ok := selection.Attr("src"); ok {
			if imageURL := m.absoluteURL(src); imageURL != "" {
				info.Images = append(info.Images, imageURL)
			}
		}
	})

	document.Find("#main-section .act-profile table tr").Each(func(_ int, row *goquery.Selection) {
		label := strings.TrimSpace(row.Find("td > span").First().Text())
		value := strings.TrimSpace(row.Find("td > p").First().Text())
		switch label {
		case "別名":
			row.Find("td > p").Each(func(_ int, alias *goquery.Selection) {
				if value := strings.TrimSpace(alias.Text()); value != "" {
					info.Aliases = append(info.Aliases, value)
				}
			})
		case "生年月日":
			info.Birthday = parser.ParseDate(value)
		case "サイズ":
			height, bust, cup, waist, hips := parseMeasurements(value)
			info.Height = height
			if bust != 0 && waist != 0 && hips != 0 {
				info.Measurements = fmt.Sprintf("B:%d / W:%d / H:%d", bust, waist, hips)
			}
			info.CupSize = cup
		case "血液型":
			info.BloodType = strings.TrimSpace(strings.TrimSuffix(value, "型"))
		case "出身", "出身地":
			info.Nationality = value
		case "趣味・特技":
			info.Hobby = value
		case "デビュー作品":
			info.DebutDate = parseDebutDate(value)
		}
	})
	return nil
}

// parseSearchResults 將演員搜尋頁第一頁解析為搜尋結果。
func (m *MinnanoAV) parseSearchResults(response *colly.Response, results *[]*model.ActorSearchResult) error {
	document, err := goquery.NewDocumentFromReader(bytes.NewReader(response.Body))
	if err != nil {
		return err
	}
	document.Find("table.tbllist.actress tr").Each(func(_ int, row *goquery.Selection) {
		if row.Find("td.details").Length() == 0 {
			return
		}
		link := row.Find("td.details > a.detail-link").First().AttrOr("href", "")
		if link == "" {
			link = row.Find("td.details h2.ttl > a").First().AttrOr("href", "")
		}
		homepage := m.absoluteURLFor(response.Request.URL.String(), link)
		id, parseErr := m.ParseActorIDFromURL(homepage)
		if parseErr != nil {
			return
		}
		name := strings.TrimSpace(row.Find("td.details h2.ttl > a").First().Text())
		image := m.absoluteURLFor(response.Request.URL.String(), row.Find("td:first-child > a > img").First().AttrOr("src", ""))
		if id == "" || name == "" || homepage == "" {
			return
		}
		result := &model.ActorSearchResult{
			ID:       id,
			Name:     name,
			Provider: m.Name(),
			Homepage: homepage,
			Aliases:  []string{},
			Images:   []string{},
		}
		if image != "" {
			result.Images = append(result.Images, image)
		}
		*results = append(*results, result)
	})
	return nil
}

// absoluteURL 將相對 URL 轉換為 provider base URL 下的絕對 URL。
func (m *MinnanoAV) absoluteURL(rawURL string) string {
	return m.absoluteURLFor(m.URL().String(), rawURL)
}

// absoluteURLFor 以指定 URL 為基準解析相對 URL。
func (m *MinnanoAV) absoluteURLFor(base, rawURL string) string {
	if strings.TrimSpace(rawURL) == "" {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil {
		return ""
	}
	resolved, err := baseURL.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return ""
	}
	return resolved.String()
}

// directText 取得選擇器下未被子元素包住的文字。
func directText(selection *goquery.Selection) string {
	var text strings.Builder
	selection.Contents().Each(func(_ int, child *goquery.Selection) {
		if node := child.Get(0); node != nil && node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
	})
	return strings.TrimSpace(text.String())
}

// parseMeasurements 解析身高、胸圍、罩杯、腰圍及臀圍。
func parseMeasurements(raw string) (height, bust int, cup string, waist, hips int) {
	if match := heightPattern.FindStringSubmatch(raw); len(match) == 2 {
		height = parser.ParseInt(match[1])
	}
	if match := bustPattern.FindStringSubmatch(raw); len(match) == 2 {
		bust = parser.ParseInt(match[1])
	}
	if match := waistPattern.FindStringSubmatch(raw); len(match) == 2 {
		waist = parser.ParseInt(match[1])
	}
	if match := hipsPattern.FindStringSubmatch(raw); len(match) == 2 {
		hips = parser.ParseInt(match[1])
	}
	if match := cupPattern.FindStringSubmatch(raw); len(match) == 2 {
		cup = strings.ToUpper(match[1])
	}
	return
}

// parseDebutDate 從出道作品文字中解析完整出道日期。
func parseDebutDate(raw string) (date dt.Date) {
	match := debutDatePattern.FindStringSubmatch(raw)
	if len(match) != 4 {
		return
	}
	return dt.Date(time.Date(
		parser.ParseInt(match[1]),
		time.Month(parser.ParseInt(match[2])),
		parser.ParseInt(match[3]),
		0, 0, 0, 0, time.UTC,
	))
}

// init 註冊 MinnanoAV actor provider。
func init() {
	provider.Register(Name, New)
}
