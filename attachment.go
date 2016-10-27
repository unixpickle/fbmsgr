package fbmsgr

import "errors"

// These are attachment type IDs used by Messenger.
const (
	ImageAttachmentType   = "photo"
	StickerAttachmentType = "sticker"
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

// decodeAttachment decodes any attachment, attempting to
// use one of the built-in Attachment structs if possible.
func decodeAttachment(raw map[string]interface{}) Attachment {
	image, err := decodeImageAttachment(raw)
	if err == nil {
		return image
	}
	sticker, err := decodeStickerAttachment(raw)
	if err == nil {
		return sticker
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

// An ImageAttachment is an Attachment with specific info
// about an image.
type ImageAttachment struct {
	FBID     string
	Filename string
	MimeType string

	Width  int
	Height int

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
		FBID     string `json:"fbid"`
		Filename string `json:"filename"`
		MimeType string `json:"mimeType"`
		Meta     struct {
			Width  int `json:"width"`
			Height int `json:"height"`
		} `json:"imageMetadata"`
		Mercury struct {
			AttachType string `json:"attach_type"`

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
	if usableObject.Mercury.AttachType != ImageAttachmentType {
		return nil, errors.New("unexpected type: " + usableObject.Mercury.AttachType)
	}
	return &ImageAttachment{
		FBID:               usableObject.FBID,
		Filename:           usableObject.Filename,
		MimeType:           usableObject.MimeType,
		Width:              usableObject.Meta.Width,
		Height:             usableObject.Meta.Height,
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

// AttachmentType returns the internal attachment type for
// image attachments.
func (i *ImageAttachment) AttachmentType() string {
	return ImageAttachmentType
}

// URL returns the high-resolution URL.
func (i *ImageAttachment) URL() string {
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
			AttachType string `json:"attach_type"`
			URL        string `json:"url"`
			Meta       struct {
				StickerID         float64 `json:"stickerID"`
				PackID            float64 `json:"packID"`
				FrameCount        int     `json:"frameCount"`
				FrameRate         int     `json:"frameRate"`
				FramesPerRow      int     `json:"framesPerRow"`
				FramesPerCol      int     `json:"framesPerCol"`
				Width             int     `json:"width"`
				Height            int     `json:"height"`
				SpriteURI         string  `json:"spriteURI"`
				SpriteURI2x       string  `json:"spriteURI2x"`
				PaddedSpriteURI   string  `json:"paddedSpriteURI"`
				PaddedSpriteURI2x string  `json:"paddedSpriteURI2x"`
			} `json:"metadata"`
		} `json:"mercury"`
	}
	if err := putJSONIntoObject(raw, &usableObject); err != nil {
		return nil, err
	}
	if usableObject.Mercury.AttachType != StickerAttachmentType {
		return nil, errors.New("unexpected type: " + usableObject.Mercury.AttachType)
	}
	return &StickerAttachment{
		RawURL:            usableObject.Mercury.URL,
		StickerID:         int64(usableObject.Mercury.Meta.StickerID),
		PackID:            int64(usableObject.Mercury.Meta.PackID),
		SpriteURI:         usableObject.Mercury.Meta.SpriteURI,
		SpriteURI2x:       usableObject.Mercury.Meta.SpriteURI2x,
		PaddedSpriteURI:   usableObject.Mercury.Meta.PaddedSpriteURI,
		PaddedSpriteURI2x: usableObject.Mercury.Meta.PaddedSpriteURI2x,
		FrameCount:        usableObject.Mercury.Meta.FrameCount,
		FrameRate:         usableObject.Mercury.Meta.FrameRate,
		FramesPerRow:      usableObject.Mercury.Meta.FramesPerRow,
		FramesPerCol:      usableObject.Mercury.Meta.FramesPerCol,
		Width:             usableObject.Mercury.Meta.Width,
		Height:            usableObject.Mercury.Meta.Height,
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
