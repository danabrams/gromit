package pipeline

// These should compile if the interfaces are properly defined
var _ AgentResolver = (*testAgentResolver)(nil)
var _ ClaudeClient = (*testClaudeClient)(nil)
var _ BeadClient = (*testBeadClient)(nil)
var _ BacklogClient = (*testBacklogClient)(nil)
var _ PromptRenderer = (*testPromptRenderer)(nil)
var _ LearningsManager = (*testLearningsManager)(nil)
var _ StateManager = (*testStateManager)(nil)
var _ LogWriter = (*testLogWriter)(nil)

// testAgentResolver is a mock for unit tests
type testAgentResolver struct{}

func (m *testAgentResolver) Resolve(phase string, flagOverride string, choosePicker bool) (Agent, error) {
	return nil, nil
}

// testClaudeClient is a mock for unit tests
type testClaudeClient struct{}

func (m *testClaudeClient) Run(prompt string, model string) (*ClaudeRunResult, error) {
	return nil, nil
}

// testBeadClient is a mock for unit tests
type testBeadClient struct{}

func (m *testBeadClient) Ready() (*BeadInfo, error) {
	return nil, nil
}

func (m *testBeadClient) Show(id string) (*BeadInfo, error) {
	return nil, nil
}

func (m *testBeadClient) Create(title string, priority int, labels []string, outputs []string) (*BeadInfo, error) {
	return nil, nil
}

func (m *testBeadClient) CreateWithDepsAndDescription(title string, priority int, labels []string, criteria []string, deps []string, desc string) (*BeadInfo, error) {
	return nil, nil
}

func (m *testBeadClient) Close(id string) error {
	return nil
}

// testBacklogClient is a mock for unit tests
type testBacklogClient struct{}

func (m *testBacklogClient) List() ([]*Idea, error) {
	return nil, nil
}

func (m *testBacklogClient) Get(id string) (*Idea, error) {
	return nil, nil
}

func (m *testBacklogClient) Add(item *Idea) error {
	return nil
}

func (m *testBacklogClient) Update(id string, fn func(*Idea)) error {
	return nil
}

// testPromptRenderer is a mock for unit tests
type testPromptRenderer struct{}

func (m *testPromptRenderer) RenderRefine(input interface{}) (string, error) {
	return "", nil
}

func (m *testPromptRenderer) RenderPlan(input interface{}) (string, error) {
	return "", nil
}

func (m *testPromptRenderer) RenderDecompose(input interface{}) (string, error) {
	return "", nil
}

func (m *testPromptRenderer) RenderThoroughReview(input *ThoroughReviewPromptInput) (string, error) {
	return "", nil
}

func (m *testPromptRenderer) RenderExplore(ctx interface{}) (string, error) {
	return "", nil
}

// testLearningsManager is a mock for unit tests
type testLearningsManager struct{}

func (m *testLearningsManager) Add(content string) error {
	return nil
}

// testStateManager is a mock for unit tests
type testStateManager struct{}

func (m *testStateManager) GetLastReviewCommit() (string, error) {
	return "", nil
}

func (m *testStateManager) SetLastReviewCommit(commit string) error {
	return nil
}

// testLogWriter is a mock for unit tests
type testLogWriter struct{}

func (m *testLogWriter) Write(entry any) error {
	return nil
}
