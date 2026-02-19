package workflow

import (
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
	"github.com/edvin/hosting/internal/model"
)

// registerActivities registers activity structs with the test workflow
// environment so that parameter and return types can be deserialized correctly
// by the Temporal test framework. In unit tests, all activities are mocked via
// OnActivity, but the framework still needs the type information for proper
// serialization/deserialization of activity parameters and return values.
func registerActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivity(&activity.CoreDB{})
	env.RegisterActivity(&activity.NodeLocal{})
	env.RegisterActivity(&activity.PowerDNSDB{})
	env.RegisterActivity(&activity.DNS{})
	env.RegisterActivity(&activity.CertificateActivity{})
	env.RegisterActivity(&activity.ACMEActivity{})
	env.RegisterActivity(&activity.NodeACMEActivity{})
	env.RegisterActivity(&activity.NodeLB{})
	env.RegisterActivity(&activity.Migrate{})
	env.RegisterActivity(&activity.Stalwart{})
	env.RegisterActivity(&activity.Callback{})
	env.RegisterActivity(&activity.Webhook{})
	env.RegisterActivity(&activity.AgentActivities{})
}

// matchFailedStatus returns a mock.MatchedBy matcher for UpdateResourceStatusParams
// that checks table, id, status=failed, and that StatusMessage is non-nil.
// This is needed because setResourceFailed now sets StatusMessage to the error
// string, and the exact message includes Temporal activity error wrapping that
// is not predictable in tests.
func matchFailedStatus(table, id string) interface{} {
	return mock.MatchedBy(func(params activity.UpdateResourceStatusParams) bool {
		return params.Table == table &&
			params.ID == id &&
			params.Status == model.StatusFailed &&
			params.StatusMessage != nil
	})
}
