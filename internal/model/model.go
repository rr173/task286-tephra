// Package model 定义火山灰层玻璃成分对比服务的核心领域实体与枚举。
//
// 领域核心：火山学研究者把玻璃颗粒主量元素分析（EPMA 探针数据）与层位
// 信息结合，判断两个灰层是否来自同一喷发事件（可相关）。所有状态机的
// 合法流转都在本包集中校验，store / service 层不得绕过。
package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// BatchStatus 描述灰层批次的生命周期状态。
//
//	collecting(整理中) → ready(待比较) → review(需复核) → published(已发布) → sealed(封存)
//	      └──────────────→ sealed(直接封存作废)
//	  review ────────────→ ready(复核通过后恢复比较)
type BatchStatus string

const (
	BatchCollecting BatchStatus = "collecting" // 整理中：仍在导入观测
	BatchReady      BatchStatus = "ready"      // 待比较：观测齐备可参与比较
	BatchReview     BatchStatus = "review"     // 需复核：存在异常观测待人工处置
	BatchPublished  BatchStatus = "published"  // 已发布：观测被纳入已发布快照
	BatchSealed     BatchStatus = "sealed"     // 封存：数据冻结，禁止任何修改
)

// String 返回状态的字符串形式。
func (s BatchStatus) String() string { return string(s) }

// ObservationStatus 描述单条玻璃成分观测的状态。
//
//	raw(原始) → normalized(已标准化) → redeposited(再搬运) → excluded(排除)
//	raw ──────→ redeposited(再搬运检测直接标记)
//	normalized ─→ excluded(复核排除)
//	excluded ───→ normalized(误判恢复)
type ObservationStatus string

const (
	ObsRaw         ObservationStatus = "raw"         // 原始：刚导入
	ObsNormalized  ObservationStatus = "normalized"  // 已标准化：误差标准化完成
	ObsRedeposited ObservationStatus = "redeposited" // 再搬运：疑似再沉积颗粒
	ObsExcluded    ObservationStatus = "excluded"    // 排除：不参与比较
)

// String 返回状态的字符串形式。
func (s ObservationStatus) String() string { return string(s) }

// CorrelationStatus 描述两个灰层之间相关关系候选的判定状态。
//
//	candidate(候选) → compatible(成分相符) / stratigraphic_conflict(层位冲突)
//	compatible / stratigraphic_conflict → confirmed(确认) / rejected(否决)
type CorrelationStatus string

const (
	CorrCandidate  CorrelationStatus = "candidate"            // 候选：待计算
	CorrCompatible CorrelationStatus = "compatible"           // 成分相符：指纹距离低于判定阈值
	CorrStratDiff  CorrelationStatus = "stratigraphic_conflict" // 层位冲突：层位约束不满足
	CorrConfirmed  CorrelationStatus = "confirmed"            // 确认：研究者裁决可相关
	CorrRejected   CorrelationStatus = "rejected"             // 否决：研究者裁决不相关
)

// String 返回状态的字符串形式。
func (s CorrelationStatus) String() string { return string(s) }

// SnapshotStatus 描述相关快照的生命周期。
//
//	draft(草稿) → published(发布) → superseded(替代)
type SnapshotStatus string

const (
	SnapDraft      SnapshotStatus = "draft"      // 草稿：可修改
	SnapPublished  SnapshotStatus = "published"  // 发布：不可变，固定标准化版本
	SnapSuperseded SnapshotStatus = "superseded" // 替代：被新快照取代
)

// String 返回状态的字符串形式。
func (s SnapshotStatus) String() string { return string(s) }

// ElementValue 描述单个主量元素的含量与 1σ 分析误差。
type ElementValue struct {
	Value float64 `json:"value"` // 含量，单位 wt%
	Error float64 `json:"error"` // 1σ 分析误差（绝对值，必须 > 0）
}

// Observation 描述一个灰层内的一次玻璃颗粒主量元素分析观测。
type Observation struct {
	ID         string                    `json:"id"`
	BatchID    string                    `json:"batch_id"`
	SampleNo   string                    `json:"sample_no"`   // 样品编号（批内唯一）
	GrainCount int                       `json:"grain_count"` // 参与平均的颗粒数
	Elements   map[string]ElementValue   `json:"elements"`    // 元素 -> 含量/误差
	Unit       string                    `json:"unit"`        // 默认 wt%
	Status     ObservationStatus         `json:"status"`
	ContentHash string                   `json:"content_hash"` // 幂等指纹
	Normalized map[string]ElementValue   `json:"normalized,omitempty"` // 归一化后的元素值
	Note       string                    `json:"note,omitempty"`
	CreatedAt  string                    `json:"created_at"`
	UpdatedAt  string                    `json:"updated_at"`
}

// Batch 描述一个火山灰沉积层（灰层批次）。
type Batch struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`          // 层名，如 KR-03
	Site        string        `json:"site"`          // 采样点/剖面
	Formation   string        `json:"formation"`     // 地层单元
	TopDepth    float64       `json:"top_depth"`     // 层位顶深
	BottomDepth float64       `json:"bottom_depth"`  // 层位底深（必须 > TopDepth）
	DepthUnit   string        `json:"depth_unit"`    // 深度单位，默认 m
	Status      BatchStatus   `json:"status"`
	Description string        `json:"description"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	ObservationCount int      `json:"observation_count,omitempty"` // 非持久化：活跃观测数
}

// Correlation 描述两个灰层（upper 在上层、lower 在下层）之间的相关关系候选。
type Correlation struct {
	ID               string            `json:"id"`
	UpperBatchID     string            `json:"upper_batch_id"`
	LowerBatchID     string            `json:"lower_batch_id"`
	Status           CorrelationStatus `json:"status"`
	Distance         float64           `json:"distance"`          // 标准化指纹距离 D²
	DOF              int               `json:"dof"`               // 自由度（参与比较的元素数）
	PValue           float64           `json:"p_value"`           // χ² 检验 p 值
	Compatible       bool              `json:"compatible"`        // 成分相符
	StratOK          bool              `json:"strat_ok"`          // 层位可行
	StratGap         float64           `json:"strat_gap"`         // 层位间隔（重叠为 0）
	StratNote        string            `json:"strat_note"`
	Verdict          string            `json:"verdict"`           // 裁决者
	VerdictNote      string            `json:"verdict_note"`
	StandardVersion  string            `json:"standard_version"`  // 所用标准化版本
	CreatedAt        string            `json:"created_at"`
	UpdatedAt        string            `json:"updated_at"`
}

// Snapshot 描述一个已发布的相关快照（不可变）。
type Snapshot struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	Status          SnapshotStatus `json:"status"`
	StandardVersion string         `json:"standard_version"` // 固定标准化版本
	Note            string         `json:"note"`
	CreatedAt       string         `json:"created_at"`
	PublishedAt     string         `json:"published_at,omitempty"`
	SupersededBy    string         `json:"superseded_by,omitempty"`
	Links           []SnapshotLink `json:"links,omitempty"` // 快照内的相关关系条目
}

// SnapshotLink 是快照内固定的相关关系快照行。
type SnapshotLink struct {
	CorrelationID string  `json:"correlation_id"`
	UpperBatchID  string  `json:"upper_batch_id"`
	LowerBatchID  string  `json:"lower_batch_id"`
	Status        string  `json:"status"`
	Distance      float64 `json:"distance"`
	PValue        float64 `json:"p_value"`
	StratGap      float64 `json:"strat_gap"`
}

// ElementsToJSON 把元素表编码为 JSON 字符串（用于持久化）。
func ElementsToJSON(elements map[string]ElementValue) (string, error) {
	raw, err := json.Marshal(elements)
	if err != nil {
		return "", fmt.Errorf("marshal elements: %w", err)
	}
	return string(raw), nil
}

// ElementsFromJSON 从 JSON 字符串解码元素表。
func ElementsFromJSON(s string) (map[string]ElementValue, error) {
	if s == "" {
		return map[string]ElementValue{}, nil
	}
	var out map[string]ElementValue
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil, fmt.Errorf("unmarshal elements: %w", err)
	}
	return out, nil
}

// NowISO 返回当前时间的 ISO8601 文本。
func NowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}

// ValidDepthRange 校验层位区间合法：顶深不小于 0 且底深严格大于顶深。
func ValidDepthRange(top, bottom float64) bool {
	return top >= 0 && bottom > top
}

// 统一的领域错误。store 与 httpapi 层按此映射 HTTP 状态码。
var (
	ErrNotFound       = fmt.Errorf("model: not found")
	ErrInvalidInput   = fmt.Errorf("model: invalid input")
	ErrInvalidState   = fmt.Errorf("model: invalid state transition")
	ErrConflict       = fmt.Errorf("model: conflict")
	ErrSealed         = fmt.Errorf("model: sealed data is immutable")
	ErrDuplicate      = fmt.Errorf("model: duplicate entry")
	ErrUnitMismatch   = fmt.Errorf("model: element unit mismatch")
	ErrNegativeError  = fmt.Errorf("model: negative analytical error")
	ErrInvertedRange  = fmt.Errorf("model: inverted depth range")
	ErrCorrNotReady   = fmt.Errorf("model: correlation not ready")
)

// IsSealedErr 判断错误是否为封存不可变错误。
func IsSealedErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), ErrSealed.Error())
}
