# 商品全网搜索详情功能策划

> 状态：策划（未实施） · 关联提案：`openspec/changes/add-product-detail-enrichment`（存储层已完成）

## 1. 背景与现状

CSV 导入的商品往往只有标题和少量指标，详情字段（图片、规格、卖点、评价）大面积缺失。本功能让运营在商品详情页一键"搜索全网详情"，自动补全并展示该商品的图片图集与结构化详情。

### 已具备（复用，不重建）

- **版本化详情快照**：`product_detail_snapshots` 表已含 `product_url / image_url / brand / price / currency / platform / shop_name / specs / selling_points / review_highlights / review_summary / warnings`，以及完整度字段 `completeness_status + missing_fields`（`internal/store/sqlite.go:256-282`）；写入走 `AddDetailSnapshot`（`internal/product/repository.go:155`），读取走 `LatestDetail`，历史版本天然保留、可回滚。
- **API 已透出**：`GET /api/v1/products/{id}` 已返回 `detail`，前端类型已就绪（`web/src/api/client.ts`）。
- **LLM 调用通道**：`internal/copygen` 的 OpenAI 兼容 provider（`TTS_LLM_BASE_URL / TTS_LLM_API_KEY / TTS_LLM_MODEL`），可直接复用做"网页正文 → 结构化详情"的抽取。
- **导入期解析**：`internal/importer` 已把 CSV 富化列写进快照（`internal/importer/importer.go:442-467`）。

### 缺口（本功能新增）

1. 没有"搜索/抓取"数据来源——详情目前只能靠人工 CSV 补全；
2. `image_url` 是单值字段，**没有多图图集结构**；
3. 详情页已拿到 `image_url / specs / product_url / review_highlights` 但**完全未渲染**（`web/src/routes/ProductDetailPage.tsx`）；
4. Go 侧无任何资产存储（图片若要本地化需先建 assets 表，与 `docs/comfyui-integration-plan.md` 共用同一套设计）。

## 2. 功能范围

**做**：

- 详情页手动触发"搜索全网详情"：按商品标题/品牌/平台/ASIN 构造查询，检索候选商品页，抽取结构化详情与全部图片，写为新版本快照并展示；
- 图片图集展示（一期外链直显，本地化后置）；
- 来源可溯源：每次富化记录来源 URL、provider、抓取时间；
- 低置信度结果必须人工确认后才写入，不覆盖人工已确认过的完整详情。

**不做（本期）**：

- 不做全网爬虫集群 / 反爬对抗；
- 不做实时比价、库存监控；
- 不做视频素材抓取。

## 3. 用户流程

### 3.1 手动触发（主流程，P1）

1. 运营打开商品详情页 → 点击"搜索全网详情"；
2. 后端构造查询（优先级：ASIN > 标题+品牌+平台）调搜索 provider，取 Top-N 候选页；
3. 抓取候选页正文（Reader 化），LLM 抽取为 DetailDraft（规格/卖点/评价/图集 URL）；
4. 置信度 ≥ 阈值：直接写新快照并刷新展示；
5. 置信度 < 阈值：前端弹出候选列表（来源链接 + 抽取摘要），运营选定一条后确认写入。

### 3.2 批量自动（P2）

- CSV 导入完成后，对 `completeness_status = incomplete` 的商品自动入队富化；异步执行，结果统一进人工确认列表。

## 4. 数据来源与采集方式

| 方案 | 说明 | 成本 | 稳定性/合规性 | 结论 |
|---|---|---|---|---|
| A. 平台官方 API | Amazon PA-API 5.0（按 ASIN 直查，products 表已有 `asin/marketplace` 字段）；淘宝联盟/京东联盟 | 需申请资质 | 高、合规 | **Amazon 首选** |
| B. 通用搜索 API + LLM 抽取 | SerpAPI / Bing Web Search / Google CSE 拿候选链接 → Jina Reader / Firecrawl 取正文 → 复用现有 LLM 做结构化抽取 | 按量付费、接入快 | 中 | **通用兜底首选** |
| C. 自建 HTML 抓取解析 | 自己 fetch + 解析 | 免费 | 低（反爬、页面结构漂移） | 不推荐，仅作 fallback |

推荐组合：**Amazon 商品走 A，其余走 B**。LLM 抽取要求严格 JSON 输出，容错照抄 `internal/copygen/openai.go` 的 `stripCodeFence` 模式。

图片采集：一期只登记图片 URL（快照 JSON 列），前端外链展示；二期选择性下载到本地 assets（与 ComfyUI 策划共用存储）。

## 5. API 与数据库设计

### 5.1 迁移（纯 additive）

- `product_detail_snapshots` 新增列：
  - `image_urls_json TEXT` — JSON 数组，约定首张为主图；
  - `enrich_provider TEXT` — 来源 provider（serpapi / paapi / manual…）；
  - `enrich_confidence REAL` — 匹配置信度 0–1。
- 不改既有列；旧快照 `image_urls_json` 为 NULL 时前端回退展示单张 `image_url`。

### 5.2 Go 接口

- `POST /api/v1/products/{id}/enrich`（operator）— 同步执行搜索 + 抽取；高置信落新快照返回 `{detail, confidence}`，低置信返回 `{candidates}` 不落库；
- `POST /api/v1/products/{id}/enrich/confirm`（operator）— body 为选定候选，确认后落新快照；
- `GET /api/v1/products/{id}` — `detail` 内增加 `image_urls` 字段（前端类型同步补充）。

### 5.3 Provider 抽象（照 copygen 范式）

```go
type EnrichProvider interface {
    Name() string
    Search(ctx context.Context, query SearchQuery) ([]SearchHit, error)
    Extract(ctx context.Context, hit SearchHit, seed product.DetailInput) (DetailDraft, error)
}
```

- env 工厂：`TTS_ENRICH_PROVIDER=serpapi|paapi|mock`、`TTS_ENRICH_API_KEY`、`TTS_SEARCH_TIMEOUT`；未配置时回退 Mock，前端按钮置灰并提示未配置；
- service 层注入，落库复用 `AddDetailSnapshot`（自动获得版本化与完整度重算）。

## 6. 前端展示设计（ProductDetailPage）

- 详情卡顶部加**图片画廊**：AntD `Image.PreviewGroup`，主图大图 + 缩略图条，点击放大预览；
- **规格参数**：`Descriptions` 表格渲染 `specs`；
- **卖点/评价**：`selling_points` 列表、`review_highlights` 标签组、`review_summary` 段落、`warnings` 警示条；
- 操作区："搜索全网详情"按钮 + 上次富化标注（provider、来源链接可点、抓取时间）；低置信候选用 `Modal` 列表供选择确认；
- 状态：搜索中按钮 loading + 骨架屏；失败 toast 提示原因。

## 7. 风险与合规

| 风险 | 说明 | 对策 |
|---|---|---|
| 平台 ToS / robots | 直接抓取 Amazon/淘宝页面违反其条款 | 优先官方 API；抓取仅走合规通道（Reader 类服务自带合规策略）；控制频率 |
| 图片版权 | 商品图版权归品牌方/平台 | 一期只存 URL 外链展示，不复制文件；本地化版本仅限内部使用，不二次分发 |
| 错配商品 | 同名/相似商品导致错误富化 | 置信度机制 + 低置信人工确认；快照历史可回滚 |
| API 成本 | 搜索 API 与 LLM 均按量计费 | 手动触发为主；批量自动富化设每日上限 |
| 数据时效 | 价格/评价随时间变化 | 快照带 `captured_at`，页面展示抓取时间 |

## 8. 分期实施计划

- **P1 — 手动富化 + 展示闭环**：迁移加列 → EnrichProvider（搜索 API + Reader + LLM 抽取）→ enrich/confirm 接口 → 详情页画廊/规格区/来源标注。验收：对一个只有标题的 CSV 商品执行搜索，图集与规格入库并展示，来源链接可点击回溯。
- **P2 — 批量异步 + Amazon PA-API**：异步任务（复用 import_jobs 表结构模式）+ 导入后自动富化队列 + PA-API provider。验收：导入 50 行 CSV 后 incomplete 商品自动富化，失败可重试，结果进人工确认。
- **P3 — 图片本地化**：assets 表 + 本地下载缓存（与 ComfyUI 策划共用存储设计），前端改用本地 URI。验收：断网环境下已富化商品的图片仍可正常展示。
