package emitter

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
	"io"
	"os"
	"strings"
)

// EmitFormatter interface is used to implement different types of formats for a PrintEmitter.
type EmitFormatter interface {
	// Format will receive all Emitter information to prepare the output as a string.
	Format(definition integration.Definition, ExtraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) (string, error)
}

// PrintEmitter implements emitter.Emitter interface to format the integration payload using
// an EmitFormatter and output the result to a io.Writer implementation.
type PrintEmitter struct {
	w   io.Writer
	fmt EmitFormatter
}

func (s *PrintEmitter) Emit(
	definition integration.Definition,
	ExtraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationJSON []byte,
) error {
	if s.w == nil {
		return fmt.Errorf("failed to write output, error: nil target")
	}

	if s.fmt == nil {
		return fmt.Errorf("failed to format output, error: nil formatter")
	}

	output, err := s.fmt.Format(definition, ExtraLabels, entityRewrite, integrationJSON)
	if err != nil {
		return fmt.Errorf("failed to format output, error: %v", err)
	}
	_, err = s.w.Write([]byte(output))

	return err
}

// NewPrintEmitter instantiate a new PrintEmitter.
func NewPrintEmitter(w io.Writer, fmt EmitFormatter) Emitter {
	return &PrintEmitter{
		w:   w,
		fmt: fmt,
	}
}

// NewStdoutEmitter instantiate a PrintEmitter that is configured to output to stdout using
// a SimpleFormat.
func NewStdoutEmitter() Emitter {
	return NewPrintEmitter(os.Stdout, NewSimpleFormat())
}

// simpleFormat is a EmitFormatter implementation.
type simpleFormat struct {
}

// NewSimpleFormat instantiate a new SimpleFormat.
func NewSimpleFormat() EmitFormatter {
	return &simpleFormat{}
}

func (sf *simpleFormat) Format(
	definition integration.Definition,
	ExtraLabels data.Map,
	entityRewrite []data.EntityRewrite,
	integrationJSON []byte,
) (string, error) {
	var sb strings.Builder

	sb.WriteString("----------\n")
	sb.WriteString("Integration Name: " + definition.Name + "\n")
	sb.WriteString("Integration Output: " + string(integrationJSON) + "\n")
	sb.WriteString("----------\n")

	return sb.String(), nil
}
