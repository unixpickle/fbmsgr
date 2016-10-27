package fbmsgr

import (
	"math/rand"
	"strconv"
	"time"
)

// SendText attempts to send a textual message to the user
// with the given fbid.
func (s *Session) SendText(fbid, message string) error {
	reqParams, err := s.commonParams()
	if err != nil {
		return err
	}
	reqParams.Add("action_type", "ma-type:user-generated-message")
	reqParams.Add("body", message)
	reqParams.Add("ephemeral_ttl_mode", "0")
	reqParams.Add("has_attachment", "false")
	msgID := randomMessageID()
	reqParams.Add("message_id", msgID)
	reqParams.Add("offline_threading_id", msgID)
	reqParams.Add("other_user_fbid", fbid)
	reqParams.Add("source", "source:messenger:web")

	timestamp := time.Now().UnixNano() / 1000000
	ts := strconv.FormatInt(timestamp, 10)
	reqParams.Add("timestamp", ts)

	res, err := s.client.PostForm(BaseURL+"/messaging/send/?dpr=1", reqParams)
	if res != nil {
		res.Body.Close()
	}
	return err
}

func randomMessageID() string {
	res := "6"
	for i := 1; i < 18; i++ {
		res += strconv.Itoa(rand.Intn(10))
	}
	return res
}
