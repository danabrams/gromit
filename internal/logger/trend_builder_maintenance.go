package logger

import "sort"

const maintenanceBreachThreshold = 3

func detectPackageMaintenanceCosts(metrics []IterationMetric, limit TrendControlLimit) []PackageMaintenanceCost {
	if limit.Metric == "" || len(metrics) == 0 {
		return nil
	}

	streaks := make(map[string]int, len(metrics))
	flagged := make(map[string]PackageMaintenanceCost, len(metrics))

	for _, metric := range metrics {
		breached := metric.RollingAvgValidationMs > limit.UCL
		packages := uniquePackages(metric.TouchedPackages)
		if len(packages) == 0 {
			continue
		}
		for _, pkg := range packages {
			if pkg == "" {
				continue
			}
			if breached {
				streaks[pkg]++
				if streaks[pkg] >= maintenanceBreachThreshold {
					entry := flagged[pkg]
					entry.Package = pkg
					entry.Metric = limit.Metric
					entry.Severity = anomalySeverityHigh
					entry.ConsecutiveBreaches = streaks[pkg]
					entry.LatestValue = metric.RollingAvgValidationMs
					entry.UCL = limit.UCL
					entry.DetectedAt = metric.Timestamp
					flagged[pkg] = entry
				}
			} else {
				streaks[pkg] = 0
				delete(flagged, pkg)
			}
		}
	}

	if len(flagged) == 0 {
		return nil
	}

	out := make([]PackageMaintenanceCost, 0, len(flagged))
	for _, entry := range flagged {
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Package < out[j].Package
	})
	return out
}

func uniquePackages(packages []string) []string {
	if len(packages) == 0 {
		return nil
	}
	out := make([]string, 0, len(packages))
	seen := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		if pkg == "" {
			continue
		}
		if _, ok := seen[pkg]; ok {
			continue
		}
		seen[pkg] = struct{}{}
		out = append(out, pkg)
	}
	return out
}

func summarizeConvergence(metrics []IterationMetric) ConvergenceSummary {
	summary := ConvergenceSummary{}
	for _, metric := range metrics {
		inst := metric.ConvergenceInstability
		if inst == "" {
			continue
		}
		summary.LatestInstability = inst
		summary.LatestIteration = metric.Iteration
		summary.LatestTimestamp = metric.Timestamp
		switch inst {
		case "deadlock":
			summary.DeadlockCount++
		case "oscillation":
			summary.OscillationCount++
		}
	}
	return summary
}
