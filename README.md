# fbmsgr

This will be a wrapper around [Facebook Messenger's](https://messenger.com) internal AJAX protocol. I hope it will make it possible to implement all sorts of Facebook bots. Below is a list of potential applications:

 * Tracking your friends' Messenger activity.
 * Analyzing your conversations (e.g. keywords analysis)
 * Automating certain kinds of messages (e.g. "Away" statuses)

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
