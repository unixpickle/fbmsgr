package fbmsgr

import (
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"path"
	"strconv"
	"time"
)

type EmojiSize string

const (
	SmallEmoji  EmojiSize = "small"
	MediumEmoji           = "medium"
	LargeEmoji            = "large"
)

// UploadResult is the result of uploading a file.
type UploadResult struct {
	// One of the following strings will be non-nil after
	// a successful upload.
	VideoID string
	FileID  string
	AudioID string
	ImageID string
}

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

// SendAttachment sends an attachment to another user.
// For group chats, use SendGroupAttachment.
func (s *Session) SendAttachment(userFBID string, a *UploadResult) (mid string, err error) {
	reqParams, err := s.attachmentMessageParams(a)
	if err != nil {
		return "", err
	}
	reqParams.Add("other_user_fbid", userFBID)
	return s.sendMessage(reqParams)
}

// SendGroupAttachment is like SendAttachment for groups.
func (s *Session) SendGroupAttachment(groupFBID string, a *UploadResult) (mid string, err error) {
	reqParams, err := s.attachmentMessageParams(a)
	if err != nil {
		return "", err
	}
	reqParams.Add("thread_fbid", groupFBID)
	return s.sendMessage(reqParams)
}

// Upload uploads a file to be sent as an attachment.
func (s *Session) Upload(filename string, file io.Reader) (*UploadResult, error) {
	values, err := s.commonParams()
	if err != nil {
		return nil, err
	}
	values.Set("dpr", "1")

	reader, writer, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	mp := multipart.NewWriter(writer)

	url := BaseURL + "/ajax/mercury/upload.php?" + values.Encode()
	req, err := http.NewRequest("POST", url, reader)
	req.Header.Set("Content-Type", mp.FormDataContentType())

	errChan := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if err != nil {
				errChan <- err
			}
			closeErr := mp.Close()
			if err == nil && closeErr != nil {
				errChan <- closeErr
			}
			close(errChan)
			writer.Close()
		}()
		header := textproto.MIMEHeader{}
		ext := path.Ext(filename)
		header.Set("Content-Disposition", "form-data; name=\"upload_1000\"; filename=\"file"+
			ext+"\"")
		header.Set("Content-Type", mime.TypeByExtension(ext))
		sender, err := mp.CreatePart(header)
		if err != nil {
			return
		}
		_, err = io.Copy(sender, file)
	}()

	body, err := jsonForResp(s.client.Do(req))
	if err != nil {
		return nil, err
	}
	if err := <-errChan; err != nil {
		return nil, err
	}

	var msg struct {
		Payload struct {
			Meta []struct {
				VideoID float64 `json:"video_id"`
				FileID  float64 `json:"file_id"`
				AudioID float64 `json:"audio_id"`
				ImageID float64 `json:"image_id"`
			} `json:"metadata"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, err
	}
	if len(msg.Payload.Meta) != 1 {
		return nil, errors.New("unexpected result")
	}
	return &UploadResult{
		VideoID: floatIDToString(msg.Payload.Meta[0].VideoID),
		AudioID: floatIDToString(msg.Payload.Meta[0].AudioID),
		ImageID: floatIDToString(msg.Payload.Meta[0].ImageID),
		FileID:  floatIDToString(msg.Payload.Meta[0].FileID),
	}, nil
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

func (s *Session) attachmentMessageParams(a *UploadResult) (url.Values, error) {
	values, err := s.textMessageParams("")
	if err != nil {
		return nil, err
	}
	values.Del("body")
	values.Set("has_attachment", "true")
	if a.FileID != "" {
		values.Set("file_ids[0]", a.FileID)
	} else if a.AudioID != "" {
		values.Set("audio_ids[0]", a.AudioID)
	} else if a.ImageID != "" {
		values.Set("image_ids[0]", a.ImageID)
	} else if a.VideoID != "" {
		values.Set("video_ids[0]", a.VideoID)
	} else {
		return nil, errors.New("no attachment ID")
	}
	return values, nil
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
