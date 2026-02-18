package policy

import "github.com/danabrams/gromit/internal/config"

const (
	timeoutTypeStall      = "stall"
	timeoutTypeBead       = "bead"
	timeoutTypeInvocation = "invocation"
)

// EscalationPolicy decides tier/model selection, retry caps, and timeout classification.
type EscalationPolicy interface {
	SelectInitialTier(priority int, labels []string) string
	SelectModel(priority int, labels []string) string
	NextTier(currentTier string) string
	MaxRetriesPerModel() int
	MaxRetriesPerBead() int
	ClassifyTimeout(ctxErr, parentErr error, stallFired bool) TimeoutClassification
}

// TimeoutClassification describes the timeout context.
type TimeoutClassification struct {
	TimeoutType    string
	ParentCanceled bool
}

// ConfigEscalationPolicy implements EscalationPolicy backed by *config.Config.
type ConfigEscalationPolicy struct {
	cfg *config.Config
}

var _ EscalationPolicy = (*ConfigEscalationPolicy)(nil)

// NewConfigEscalationPolicy returns an EscalationPolicy backed by cfg.
func NewConfigEscalationPolicy(cfg *config.Config) EscalationPolicy {
	return &ConfigEscalationPolicy{cfg: cfg}
}

// SelectInitialTier returns the initial tier for a bead based on priority/labels.
func (p *ConfigEscalationPolicy) SelectInitialTier(priority int, labels []string) string {
	return p.cfg.SelectTier(priority, labels)
}

// SelectModel returns the model for a bead based on priority/labels.
func (p *ConfigEscalationPolicy) SelectModel(priority int, labels []string) string {
	return p.cfg.SelectModel(priority, labels)
}

// NextTier returns the next escalation tier.
func (p *ConfigEscalationPolicy) NextTier(currentTier string) string {
	return p.cfg.NextEscalationTier(currentTier)
}

// MaxRetriesPerModel returns the per-model retry cap.
func (p *ConfigEscalationPolicy) MaxRetriesPerModel() int {
	return p.cfg.Escalation.MaxRetriesPerModel
}

// MaxRetriesPerBead returns the per-bead retry cap.
func (p *ConfigEscalationPolicy) MaxRetriesPerBead() int {
	return p.cfg.Escalation.MaxRetriesPerBead
}

// ClassifyTimeout determines the timeout type based on context and stall state.
func (p *ConfigEscalationPolicy) ClassifyTimeout(ctxErr, parentErr error, stallFired bool) TimeoutClassification {
	if stallFired && ctxErr == nil {
		return TimeoutClassification{TimeoutType: timeoutTypeStall}
	}
	if ctxErr != nil && parentErr == nil {
		return TimeoutClassification{TimeoutType: timeoutTypeBead}
	}
	if parentErr != nil {
		return TimeoutClassification{ParentCanceled: true}
	}
	return TimeoutClassification{TimeoutType: timeoutTypeInvocation}
}
