package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"

	"gopkg.in/h2non/bimg.v1"
)

// Image stores an image binary buffer and its MIME type
type Image struct {
	Body []byte
	Mime string
}

// Save store details for saving an image to S3
type save struct {
	BucketName string
	ImageName  string
}

// Operation implements an image transformation runnable interface
type Operation func([]byte, ImageOptions) (Image, error)

// Run performs the image transformation
func (o Operation) Run(buf []byte, opts ImageOptions) (Image, error) {
	return o(buf, opts)
}

// ImageInfo represents an image details and additional metadata
type ImageInfo struct {
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Type        string `json:"type"`
	Space       string `json:"space"`
	Alpha       bool   `json:"hasAlpha"`
	Profile     bool   `json:"hasProfile"`
	Channels    int    `json:"channels"`
	Orientation int    `json:"orientation"`
}

func Info(buf []byte, o ImageOptions) (Image, error) {
	// We're not handling an image here, but we reused the struct.
	// An interface will be definitively better here.
	image := Image{Mime: "application/json"}

	meta, err := bimg.Metadata(buf)
	if err != nil {
		return image, NewError("Cannot retrieve image medatata: %s"+err.Error(), BadRequest)
	}

	info := ImageInfo{
		Width:       meta.Size.Width,
		Height:      meta.Size.Height,
		Type:        meta.Type,
		Space:       meta.Space,
		Alpha:       meta.Alpha,
		Profile:     meta.Profile,
		Channels:    meta.Channels,
		Orientation: meta.Orientation,
	}

	body, _ := json.Marshal(info)
	image.Body = body

	return image, nil
}

func Resize(buf []byte, o ImageOptions) (Image, error) {
	if o.Width == 0 && o.Height == 0 {
		return Image{}, NewError("Missing required param: height or width", BadRequest)
	}

	opts := BimgOptions(o)
	opts.Embed = true

	if o.NoCrop == false {
		opts.Crop = true
	}

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}
	fmt.Println("Bucket Name - ", o.BucketName)
	fmt.Println(o.BucketName)
	fmt.Println(o.ImageName)
	return Process(buf, opts, save)
}

func Enlarge(buf []byte, o ImageOptions) (Image, error) {
	if o.Width == 0 || o.Height == 0 {
		return Image{}, NewError("Missing required params: height, width", BadRequest)
	}

	opts := BimgOptions(o)
	opts.Enlarge = true

	if o.NoCrop == false {
		opts.Crop = true
	}

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Extract(buf []byte, o ImageOptions) (Image, error) {
	if o.AreaWidth == 0 || o.AreaHeight == 0 {
		return Image{}, NewError("Missing required params: areawidth or areaheight", BadRequest)
	}

	opts := BimgOptions(o)
	opts.Top = o.Top
	opts.Left = o.Left
	opts.AreaWidth = o.AreaWidth
	opts.AreaHeight = o.AreaHeight

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Crop(buf []byte, o ImageOptions) (Image, error) {
	if o.Width == 0 && o.Height == 0 {
		return Image{}, NewError("Missing required param: height or width", BadRequest)
	}

	opts := BimgOptions(o)
	opts.Crop = true
	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Rotate(buf []byte, o ImageOptions) (Image, error) {
	if o.Rotate == 0 {
		return Image{}, NewError("Missing required param: rotate", BadRequest)
	}

	opts := BimgOptions(o)
	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Flip(buf []byte, o ImageOptions) (Image, error) {
	opts := BimgOptions(o)
	opts.Flip = true
	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Flop(buf []byte, o ImageOptions) (Image, error) {
	opts := BimgOptions(o)
	opts.Flop = true
	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Thumbnail(buf []byte, o ImageOptions) (Image, error) {
	if o.Width == 0 && o.Height == 0 {
		return Image{}, NewError("Missing required params: width or height", BadRequest)
	}

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, BimgOptions(o), save)
}

func Zoom(buf []byte, o ImageOptions) (Image, error) {
	if o.Factor == 0 {
		return Image{}, NewError("Missing required param: factor", BadRequest)
	}

	opts := BimgOptions(o)

	if o.Top > 0 || o.Left > 0 {
		if o.AreaWidth == 0 && o.AreaHeight == 0 {
			return Image{}, NewError("Missing required params: areawidth, areaheight", BadRequest)
		}

		opts.Top = o.Top
		opts.Left = o.Left
		opts.AreaWidth = o.AreaWidth
		opts.AreaHeight = o.AreaHeight

		if o.NoCrop == false {
			opts.Crop = true
		}
	}

	opts.Zoom = o.Factor
	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Convert(buf []byte, o ImageOptions) (Image, error) {
	if o.Type == "" {
		return Image{}, NewError("Missing required param: type", BadRequest)
	}
	if ImageType(o.Type) == bimg.UNKNOWN {
		return Image{}, NewError("Invalid image type: "+o.Type, BadRequest)
	}
	opts := BimgOptions(o)

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

func Watermark(buf []byte, o ImageOptions) (Image, error) {
	if o.Text == "" {
		return Image{}, NewError("Missing required param: text", BadRequest)
	}

	opts := BimgOptions(o)
	opts.Watermark.DPI = o.DPI
	opts.Watermark.Text = o.Text
	opts.Watermark.Font = o.Font
	opts.Watermark.Margin = o.Margin
	opts.Watermark.Width = o.TextWidth
	opts.Watermark.Opacity = o.Opacity
	opts.Watermark.NoReplicate = o.NoReplicate

	if len(o.Color) > 2 {
		opts.Watermark.Background = bimg.Color{o.Color[0], o.Color[1], o.Color[2]}
	}

	save := save{}

	if o.BucketName != "" && o.ImageName != "" {
		save.BucketName = o.BucketName
		save.ImageName = o.ImageName
	}

	return Process(buf, opts, save)
}

// func Save(buf []byte, o ImageOptions) (Image, error) {
// 	if o.Width == 0 && o.Height == 0 {
// 		return Image{}, NewError("Missing required param: height or width", BadRequest)
// 	}
// 	if o.BucketName == "" {
// 		return Image{}, NewError("Missing required param: S3 Bucket name", BadRequest)
// 	}
// 	if o.ImageName == "" {
// 		return Image{}, NewError("Missing required param: Image name", BadRequest)
// 	}

// 	opts := BimgOptions(o)
// 	opts.Embed = true

// 	if o.NoCrop == false {
// 		opts.Crop = true
// 	}
// 	// uploadToS3("restapptest", buf)
// 	return Process(buf, opts)
// }

func Process(buf []byte, opts bimg.Options, save save) (out Image, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch value := r.(type) {
			case error:
				err = value
			case string:
				err = errors.New(value)
			default:
				err = errors.New("libvips internal error")
			}
			out = Image{}
		}
	}()

	buf, err = bimg.Resize(buf, opts)
	if err != nil {
		return Image{}, err
	}

	fmt.Println(save)
	if save.BucketName != "" && save.ImageName != "" {
		uploadToS3(save.BucketName, save.ImageName, buf)
	}
	mime := GetImageMimeType(bimg.DetermineImageType(buf))
	return Image{Body: buf, Mime: mime}, nil
}

func uploadToS3(bucketName string, fileName string, buf []byte) {

	fileBytes := bytes.NewReader(buf)
	fileType := http.DetectContentType(buf)

	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))

	uploader := s3manager.NewUploader(sess)

	_, err := uploader.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucketName),
		Key:         aws.String(fileName),
		Body:        fileBytes,
		ContentType: aws.String(fileType),
	})
	if err != nil {
		// Print the error and exit.
		exitErrorf("Unable to upload %q to %q, %v", fileName, bucketName, err)
	}

	fmt.Printf("Successfully uploaded %q to %q\n", fileName, bucketName)
	return
}

func exitErrorf(msg string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, msg+"\n", args...)
	os.Exit(1)
}
