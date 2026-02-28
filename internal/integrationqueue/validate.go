package integrationqueue

import (
	"fmt"
	"strings"
)

func (e Entry) Validate() error {
	return validateEntry(e)
}

func validateEntry(entry Entry) error {
	if err := requireField(entry.Branch, "branch"); err != nil {
		return err
	}
	if err := requireField(entry.SessionID, "session_id"); err != nil {
		return err
	}
	if err := requireField(entry.OriginCommand, "origin_command"); err != nil {
		return err
	}
	if !Lane(entry.Lane).Valid() {
		return fmt.Errorf("lane %q is not supported", entry.Lane)
	}
	if err := requireField(entry.BaseRef, "base_ref"); err != nil {
		return err
	}
	if err := requireField(entry.HeadSHA, "head_sha"); err != nil {
		return err
	}
	if !entry.State.Valid() {
		return fmt.Errorf("state %q is not supported", entry.State)
	}
	if err := validateErrorContract(entry.LastErrorCode, entry.LastErrorMessage); err != nil {
		return err
	}
	return nil
}

func requireField(value, name string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func validateErrorContract(code, message string) error {
	hasCode := strings.TrimSpace(code) != ""
	hasMessage := strings.TrimSpace(message) != ""
	if hasCode != hasMessage {
		return fmt.Errorf("last_error_code and last_error_message must both be set together")
	}
	return nil
}
