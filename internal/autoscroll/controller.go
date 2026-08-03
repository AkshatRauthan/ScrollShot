package autoscroll

import (
	"errors"
	"image"
	"time"

	"scrollshot/internal/capture"
	"scrollshot/internal/session"
)

// Controller coordinates automatic scrolling and screenshot capture.
//
// Stage 1 intentionally performs no movement analysis. It repeatedly:
//
//	capture → save frame → scroll → wait
//
// until MaxFrames is reached.
//
// Later stages will add overlap analysis to detect when scrolling has
// reached the end of the document.
type Controller struct {
	Capturer capture.Capturer
	Scroller Scroller
	Session  *session.Session
	Config   Config
}

func New(
	c capture.Capturer,
	s Scroller,
	sess *session.Session,
	cfg Config,
) (*Controller, error) {

	if c == nil {
		return nil, errors.New("nil capturer")
	}

	if s == nil {
		return nil, errors.New("nil scroller")
	}

	if sess == nil {
		return nil, errors.New("nil session")
	}

	if cfg.ScrollFraction <= 0 || cfg.ScrollFraction > 1 {
		cfg = DefaultConfig()
	}

	return &Controller{
		Capturer: c,
		Scroller: s,
		Session:  sess,
		Config:   cfg,
	}, nil
}

// Capture captures the current window and stores it in the active
// session.
func (c *Controller) Capture() (image.Image, error) {

	img, err := c.Capturer.CaptureActiveWindow()
	if err != nil {
		return nil, err
	}

	if _, _, err := c.Session.SaveFrame(img); err != nil {
		return nil, err
	}

	return img, nil
}

// Scroll scrolls approximately one viewport.
func (c *Controller) Scroll(img image.Image) error {

	height := img.Bounds().Dy()
	amount := int(float64(height) * c.Config.ScrollFraction)

	return c.Scroller.ScrollDown(amount)
}

// Run performs an automatic capture session.
func (c *Controller) Run() error {

	for i := 0; i < c.Config.MaxFrames; i++ {

		img, err := c.Capture()
		if err != nil {
			return err
		}

		if err := c.Scroll(img); err != nil {
			return err
		}

		time.Sleep(c.Config.Delay)
	}

	return nil
}
