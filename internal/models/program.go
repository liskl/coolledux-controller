package models

// GraffitiProgram represents a static image (graffiti) to display on the matrix.
// Data is column-major RGB444 pixel data.
type GraffitiProgram struct {
	Width    int          `json:"width"`
	Height   int          `json:"height"`
	Mode     TextShowMode `json:"mode"`
	Speed    uint8        `json:"speed"`
	StayTime uint8        `json:"stay_time"`
	Data     []byte       `json:"data"`
}

// AnimationProgram represents an animated sequence of frames.
// Each frame is column-major RGB444 pixel data. FrameDelays are in milliseconds.
type AnimationProgram struct {
	Width       int      `json:"width"`
	Height      int      `json:"height"`
	Frames      [][]byte `json:"frames"`
	FrameDelays []uint16 `json:"frame_delays"`
}

// TextProgram represents text to render and display on the matrix.
type TextProgram struct {
	Text      string       `json:"text"`
	FontSize  int          `json:"font_size"`
	Color     uint32       `json:"color"`
	Mode      TextShowMode `json:"mode"`
	Speed     uint8        `json:"speed"`
	StayTime  uint8        `json:"stay_time"`
	MoveSpace uint16       `json:"move_space"`
}
