package fbmsgr

import (
	"errors"

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
