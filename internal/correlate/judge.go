// Package correlate 组合指纹比较与层位检查结果，形成相关关系判定与裁决支持。
package correlate

import (
	"fmt"

	"task286-tephra/internal/fingerprint"
	"task286-tephra/internal/model"
	"task286-tephra/internal/stratigraphy"
)

// Evaluation 描述一次完整的相关关系评估（成分 + 层位）。
type Evaluation struct {
	Fingerprint *fingerprint.Verdict         `json:"fingerprint"`
	Stratigraphy stratigraphy.CheckResult    `json:"stratigraphy"`
	Suggest     string                       `json:"suggest"`     // confirm / reject
	Reasons     []string                     `json:"reasons"`     // 判定理由
}

// Evaluate 综合指纹与层位结果评估一个相关候选。
func Evaluate(fp *fingerprint.Verdict, st stratigraphy.CheckResult) Evaluation {
	ev := Evaluation{Fingerprint: fp, Stratigraphy: st}
	if fp != nil {
		if fp.Compatible {
			ev.Reasons = append(ev.Reasons, fmt.Sprintf("成分相符：D²=%.3f, df=%d, p=%.4f", fp.Distance, fp.DOF, fp.PValue))
		} else {
			ev.Reasons = append(ev.Reasons, fmt.Sprintf("成分显著不同：D²=%.3f, df=%d, p=%.4f < %.2f", fp.Distance, fp.DOF, fp.PValue, fp.Alpha))
		}
	} else {
		ev.Reasons = append(ev.Reasons, "指纹不可用：观测不足")
	}
	if st.OK {
		ev.Reasons = append(ev.Reasons, "层位可行："+st.Note)
	} else {
		ev.Reasons = append(ev.Reasons, "层位冲突："+st.Note)
	}
	ev.Suggest = fingerprint.SuggestAdjudication(fp != nil && fp.Compatible, st.OK)
	return ev
}

// Adjudicate 执行裁决：把候选/相符/冲突状态推进到确认或否决。
// 仅允许在 compatible 或 stratigraphic_conflict 状态下裁决。
func Adjudicate(c *model.Correlation, verdict string, who, note string) error {
	if c.Status != model.CorrCompatible && c.Status != model.CorrStratDiff {
		return fmt.Errorf("%w: only compatible/conflict correlations can be adjudicated, got %s", model.ErrInvalidState, c.Status)
	}
	target := model.CorrelationStatus(verdict)
	if target != model.CorrConfirmed && target != model.CorrRejected {
		return fmt.Errorf("%w: verdict must be confirmed or rejected", model.ErrInvalidInput)
	}
	if err := model.TransitionCorrelation(c.Status, target); err != nil {
		return err
	}
	c.Status = target
	c.Verdict = who
	c.VerdictNote = note
	return nil
}

// ReopenForRecompute 把一个已裁决或候选的关系重置回 candidate，
// 供指纹版本升级或观测变更后重新计算（仅限未入快照的关系）。
func ReopenForRecompute(c *model.Correlation) error {
	if c.Status == model.CorrConfirmed || c.Status == model.CorrRejected {
		c.Status = model.CorrCandidate
		c.Verdict = ""
		c.VerdictNote = ""
	}
	return nil
}
