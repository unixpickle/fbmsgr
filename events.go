package fbmsgr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strconv"
	"sync"
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

// An EventStream is a live stream of events.
//
// Create an event stream using Session.EventStream().
// Destroy an event stream using EventStream.Close().
type EventStream struct {
	session *Session

	evtChan chan Event
	ctx     context.Context
	cancel  context.CancelFunc

	lock   sync.RWMutex
	err    error
	closed bool
}

func newEventStream(s *Session, closed bool) *EventStream {
	res := &EventStream{
		session: s,
		evtChan: make(chan Event, 1),
		closed:  closed,
	}
	res.ctx, res.cancel = context.WithCancel(context.Background())
	if closed {
		res.cancel()
		close(res.evtChan)
	} else {
		go res.poll()
	}
	return res
}

// Chan returns a channel of events for the stream.
//
// The channel is closed if the stream is closed or if an
// error is encountered.
func (e *EventStream) Chan() <-chan Event {
	return e.evtChan
}

// Error returns the first error encountered while reading
// the stream.
func (e *EventStream) Error() error {
	e.lock.RLock()
	defer e.lock.RUnlock()
	return e.err
}

// Close closes the stream.
//
// This will cause the event channel to be closed.
// However, the result from Error() will not be changed.
func (e *EventStream) Close() error {
	e.lock.Lock()
	defer e.lock.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true
	e.cancel()
	return nil
}

func (e *EventStream) poll() {
	defer close(e.evtChan)

	host, err := e.callReconnect()
	if err != nil {
		e.pollFailed(errors.New("reconnect: " + err.Error()))
		return
	}
	pool, token, err := e.fetchPollingInfo(host)
	if err != nil {
		e.pollFailed(err)
		return
	}

	var seq int
	startTime := time.Now().Unix()
	for !e.checkClosed() {
		values := url.Values{}
		values.Set("cap", "8")
		values.Set("cb", "anuk")
		values.Set("channel", "p_"+e.session.userID)
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
		values.Set("uid", e.session.userID)
		values.Set("viewer_uid", e.session.userID)
		values.Set("sticky_pool", pool)
		values.Set("sticky_token", token)
		u := "https://0-edge-chat.messenger.com/pull?" + values.Encode()
		response, err := e.session.jsonForGetContext(e.ctx, u)
		if e.checkClosed() {
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
			e.dispatchMessages(msgs)
		}
	}
}

func (e *EventStream) dispatchMessages(msgs []map[string]interface{}) {
	for _, m := range msgs {
		t, ok := m["type"].(string)
		if !ok {
			continue
		}
		switch t {
		case "delta":
			e.dispatchDelta(m)
		case "buddylist_overlay":
			e.dispatchBuddylistOverlay(m)
		case "ttyp", "typ":
			e.dispatchTyping(m)
		case "messaging":
			evt, _ := m["event"].(string)
			if evt == "delete_messages" {
				e.dispatchDelete(m)
			}
		}
	}
}

func (e *EventStream) dispatchDelta(obj map[string]interface{}) {
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
	e.emitEvent(MessageEvent{
		MessageID:   deltaObj.Delta.Meta.MessageID,
		Body:        deltaObj.Delta.Body,
		Attachments: attachments,
		SenderFBID:  deltaObj.Delta.Meta.Actor,
		GroupThread: deltaObj.Delta.Meta.ThreadKey.ThreadFBID,
		OtherUser:   deltaObj.Delta.Meta.ThreadKey.OtherUser,
	})
}

func (e *EventStream) dispatchBuddylistOverlay(obj map[string]interface{}) {
	var deltaObj struct {
		Overlay map[string]struct {
			LastActive float64 `json:"la"`
		} `json:"overlay"`
	}

	if putJSONIntoObject(obj, &deltaObj) != nil {
		return
	}

	for user, info := range deltaObj.Overlay {
		e.emitEvent(BuddyEvent{
			FBID:       user,
			LastActive: time.Unix(int64(info.LastActive), 0),
		})
	}
}

func (e *EventStream) dispatchTyping(m map[string]interface{}) {
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
		e.emitEvent(TypingEvent{
			SenderFBID:  floatIDToString(obj.From),
			Typing:      obj.State == 1,
			GroupThread: floatIDToString(obj.ThreadFBID),
		})
	} else {
		e.emitEvent(TypingEvent{
			SenderFBID: floatIDToString(obj.From),
			Typing:     obj.State == 1,
		})
	}
}

func (e *EventStream) dispatchDelete(m map[string]interface{}) {
	var obj struct {
		IDs    []string    `json:"mids"`
		Thread *ThreadInfo `json:"updated_thread"`
	}
	if putJSONIntoObject(m, &obj) != nil || obj.Thread == nil {
		return
	}
	obj.Thread.canonicalizeFBIDs()
	e.emitEvent(DeleteMessageEvent{
		MessageIDs:    obj.IDs,
		UpdatedThread: obj.Thread,
	})
}

func (e *EventStream) checkClosed() bool {
	select {
	case <-e.ctx.Done():
		return true
	default:
		return false
	}
}

func (e *EventStream) emitEvent(evt Event) {
	select {
	case e.evtChan <- evt:
	case <-e.ctx.Done():
	}
}

func (e *EventStream) pollFailed(err error) {
	e.lock.Lock()
	e.err = err
	e.lock.Unlock()
}

func (e *EventStream) fetchPollingInfo(host string) (stickyPool, stickyToken string, err error) {
	values := url.Values{}
	values.Set("cap", "8")

	cbStr := ""
	e.session.randLock.Lock()
	for i := 0; i < 4; i++ {
		cbStr += string(byte(e.session.randGen.Intn(26)) + 'a')
	}
	e.session.randLock.Unlock()

	values.Set("cb", cbStr)
	values.Set("channel", "p_"+e.session.userID)
	values.Set("clientid", "3342de8f")
	values.Set("idle", "0")
	values.Set("msgr_region", "FRC")
	values.Set("msgs_recv", "0")
	values.Set("partition", "-2")
	values.Set("pws", "fresh")
	values.Set("qp", "y")
	values.Set("seq", "0")
	values.Set("state", "offline")
	values.Set("uid", e.session.userID)
	values.Set("viewer_uid", e.session.userID)
	u := "https://0-" + host + ".messenger.com/pull?" + values.Encode()
	response, err := e.session.jsonForGet(u)
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

func (e *EventStream) callReconnect() (host string, err error) {
	values, err := e.session.commonParams()
	if err != nil {
		return "", err
	}
	values.Set("reason", "6")
	u := "https://www.messenger.com/ajax/presence/reconnect.php?" + values.Encode()
	response, err := e.session.jsonForGet(u)
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

// EventStream creates a new EventStream for the session.
//
// You must close the result when you are done with it.
func (s *Session) EventStream() *EventStream {
	return newEventStream(s, false)
}

// ReadEvent reads the next event from a default event
// stream.
// The first call will create the default event stream.
//
// ReadEvent is present for backward-compatibility.
// You should consider using the EventStream API rather
// than ReadEvent.
//
// If the stream is closed or fails with an error, a nil
// event is returned with an error (io.EOF if the read
// only failed because the stream was closed).
func (s *Session) ReadEvent() (Event, error) {
	s.defaultStreamLock.Lock()
	if s.defaultStream == nil {
		s.defaultStream = s.EventStream()
	}
	stream := s.defaultStream
	s.defaultStreamLock.Unlock()

	if evt, ok := <-stream.Chan(); ok {
		return evt, nil
	}
	err := stream.Error()
	if err == nil {
		err = io.EOF
	}
	return nil, err
}

// Close cleans up the session's resources.
// Any running EventStreams created from this session
// should be closed separately.
//
// This closes the default event stream, meaning that all
// ReadEvent calls will fail after Close() is finished.
func (s *Session) Close() error {
	s.defaultStreamLock.Lock()
	if s.defaultStream != nil {
		s.defaultStream.Close()
	} else {
		s.defaultStream = newEventStream(s, true)
	}
	s.defaultStreamLock.Unlock()
	return nil
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
