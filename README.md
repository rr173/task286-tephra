# 火山灰层玻璃成分对比服务（tephracorr）

火山学研究者用本服务判断两个火山灰沉积层是否可相关（同一喷发事件的产物）。
服务接收灰层层位与玻璃颗粒主量元素分析（EPMA），标准化分析误差、计算指纹距离
与同源显著性、检查层位可行性约束，研究者裁决相关关系并发布不可变相关快照。

## 业务闭环

1. 创建灰层批次（层位区间）并导入玻璃成分观测（元素含量 + 1σ 分析误差），观测按内容指纹幂等。
2. 误差标准化：观测归一化到 100 wt% 并传播分析误差，进入可比较状态。
3. 指纹比较：按批内统计量（均值/标准误/平均误差）计算标准化距离 D²，χ² 检验得到同源 p 值。
4. 层位约束：检查两灰层深度区间是否重叠/相邻（允许最大间隙），识别层位倒置与相距过远。
5. 裁决：成分相符且层位可行 → 可确认相关；任一不满足 → 建议否决，研究者裁决终态。
6. 快照：把已裁决关系打包为不可变相关快照，固定标准化版本；新证据仅生成修订候选。

## 运行

```bash
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./cmd/tephracorr --addr :8080 --db tephra.db
```

自检（不启动长驻服务）：`go run ./cmd/tephracorr --smoke-test`

## 状态机

- 灰层批次：`collecting → ready → review → published → sealed`（review 可回 ready，sealed 终止）。
- 成分观测：`raw → normalized → redeposited → excluded`（excluded 可恢复）。
- 相关关系：`candidate → compatible | stratigraphic_conflict → confirmed | rejected`。
- 相关快照：`draft → published → superseded`。

## 核心不变量

- 观测指纹幂等：同一批次 + 样品号 + 内容哈希 重复导入自动跳过。
- 快照不可变：发布后固定标准化版本与关系集合，关系入快照后拒绝重算；已发布快照拒绝重复发布。
- 数据合法性：拒绝未知元素、负误差、层位区间倒置、封存批次的一切修改。
- 同批次自相关、重复 (upper,lower) 候选均被拒绝。

## 评测

见 `BENZHI_README.md`。
