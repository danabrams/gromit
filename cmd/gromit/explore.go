package main

// getEpicFiles returns a list of .md files in the epics directory.
// Creates the directory if it doesn't exist.
func getEpicFiles(epicsDir string) ([]string, error) {
	return listMarkdownFiles(epicsDir)
}
