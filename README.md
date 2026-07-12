# TikTok Trend Shop

Foundation services for a TikTok hot-product-to-AI-video workflow.

## Stack

- Python 3.11+
- Standard library only for the foundation layer
- SQLite for local durable workflow records
- Local filesystem asset storage by default
- OpenSpec for change management

This stack keeps the first implementation portable and easy to test while later modules add provider integrations, UI, video rendering, or queue backends.

## Local Commands

```powershell
$env:PYTHONPATH = "src"
python -m tiktok_trend_shop.cli config
python -m tiktok_trend_shop.cli migrate
python -m tiktok_trend_shop.cli run-demo --title "Mini Fan" --category home --views 1800 --sales 20
python -m tiktok_trend_shop.cli import-csv --file data/products_cn.csv --region CN
python -m tiktok_trend_shop.cli run-batch --limit 10
python -m tiktok_trend_shop.cli show-results --limit 10
python -m unittest discover -s tests
openspec validate bootstrap-app-foundation
```

## Configuration

Copy `.env.example` to `.env` for local development and replace values locally. Real API keys must stay out of source control.

The foundation reads environment variables directly. Provider names are listed in `TTS_SOURCE_PROVIDERS` and `TTS_AI_PROVIDERS`. A provider marked enabled requires its matching API key unless it is a local/manual provider that does not need credentials.

## Modules

- `tiktok_trend_shop.config`: environment loading and secret redaction
- `tiktok_trend_shop.db`: SQLite connection and migrations
- `tiktok_trend_shop.repositories`: workflow record persistence
- `tiktok_trend_shop.jobs`: background job lifecycle
- `tiktok_trend_shop.assets`: local and external asset references
- `tiktok_trend_shop.audit`: source and generated output audit events
- `tiktok_trend_shop.api`: minimal API scaffold
- `tiktok_trend_shop.worker`: worker scaffold

## Demo Workflow

Run a local end-to-end workflow with manually supplied product metrics:

```powershell
$env:PYTHONPATH = "src"
$env:TTS_DATABASE_URL = "sqlite:///./data/demo.sqlite3"
$env:TTS_STORAGE_ROOT = "./assets"
python -m tiktok_trend_shop.cli migrate
python -m tiktok_trend_shop.cli run-demo --title "Mini Fan" --category home --views 1800 --sales 20 --engagement 160 --clicks 30 --conversions 2 --revenue 59
```

The command outputs JSON containing the created product, score, research card, script version, video generation, render asset, export package, publish attempt, metrics snapshot, and feedback signal IDs.

The current video output is a placeholder `video/mp4` asset record that validates the workflow. It is not a real playable TikTok ad yet.

## CSV Import

Use CSV import for product data from Chanmama, Kaogujia, FastMoss, Kalodata, or manual collection.

```powershell
$env:PYTHONPATH = "src"
$env:TTS_DATABASE_URL = "sqlite:///./data/products.sqlite3"
$env:TTS_STORAGE_ROOT = "./assets"
python -m tiktok_trend_shop.cli import-csv --file data/products_cn.csv --region CN
python -m tiktok_trend_shop.cli run-batch --limit 10
python -m tiktok_trend_shop.cli show-results --limit 10
```

CSV columns:

```csv
title,region,category,canonical_url,source_id,views,sales,engagement,growth_rate,commission_rate,margin,competitor_count
```

Numeric fields accept normalized values like `75000` and copied ranking text like `5w~10w`, `100w+`, `5%~10%`.

Optional enrichment columns can be added to the same CSV:

```csv
product_url,platform,item_id,shop_name,brand,price,currency,image_url,selling_points,specs,review_summary,review_highlights,warnings
```

Use `|`, `;`, `、`, or new lines to separate multiple selling points or review highlights. Example:

```csv
title,region,category,product_url,platform,shop_name,brand,price,image_url,selling_points,specs,review_summary,views,sales
Desk Lamp,CN,home,https://item.example/lamp,taobao,Demo Shop,Demo Brand,29.9,https://img.example/lamp.jpg,Soft light|USB powered,color:warm|height:38cm,Looks good on desks,10000,200
```

`show-results` prints the latest local state for each product, including detail completeness, product URL, score, caption, hashtags, render URI, and export package ID.

## Go Backend Migration

The Go backend is being introduced in stages. The Python workflow remains available as the reference implementation while Go takes over production API and worker responsibilities.

Current Go commands:

```powershell
$env:TTS_DATABASE_URL = "sqlite:///./data/go-demo.sqlite3"
go run ./cmd/tkshop migrate
go run ./cmd/tkshop import-csv --file data/products_cn.csv --region CN
go run ./cmd/tkshop show-products --limit 10
go run ./cmd/tkshop serve --addr :8080
```

The first Go slice supports local SQLite migration, CSV product import, detail-column parsing, product listing, and HTTP endpoints:

```text
GET /healthz
GET /products?limit=20
```

Go is intended to become the customer-facing API and worker runtime. Python should remain useful for prompt experiments, data exploration, and reference behavior until each module is migrated.
