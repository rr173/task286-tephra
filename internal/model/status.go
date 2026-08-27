package model

import "fmt"

// TransitionBatch 校验灰层批次状态机流转。
//
//	collecting → ready | sealed
//	ready      → review | sealed | published(纳入已发布快照)
//	review     → ready | sealed
//	published  → sealed
//	sealed     → (终止态，无出口)
func TransitionBatch(from, to BatchStatus) error {
	switch from {
	case BatchCollecting:
		if to == BatchReady || to == BatchSealed {
			return nil
		}
	case BatchReady:
		if to == BatchReview || to == BatchSealed || to == BatchPublished {
			return nil
		}
	case BatchReview:
		if to == BatchReady || to == BatchSealed {
			return nil
		}
	case BatchPublished:
		if to == BatchSealed {
			return nil
		}
	case BatchSealed:
		return fmt.Errorf("%w: batch sealed is terminal", ErrInvalidState)
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidState, from, to)
}

// TransitionObservation 校验成分观测状态机流转。
//
//	raw        → normalized | redeposited | excluded
//	normalized → redeposited | excluded
//	redeposited→ normalized | excluded
//	excluded   → normalized（误判恢复）
func TransitionObservation(from, to ObservationStatus) error {
	switch from {
	case ObsRaw:
		if to == ObsNormalized || to == ObsRedeposited || to == ObsExcluded {
			return nil
		}
	case ObsNormalized:
		if to == ObsRedeposited || to == ObsExcluded {
			return nil
		}
	case ObsRedeposited:
		if to == ObsNormalized || to == ObsExcluded {
			return nil
		}
	case ObsExcluded:
		if to == ObsNormalized {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidState, from, to)
}

// TransitionCorrelation 校验相关关系状态机流转。
//
//	candidate → compatible | stratigraphic_conflict
//	compatible / stratigraphic_conflict → confirmed | rejected
//	confirmed / rejected 为裁决终态（除重建候选外不可再流转）
func TransitionCorrelation(from, to CorrelationStatus) error {
	switch from {
	case CorrCandidate:
		if to == CorrCompatible || to == CorrStratDiff {
			return nil
		}
	case CorrCompatible, CorrStratDiff:
		if to == CorrConfirmed || to == CorrRejected {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidState, from, to)
}

// TransitionSnapshot 校验相关快照状态机流转。
//
//	draft → published
//	published → superseded
//	superseded → (终止态)
func TransitionSnapshot(from, to SnapshotStatus) error {
	switch from {
	case SnapDraft:
		if to == SnapPublished {
			return nil
		}
	case SnapPublished:
		if to == SnapSuperseded {
			return nil
		}
	}
	return fmt.Errorf("%w: %s -> %s", ErrInvalidState, from, to)
}

// ActiveBatchStatuses 返回可参与比较的批次状态集合。
func ActiveBatchStatuses() map[BatchStatus]bool {
	return map[BatchStatus]bool{
		BatchReady:      true,
		BatchPublished:  true,
	}
}

// ActiveObservationStatuses 返回参与指纹统计的观测状态集合。
func ActiveObservationStatuses() map[ObservationStatus]bool {
	return map[ObservationStatus]bool{
		ObsNormalized: true,
	}
}

// BatchCanCompare 判断批次是否具备参与比较的资格。
func BatchCanCompare(b Batch) bool {
	return ActiveBatchStatuses()[b.Status]
}

// ObservationParticipates 判断观测是否参与比较统计。
func ObservationParticipates(o Observation) bool {
	return ActiveObservationStatuses()[o.Status]
}
