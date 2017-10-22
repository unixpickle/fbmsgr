package fbmsgr

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const dtsgTimeout = time.Hour * 8

// fetchDTSG fetches a usable value for the fb_dtsg field
// present in many AJAX requests.
//
// The value is cached, so this may not block.
func (s *Session) fetchDTSG() (string, error) {
	s.fbDTSGLock.Lock()
	defer s.fbDTSGLock.Unlock()
	if s.fbDTSG != "" {
		if time.Since(s.fbDTSGTime) > dtsgTimeout {
			s.fbDTSG = ""
		} else {
			return s.fbDTSG, nil
		}
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
	s.fbDTSGTime = time.Now()
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

// graphQLDoc runs a GraphQL query with a "doc_id".
//
// If the query is successful, the resulting data is
// unmarshalled into dataOut.
func (s *Session) graphQLDoc(docID string, params map[string]interface{},
	dataOut interface{}) error {
	reqParams, err := s.commonParams()
	if err != nil {
		return err
	}
	reqObj := map[string]interface{}{"doc_id": docID, "query_params": params}
	reqJSON, err := json.Marshal(map[string]interface{}{"o0": reqObj})
	if err != nil {
		return err
	}
	reqParams.Add("queries", string(reqJSON))

	resp, err := s.client.PostForm(BaseURL+"/api/graphqlbatch", reqParams)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	decoder := json.NewDecoder(resp.Body)

	var respObj struct {
		Object struct {
			Data   interface{} `json:"data"`
			Errors []struct {
				Message string `json:"message"`
			} `json:"errors"`
		} `json:"o0"`
		Error struct {
			Message string `json:"description"`
		} `json:"error"`
	}
	respObj.Object.Data = dataOut
	if err := decoder.Decode(&respObj); err != nil {
		return err
	}
	if len(respObj.Object.Errors) > 0 {
		return errors.New("GraphQL error: " + respObj.Object.Errors[0].Message)
	} else if respObj.Error.Message != "" {
		return errors.New("GraphQL error: " + respObj.Error.Message)
	}
	return nil
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

// jsonForGetContext is like jsonForGet but with an added
// request context.
func (s *Session) jsonForGetContext(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	return jsonForResp(s.client.Do(req))
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
