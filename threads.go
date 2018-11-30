package fbmsgr

import (
	"strconv"
	"time"

	"github.com/unixpickle/essentials"
)

const (
	threadBufferSize = 100
	actionBufferSize = 500
)

const (
	actionLogDocID = "1547392382048831"
	threadLogDocID = "2276493972392017"
)

// ThreadInfo stores information about a chat thread.
// A chat thread is facebook's internal name for a
// conversation (either a group chat or a 1-on-1).
type ThreadInfo struct {
	ThreadFBID string

	// Other user (nil for group chats).
	OtherUserID *string

	// Optional custom name for the chat.
	Name *string

	// Optional image URL.
	Image *string

	Participants []*ParticipantInfo

	Snippet       string
	SnippetSender string

	UnreadCount  int
	MessageCount int

	UpdatedTime time.Time
}

// ParticipantInfo stores information about a user.
type ParticipantInfo struct {
	FBID   string
	Gender string
	URL    string

	ImageSrc    string
	BigImageSrc string

	Username  string
	Name      string
	ShortName string
}

// Threads reads a range of the user's chat threads.
//
// The timestamp parameter specifies the timestamp of the
// earliest thread seen from the last call to Threads.
// It may be the 0 time, in which case the most recent
// threads will be fetched.
//
// The limit specifies the maximum number of threads.
func (s *Session) Threads(timestamp time.Time, limit int) (res []*ThreadInfo, err error) {
	defer essentials.AddCtxTo("fbmsgr: threads", &err)

	var response struct {
		Viewer struct {
			MessageThreads struct {
				Nodes []*threadInfoResult `json:"nodes"`
			} `json:"message_threads"`
		} `json:"viewer"`
	}
	params := map[string]interface{}{
		"limit": limit,
		"tags":  []string{"INBOX"},
		"includeDeliveryReceipts": true,
		"includeSeqID":            false,
	}
	if timestamp.IsZero() {
		params["before"] = nil
	} else {
		params["before"] = strconv.FormatInt(timestamp.UnixNano()/1e6, 10)
	}
	if err := s.graphQLDoc(threadLogDocID, params, &response); err != nil {
		return nil, err
	}
	for _, result := range response.Viewer.MessageThreads.Nodes {
		res = append(res, result.ThreadInfo())
	}
	return
}

// AllThreads reads the full list of chat threads.
func (s *Session) AllThreads() (res []*ThreadInfo, err error) {
	defer essentials.AddCtxTo("fbmsgr: all threads", &err)

	var lastTime time.Time
	for {
		listing, err := s.Threads(lastTime, threadBufferSize)
		if err != nil {
			return nil, err
		}
		if len(res) > 0 && len(listing) > 0 {
			if res[len(res)-1].ThreadFBID == listing[0].ThreadFBID {
				listing = listing[1:]
			}
		}
		res = append(res, listing...)
		if len(listing) < threadBufferSize {
			break
		}
		lastTime = listing[len(listing)-1].UpdatedTime
	}

	return res, nil
}

// ActionLog reads the contents of a thread.
//
// The fbid parameter specifies the other user ID or the
// group thread ID.
//
// The timestamp parameter specifies the timestamp of the
// earliest action seen from the last call to ActionLog.
// It may be the 0 time, in which case the most recent
// actions will be fetched.
//
// The limit parameter indicates the maximum number of
// actions to fetch.
func (s *Session) ActionLog(fbid string, timestamp time.Time,
	limit int) (log []Action, err error) {
	defer essentials.AddCtxTo("fbmsgr: action log", &err)

	var response struct {
		Thread struct {
			Messages struct {
				Nodes []map[string]interface{} `json:"nodes"`
			} `json:"messages"`
		} `json:"message_thread"`
	}
	params := map[string]interface{}{
		"id":                 fbid,
		"message_limit":      limit,
		"load_messages":      1,
		"load_read_receipts": true,
	}
	if timestamp.IsZero() {
		params["before"] = nil
	} else {
		params["before"] = strconv.FormatInt(timestamp.UnixNano()/1e6, 10)
	}
	if err := s.graphQLDoc(actionLogDocID, params, &response); err != nil {
		return nil, err
	}
	for _, x := range response.Thread.Messages.Nodes {
		log = append(log, decodeAction(x))
	}
	return
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
			listing, err := s.ActionLog(fbid, lastTime, actionBufferSize)
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
func (s *Session) DeleteMessage(id string) (err error) {
	defer essentials.AddCtxTo("fbmsgr: delete message", &err)

	url := BaseURL + "/ajax/mercury/delete_messages.php?dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return err
	}
	values.Set("message_ids[0]", id)
	_, err = s.jsonForPost(url, values)
	return err
}

type threadInfoResult struct {
	ThreadKey struct {
		ThreadFBID  *string `json:"thread_fbid"`
		OtherUserID *string `json:"other_user_id"`
	} `json:"thread_key"`
	Name        *string `json:"name"`
	UpdatedTime string  `json:"updated_time_precise"`
	LastMessage struct {
		Nodes []struct {
			Snippet       string `json:"snippet"`
			MessageSender struct {
				MessagingActor struct {
					ID string `json:"id"`
				} `json:"messaging_actor"`
			} `json:"message_sender"`
		} `json:"nodes"`
	} `json:"last_message"`
	UnreadCount   int `json:"unread_count"`
	MessagesCount int `json:"messages_count"`
	Image         *struct {
		URI string `json:"uri"`
	} `json:"image"`
	Participants struct {
		Edges []struct {
			Node struct {
				MessagingActor struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Gender      string `json:"gender"`
					URL         string `json:"url"`
					BigImageSrc *struct {
						URI string `json:"uri"`
					} `json:"big_image_src"`
					ImageSrc *struct {
						URI string `json:"uri"`
					} `json:"image_src"`
					ShortName string `json:"short_name"`
					Username  string `json:"username"`
				} `json:"messaging_actor"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"all_participants"`
}

func (t *threadInfoResult) ThreadInfo() *ThreadInfo {
	res := &ThreadInfo{
		OtherUserID:  t.ThreadKey.OtherUserID,
		Name:         t.Name,
		UnreadCount:  t.UnreadCount,
		MessageCount: t.MessagesCount,
	}
	if t.ThreadKey.ThreadFBID != nil {
		res.ThreadFBID = *t.ThreadKey.ThreadFBID
	} else {
		res.ThreadFBID = *t.ThreadKey.OtherUserID
	}
	if t.Image != nil {
		res.Image = &t.Image.URI
	}
	if len(t.LastMessage.Nodes) == 1 {
		res.Snippet = t.LastMessage.Nodes[0].Snippet
		res.SnippetSender = t.LastMessage.Nodes[0].MessageSender.MessagingActor.ID
	}
	fullTime, _ := strconv.ParseInt(t.UpdatedTime, 10, 64)
	res.UpdatedTime = time.Unix(fullTime/1000, (fullTime%1000)*1e6)

	for _, node := range t.Participants.Edges {
		pInfo := node.Node.MessagingActor
		p := &ParticipantInfo{
			FBID:      pInfo.ID,
			Gender:    pInfo.Gender,
			URL:       pInfo.URL,
			Username:  pInfo.Username,
			Name:      pInfo.Name,
			ShortName: pInfo.ShortName,
		}
		if pInfo.BigImageSrc != nil {
			p.BigImageSrc = pInfo.BigImageSrc.URI
		}
		if pInfo.ImageSrc != nil {
			p.BigImageSrc = pInfo.ImageSrc.URI
		}
		res.Participants = append(res.Participants, p)
	}

	return res
}
