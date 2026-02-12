package workflow

import (
	"go.temporal.io/sdk/testsuite"

	"github.com/edvin/hosting/internal/activity"
)

// registerActivities registers activity structs with the test workflow
// environment so that parameter and return types can be deserialized correctly
// by the Temporal test framework. In unit tests, all activities are mocked via
// OnActivity, but the framework still needs the type information for proper
// serialization/deserialization of activity parameters and return values.
func registerActivities(env *testsuite.TestWorkflowEnvironment) {
	env.RegisterActivity(&activity.CoreDB{})
	env.RegisterActivity(&activity.NodeLocal{})
	env.RegisterActivity(&activity.ServiceDB{})
	env.RegisterActivity(&activity.DNS{})
	env.RegisterActivity(&activity.CertificateActivity{})
	env.RegisterActivity(&activity.LB{})
	env.RegisterActivity(&activity.Stalwart{})
	env.RegisterActivity(&activity.Cluster{})
	env.RegisterActivity(&activity.Deploy{})
}
