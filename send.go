package fbmsgr

import (
	"math/rand"
	"net/url"
	"strconv"
)

// SendText attempts to send a textual message to the user
// with the given fbid.
func (s *Session) SendText(fbid, message string) error {
	dtsg, err := s.fetchDTSG()
	if err != nil {
		return err
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
	reqParams.Add("action_type", "ma-type:user-generated-message")
	reqParams.Add("body", message)
	reqParams.Add("client", "mercury")
	reqParams.Add("ephemeral_ttl_mode", "0")
	reqParams.Add("fb_dtsg", dtsg)
	reqParams.Add("has_attachment", "false")
	msgID := randomMessageID()
	reqParams.Add("message_id", msgID)
	reqParams.Add("offline_threading_id", msgID)
	reqParams.Add("other_user_fbid", fbid)
	reqParams.Add("source", "source:messenger:web")
	reqParams.Add("timestamp", "1477433223060")

	res, err := s.client.PostForm(BaseURL+"/messaging/send/?dpr=1", reqParams)
	if res != nil {
		res.Body.Close()
	}
	return err
}

func randomMessageID() string {
	res := ""
	for i := 0; i < 19; i++ {
		res += strconv.Itoa(rand.Intn(10))
	}
	return res
}
