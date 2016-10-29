package fbmsgr

import (
	"encoding/json"
	"errors"
	"strconv"
	"time"
)

const actionBufferSize = 500

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

	Timestamp       float64 `json:"timestamp"`
	ServerTimestamp float64 `json:"server_timestamp"`
}

func (t *ThreadInfo) canonicalizeFBIDs() {
	t.SnippetSender = stripFBIDPrefix(t.SnippetSender)
	for i, p := range t.Participants {
		t.Participants[i] = stripFBIDPrefix(p)
	}
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
		x.FBID = stripFBIDPrefix(x.FBID)
	}
	for _, x := range respObj.Payload.Threads {
		x.canonicalizeFBIDs()
	}

	return &respObj.Payload, nil
}

// ActionLog reads the action backlog from a thread.
//
// The fbid parameter specifies the other user ID or the
// group thread ID.
//
// The timestamp parameter specifies the timestamp of the
// earliest action seen from the last call to ActionLog.
// It may be the 0 time.
//
// Together, offset and limit define a message range.
// The offset parameter specifies the offset from first
// action in the log.
// The limit parameter specifies the maximum number of
// actions to fetch.
func (s *Session) ActionLog(fbid string, timestamp time.Time, offset,
	limit int) ([]Action, error) {
	url := BaseURL + "/ajax/mercury/thread_info.php?dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return nil, err
	}
	chatID := "thread_fbids][" + fbid
	values.Add("messages["+chatID+"][offset]", strconv.Itoa(offset))
	values.Add("messages["+chatID+"][limit]", strconv.Itoa(limit))
	timestampKey := "messages[" + chatID + "][timestamp]"
	if timestamp.IsZero() {
		values.Add(timestampKey, "")
	} else {
		values.Add(timestampKey, strconv.FormatInt(timestamp.UnixNano()/1000000, 10))
	}
	response, err := s.jsonForPost(url, values)
	if err != nil {
		return nil, err
	}
	var messageData struct {
		Payload struct {
			Actions []map[string]interface{} `json:"actions"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(response, &messageData); err != nil {
		return nil, err
	}
	var decoded []Action
	for _, x := range messageData.Payload.Actions {
		decoded = append(decoded, decodeAction(x))
	}
	return decoded, nil
}

// FullActionLog fetches all of the actions in a thread
// and returns them in reverse chronological order over
// a channel.
//
// The cancel channel, if non-nil, can be closed to stop
// the fetch early.
//
// The returned channels will both be closed once the
// fetch has completed or been cancelled.
// If an error is encountered during the fetch, it is sent
// over the (buffered) error channel and the fetch will be
// aborted.
func (s *Session) FullActionLog(fbid string, cancel <-chan struct{}) (<-chan Action, <-chan error) {
	if cancel == nil {
		cancel = make(chan struct{})
	}

	res := make(chan Action, actionBufferSize)
	errRes := make(chan error, 1)
	go func() {
		defer close(res)
		defer close(errRes)
		var lastTime time.Time
		var offset int
		for {
			listing, err := s.ActionLog(fbid, lastTime, offset, actionBufferSize)
			if err != nil {
				errRes <- err
				return
			}

			// Remove the one overlapping action.
			if offset > 0 && len(listing) > 0 {
				listing = listing[:len(listing)-1]
			}
			if len(listing) == 0 {
				return
			}

			for i := len(listing) - 1; i >= 0; i-- {
				x := listing[i]
				select {
				case <-cancel:
					return
				default:
				}

				select {
				case res <- x:
				case <-cancel:
					return
				}
			}

			offset += len(listing)
			lastTime = listing[0].ActionTime()
		}
	}()

	return res, errRes
}

// DeleteMessage deletes a message given its ID.
func (s *Session) DeleteMessage(id string) error {
	url := BaseURL + "/ajax/mercury/delete_messages.php?dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return err
	}
	values.Set("message_ids[0]", id)
	_, err = s.jsonForPost(url, values)
	return err
}
