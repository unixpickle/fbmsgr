package fbmsgr

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

// ThreadInfo stores information about a chat thread.
// A chat thread is facebook's internal name for a
// conversation (either a group chat or a 1-on-1).
type ThreadInfo struct {
	ThreadID   string `json:"thread_id"`
	ThreadFBID string `json:"thread_fbid"`
	Name       string `json:"name"`

	// OtherUserFBID is nil for group chats.
	OtherUserFBID *string `json:"other_user_fbid"`

	// Participants contains a list of FBIDs.
	Participants []string `json:"participants"`

	// Snippet stores the last message sent in the thread.
	Snippet       string `json:"snippet"`
	SnippetSender string `json:"snippet_sender"`

	UnreadCount  int `json:"unread_count"`
	MessageCount int `json:"message_count"`

	Timestamp       int64 `json:"timestamp"`
	ServerTimestamp int64 `json:"server_timestamp"`
}

// ParticipantInfo stores information about a user.
type ParticipantInfo struct {
	// ID is typically "fbid:..."
	ID string `json:"id"`

	FBID   string `json:"fbid"`
	Gender int    `json:"gender"`
	HREF   string `json:"href"`

	ImageSrc    string `json:"image_src"`
	BigImageSrc string `json:"big_image_src"`

	Name      string `json:"name"`
	ShortName string `json:"short_name"`
}

// A ThreadListResult stores the result of listing the
// user's chat threads.
type ThreadListResult struct {
	Threads      []*ThreadInfo      `json:"threads"`
	Participants []*ParticipantInfo `json:"participants"`
}

// Threads reads a range of the user's chat threads.
// The offset specifiecs the index of the first thread
// to fetch, starting at 0.
// The limit specifies the maximum number of threads.
func (s *Session) Threads(offset, limit int) (*ThreadListResult, error) {
	params, err := s.commonParams()
	if err != nil {
		return nil, err
	}
	params.Set("inbox[filter]", "")
	params.Set("inbox[offset]", strconv.Itoa(offset))
	params.Set("inbox[limit]", strconv.Itoa(limit))
	reqURL := BaseURL + "/ajax/mercury/threadlist_info.php?dpr=1"
	body, err := s.jsonForPost(reqURL, params)
	if err != nil {
		return nil, err
	}

	var respObj struct {
		Payload ThreadListResult `json:"payload"`
	}
	if err := json.Unmarshal(body, &respObj); err != nil {
		return nil, errors.New("parse json: " + err.Error())
	}
	for _, x := range respObj.Payload.Participants {
		if strings.HasPrefix(x.FBID, "fbid:") {
			x.FBID = x.FBID[5:]
		}
	}

	return &respObj.Payload, nil
}
