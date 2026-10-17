package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"yingce/backend/internal/buildinfo"
	"yingce/backend/internal/bundled"
	"yingce/backend/internal/database"
	"yingce/backend/internal/programupdate"

	"gorm.io/gorm"
)

func runCommand(ctx context.Context, args []string, output io.Writer) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}
	if len(args) != 1 {
		return true, errors.New("用法：yingce-backend [--version|--check|--check-schema|--restore-program]")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	switch args[0] {
	case "--version":
	case "--check", "--check-schema":
		if !bundled.Enabled() {
			return true, errors.New("当前程序未包含完整发布资源，请使用正式 Release 程序")
		}
		if err := bundled.Check(ctx); err != nil {
			return true, err
		}
		if args[0] == "--check-schema" {
			if err := checkDatabaseSchema(ctx); err != nil {
				return true, err
			}
		}
	case "--restore-program":
		path := strings.TrimSpace(os.Getenv("CANVAS_MANAGED_BINARY_PATH"))
		if path == "" {
			return true, errors.New("请设置 CANVAS_MANAGED_BINARY_PATH 为持久化程序路径")
		}
		if err := programupdate.RestoreProgram(ctx, path, env("CANVAS_BACKEND_DATA_DIR", "data")); err != nil {
			return true, err
		}
		_, err := fmt.Fprintln(output, "上一版程序已恢复，请重启应用容器；数据库未修改。")
		return true, err
	default:
		return true, fmt.Errorf("未知参数 %q", args[0])
	}
	return true, json.NewEncoder(output).Encode(buildinfo.Current())
}

func checkDatabaseSchema(ctx context.Context) error {
	config := database.Config{
		Driver:  env("CANVAS_DATABASE_DRIVER", "sqlite"),
		DSN:     os.Getenv("DATABASE_URL"),
		DataDir: env("CANVAS_BACKEND_DATA_DIR", "data"),
	}
	if strings.EqualFold(config.Driver, "sqlite") {
		dsn, err := readOnlySQLiteDSN(config)
		if err != nil {
			return err
		}
		config.DSN = dsn
	}
	db, err := database.Open(config)
	if err != nil {
		return err
	}
	connection, err := db.DB()
	if err != nil {
		return err
	}
	defer connection.Close()
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return database.RequireSchemaVersion(tx)
	}, &sql.TxOptions{ReadOnly: true})
}

func readOnlySQLiteDSN(config database.Config) (string, error) {
	dsn := strings.TrimSpace(config.DSN)
	if dsn == "" {
		path, err := filepath.Abs(filepath.Join(config.DataDir, "open_ai_canvas.db"))
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("读取现有数据库：%w", err)
		}
		dsn = (&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String()
	}
	base, query, _ := strings.Cut(dsn, "?")
	params, err := url.ParseQuery(query)
	if err != nil {
		return "", err
	}
	if strings.Contains(base, ":memory:") || params.Get("mode") == "memory" {
		return "", errors.New("回退校验需要现有持久化数据库")
	}
	if !strings.HasPrefix(base, "file:") {
		base = "file:" + base
	}
	params.Set("mode", "ro")
	params.Set("_query_only", "1")
	params.Del("_journal_mode")
	params.Del("_synchronous")
	return base + "?" + params.Encode(), nil
}
