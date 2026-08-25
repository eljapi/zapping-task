package stream

type Segment struct {
	Name     string
	Duration float64
}

type Pool struct {
	Segments []Segment
	/*HLS RFC 8216 must be an integer*/
	TargetDuration int
}
