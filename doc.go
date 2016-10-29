// Package fbmsgr provides an API for interacting with
// Facebook Messenger.
//
// Authentication
//
// The first step is to create a new Messenger session.
// Do this as follows, replacing "USER" and "PASS" with
// your Facebook login credentials:
//
//     sess, err := fbmsgr.Auth("USER", "PASS")
//     if err != nil {
//         // Handle login failure.
//     }
//
// Sending messages
//
// When sending a message, you specify a receiver by their
// FBID.
// The receiver may be another user, or it may be a group.
// For most methods related to message sending, there is
// one version of the method for a user and one for a
// group:
//
//     sess.SendText("USER_FBID", "what's up?")
//     sess.SendGroupText("GROUP_FBID", "what's up?")
//
// To send or retract a typing notification, you might do:
//
//     sess.SendTyping("USER_FBID", true) // typing
//     sess.SendTyping("USER_FBID", false) // stopped typing
//     sess.SendGroupTyping("GROUP_FBID", true)
//
// To send an attachment such as an image or a video, you
// can do the following:
//
//     f, err := os.Open("/path/to/image.png")
//     if err != nil {
//         // Handle failure.
//     }
//     defer f.Close()
//     upload, err := sess.Upload("image.png", f)
//     if err != nil {
//         // Handle failure.
//     }
//     _, err = sess.SendAttachment("USER_ID", upload)
//     // or sess.SendGroupAttachment("GROUP_ID", upload)
//     if err != nil {
//         // Handle failure.
//     }
//
// Events
//
// It is easy to receive events such as incoming messages.
// All you must do is read from the events channel:
//
//     for x := range sess.Events() {
//         if msg, ok := x.(fbmsgr.MessageEvent); ok {
//             if msg.SenderFBID == sess.FBID() {
//                 // It is a message that we sent.
//                 // This allows us to see messages we send
//                 // from a different device.
//                 continue
//             }
//             fmt.Println("received message:", msg)
//             if msg.GroupThread != "" {
//                 sess.SendReadReceipt(msg.GroupThread)
//             } else {
//                 sess.SendReadReceipt(msg.OtherUser)
//             }
//         } else if typ, ok := x.(fbmsgr.TypingEvent); ok {
//             fmt.Println("user is typing:", typ)
//         } else if del, ok := x.(fbmsgr.DeleteMessageEvent); ok {
//             fmt.Println("we deleted a message:", del)
//         }
//     }
//
// Listing threads
//
// To list the threads (conversations) a user is in, you
// can use the Threads method to fetch a subset of threads
// at a time.
// For example, you can print out the IDs of every thread
// as follows:
//
//     var idx int
//     for {
//         listing, err := sess.Threads(idx, 20)
//         if err != nil {
//             panic(err)
//         }
//         for _, entry := range listing.Threads {
//             fmt.Println("Thread with ID", entry.ThreadFBID)
//         }
//         if len(listing.Threads) < 20 {
//             break
//         }
//         idx += len(listing.Threads)
//     }
//
package fbmsgr
