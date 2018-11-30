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
// Once you are done with a session you have allocated,
// you should call Close() on it to clear any resources
// (e.g. goroutines) that it is using.
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
// It is easy to receive events such as incoming messages
// using the ReadEvent method:
//
//     for {
//         x, err := sess.ReadEvent()
//         if err != nil {
//             // Handle error.
//         }
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
// With the EventStream API, you can get more fine-grained
// control over how you receive events.
// For example, you can read the next minute's worth of
// events like so:
//
//     stream := sess.EventStream()
//     defer stream.Close()
//     timeout := time.After(time.Minute)
//     for {
//         select {
//         case evt := <-stream.Chan():
//             // Process event here.
//         case <-timeout:
//             return
//         }
//     }
//
// You can also create multiple EventStreams and read from
// different streams in different places.
//
// Listing threads
//
// To list the threads (conversations) a user is in, you
// can use the Threads method to fetch a subset of threads
// at a time. You can also use the AllThreads method to
// fetch all the threads at once.
//
package fbmsgr
