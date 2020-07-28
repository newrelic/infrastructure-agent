package emitter

import (
	"context"
	"github.com/newrelic/infrastructure-agent/internal/agent"
	"github.com/newrelic/infrastructure-agent/pkg/identity-client"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
	"io/ioutil"
	"testing"
)

func TestLegacy_EmitV4(t *testing.T) {

	icfg := identity.NewConfiguration()
	icfg.Host = "staging-identity-api.newrelic.com"
	identityClient := identity.NewAPIClient(icfg)

	request := identity.RegisterRequest{
		EntityName: "entityName",
		EntityType: "entityType",
		//DisplayName: dataSet.Entity.DisplayName,
	}

	resp, httpResp, err := identityClient.DefaultApi.RegisterPost(context.Background(), "userAgent", "<SECRET>", request, nil)
	if err != nil {
		bs, _ := ioutil.ReadAll(httpResp.Body)

		body := string(bs)
		logrus.WithError(err).
			WithField("Body", body).
			WithField("StatusCode", httpResp.StatusCode).
			WithField("Warnings", resp.Warnings).Info("Did not register entity")
		return
	}
	logrus.Info("Done")
}

type mockAgentcontext struct {
	agent.AgentContext
	mock.Mock
}

//func newLegacyEmitter() Emitter {
//	mac := &mockAgentcontext{}
//	a := &agent.Agent{
//		Context: mac,
//	}
//	return NewIntegrationEmitter(
//		)
//}
