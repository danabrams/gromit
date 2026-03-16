# Plan (Cycle 3)

## t-015

Fix TestDebugCompatibilityDiagnostics_CommandSurfaceSupportsLegacyAndExplicitConfig: replace relative fixture path with resolveProjectPath to prevent CWD-contamination from t.Chdir() in concurrent tests

