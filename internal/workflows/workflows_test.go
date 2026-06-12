package workflows_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"

	"github.com/campoy/techcheck/internal/workflows"
)

// TestHello validates the M1 placeholder workflow with the SDK's test
// environment: the workflow runs the SayHello activity and returns its
// greeting. No infrastructure required.
func TestHello(t *testing.T) {
	var ts testsuite.WorkflowTestSuite
	env := ts.NewTestWorkflowEnvironment()
	env.RegisterActivity(workflows.SayHello)

	env.ExecuteWorkflow(workflows.Hello, "techcheck")

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var got string
	require.NoError(t, env.GetWorkflowResult(&got))
	require.Equal(t, "Hello, techcheck!", got)
}
