package fbmsgr

import (
	"bytes"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/yhat/scrape"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	BaseURL          = "https://www.messenger.com"
	SpoofedUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.11; rv:43.0) Gecko/20100101 Firefox/43.0"
)

// A Session is an authenticated session with the
// messenger backend.
type Session struct {
	client *http.Client

	userID string

	fbDTSGLock sync.Mutex
	fbDTSG     string

	pollLock sync.Mutex
	pollChan <-chan Event
	pollErr  error

	randLock sync.Mutex
	randGen  *rand.Rand
}

// Auth creates a new Session by authenticating with the
// Facebook backend.
func Auth(user, password string) (*Session, error) {
	jar, _ := cookiejar.New(nil)
	client := &http.Client{
		Jar: jar,
	}

	loginPage, err := client.Get(BaseURL + "/")
	if loginPage != nil {
		defer loginPage.Body.Close()
	}
	if err != nil {
		return nil, errors.New("request login page: " + err.Error())
	}
	root, err := html.Parse(loginPage.Body)
	if err != nil {
		return nil, errors.New("parse login page: " + err.Error())
	}
	formValues, action, err := loginFormValues(root)
	if err != nil {
		return nil, errors.New("read login form: " + err.Error())
	}

	if err := requestLoginCookies(client, root); err != nil {
		return nil, errors.New("gather cookies: " + err.Error())
	}

	formValues.Set("email", user)
	formValues.Set("pass", password)
	formValues.Set("persistent", "1")
	formValues.Set("login", "1")

	body := []byte(formValues.Encode())
	req, err := http.NewRequest("POST", BaseURL+action, bytes.NewBuffer(body))
	if err != nil {
		return nil, errors.New("create login request: " + err.Error())
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
	req.Header.Set("User-Agent", SpoofedUserAgent)
	req.Header.Set("Referer", BaseURL+"/")
	postRes, err := client.Do(req)
	if postRes != nil {
		defer postRes.Body.Close()
	}
	if err != nil {
		return nil, errors.New("failed to login: " + err.Error())
	}

	if postRes.Request.URL.Path == "/" {
		return sessionForHomepage(client, postRes.Body)
	}

	return nil, errors.New("login failed")
}

// FBID returns the authenticated user's FBID.
func (s *Session) FBID() string {
	return s.userID
}

func sessionForHomepage(c *http.Client, body io.Reader) (*Session, error) {
	root, err := html.Parse(body)
	if err != nil {
		return nil, errors.New("parse homepage: " + err.Error())
	}
	userID, err := findJSField(root, "USER_ID")
	if err != nil {
		return nil, errors.New("find USER_ID: " + err.Error())
	}
	return &Session{
		client:  c,
		userID:  userID,
		randGen: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func requestLoginCookies(c *http.Client, body *html.Node) error {
	reqID, err := findJSField(body, "initialRequestID")
	if err != nil {
		return errors.New("find initialRequestID: " + err.Error())
	}
	identifier, err := findJSField(body, "identifier")
	if err != nil {
		return errors.New("find identifier: " + err.Error())
	}
	dAtr, err := findJSField(body, "_js_datr")
	if err != nil {
		return errors.New("find _js_datr: " + err.Error())
	}

	cookieGetter := "https://www.facebook.com/login/messenger_dot_com_iframe/" +
		"?redirect_uri=https%3A%2F%2Fwww.messenger.com%2Flogin%2Ffb_iframe_target%2F" +
		"%3Finitial_request_id%3D" + reqID + "&identifier=" + identifier +
		"&initial_request_id=" + reqID

	req, err := http.NewRequest("GET", cookieGetter, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Referer", "https://www.messenger.com")
	req.Header.Set("User-Agent", SpoofedUserAgent)
	resp, err := c.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return err
	}

	u, err := url.Parse(BaseURL)
	if err != nil {
		panic(err)
	}
	c.Jar.SetCookies(u, []*http.Cookie{&http.Cookie{
		Name:  "_js_datr",
		Value: dAtr,
	}})
	getURL := BaseURL + "/login/fb_iframe_target/?userid=0&initial_request_id=" +
		reqID
	req, err = http.NewRequest("GET", getURL, nil)
	req.Header.Set("Referer", "https://www.messenger.com")
	req.Header.Set("User-Agent", SpoofedUserAgent)
	resp, err = c.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	return err
}

func loginFormValues(body *html.Node) (vals url.Values, action string, err error) {
	form, ok := scrape.Find(body, scrape.ById("login_form"))
	if !ok {
		return nil, "", errors.New("form not found")
	}

	action = scrape.Attr(form, "action")
	if action == "" {
		return nil, "", errors.New("no action attribute")
	}

	inputs := scrape.FindAll(form, scrape.ByTag(atom.Input))
	vals = url.Values{}
	for _, input := range inputs {
		if scrape.Attr(input, "type") == "hidden" {
			vals.Set(scrape.Attr(input, "name"), scrape.Attr(input, "value"))
		}
	}

	return
}

func findJSField(body *html.Node, field string) (string, error) {
	var out bytes.Buffer
	html.Render(&out, body)
	expr := regexp.MustCompile("\"" + field + "\"(,|:)\"(.*?)\"")
	match := expr.FindSubmatch(out.Bytes())
	if match == nil {
		return "", errors.New("could not locate JS field")
	}
	return string(match[2]), nil
}
