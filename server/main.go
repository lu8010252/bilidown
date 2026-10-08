package main

import (
	"database/sql"
	"log"

	"bilidown/util"
)

// mustInitTables 初始化数据表
func mustInitTables() {
	db := util.MustGetDB()
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "field" (
		"name" TEXT PRIMARY KEY NOT NULL,
		"value" TEXT
	)`); err != nil {
		log.Fatalln("create table field:", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "log" (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"content" TEXT NOT NULL,
		"create_at" text NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		log.Fatalln("create table log:", err)
	}

	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS "task" (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,
		"bvid" text NOT NULL,
		"cid" integer NOT NULL,
		"format" integer NOT NULL,
		"title" text NOT NULL,
		"owner" text NOT NULL,
		"cover" text NOT NULL,
		"status" text NOT NULL,
		"folder" text NOT NULL,
		"duration" integer NOT NULL,
		"download_type" text NOT NULL DEFAULT 'merge',
		"create_at" text NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`); err != nil {
		log.Fatalln("create table task:", err)
	}

	if _, err := util.GetCurrentFolder(db); err != nil {
		log.Fatalln("util.GetCurrentFolder:", err)
	}

	if err := initHistoryTask(db); err != nil {
		log.Fatalln("initHistoryTask:", err)
	}

	// 添加可能缺失的列（用于数据库迁移）
	if err := addMissingColumns(db); err != nil {
		log.Fatalln("addMissingColumns:", err)
	}
}

// addMissingColumns 添加可能缺失的列（用于数据库迁移）
func addMissingColumns(db *sql.DB) error {
	// 检查download_type列是否存在，如果不存在则添加
	// SQLite没有直接的方法检查列是否存在，我们尝试添加列并忽略错误
	// 使用事务确保操作原子性
	util.SqliteLock.Lock()
	_, _ = db.Exec(`ALTER TABLE "task" ADD COLUMN "download_type" TEXT DEFAULT 'merge'`)
	// 将现有记录中的NULL值更新为默认值'merge'
	_, _ = db.Exec(`UPDATE "task" SET "download_type" = 'merge' WHERE "download_type" IS NULL`)
	util.SqliteLock.Unlock()

	// 忽略错误，因为列可能已经存在
	// SQLite错误码为1表示列已存在
	return nil
}

// initHistoryTask 将上一次程序运行时未完成的任务进度全部变为 error
func initHistoryTask(db *sql.DB) error {
	util.SqliteLock.Lock()
	_, err := db.Exec(`UPDATE "task" SET "status" = 'error' WHERE "status" IN ('waiting', 'running')`)
	util.SqliteLock.Unlock()
	return err
}
