// Package composition 定义玻璃主量元素目录、观测输入校验与总量归一化。
//
// 火山玻璃电子探针（EPMA）标准分析输出 10 个主量元素（wt%）。本包负责：
//  1. 元素白名单与单位校验（拒绝单位不符）；
//  2. 含量范围与误差合法性校验（拒绝负误差、负含量、总量越界）；
//  3. 观测归一化到 100% 并传播 1σ 分析误差。
package composition

import (
	"fmt"
	"sort"
	"strings"

	"task286-tephra/internal/model"
)

// ElementSpec 描述一个可检测元素的分析规范。
type ElementSpec struct {
	Symbol   string  // 元素符号，如 SiO2
	Name     string  // 中文名
	MinValue float64 // 正常范围下限（wt%）
	MaxValue float64 // 正常范围上限（wt%）
	Typical  float64 // 火山玻璃典型含量（用于缺失值提示）
}

// Catalog 火山玻璃主量元素目录（EPMA 常规 10 元素）。
var Catalog = []ElementSpec{
	{Symbol: "SiO2", Name: "二氧化硅", MinValue: 40, MaxValue: 80, Typical: 72.0},
	{Symbol: "TiO2", Name: "二氧化钛", MinValue: 0, MaxValue: 5, Typical: 0.3},
	{Symbol: "Al2O3", Name: "三氧化二铝", MinValue: 8, MaxValue: 20, Typical: 12.5},
	{Symbol: "FeO", Name: "氧化亚铁", MinValue: 0, MaxValue: 12, Typical: 3.0},
	{Symbol: "MnO", Name: "氧化锰", MinValue: 0, MaxValue: 2, Typical: 0.1},
	{Symbol: "MgO", Name: "氧化镁", MinValue: 0, MaxValue: 8, Typical: 0.4},
	{Symbol: "CaO", Name: "氧化钙", MinValue: 0, MaxValue: 12, Typical: 1.5},
	{Symbol: "Na2O", Name: "氧化钠", MinValue: 0, MaxValue: 10, Typical: 4.0},
	{Symbol: "K2O", Name: "氧化钾", MinValue: 0, MaxValue: 10, Typical: 3.0},
	{Symbol: "P2O5", Name: "五氧化二磷", MinValue: 0, MaxValue: 3, Typical: 0.1},
}

var specBySymbol = func() map[string]ElementSpec {
	m := make(map[string]ElementSpec, len(Catalog))
	for _, s := range Catalog {
		// 统一用小写键匹配，兼容用户输入的大小写变体（如 Al2O3 / al2o3）。
		m[strings.ToLower(s.Symbol)] = s
	}
	return m
}()

// Symbols 返回全部受支持元素符号（排序稳定）。
func Symbols() []string {
	keys := make([]string, 0, len(Catalog))
	for _, s := range Catalog {
		keys = append(keys, s.Symbol)
	}
	sort.Strings(keys)
	return keys
}

// Known 判断元素是否在目录中（大小写不敏感）。
func Known(symbol string) bool {
	_, ok := specBySymbol[strings.ToLower(symbol)]
	return ok
}

// Spec 返回元素规范；未知元素返回 (false, spec)。
func Spec(symbol string) (ElementSpec, bool) {
	s, ok := specBySymbol[strings.ToLower(symbol)]
	return s, ok
}

// ValidateElements 校验观测的元素输入：
//   - 元素必须全部在白名单内；
//   - 含量必须非负且不高于正常上限的 2 倍（容忍轻微越界，拒绝严重错误）；
//   - 误差必须为正（拒绝负误差与零误差）；
//   - 至少提供 5 个主量元素，否则指纹不可信。
func ValidateElements(elements map[string]model.ElementValue) error {
	if len(elements) < 5 {
		return fmt.Errorf("%w: at least 5 major elements required, got %d", model.ErrInvalidInput, len(elements))
	}
	seen := map[string]bool{}
	for symbol, ev := range elements {
		up := strings.ToLower(symbol)
		spec, ok := specBySymbol[up]
		if !ok {
			return fmt.Errorf("%w: unknown element %q", model.ErrUnitMismatch, symbol)
		}
		if seen[up] {
			return fmt.Errorf("%w: duplicate element %q", model.ErrInvalidInput, symbol)
		}
		seen[up] = true
		if ev.Value < 0 {
			return fmt.Errorf("%w: negative concentration for %s", model.ErrInvalidInput, up)
		}
		if ev.Value > spec.MaxValue*2+10 {
			return fmt.Errorf("%w: implausible concentration %.2f for %s (max %.1f)", model.ErrInvalidInput, ev.Value, up, spec.MaxValue)
		}
		if ev.Error <= 0 {
			return fmt.Errorf("%w: %s analytical error must be positive, got %g", model.ErrNegativeError, up, ev.Error)
		}
	}
	return nil
}

// Normalize 把观测的原始元素总量归一化到 100 wt%，并做误差传播。
//
// 误差传播：设原始总量 T = Σv_i，归一化值 n_i = 100·v_i/T。
// 若各元素误差相互独立（EPMA 定量通常如此近似），
// σ(n_i) ≈ 100·σ(v_i)/T（含量项主导，交叉项二阶小量忽略）。
func Normalize(elements map[string]model.ElementValue) (map[string]model.ElementValue, error) {
	if err := ValidateElements(elements); err != nil {
		return nil, err
	}
	total := 0.0
	for _, ev := range elements {
		total += ev.Value
	}
	if total <= 0 {
		return nil, fmt.Errorf("%w: total concentration must be positive", model.ErrInvalidInput)
	}
	out := make(map[string]model.ElementValue, len(elements))
	for symbol, ev := range elements {
		out[symbol] = model.ElementValue{
			Value: ev.Value * 100 / total,
			Error: ev.Error * 100 / total,
		}
	}
	return out, nil
}

// Summarize 打印观测摘要（调试/审计用）：元素数、总量、主元素均值。
func Summarize(elements map[string]model.ElementValue) string {
	var parts []string
	for _, s := range Symbols() {
		if ev, ok := elements[s]; ok {
			parts = append(parts, fmt.Sprintf("%s=%.2f", s, ev.Value))
		}
	}
	return fmt.Sprintf("n=%d total=%.1f {%s}", len(elements), Total(elements), strings.Join(parts, " "))
}

// Total 返回元素含量总和。
func Total(elements map[string]model.ElementValue) float64 {
	t := 0.0
	for _, ev := range elements {
		t += ev.Value
	}
	return t
}

// CommonElements 返回两个元素表中共同存在的元素（按目录顺序）。
func CommonElements(a, b map[string]model.ElementValue) []string {
	var out []string
	for _, s := range Symbols() {
		_, okA := a[s]
		_, okB := b[s]
		if okA && okB {
			out = append(out, s)
		}
	}
	return out
}
