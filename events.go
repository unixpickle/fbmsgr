package fbmsgr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"time"
)

const pollErrTimeout = time.Second * 5

// An Event is a notification pushed to the client by the
// server.
type Event interface{}

// A MessageEvent is an Event containing a new message.
type MessageEvent struct {
	// MessageID is a unique ID used to distinguish a message
	// from others in a chat log.
	MessageID string

	// Body is the text in the message.
	// It is "" if the message contains no text.
	Body string

	// Attachments contains the message's attachments.
	Attachments []Attachment

	// SenderFBID is the fbid of the sending user.
	// This may be the current user, especially if the user
	// sent the message from a different device.
	SenderFBID string

	// If non-empty, this specifies the group chat ID.
	GroupThread string

	// If non-empty, this specifies the other user in a
	// one-on-one chat (as opposed to a group chat).
	OtherUser string
}

// A BuddyEvent is an Event containing information about a
// buddy's updated status.
type BuddyEvent struct {
	FBID       string
	LastActive time.Time
}

// A TypingEvent indicates that a user has started or
// stopped typing.
type TypingEvent struct {
	// SenderFBID is the user who is typing.
	SenderFBID string

	// Typing indicates whether or not the user is typing.
	Typing bool

	// If non-empty, this specifies the group chat ID.
	GroupThread string
}

// DeleteMessageEvent indicates that a message has been
// deleted.
type DeleteMessageEvent struct {
	MessageIDs    []string
	UpdatedThread *ThreadInfo
}

// ReadEvent reads the next event from the server.
//
// The first call will start polling for events.
//
// If the session is closed or fails with an error, a nil
// event is returned with an error (io.EOF if the read
// only failed because the session was closed).
func (s *Session) ReadEvent() (Event, error) {
	s.pollLock.Lock()
	if s.pollChan == nil {
		s.pollCtx, s.pollCancel = context.WithCancel(context.Background())
		s.pollChan = make(chan Event, 1)
		go s.poll()
	}
	s.pollLock.Unlock()
	select {
	case <-s.pollCtx.Done():
	case evt, ok := <-s.pollChan:
		if ok {
			return evt, nil
		}
	}
	s.pollLock.Lock()
	defer s.pollLock.Unlock()
	return nil, s.pollErr
}

// Close tells the session to terminate and clean up its
// resources.
//
// After closing a session, all ReadEvent calls will fail.
func (s *Session) Close() error {
	s.pollLock.Lock()
	defer s.pollLock.Unlock()
	if s.pollErr == nil {
		s.pollErr = io.EOF
	}
	if s.pollChan == nil {
		s.pollChan = make(chan Event)
		close(s.pollChan)
		s.pollCtx, s.pollCancel = context.WithCancel(context.Background())
		s.pollCancel()
	} else {
		select {
		case <-s.pollCtx.Done():
			return errors.New("already closed")
		default:
			s.pollCancel()
		}
	}
	return nil
}

func (s *Session) poll() {
	defer close(s.pollChan)

	host, err := s.callReconnect()
	if err != nil {
		s.pollFailed(errors.New("reconnect: " + err.Error()))
		return
	}
	pool, token, err := s.fetchPollingInfo(host)
	if err != nil {
		s.pollFailed(err)
		return
	}

	var seq int
	startTime := time.Now().Unix()
	for !s.checkClosed() {
		values := url.Values{}
		values.Set("cap", "8")
		values.Set("cb", "anuk")
		values.Set("channel", "p_"+s.userID)
		values.Set("clientid", "3342de8f")
		values.Set("idle", strconv.FormatInt(time.Now().Unix()-startTime, 10))
		values.Set("isq", "243")
		values.Set("msgr_region", "FRC")
		values.Set("msgs_recv", strconv.Itoa(seq))
		values.Set("partition", "-2")
		values.Set("pws", "fresh")
		values.Set("qp", "y")
		values.Set("seq", strconv.Itoa(seq))
		values.Set("state", "offline")
		values.Set("uid", s.userID)
		values.Set("viewer_uid", s.userID)
		values.Set("sticky_pool", pool)
		values.Set("sticky_token", token)
		u := "https://0-edge-chat.messenger.com/pull?" + values.Encode()
		response, err := s.jsonForGetContext(s.pollCtx, u)
		if s.checkClosed() {
			return
		}
		if err != nil {
			time.Sleep(pollErrTimeout)
			continue
		}
		msgs, newSeq, err := parseMessages(response)
		if newSeq > 0 {
			seq = newSeq
		}
		if err != nil {
			time.Sleep(pollErrTimeout)
		} else {
			s.dispatchMessages(msgs)
		}
	}
}

func (s *Session) dispatchMessages(msgs []map[string]interface{}) {
	for _, m := range msgs {
		t, ok := m["type"].(string)
		if !ok {
			continue
		}
		switch t {
		case "delta":
			s.dispatchDelta(m)
		case "buddylist_overlay":
			s.dispatchBuddylistOverlay(m)
		case "ttyp", "typ":
			s.dispatchTyping(m)
		case "messaging":
			evt, _ := m["event"].(string)
			if evt == "delete_messages" {
				s.dispatchDelete(m)
			}
		}
	}
}

func (s *Session) dispatchDelta(obj map[string]interface{}) {
	var deltaObj struct {
		Delta struct {
			Body        string                   `json:"body"`
			Attachments []map[string]interface{} `json:"attachments"`
			Meta        struct {
				Actor     string `json:"actorFbId"`
				MessageID string `json:"messageId"`
				ThreadKey struct {
					ThreadFBID string `json:"threadFbId"`
					OtherUser  string `json:"otherUserFbId"`
				} `json:"threadKey"`
			} `json:"messageMetadata"`
		} `json:"delta"`
	}

	if putJSONIntoObject(obj, &deltaObj) != nil {
		return
	}
	if len(deltaObj.Delta.Attachments) == 0 && deltaObj.Delta.Body == "" {
		return
	}

	var attachments []Attachment
	for _, a := range deltaObj.Delta.Attachments {
		attachments = append(attachments, decodeAttachment(a))
	}
	s.emitEvent(MessageEvent{
		MessageID:   deltaObj.Delta.Meta.MessageID,
		Body:        deltaObj.Delta.Body,
		Attachments: attachments,
		SenderFBID:  deltaObj.Delta.Meta.Actor,
		GroupThread: deltaObj.Delta.Meta.ThreadKey.ThreadFBID,
		OtherUser:   deltaObj.Delta.Meta.ThreadKey.OtherUser,
	})
}

func (s *Session) dispatchBuddylistOverlay(obj map[string]interface{}) {
	var deltaObj struct {
		Overlay map[string]struct {
			LastActive float64 `json:"la"`
		} `json:"overlay"`
	}

	if putJSONIntoObject(obj, &deltaObj) != nil {
		return
	}

	for user, info := range deltaObj.Overlay {
		s.emitEvent(BuddyEvent{
			FBID:       user,
			LastActive: time.Unix(int64(info.LastActive), 0),
		})
	}
}

func (s *Session) dispatchTyping(m map[string]interface{}) {
	var obj struct {
		State      int     `json:"st"`
		From       float64 `json:"from"`
		ThreadFBID float64 `json:"thread_fbid"`
		Type       string  `json:"type"`
	}
	if putJSONIntoObject(m, &obj) != nil {
		return
	}
	if obj.Type == "ttyp" {
		s.emitEvent(TypingEvent{
			SenderFBID:  floatIDToString(obj.From),
			Typing:      obj.State == 1,
			GroupThread: floatIDToString(obj.ThreadFBID),
		})
	} else {
		s.emitEvent(TypingEvent{
			SenderFBID: floatIDToString(obj.From),
			Typing:     obj.State == 1,
		})
	}
}

func (s *Session) dispatchDelete(m map[string]interface{}) {
	var obj struct {
		IDs    []string    `json:"mids"`
		Thread *ThreadInfo `json:"updated_thread"`
	}
	if putJSONIntoObject(m, &obj) != nil || obj.Thread == nil {
		return
	}
	obj.Thread.canonicalizeFBIDs()
	s.emitEvent(DeleteMessageEvent{
		MessageIDs:    obj.IDs,
		UpdatedThread: obj.Thread,
	})
}

func (s *Session) checkClosed() bool {
	select {
	case <-s.pollCtx.Done():
		return true
	default:
		return false
	}
}

func (s *Session) emitEvent(e Event) {
	select {
	case s.pollChan <- e:
	case <-s.pollCtx.Done():
	}
}

func (s *Session) pollFailed(e error) {
	s.pollLock.Lock()
	s.pollErr = e
	s.pollLock.Unlock()
}

func (s *Session) fetchPollingInfo(host string) (stickyPool, stickyToken string, err error) {
	values := url.Values{}
	values.Set("cap", "8")

	cbStr := ""
	s.randLock.Lock()
	for i := 0; i < 4; i++ {
		cbStr += string(byte(s.randGen.Intn(26)) + 'a')
	}
	s.randLock.Unlock()

	values.Set("cb", cbStr)
	values.Set("channel", "p_"+s.userID)
	values.Set("clientid", "3342de8f")
	values.Set("idle", "0")
	values.Set("msgr_region", "FRC")
	values.Set("msgs_recv", "0")
	values.Set("partition", "-2")
	values.Set("pws", "fresh")
	values.Set("qp", "y")
	values.Set("seq", "0")
	values.Set("state", "offline")
	values.Set("uid", s.userID)
	values.Set("viewer_uid", s.userID)
	u := "https://0-" + host + ".messenger.com/pull?" + values.Encode()
	response, err := s.jsonForGet(u)
	if err != nil {
		return "", "", err
	}
	var respObj struct {
		Type   string `json:"t"`
		LbInfo *struct {
			Sticky string `json:"sticky"`
			Pool   string `json:"pool"`
		} `json:"lb_info"`
	}
	if err := json.Unmarshal(response, &respObj); err != nil {
		return "", "", errors.New("parse init JSON: " + err.Error())
	}
	if respObj.Type == "lb" && respObj.LbInfo != nil {
		return respObj.LbInfo.Pool, respObj.LbInfo.Sticky, nil
	}
	return "", "", errors.New("unexpected initial polling response")
}

func (s *Session) callReconnect() (host string, err error) {
	values, err := s.commonParams()
	if err != nil {
		return "", err
	}
	values.Set("reason", "6")
	u := "https://www.messenger.com/ajax/presence/reconnect.php?" + values.Encode()
	response, err := s.jsonForGet(u)
	if err != nil {
		return "", err
	}

	var respObj struct {
		Payload struct {
			Host string `json:"host"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(response, &respObj); err != nil {
		return "", err
	}
	return respObj.Payload.Host, nil
}

// parseMessages extracts all of the "msg" payloads from a
// polled event body.
func parseMessages(data []byte) (list []map[string]interface{}, newSeq int, err error) {
	reader := json.NewDecoder(bytes.NewBuffer(data))
	for reader.More() {
		var objVal struct {
			Type     string                   `json:"t"`
			Seq      int                      `json:"seq"`
			Messages []map[string]interface{} `json:"ms"`
		}
		if err := reader.Decode(&objVal); err != nil {
			return nil, 0, err
		}
		if objVal.Seq > newSeq {
			newSeq = objVal.Seq
		}
		if objVal.Type == "msg" {
			list = append(list, objVal.Messages...)
		}
	}
	return
}
