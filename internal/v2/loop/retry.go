package loop

type stageIterationKey struct {
	beadID    string
	stageName string
}

func (b *BeadLoop) nextStageIteration(beadID, stageName string) int {
	key := stageIterationKey{beadID: beadID, stageName: stageName}
	b.stageIterations[key]++
	return b.stageIterations[key]
}
