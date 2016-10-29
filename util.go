package fbmsgr

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// fetchDTSG fetches a usable value for the fb_dtsg field
// present in many AJAX requests.
//
// The value is cached, so this may not block.
func (s *Session) fetchDTSG() (string, error) {
	s.fbDTSGLock.Lock()
	defer s.fbDTSGLock.Unlock()
	if s.fbDTSG != "" {
		return s.fbDTSG, nil
	}
	homepage, err := s.client.Get(BaseURL)
	if homepage != nil {
		defer homepage.Body.Close()
	}
	if err != nil {
		return "", errors.New("fetch dtsg: " + err.Error())
	}
	parsed, err := html.Parse(homepage.Body)
	if err != nil {
		return "", errors.New("fetch dtsg: " + err.Error())
	}
	const fieldName = `DTSGInitialData",\[\],{"token`
	keyVal, err := findJSField(parsed, fieldName)
	if err != nil {
		return "", errors.New("fetch dtsg: " + err.Error())
	}
	s.fbDTSG = keyVal
	return s.fbDTSG, nil
}

// commonParams generates a set of parameters which are
// passed to most observed JSON endpoints.
func (s *Session) commonParams() (url.Values, error) {
	dtsg, err := s.fetchDTSG()
	if err != nil {
		return nil, err
	}

	reqParams := url.Values{}
	reqParams.Add("__a", "1")
	reqParams.Add("__af", "o")
	reqParams.Add("__be", "-1")
	reqParams.Add("__pc", "EXP1:messengerdotcom_pkg")
	reqParams.Add("__req", "14")
	reqParams.Add("__rev", "2643465")
	reqParams.Add("__srp_t", "1477432416")
	reqParams.Add("__user", s.userID)
	reqParams.Add("client", "mercury")
	reqParams.Add("fb_dtsg", dtsg)

	return reqParams, nil
}

// jsonForPost posts the form and returns the raw JSON
// from the response.
func (s *Session) jsonForPost(url string, params url.Values) ([]byte, error) {
	return jsonForResp(s.client.PostForm(url, params))
}

// jsonForGet runs a get and returns the raw JSON.
func (s *Session) jsonForGet(url string) ([]byte, error) {
	return jsonForResp(s.client.Get(url))
}

// jsonForResp returns the json for the response.
func jsonForResp(resp *http.Response, err error) ([]byte, error) {
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) < 9 {
		return nil, errors.New("jsonForResp: response too short")
	}
	return body[9:], nil
}

// putJSONIntoObject turns source into JSON, then
// unmarshals it back into the destination.
func putJSONIntoObject(source, dest interface{}) error {
	encoded, err := json.Marshal(source)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, &dest)
}

// stripFBIDPrefix turns strings like "fbid:12" into "12".
// If the string does not start with "fbid:", no change is
// performed.
func stripFBIDPrefix(s string) string {
	if strings.HasPrefix(s, "fbid:") {
		return s[5:]
	}
	return s
}

// floatIDToString converts a floating point to an integer
// string.
// If id is 0, it returns "" for convenience.
func floatIDToString(id float64) string {
	if id == 0 {
		return ""
	}
	return strconv.FormatInt(int64(id), 10)
}

// canonicalFBID converts a float64 or a string into a
// textual FBID with no prefix.
func canonicalFBID(val interface{}) string {
	switch val := val.(type) {
	case string:
		return stripFBIDPrefix(val)
	case float64:
		return floatIDToString(val)
	}
	return ""
}
