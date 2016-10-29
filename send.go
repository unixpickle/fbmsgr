package fbmsgr

import (
	"encoding/json"
	"errors"
	"math/rand"
	"net/url"
	"strconv"
	"time"
)

type EmojiSize string

const (
	SmallEmoji  EmojiSize = "small"
	MediumEmoji           = "medium"
	LargeEmoji            = "large"
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

// SendLike is like SendText, but it sends an emoji at a
// given size.
// It is crutial that the emoji is valid and that the size
// is also valid.
// Otherwise, this can trigger a bug in the web client
// that essentially bricks the conversation.
func (s *Session) SendLike(fbid, emoji string, size EmojiSize) (msgID string, err error) {
	reqParams, err := s.textMessageParams(emoji)
	if err != nil {
		return "", err
	}
	reqParams.Add("other_user_fbid", fbid)
	reqParams.Add("tags[0]", "hot_emoji_size:"+string(size))
	reqParams.Add("tags[1]", "hot_emoji_source:hot_like")
	return s.sendMessage(reqParams)
}

// SendGroupLike is like SendLike, but for a group thread.
func (s *Session) SendGroupLike(groupFBID, emoji string, size EmojiSize) (msgID string, err error) {
	reqParams, err := s.textMessageParams(emoji)
	if err != nil {
		return "", err
	}
	reqParams.Add("thread_fbid", groupFBID)
	reqParams.Add("tags[0]", "hot_emoji_size:"+string(size))
	reqParams.Add("tags[1]", "hot_emoji_source:hot_like")
	return s.sendMessage(reqParams)
}

// SendReadReceipt sends a read receipt to a group chat or
// a chat with an individual user.
func (s *Session) SendReadReceipt(fbid string) error {
	url := BaseURL + "/ajax/mercury/change_read_status.php?dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return err
	}
	values.Set("ids["+fbid+"]", "true")
	values.Set("shouldSendReadReceipt", "true")
	values.Set("watermarkTimestamp", strconv.FormatInt(time.Now().UnixNano()/1000000, 10))
	values.Set("commerce_last_message_type", "non_ad")
	_, err = s.jsonForPost(url, values)
	return err
}

// SendTyping sends a typing notification to a user.
// For group chats, use SendGroupTyping.
func (s *Session) SendTyping(userFBID string, typing bool) error {
	return s.sendTyping(userFBID, userFBID, typing)
}

// SendGroupTyping sends a typing notification to a group.
func (s *Session) SendGroupTyping(groupFBID string, typing bool) error {
	return s.sendTyping(groupFBID, "", typing)
}

func (s *Session) sendTyping(thread, to string, typ bool) error {
	url := BaseURL + "/ajax/messaging/typ.php?dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return err
	}
	values.Set("source", "source:messenger:web")
	values.Set("thread", thread)
	values.Set("to", to)
	if typ {
		values.Set("typ", "1")
	} else {
		values.Set("type", "0")
	}
	_, err = s.jsonForPost(url, values)
	return err
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
