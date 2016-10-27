package fbmsgr

import (
	"encoding/json"
	"errors"
	"net/url"
	"strconv"
	"time"
)

const pollErrTimeout = time.Second * 5

// An Event is a notification pushed to the client by the
// server.
type Event interface {
	EventType() string
}

// Events returns a channel of events.
// This will start listening for events if no listener was
// already running.
func (s *Session) Events() <-chan Event {
	s.pollLock.Lock()
	defer s.pollLock.Unlock()
	if s.pollChan == nil {
		ch := make(chan Event, 1)
		s.pollChan = ch
		go s.poll(ch)
	}
	return s.pollChan
}

// EventsError returns the error which caused the events
// channel to be closed (if it is closed).
func (s *Session) EventsError() error {
	s.pollLock.Lock()
	defer s.pollLock.Unlock()
	return s.pollErr
}

func (s *Session) poll(ch chan<- Event) {
	host, err := s.callReconnect()
	if err != nil {
		s.pollFailed(errors.New("reconnect: "+err.Error()), ch)
		return
	}
	pool, token, err := s.fetchPollingInfo(host)
	if err != nil {
		s.pollFailed(err, ch)
		return
	}

	var msgsRecv int
	seq := 1
	for {
		values := url.Values{}
		values.Set("cap", "8")
		values.Set("cb", "anuk")
		values.Set("channel", "p_"+s.userID)
		values.Set("clientid", "3342de8f")
		values.Set("idle", "0")
		values.Set("isq", "243")
		values.Set("msgr_region", "FRC")
		values.Set("msgs_recv", strconv.Itoa(msgsRecv))
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
		_, err := s.jsonForGet(u)
		if err != nil {
			time.Sleep(pollErrTimeout)
			continue
		}
		// fmt.Println("got some data", string(response))
		// TODO: process response here.
	}
}

func (s *Session) fetchPollingInfo(host string) (stickyPool, stickyToken string, err error) {
	// https://0-edge-chat.messenger.com/pull?channel=p_100013975812075&seq=0&partition=-2&clientid=3342de8f&cb=anuk&idle=0&qp=y&cap=8&pws=fresh&isq=243&msgs_recv=0&uid=100013975812075&viewer_uid=100013975812075&request_batch=1&msgr_region=FRC&state=offline
	values := url.Values{}
	values.Set("cap", "8")
	values.Set("cb", "anuk")
	values.Set("channel", "p_"+s.userID)
	values.Set("clientid", "3342de8f")
	values.Set("idle", "0")
	values.Set("isq", "243")
	values.Set("msgr_region", "FRC")
	values.Set("msgs_recv", "0")
	values.Set("partition", "-2")
	values.Set("pws", "fresh")
	values.Set("qp", "y")
	values.Set("request_batch", "1")
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
		Batches []struct {
			Type   string `json:"t"`
			LbInfo *struct {
				Sticky string `json:"sticky"`
				Pool   string `json:"pool"`
			} `json:"lb_info"`
		} `json:"batches"`
	}
	if err := json.Unmarshal(response, &respObj); err != nil {
		return "", "", errors.New("parse init JSON: " + err.Error())
	}
	for _, batch := range respObj.Batches {
		if batch.Type == "lb" && batch.LbInfo != nil {
			return batch.LbInfo.Pool, batch.LbInfo.Sticky, nil
		}
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

func (s *Session) pollFailed(e error, ch chan<- Event) {
	s.pollLock.Lock()
	s.pollErr = e
	close(ch)
	s.pollLock.Unlock()
}
