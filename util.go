package fbmsgr

import (
	"errors"
	"io/ioutil"
	"net/url"

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
	resp, err := s.client.PostForm(url, params)
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
		return nil, errors.New("jsonForPost: response too short")
	}
	return body[9:], nil
}
