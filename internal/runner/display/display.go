package display

// FormatRun renders the current run state for display.
func FormatRun(status *RunStatus) string {
    if status == nil {
        return "Run: not running"
    }
    return ""
}
