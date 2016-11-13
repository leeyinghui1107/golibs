package gpio

type Mode string

const (
	ModeInput  Mode = "in"
	ModeOutput Mode = "out"
	ModePWM    Mode = "pwm"
)

// Edge represents the edge on which a pin interrupt is triggered
type Edge string

// Pin represents a GPIO pin.
type Pin interface {
	Mode() Mode   // gets the current pin mode
	SetMode(Mode) // set the current pin mode
	Set(bool)     // sets the pin state high
	Close()       // if applicable, closes the pin
	Get() bool    // returns the current pin state
	Err() error   // returns the last error state
}
