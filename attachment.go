package fbmsgr

import (
	"errors"
	"regexp"
	"strconv"
)

// These are attachment type IDs used by Messenger.
const (
	AudioAttachmentType         = "audio"
	ImageAttachmentType         = "photo"
	AnimatedImageAttachmentType = "animated_image"
	StickerAttachmentType       = "sticker"
	FileAttachmentType          = "file"
	VideoAttachmentType         = "video"
)

const (
	blobAudioAttachmentType         = "MessageAudio"
	blobImageAttachmentType         = "MessageImage"
	blobAnimatedImageAttachmentType = "MessageAnimatedImage"
	blobFileAttachmentType          = "MessageFile"
	blobVideoAttachmentType         = "MessageVideo"
)

// An Attachment is an abstract non-textual entity
// attached to a message.
type Attachment interface {
	// AttachmentType returns the internal type name for this
	// attachment (such as ImageAttachmentType).
	AttachmentType() string

	// URL returns the most relevant URL for the attachment.
	// For instance, this might be a download URL for a file
	// or an image URL for a sticker.
	URL() string
}

// decodeAttachment decodes any attachment from an event,
// attempting to use one of the built-in Attachment
// structs if possible.
func decodeAttachment(raw map[string]interface{}) Attachment {
	if _, ok := raw["mercury"]; !ok {
		raw = map[string]interface{}{"mercury": raw}
	}

	audio, err := decodeAudioAttachment(raw)
	if err == nil {
		return audio
	}
	image, err := decodeImageAttachment(raw)
	if err == nil {
		return image
	}
	sticker, err := decodeStickerAttachment(raw)
	if err == nil {
		return sticker
	}
	file, err := decodeFileAttachment(raw)
	if err == nil {
		return file
	}
	video, err := decodeVideoAttachment(raw)
	if err == nil {
		return video
	}

	var typeObj struct {
		Mercury struct {
			AttachType string `json:"attach_type"`
		} `json:"mercury"`
	}
	putJSONIntoObject(raw, &typeObj)

	return &UnknownAttachment{
		Type:    typeObj.Mercury.AttachType,
		RawData: raw,
	}
}

// decodeBlobAttachment decodes a blob_attachment,
// attempting to use one of the built-in Attachment
// structs if possible.
func decodeBlobAttachment(raw map[string]interface{}) Attachment {
	audio, err := decodeBlobAudioAttachment(raw)
	if err == nil {
		return audio
	}
	image, err := decodeBlobImageAttachment(raw)
	if err == nil {
		return image
	}
	file, err := decodeBlobFileAttachment(raw)
	if err == nil {
		return file
	}
	video, err := decodeBlobVideoAttachment(raw)
	if err == nil {
		return video
	}

	var typeObj struct {
		TypeName string `json:"__typename"`
	}
	putJSONIntoObject(raw, &typeObj)

	return &UnknownAttachment{
		Type:    typeObj.TypeName,
		RawData: raw,
	}
}

// An UnknownAttachment is an Attachment of an unknown or
// unsupported type.
type UnknownAttachment struct {
	Type string

	// RawData contains the attachment's JSON data.
	RawData map[string]interface{}
}

// AttachmentType returns the attachment's type.
// This may be "".
func (u *UnknownAttachment) AttachmentType() string {
	return u.Type
}

// URL returns the empty string.
func (u *UnknownAttachment) URL() string {
	return ""
}

// String returns a brief description of the attachment.
func (u *UnknownAttachment) String() string {
	return "UnknownAttachment<" + u.Type + ">"
}

// A AudioAttachment is an attachment for an audio file.
type AudioAttachment struct {
	Name     string
	AudioURL string
}

func decodeAudioAttachment(raw map[string]interface{}) (*AudioAttachment, error) {
	var obj struct {
		Mercury struct {
			BlobAttachment map[string]interface{} `json:"blob_attachment"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	blob := obj.Mercury.BlobAttachment
	return decodeBlobAudioAttachment(blob)
}

func decodeBlobAudioAttachment(raw map[string]interface{}) (*AudioAttachment, error) {
	var obj struct {
		TypeName string `json:"__typename"`
		Name     string `json:"filename"`
		URL      string `json:"playable_url"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	if obj.TypeName != blobAudioAttachmentType {
		return nil, errors.New("unexpected type: " + obj.TypeName)
	}
	return &AudioAttachment{
		Name:     obj.Name,
		AudioURL: obj.URL,
	}, nil
}

// AttachmentType returns the internal attachment type for
// file attachments.
func (f *AudioAttachment) AttachmentType() string {
	return AudioAttachmentType
}

// URL returns the file download URL.
func (f *AudioAttachment) URL() string {
	return f.AudioURL
}

// String returns a brief description of the attachment.
func (f *AudioAttachment) String() string {
	return "AudioAttachment<" + f.URL() + ">"
}

// An ImageAttachment is an Attachment with specific info
// about an image.
type ImageAttachment struct {
	FBID   string
	Width  int
	Height int

	Animated bool

	PreviewURL    string
	PreviewWidth  int
	PreviewHeight int

	LargePreviewURL    string
	LargePreviewWidth  int
	LargePreviewHeight int

	ThumbnailURL string
	HiResURL     string
}

func decodeImageAttachment(raw map[string]interface{}) (*ImageAttachment, error) {
	var usableObject struct {
		Mercury struct {
			AttachType string `json:"attach_type"`

			Meta struct {
				FBID interface{} `json:"fbid"`
				Dims string      `json:"dimensions"`
			} `json:"metadata"`

			PreviewURL    string `json:"preview_url"`
			PreviewWidth  int    `json:"preview_width"`
			PreviewHeight int    `json:"preview_height"`

			LargePreviewURL    string `json:"large_preview_url"`
			LargePreviewWidth  int    `json:"large_preview_width"`
			LargePreviewHeight int    `json:"large_preview_height"`

			ThumbnailURL string `json:"thumbnail_url"`
			HiResURL     string `json:"hires_url"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &usableObject); err != nil {
		return nil, err
	}
	if usableObject.Mercury.AttachType != ImageAttachmentType &&
		usableObject.Mercury.AttachType != AnimatedImageAttachmentType {
		return nil, errors.New("unexpected type: " + usableObject.Mercury.AttachType)
	}
	dimRegexp := regexp.MustCompile("^([0-9]*),([0-9]*)$")
	matches := dimRegexp.FindStringSubmatch(usableObject.Mercury.Meta.Dims)
	if matches == nil {
		return nil, errors.New("invalid dimension: " + usableObject.Mercury.Meta.Dims)
	}
	width, _ := strconv.Atoi(matches[1])
	height, _ := strconv.Atoi(matches[2])
	return &ImageAttachment{
		FBID:               canonicalFBID(usableObject.Mercury.Meta.FBID),
		Width:              width,
		Height:             height,
		Animated:           usableObject.Mercury.AttachType == AnimatedImageAttachmentType,
		PreviewURL:         usableObject.Mercury.PreviewURL,
		PreviewWidth:       usableObject.Mercury.PreviewWidth,
		PreviewHeight:      usableObject.Mercury.PreviewHeight,
		LargePreviewURL:    usableObject.Mercury.LargePreviewURL,
		LargePreviewWidth:  usableObject.Mercury.LargePreviewWidth,
		LargePreviewHeight: usableObject.Mercury.LargePreviewHeight,
		ThumbnailURL:       usableObject.Mercury.ThumbnailURL,
		HiResURL:           usableObject.Mercury.HiResURL,
	}, nil
}

func decodeBlobImageAttachment(raw map[string]interface{}) (*ImageAttachment, error) {
	var usableObject struct {
		TypeName       string     `json:"__typename"`
		Preview        imageField `json:"preview"`
		LargePreview   imageField `json:"large_preview"`
		AnimatedImage  imageField `json:"animated_image"`
		Thumbnail      imageField `json:"thumbnail"`
		FBID           string     `json:"legacy_attachment_id"`
		OrigDimensions struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"original_dimensions"`
	}
	if err := putJSONIntoObject(raw, &usableObject); err != nil {
		return nil, err
	}
	if usableObject.TypeName != blobImageAttachmentType &&
		usableObject.TypeName != blobAnimatedImageAttachmentType {
		return nil, errors.New("unexpected type: " + usableObject.TypeName)
	}

	if usableObject.AnimatedImage.URI != "" {
		usableObject.LargePreview = usableObject.AnimatedImage
	}

	return &ImageAttachment{
		FBID:               canonicalFBID(usableObject.FBID),
		Width:              usableObject.OrigDimensions.X,
		Height:             usableObject.OrigDimensions.X,
		Animated:           usableObject.TypeName == blobAnimatedImageAttachmentType,
		PreviewURL:         usableObject.Preview.URI,
		PreviewWidth:       usableObject.Preview.Width,
		PreviewHeight:      usableObject.Preview.Height,
		LargePreviewURL:    usableObject.LargePreview.URI,
		LargePreviewWidth:  usableObject.LargePreview.Width,
		LargePreviewHeight: usableObject.LargePreview.Height,
		ThumbnailURL:       usableObject.Thumbnail.URI,
		HiResURL:           usableObject.LargePreview.URI,
	}, nil
}

// AttachmentType returns the internal attachment type for
// image attachments.
func (i *ImageAttachment) AttachmentType() string {
	if i.Animated {
		return AnimatedImageAttachmentType
	}
	return ImageAttachmentType
}

// URL returns the high-resolution URL.
func (i *ImageAttachment) URL() string {
	if i.HiResURL == "" {
		return i.LargePreviewURL
	}
	return i.HiResURL
}

// String returns a brief description of the attachment.
func (i *ImageAttachment) String() string {
	return "ImageAttachment<" + i.URL() + ">"
}

// A StickerAttachment is an Attachment with specific info
// about a sticker.
type StickerAttachment struct {
	RawURL string

	StickerID int64
	PackID    int64

	SpriteURI         string
	SpriteURI2x       string
	PaddedSpriteURI   string
	PaddedSpriteURI2x string
	FrameCount        int
	FrameRate         int
	FramesPerRow      int
	FramesPerCol      int

	Width  int
	Height int
}

func decodeStickerAttachment(raw map[string]interface{}) (*StickerAttachment, error) {
	var usableObject struct {
		Mercury struct {
			AttachType string                 `json:"attach_type"`
			Attachment map[string]interface{} `json:"sticker_attachment"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &usableObject); err != nil {
		return nil, err
	}
	if usableObject.Mercury.AttachType != StickerAttachmentType {
		return nil, errors.New("unexpected type: " + usableObject.Mercury.AttachType)
	}
	return decodeThreadStickerAttachment(usableObject.Mercury.Attachment)
}

func decodeThreadStickerAttachment(raw map[string]interface{}) (*StickerAttachment, error) {
	var usableObject struct {
		StickerID int64 `json:"id,string"`
		Pack      struct {
			ID int64 `json:"id,string"`
		} `json:"pack"`
		URL                 string   `json:"url"`
		FrameCount          int      `json:"frame_count"`
		FrameRate           int      `json:"frame_rate"`
		FramesPerRow        int      `json:"frames_per_row"`
		FramesPerCol        int      `json:"frames_per_column"`
		Width               int      `json:"width"`
		Height              int      `json:"height"`
		SpriteImage         uriField `json:"sprite_image"`
		SpritImage2x        uriField `json:"sprite_image_2x"`
		PaddedSpriteImage   uriField `json:"padded_sprite_image"`
		PaddedSpriteImage2x uriField `json:"padded_sprite_image_2x"`
	}
	if err := putJSONIntoObject(raw, &usableObject); err != nil {
		return nil, err
	}
	return &StickerAttachment{
		RawURL:            usableObject.URL,
		StickerID:         usableObject.StickerID,
		PackID:            usableObject.Pack.ID,
		SpriteURI:         usableObject.SpriteImage.URI,
		SpriteURI2x:       usableObject.SpritImage2x.URI,
		PaddedSpriteURI:   usableObject.PaddedSpriteImage.URI,
		PaddedSpriteURI2x: usableObject.PaddedSpriteImage2x.URI,
		FrameCount:        usableObject.FrameCount,
		FrameRate:         usableObject.FrameRate,
		FramesPerRow:      usableObject.FramesPerRow,
		FramesPerCol:      usableObject.FramesPerCol,
		Width:             usableObject.Width,
		Height:            usableObject.Height,
	}, nil
}

// AttachmentType returns the internal attachment type for
// sticker attachments.
func (s *StickerAttachment) AttachmentType() string {
	return StickerAttachmentType
}

// URL returns the raw URL of the sticker.
func (s *StickerAttachment) URL() string {
	return s.RawURL
}

// String returns a brief description of the attachment.
func (s *StickerAttachment) String() string {
	return "StickerAttachment<" + s.URL() + ">"
}

// A FileAttachment is an attachment for a raw file.
type FileAttachment struct {
	Name    string
	FileURL string
}

func decodeFileAttachment(raw map[string]interface{}) (*FileAttachment, error) {
	var obj struct {
		Mercury struct {
			Type string `json:"attach_type"`
			Name string `json:"name"`
			URL  string `json:"url"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	if obj.Mercury.Type != FileAttachmentType {
		return nil, errors.New("unexpected type: " + obj.Mercury.Type)
	}
	return &FileAttachment{
		Name:    obj.Mercury.Name,
		FileURL: obj.Mercury.URL,
	}, nil
}

func decodeBlobFileAttachment(raw map[string]interface{}) (*FileAttachment, error) {
	var obj struct {
		TypeName string `json:"__typename"`
		Name     string `json:"filename"`
		URL      string `json:"url"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	if obj.TypeName != blobFileAttachmentType {
		return nil, errors.New("unexpected type: " + obj.TypeName)
	}
	return &FileAttachment{
		Name:    obj.Name,
		FileURL: obj.URL,
	}, nil
}

// AttachmentType returns the internal attachment type for
// file attachments.
func (f *FileAttachment) AttachmentType() string {
	return FileAttachmentType
}

// URL returns the file download URL.
func (f *FileAttachment) URL() string {
	return f.FileURL
}

// String returns a brief description of the attachment.
func (f *FileAttachment) String() string {
	return "FileAttachment<" + f.URL() + ">"
}

type VideoAttachment struct {
	FBID     string
	Name     string
	VideoURL string

	Width  int
	Height int

	PreviewURL    string
	PreviewWidth  int
	PreviewHeight int

	LargePreviewURL    string
	LargePreviewWidth  int
	LargePreviewHeight int

	ThumbnailURL string
}

func decodeVideoAttachment(raw map[string]interface{}) (*VideoAttachment, error) {
	var obj struct {
		Mercury struct {
			Type string `json:"attach_type"`
			Name string `json:"name"`
			URL  string `json:"url"`

			Meta struct {
				FBID       string `json:"fbid"`
				Dimensions struct {
					Width  int `json:"width"`
					Height int `json:"height"`
				} `json:"dimensions"`
			} `json:"metadata"`

			PreviewURL    string `json:"preview_url"`
			PreviewWidth  int    `json:"preview_width"`
			PreviewHeight int    `json:"preview_height"`

			LargePreviewURL    string `json:"large_preview_url"`
			LargePreviewWidth  int    `json:"large_preview_width"`
			LargePreviewHeight int    `json:"large_preview_height"`

			ThumbnailURL string `json:"thumbnail_url"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	if obj.Mercury.Type != VideoAttachmentType {
		return nil, errors.New("unexpected type: " + obj.Mercury.Type)
	}
	return &VideoAttachment{
		FBID:     obj.Mercury.Meta.FBID,
		Name:     obj.Mercury.Name,
		VideoURL: obj.Mercury.URL,

		Width:  obj.Mercury.Meta.Dimensions.Width,
		Height: obj.Mercury.Meta.Dimensions.Height,

		PreviewURL:         obj.Mercury.PreviewURL,
		PreviewWidth:       obj.Mercury.PreviewWidth,
		PreviewHeight:      obj.Mercury.PreviewHeight,
		LargePreviewURL:    obj.Mercury.LargePreviewURL,
		LargePreviewWidth:  obj.Mercury.LargePreviewWidth,
		LargePreviewHeight: obj.Mercury.LargePreviewHeight,
		ThumbnailURL:       obj.Mercury.ThumbnailURL,
	}, nil
}

func decodeBlobVideoAttachment(raw map[string]interface{}) (*VideoAttachment, error) {
	var obj struct {
		TypeName       string     `json:"__typename"`
		Name           string     `json:"filename"`
		URL            string     `json:"playable_url"`
		ChatImage      imageField `json:"chat_image"`
		LargeImage     imageField `json:"large_image"`
		InboxImage     imageField `json:"inbox_image"`
		FBID           string     `json:"legacy_attachment_id"`
		OrigDimensions struct {
			X int `json:"x"`
			Y int `json:"y"`
		} `json:"original_dimensions"`
	}
	if err := putJSONIntoObject(raw, &obj); err != nil {
		return nil, err
	}
	if obj.TypeName != blobVideoAttachmentType {
		return nil, errors.New("unexpected type: " + obj.TypeName)
	}
	return &VideoAttachment{
		FBID:     obj.FBID,
		Name:     obj.Name,
		VideoURL: obj.URL,

		Width:  obj.OrigDimensions.X,
		Height: obj.OrigDimensions.Y,

		PreviewURL:         obj.ChatImage.URI,
		PreviewWidth:       obj.ChatImage.Width,
		PreviewHeight:      obj.ChatImage.Height,
		LargePreviewURL:    obj.LargeImage.URI,
		LargePreviewWidth:  obj.LargeImage.Width,
		LargePreviewHeight: obj.LargeImage.Height,
		ThumbnailURL:       obj.ChatImage.URI,
	}, nil
}

// AttachmentType returns the internal attachment type for
// video attachments.
func (v *VideoAttachment) AttachmentType() string {
	return VideoAttachmentType
}

// URL returns the main video URL.
func (v *VideoAttachment) URL() string {
	return v.VideoURL
}

// String returns a brief description of the attachment.
func (v *VideoAttachment) String() string {
	return "VideoAttachment<" + v.URL() + ">"
}

type imageField struct {
	URI    string `json:"uri"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type uriField struct {
	URI string `json:"uri"`
}
