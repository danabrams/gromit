package loop

type stageIterationKey struct {
	beadID    string
	stageName string
}

func (b *BeadLoop) nextStageIteration(beadID, stageName string) int {
	if b.stageIterations == nil {
		b.stageIterations = make(map[stageIterationKey]int)
	}
	key := stageIterationKey{beadID: beadID, stageName: stageName}
	b.stageIterations[key]++
	return b.stageIterations[key]
}
