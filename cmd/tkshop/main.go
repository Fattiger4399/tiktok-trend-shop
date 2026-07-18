package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"tiktok-trend-shop/internal/apiv1"
	"tiktok-trend-shop/internal/auth"
	"tiktok-trend-shop/internal/category"
	"tiktok-trend-shop/internal/config"
	"tiktok-trend-shop/internal/copygen"
	"tiktok-trend-shop/internal/dossier"
	"tiktok-trend-shop/internal/httpapi"
	"tiktok-trend-shop/internal/importer"
	"tiktok-trend-shop/internal/ingest"
	"tiktok-trend-shop/internal/mediagen"
	"tiktok-trend-shop/internal/product"
	"tiktok-trend-shop/internal/request"
	"tiktok-trend-shop/internal/review"
	"tiktok-trend-shop/internal/score"
	"tiktok-trend-shop/internal/store"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		return usageError()
	}
	cfg := config.FromEnv()
	switch args[0] {
	case "migrate":
		return withDB(cfg, func(db store.DB, repo *product.Repository) error {
			return nil
		})
	case "import-csv":
		flags := flag.NewFlagSet("import-csv", flag.ContinueOnError)
		file := flags.String("file", "", "CSV file path")
		region := flags.String("region", "CN", "Default region")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		if *file == "" {
			return fmt.Errorf("--file is required")
		}
		return withDB(cfg, func(db store.DB, repo *product.Repository) error {
			result, err := ingest.NewImporter(repo).ImportCSV(context.Background(), *file, *region)
			if err != nil {
				return err
			}
			return printJSON(result)
		})
	case "show-products":
		flags := flag.NewFlagSet("show-products", flag.ContinueOnError)
		limit := flags.Int("limit", 20, "Maximum number of products")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return withDB(cfg, func(db store.DB, repo *product.Repository) error {
			results, err := repo.ListProducts(context.Background(), *limit)
			if err != nil {
				return err
			}
			return printJSON(map[string]any{"count": len(results), "results": results})
		})
	case "serve":
		flags := flag.NewFlagSet("serve", flag.ContinueOnError)
		addr := flags.String("addr", cfg.HTTPAddr, "HTTP listen address")
		staticDir := flags.String("static-dir", cfg.StaticDir, "Directory of static frontend files")
		if err := flags.Parse(args[1:]); err != nil {
			return err
		}
		return withDB(cfg, func(db store.DB, repo *product.Repository) error {
			catRepo := category.NewRepository(db)
			if err := catRepo.SeedCanonical(context.Background()); err != nil {
				return err
			}
			authSvc := auth.NewService(auth.NewRepository(db))
			if err := authSvc.SeedOperator(context.Background()); err != nil {
				return err
			}
			imp := importer.New(db, repo)
			sc := score.NewRepository(db, repo)
			rq := request.NewRepository(db)
			cgRepo := copygen.NewRepository(db)
			cg := copygen.NewService(rq, repo, cgRepo, copygen.NewProviderFromEnv())
			rv := review.NewService(rq, repo, cgRepo, review.NewRepository(db))
			ds := dossier.NewRepository(db)
			mg := mediagen.NewService(repo, mediagen.NewAssetStore(db, cfg.StorageRoot), mediagen.NewRepository(db), mediagen.NewProviderFromEnv())
			v1 := apiv1.NewServer(db, repo, catRepo, imp, sc, rq, cg, rv, authSvc, ds, mg)
			mux := http.NewServeMux()
			mux.Handle("/api/v1/", v1.Handler())
			mux.Handle("/healthz", httpapi.New(repo))
			mux.Handle("/products", httpapi.New(repo))
			if cfg.StorageRoot != "" {
				assetsServer, err := newAssetsServer(cfg.StorageRoot)
				if err != nil {
					return err
				}
				mux.Handle("/assets/", assetsServer)
			}
			if *staticDir != "" {
				staticServer, err := newStaticServer(*staticDir)
				if err != nil {
					return err
				}
				mux.Handle("/", staticServer)
			}
			log.Printf("tkshop Go API listening on %s", *addr)
			return http.ListenAndServe(*addr, mux)
		})
	default:
		return usageError()
	}
}

func withDB(cfg config.Config, fn func(store.DB, *product.Repository) error) error {
	db, err := store.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	if err := store.Migrate(db); err != nil {
		return err
	}
	return fn(db, product.NewRepository(db))
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func usageError() error {
	return fmt.Errorf(`usage: go run ./cmd/tkshop <command>

commands:
  migrate
  import-csv --file <path> [--region CN]
  show-products [--limit 20]
  serve [--addr :8080] [--static-dir <path>]`)
}
