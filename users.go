package fbmsgr

import (
	"encoding/json"
	"errors"
	"net/url"
)

// ProfilePicture gets a URL to a user's profile picture.
func (s *Session) ProfilePicture(fbid string) (*url.URL, error) {
	params, err := s.commonParams()
	if err != nil {
		return nil, err
	}
	params.Set("requests[0][fbid]", fbid)
	params.Set("requests[0][type]", "profile_picture")
	params.Set("requests[0][width]", "50")
	params.Set("requests[0][height]", "50")
	params.Set("requests[0][resize_mode]", "p")
	reqURL := BaseURL + "/ajax/image_source.php?dpr=1"
	resp, err := s.jsonForPost(reqURL, params)
	if err != nil {
		return nil, err
	}
	var respObj struct {
		Payload []struct {
			URI string `json:"uri"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(resp, &respObj); err != nil {
		return nil, err
	}
	if len(respObj.Payload) != 1 {
		return nil, errors.New("unexpected number of results")
	}
	return url.Parse(respObj.Payload[0].URI)
}
