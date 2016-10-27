package fbmsgr

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/url"
	"strconv"
	"time"
)

// SendText attempts to send a textual message to the user
// with the given fbid.
func (s *Session) SendText(fbid, message string) (msgID string, err error) {
	reqParams, err := s.textMessageParams(message)
	if err != nil {
		return "", err
	}
	reqParams.Add("other_user_fbid", fbid)
	return s.sendMessage(reqParams)
}

// SendGroupText is like SendText, but the message is sent
// to a group chat rather than to an individual.
func (s *Session) SendGroupText(groupFBID, message string) (msgID string, err error) {
	reqParams, err := s.textMessageParams(message)
	if err != nil {
		return "", err
	}
	reqParams.Add("thread_fbid", groupFBID)
	return s.sendMessage(reqParams)
}

func (s *Session) textMessageParams(body string) (url.Values, error) {
	reqParams, err := s.commonParams()
	if err != nil {
		return nil, err
	}

	reqParams.Add("action_type", "ma-type:user-generated-message")
	reqParams.Add("body", body)
	reqParams.Add("ephemeral_ttl_mode", "0")
	reqParams.Add("has_attachment", "false")
	msgID := randomMessageID()
	reqParams.Add("message_id", msgID)
	reqParams.Add("offline_threading_id", msgID)
	reqParams.Add("source", "source:messenger:web")

	timestamp := time.Now().UnixNano() / 1000000
	ts := strconv.FormatInt(timestamp, 10)
	reqParams.Add("timestamp", ts)

	return reqParams, nil
}

func (s *Session) sendMessage(values url.Values) (mid string, err error) {
	response, err := s.jsonForPost(BaseURL+"/messaging/send/?dpr=1", values)
	if err != nil {
		return "", err
	}
	var obj struct {
		Payload struct {
			Actions []struct {
				MessageID string `json:"message_id"`
			} `json:"actions"`
		} `json:"payload"`
	}
	json.Unmarshal(response, &obj)
	for _, x := range obj.Payload.Actions {
		if x.MessageID != "" {
			return x.MessageID, nil
		}
	}
	return "", errors.New("no message ID in response")
}

func randomMessageID() string {
	res := "6"
	for i := 1; i < 18; i++ {
		res += strconv.Itoa(rand.Intn(10))
	}
	return res
}
