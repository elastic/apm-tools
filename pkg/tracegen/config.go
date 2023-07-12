package tracegen

import (
	"go.elastic.co/apm/v2"
)

type Config struct {
	SampleRate   float64
	TraceID      apm.TraceID
	OTLPProtocol string
	Insecure     bool
}
