---
title: 附录——调研摘要与资产盘点
created: 2026-07-17
---

# Addendum

本文件承载不适合进入简报正文、但对下游 PRD/架构有价值的深度内容。

## A. 竞品与数据源调研摘要（2026-07-17 联网调研）

**选品/榜单数据工具**

- Helium 10：Black Box 选品库、关键词反查、销量预估；订阅 $39–399/月；无公开数据 API。
- Jungle Scout：选品库 + 机会分；纯订阅，仅企业版有数据服务。
- Keepa：价格/BSR 历史强项；**有官方 token 制 API**（€49/月起），少数可程序化接入者。
- 卖家精灵（SellerSprite）：国内主流，已有**官方开放平台 + MCP 接口**，是中外工具中最开放的 API 通道。

**亚马逊官方通道**

- SP-API：面向卖家，拿自有店铺数据；Catalog Items 可查商品详情/图片/类目排名；**拿不到全站 Best Sellers 榜单**。
- PA-API 5.0 已于 2026-05 退役，由 Creators API 接替（门槛：30 天 10 单，限流 1 req/s）。Best Sellers / Movers & Shakers 无官方 API，市面靠第三方 Scrape API 按量计费。

**AI 物料工具共同功能集**（CopyMonkey、Perci、PhotoRoom、PiccoPilot、美图设计室等）

- 白底合规主图、AI 场景图/A+ 模块、listing 文案生成、关键词埋入、多语言、批量处理、平台规范适配。亚马逊官方 A+ 已内置生成式 AI（baseline 能力）。

**模式对标**

- 最接近的是 POD / 一件代发：Printful/Printify（目录 + 在线定制 + 履约）、CJdropshipping（40 万+ SKU + 贴牌定制 + API）。
- 「平台供 AI 物料、客户选品后在线下单定制宣传物料」的 B2B 形态暂无直接大牌对标，属空档。

来源：Keepa 定价（blog.fbaleadlist.com）、Helium 10 定价（schemaninja.com）、卖家精灵开放平台（open.sellersprite.com）、Creators API 取代 PA-API（velantio.com）、CJ vs Printify（cjdropshipping.com）、抓取法律风险（scrapehero.com）。

## B. 合规风险要点

- 抓亚马逊公开数据不必然违法，但违反其 ToS（封号/诉讼风险）；抓三方工具网站同样违约。
- 商品图版权归品牌方：直接商用或二创有侵权风险；事实性数据（价格、参数）风险低。
- 建议：聚合素材定位为「生成参考」，交付甲方的图片须为 AI 重生成或甲方授权素材；合同中明确素材责任边界。

## C. 现有代码资产盘点与复用建议（原 TikTok 项目）

**可直接复用**

- Go 后端：SQLite 迁移、CSV 导入管线（`internal/ingest`、`internal/importer`）、类目树与置信度归类（`internal/category`）、趋势查询 API（`internal/apiv1`）、静态托管。
- React 工作台：趋势榜、商品详情（含指标历史图表）、分类管理、CSV 导入历史等页面骨架——可作为运营后台的起点。
- Python 参考实现：评分、研究卡、审核状态、导出包等概念模型可映射到「商品档案 + 生成任务 + 审批」域模型。

**搁置不删**

- AI 视频生成管线（`video.py`、`creative.py` 等）：MVP 不做视频，代码保留，二期评估。

**需要新建**

- 甲方端（面向客户的选品/下单/审批界面，与运营后台分离）。
- 商品档案聚合能力（全网图文采集 + 补录）。
- AI 文案/图片生成服务与任务队列。
- 甲方账号与数据隔离（当前系统无鉴权——上线前必须补齐，docs/workbench.md 已有同样警告）。

## D. 数据源路线建议

1. MVP：手工/CSV 导入三方榜单（零对接成本，当天能跑）。
2. 二期优先评估卖家精灵开放平台（国内、有官方 API/MCP）与 Keepa API（BSR/价格历史）。
3. 全站榜单（Best Sellers / Movers & Shakers）无官方通道，如需实时只能走第三方 Scrape API，需评估合规与成本。

## E. 生图与部署架构（已确认，2026-07-17）

**拓扑**

- 云服务器：Web 前端、Go API、SQLite/对象存储（资产库）、任务队列——甲方与运营方都访问这里。
- 本地 PC（RTX 5080，16GB VRAM）：ComfyUI 生图 worker，不出现在公网。

**通信模式（关键决策：worker 拉取，而非服务器推送）**

- worker 主动轮询/长轮询服务器的生成任务队列（纯出站 HTTPS 请求），领到任务后本地生成，再把产物经 HTTPS 上传到服务器资产库并回写任务状态。
- 这样家用宽带（无公网 IP、动态 IP、NAT 后）零配置即可工作，**不需要内网穿透（frp 等）**，也不把家用机暴露给公网——安全和稳定性都更好。
- worker 用专用 token 鉴权；心跳 + 任务超时自动重派，worker 下线时任务堆积在队列、恢复后继续。

**5080 适配性**

- 16GB VRAM：SDXL 全速无压力；FLUX dev 需 FP8 量化（质量损失可接受）；配合 IP-Adapter/ControlNet 做「保商品本体 + 换场景」正是 ComfyUI 强项。
- 单卡串行生成，吞吐约够 MVP（每张图 10–60 秒级，含排队）；量级上来再考虑多卡或 serverless 托管 ComfyUI 扩容。

**对 MVP 范围的影响**

- 甲方账号鉴权 + HTTPS 从「可选项」变为 MVP 必需：客户要访问服务器，当前代码库无鉴权（docs/workbench.md 已警告不可暴露公网），必须补齐。
- 文案 LLM 的部署位置待定：同服务器跑小模型、或同样走 API，PRD/架构阶段定。

## F. 数据初始化决策（2026-07-17）

- 清理现有蝉妈妈/TikTok 演示数据（`data/products_cn.csv`、`chanmama-demo*.sqlite3` 等）对当前业务的意义：仅作历史参考，不进新库。
- 新平台首批真实数据：运营从三方选品工具（卖家精灵/Helium 10 等）导出亚马逊榜单 CSV，经导入管线入库；CSV 列结构沿用现有 `marketplace/asin/category` 字段设计（`data/sample_amazon.csv` 为格式参考）。
