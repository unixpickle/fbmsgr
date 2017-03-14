package fbmsgr

import (
	"errors"
	"regexp"
	"strconv"
)

// These are attachment type IDs used by Messenger.
const (
	ImageAttachmentType         = "photo"
	AnimatedImageAttachmentType = "animated_image"
	StickerAttachmentType       = "sticker"
	FileAttachmentType          = "file"
	VideoAttachmentType         = "video"
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
	if _, ok := raw["mercury"]; !ok {
		raw = map[string]interface{}{"mercury": raw}
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
