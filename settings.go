package fbmsgr

// SetChatColor sets the chat color in a thread or for a
// one-on-one chat with a user.
// The cssColor argument is something like "#ff7e29".
func (s *Session) SetChatColor(fbid, cssColor string) error {
	url := BaseURL + "/messaging/save_thread_color/?source=thread_settings&dpr=1"
	values, err := s.commonParams()
	if err != nil {
		return err
	}
	values.Set("color_choice", cssColor)
	values.Set("thread_or_other_fbid", fbid)
	_, err = s.jsonForPost(url, values)
	return err
}
