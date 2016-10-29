# fbmsgr [![GoDoc](https://godoc.org/github.com/unixpickle/fbmsgr?status.svg)](https://godoc.org/github.com/unixpickle/fbmsgr)

This is a wrapper around [Facebook Messenger's](https://messenger.com) internal AJAX protocol. This wrapper could be used for any number of cool things, such as:

 * Tracking your friends' Messenger activity.
 * Analyzing your conversations (e.g. keywords analysis)
 * Automating "Away" messages
 * Creating chat bots

# Current status

Currently, the API is fairly reliable and can perform a bunch of basic functionalities. Here is a list of supported features (it may lag slightly behind the master branch):

 * Send textual messages to people or groups
 * Send attachments to people or groups
 * Receive messages with or without attachments
 * Send read receipts
 * Receive events for incoming messages
 * Receive events for friend "Last Active" updates
 * Set chat text colors (to arbitrary RGB colors)
 * List a user's threads.
 * List messages in a thread.
 * Send and receive typing events
 * Delete messages

# TODO

 * Emoji/sticker transmission
 * Modifying chat preferences (emoji, nicknames, etc.)
 * View pending message requests
 * Create new group chats
