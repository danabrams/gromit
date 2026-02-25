package experiment

// ArmState represents the state of a single arm in the Thompson sampling bandit.
type ArmState struct {
	ID        string
	Successes int
	Failures  int
}

// BanditState represents the state of a multi-arm bandit.
type BanditState struct {
	Arms []ArmState
}
