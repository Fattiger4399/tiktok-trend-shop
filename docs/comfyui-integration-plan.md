# 本地 ComfyUI 接入策划

> 状态：策划（未实施） · 勘察日期：2026-07-18 · 勘察对象：`D:\AI\ComfyUI-Portable`

## 1. 勘察结论（事实清单，已实地核实）

| 项 | 结论 |
|---|---|
| 版本 | ComfyUI **v0.18.2** 官方发布版（git `a0ae3f3`，2026-03-24），无本地魔改 |
| 能力 | **图片**：FLUX.2 Klein 4B fp8 文生图（1024×1024；标准 20 步 CFG=5，蒸馏 4 步 CFG=1）+ 多图参考图片编辑；**无视频生成模型** |
| 模型 | `diffusion_models/flux-2-klein-4b-fp8.safetensors`（3.8G）+ `text_encoders/qwen_3_4b.safetensors`（7.5G）+ `vae/flux2-vae.safetensors`（321M）；LoRA / ControlNet / 放大模型均未安装 |
| 第三方节点 | 无（`custom_nodes` 近乎原版），对接只需原生 API |
| API | 默认 `127.0.0.1:8188`（REST `/prompt` `/history` `/view` + WebSocket `/ws`）；启动脚本未加 `--listen` / `--enable-cors-header`；勘察时**未运行**，需双击 `启动-ComfyUI-FLUX2Klein.bat` |
| 工作流 | `workflows/` 下 2 个（文生图、图片编辑），均为 **UI 画布格式**，接入前需转成 API 格式 |
| 环境 | 内置 Python 3.12.10，torch 2.11.0+cu128，适配 sm_120 正常（早期 cu126 在 5080 上崩溃的问题已随升级解决） |
| 硬件 | RTX 5080 16GB；桌面程序常占约 5.6G，空闲约 10.5G |
| 磁盘 | 共 19G（models 占 12G） |

## 2. 功能定位与范围

**定位**：ComfyUI 作为本项目的本地图片生成后端，服务"选品 → 营销素材"链路：

- 商品营销图/场景图：按 copygen 产出的文案 + 商品详情自动组装 prompt 生成；
- 主图变体/风格重绘：以商品原图为参考的图生图编辑（利用其多图参考能力）；
- 素材入库并关联商品与文案请求，供交付打包导出。

**本期不做**：视频生成（无模型，扩展评估见 §7 P3）、LoRA 训练、多 GPU 并发。

## 3. 部署拓扑（关键决策）

tkshop 计划部署到云服务器，而 ComfyUI 在用户本地 Windows 机且仅回环监听，三种拓扑：

| 方案 | 说明 | 优点 | 缺点 | 结论 |
|---|---|---|---|---|
| A. 同机自用 | tkshop 也跑在这台 Windows 机，`127.0.0.1:8188` 直连 | 零网络配置、最安全 | 只能本机使用 | **P1 采用** |
| B. 云服务器 + 隧道 | 服务器上的 tkshop 经 Tailscale / 内网穿透访问本地 8188 | 外网可用工作台 | 本地机须开机在线；隧道增加运维 | 可选 |
| C. 本地 agent 拉单 | 本地跑一个小 worker 主动轮询服务器的生成任务，完成后回传图片 | 无需暴露本地任何端口，最安全 | 需多开发一个 agent | **P3 推荐** |

不建议把 ComfyUI 搬到 GPU 云服务器（租用成本远高于本场景产出价值）。

## 4. 系统架构与 API 设计

### 4.1 Go 侧新增 `internal/mediagen` 包（照 copygen 范式）

```go
type Provider interface {
    Name() string
    GenerateImage(ctx context.Context, in ImageInput) (AssetRef, error) // 文生图
    EditImage(ctx context.Context, in EditInput) (AssetRef, error)      // 参考图编辑（P2）
}
```

- env 工厂：`TTS_COMFYUI_BASE_URL`（默认 `http://127.0.0.1:8188`）、`TTS_COMFYUI_TIMEOUT`；未配置或不可达时回退 Mock（写占位文本资产，与 Python `LocalVisualProvider` 行为一致）；
- 健康检查：`GET /system_stats`，不可达时前端生成入口置灰并提示"请启动 ComfyUI"。

### 4.2 调用流（利用 ComfyUI 原生队列，Go 不自建重队列）

1. 组装 API 格式工作流 JSON（内置模板，替换 prompt / seed / 尺寸）；
2. `POST /prompt` 入队 → 得 `prompt_id`；
3. 后台 goroutine 轮询 `GET /history/{prompt_id}`（间隔约 2s，超时受 `TTS_COMFYUI_TIMEOUT` 控制）；
4. 完成后 `GET /view?filename=...` 下载 PNG → 写本地 assets（见 4.3）→ 更新任务状态；
5. 失败/超时：标记 `failed + last_error`，支持重试。

> ComfyUI 自身就是单 GPU 串行队列，并发提交由它排队，Go 侧无需限流，只需控制"提交后跟踪"的 goroutine 数量。Go 侧任务记录可复用 import_jobs 的表结构模式（状态机 + 幂等键）。

### 4.3 assets 存储（Go 侧从零建，抄 Python 设计）

- 迁移：`assets` 表（`id, kind, backend, uri, content_type, byte_size, checksum, product_id, request_id, metadata_json, created_at`），`kind` 约定如 `generated-image`；
- 文件落盘：sha256 内容寻址 `{TTS_STORAGE_ROOT}/generated-image/{digest[:12]}-{name}.png`（与 Python `src/tiktok_trend_shop/assets.py:20-47` 同构）；
- 对外服务：`/assets/` 静态路由，防路径穿越写法参考 `cmd/tkshop/static.go`。

### 4.4 工作流模板管理

- 仓库内置 API 格式模板，如 `assets/workflows/text_to_image_flux2_klein.json`；默认走蒸馏 4 步快速出图，标准 20 步作为"高质量"选项；
- 一次性手工转换：启动 ComfyUI → 加载 `workflows/` 里现有 UI 工作流 → 菜单 "Export (API)" 导出 → 存入模板目录；
- 模板中 prompt 节点留占位符，Go 侧文本替换；seed 每次随机。

### 4.5 prompt 组装（与 copygen 联动）

`商品标题 + 卖点(前 3 条) + 用途(主图/场景图/详情图) + 风格模板(英文)` → 生成 prompt；负向约束固定 `no text, no watermark`（沿用用户 README 的建议风格）。P2 支持以商品原图为参考图（先 `POST /upload/image` 再进编辑工作流）。

## 5. 用户流程与前端展示

1. 商品详情页/文案请求页点"生成营销图" → 选用途与风格（默认蒸馏快速档，预计 20s 内出图）；
2. 后端组装 prompt → 提交 ComfyUI → 页面显示"生成中"（轮询任务状态）；
3. 出图后展示在素材区（关联商品），可"重新生成"（换 seed）、"采用"（绑定到文案请求，随交付包导出）；
4. ComfyUI 未运行时按钮置灰，并显示启动指引（`D:\AI\ComfyUI-Portable\启动-ComfyUI-FLUX2Klein.bat`）。

## 6. 风险与合规

| 风险 | 对策 |
|---|---|
| 显存不足（空闲约 10.5G，桌面程序常占 5.6G） | 默认蒸馏 4 步 / 1024²；失败时返回明确错误并建议关闭大占用程序 |
| ComfyUI 未运行 | 健康检查 + 前端启动引导，错误信息直接给出启动 bat 路径 |
| 工作流格式漂移（ComfyUI 升级后节点 schema 变化） | 模板入库时记录 ComfyUI 版本号，升级后重新导出模板 |
| 生成内容合规 | 固定负向词排除文字/水印；素材仅内部使用，对外发布前走现有 review 人工审核流 |
| 单 GPU 串行吞吐 | 接受为约束；P3 本地 agent 模式同样串行，仅解决服务器部署拓扑 |

## 7. 分期实施计划

- **P1 — 同机文生图闭环**：assets 表 + 存储 + `/assets/` 路由 → mediagen provider（健康检查 + 提交 + 轮询 + 落库）→ 生成 API 与前端入口。验收：详情页点生成，20s 内出图并入库，服务重启后图片仍可访问；ComfyUI 关闭时给出引导提示。
- **P2 — 参考图编辑 + 业务联动**：`/upload/image` + 编辑工作流 → 商品原图参考生成 → 素材绑定文案请求进入交付包。验收：用商品主图生成 3 张风格变体，并可随 delivery 导出。
- **P3 — 拓扑升级 / 视频评估**：本地 agent 拉单模式（§3 方案 C）支持云服务器部署；视频生成需另行评估（Wan2.x 类模型 + 对应节点安装 + 16G 显存可行性验证）后单独立项。
