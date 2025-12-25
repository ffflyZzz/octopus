package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"octopus/internal/conf"
	"octopus/internal/model"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OldLLMInfo represents the old schema with ChannelType
type OldLLMInfo struct {
	Name        string  `gorm:"primaryKey;not null"`
	ChannelType int     `gorm:"primaryKey;not null"`
	Input       float64 `json:"input"`
	Output      float64 `json:"output"`
	CacheRead   float64 `json:"cache_read"`
	CacheWrite  float64 `json:"cache_write"`
}

func openDatabase(dbType, dsn string) (*gorm.DB, error) {
	gormConfig := &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)}

	switch strings.ToLower(dbType) {
	case "sqlite":
		return gorm.Open(sqlite.Open(dsn), gormConfig)
	case "mysql":
		if !strings.Contains(dsn, "?") {
			dsn += "?charset=utf8mb4&parseTime=True&loc=Local"
		}
		return gorm.Open(mysql.Open(dsn), gormConfig)
	case "postgres", "postgresql":
		return gorm.Open(postgres.Open(dsn), gormConfig)
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
	}
}

func main() {
	// 加载配置
	if err := conf.Load(""); err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 手动初始化数据库（不使用 db.InitDB 避免自动迁移）
	dbType := conf.AppConfig.Database.Type
	dbPath := conf.AppConfig.Database.Path

	fmt.Println("=== LLM Table Migration: ChannelType -> ChannelID ===")
	fmt.Printf("Database: %s (%s)\n", dbPath, dbType)

	// 使用低级别的数据库连接，不触发AutoMigrate
	database, err := openDatabase(dbType, dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer func() {
		sqlDB, _ := database.DB()
		if sqlDB != nil {
			sqlDB.Close()
		}
	}()

	ctx := context.Background()

	fmt.Println("\nThis script will:")
	fmt.Println("1. Backup existing LLM data")
	fmt.Println("2. Drop the old llm_infos table")
	fmt.Println("3. Create new table with ChannelID instead of ChannelType")
	fmt.Println("4. Migrate data by matching models to channels")
	fmt.Println("\nWARNING: This will modify the database schema!")
	fmt.Print("\nContinue? (yes/no): ")

	var response string
	fmt.Scanln(&response)
	if response != "yes" {
		fmt.Println("Migration cancelled.")
		os.Exit(0)
	}

	// 1. 备份旧数据
	fmt.Println("\n[1/4] Backing up old LLM data...")
	var oldModels []OldLLMInfo
	if err := database.WithContext(ctx).Table("llm_infos").Find(&oldModels).Error; err != nil {
		log.Fatalf("Failed to backup old data: %v", err)
	}
	fmt.Printf("Backed up %d models\n", len(oldModels))

	// 2. 获取所有渠道信息
	fmt.Println("\n[2/4] Loading channel information...")
	var channels []model.Channel
	if err := database.WithContext(ctx).Find(&channels).Error; err != nil {
		log.Fatalf("Failed to load channels: %v", err)
	}
	fmt.Printf("Found %d channels\n", len(channels))

	// 构建 channel type -> channel ID 的映射（使用第一个匹配的渠道）
	channelTypeToID := make(map[int]int)
	for _, ch := range channels {
		if _, exists := channelTypeToID[int(ch.Type)]; !exists {
			channelTypeToID[int(ch.Type)] = ch.ID
		}
	}

	// 3. 删除旧表并重建
	fmt.Println("\n[3/4] Dropping old table and creating new schema...")

	// 检查表是否存在
	if database.WithContext(ctx).Migrator().HasTable("llm_infos") {
		fmt.Println("Table llm_infos exists, dropping...")
		// 使用原生SQL删除表（更可靠）
		if err := database.WithContext(ctx).Exec("DROP TABLE IF EXISTS llm_infos").Error; err != nil {
			log.Fatalf("Failed to drop old table: %v", err)
		}
	}

	// 再次确认表已被删除
	if database.WithContext(ctx).Migrator().HasTable("llm_infos") {
		log.Fatalf("Table llm_infos still exists after drop!")
	}
	fmt.Println("Old table dropped successfully")

	// 使用原生SQL创建新表（避免AutoMigrate问题）
	createTableSQL := `
		CREATE TABLE llm_infos (
			name TEXT NOT NULL,
			channel_id INTEGER NOT NULL,
			input REAL NOT NULL DEFAULT 0,
			output REAL NOT NULL DEFAULT 0,
			cache_read REAL NOT NULL DEFAULT 0,
			cache_write REAL NOT NULL DEFAULT 0,
			PRIMARY KEY (name, channel_id)
		)
	`
	if err := database.WithContext(ctx).Exec(createTableSQL).Error; err != nil {
		log.Fatalf("Failed to create new table: %v", err)
	}
	fmt.Println("New table created successfully")

	// 4. 迁移数据
	fmt.Println("\n[4/4] Migrating data to new schema...")
	migrated := 0
	skipped := 0

	for _, oldModel := range oldModels {
		channelID, found := channelTypeToID[oldModel.ChannelType]
		if !found {
			fmt.Printf("  WARNING: No channel found for type %d, skipping model '%s'\n", oldModel.ChannelType, oldModel.Name)
			skipped++
			continue
		}

		newModel := model.LLMInfo{
			Name:      oldModel.Name,
			ChannelID: channelID,
			LLMPrice: model.LLMPrice{
				Input:      oldModel.Input,
				Output:     oldModel.Output,
				CacheRead:  oldModel.CacheRead,
				CacheWrite: oldModel.CacheWrite,
			},
		}

		if err := database.WithContext(ctx).Create(&newModel).Error; err != nil {
			fmt.Printf("  WARNING: Failed to migrate model '%s' (channel_id=%d): %v\n", newModel.Name, channelID, err)
			skipped++
		} else {
			migrated++
		}
	}

	fmt.Println("\n=== Migration Summary ===")
	fmt.Printf("Total models in backup: %d\n", len(oldModels))
	fmt.Printf("Successfully migrated: %d\n", migrated)
	fmt.Printf("Skipped: %d\n", skipped)
	fmt.Println("\nMigration completed!")
	fmt.Println("\nNOTE: Models are now associated with specific channel IDs instead of channel types.")
	fmt.Println("If you have multiple channels of the same type, you may need to manually")
	fmt.Println("assign models to the correct channels using the web interface.")
}
