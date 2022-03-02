package emitter

import (
	"fmt"
	"github.com/newrelic/infrastructure-agent/internal/integrations/v4/integration"
	"github.com/newrelic/infrastructure-agent/pkg/databind/pkg/data"
)

type stdoutEmitter struct {
}

func NewStdoutEmitter() Emitter {
	return &stdoutEmitter{}
}

func (e *stdoutEmitter) Emit(definition integration.Definition, ExtraLabels data.Map, entityRewrite []data.EntityRewrite, integrationJSON []byte) error {
	fmt.Println(string(integrationJSON))
	return nil
}
