package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Backup struct {
	ID           uint       `gorm:"primaryKey"`
	ServerID     uint       `gorm:"column:server_id"`
	UUID         string     `gorm:"column:uuid"`
	UploadID     *string    `gorm:"column:upload_id"`
	IsSuccessful bool       `gorm:"column:is_successful"`
	IsLocked     bool       `gorm:"column:is_locked"`
	Name         string     `gorm:"column:name"`
	IgnoredFiles string     `gorm:"column:ignored_files"`
	Disk         string     `gorm:"column:disk"`
	Checksum     string     `gorm:"column:checksum"`
	Bytes        int64      `gorm:"column:bytes"`
	CompletedAt  time.Time  `gorm:"column:completed_at"`
	CreatedAt    time.Time  `gorm:"column:created_at"`
	UpdatedAt    time.Time  `gorm:"column:updated_at"`
	DeletedAt    *time.Time `gorm:"column:deleted_at"`
}

func (Backup) TableName() string {
	return "backups"
}

type Config struct {
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	BackupPath string
	GCSchedule string
}

func loadConfig() *Config {
	_ = godotenv.Load()

	return &Config{
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3306"),
		DBUser:     getEnv("DB_USER", "pterodactyl"),
		DBPassword: getEnv("DB_PASSWORD", ""),
		DBName:     getEnv("DB_NAME", "panel"),
		BackupPath: getEnv("BACKUP_PATH", "/mnt/pterodactyl"),
		GCSchedule: getEnv("GC_SCHEDULE", "0 2 * * *"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	log.Printf("Environment variable %s not set, using default value: %s", key, defaultValue)
	return defaultValue
}

func connectDB(config *Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.DBUser, config.DBPassword, config.DBHost, config.DBPort, config.DBName)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %v", err)
	}

	log.Println("Database connection established successfully")
	return db, nil
}

func getValidBackupUUIDs(db *gorm.DB) (map[string]bool, error) {
	var backups []Backup

	// Query all non-soft-deleted backup records
	result := db.Where("deleted_at IS NULL").Find(&backups)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to query backup records: %v", result.Error)
	}

	validUUIDs := make(map[string]bool)
	for _, backup := range backups {
		validUUIDs[backup.UUID] = true
	}

	log.Printf("Found %d valid backup records in database", len(validUUIDs))
	return validUUIDs, nil
}

// UUID regex (8-4-4-4-12)
var uuidRegex = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// Validate UUID format
func isValidUUID(uuid string) bool {
	return uuidRegex.MatchString(strings.ToLower(uuid))
}

func cleanOrphanedBackups(config *Config, validUUIDs map[string]bool) error {
	backupDir := config.BackupPath

	// Check if backup directory exists
	if _, err := os.Stat(backupDir); os.IsNotExist(err) {
		return fmt.Errorf("backup directory does not exist: %s", backupDir)
	}

	// Read all files in backup directory
	files, err := filepath.Glob(filepath.Join(backupDir, "*.tar.gz"))
	if err != nil {
		return fmt.Errorf("failed to read backup directory: %v", err)
	}

	log.Printf("Found %d backup files", len(files))

	deletedCount := 0
	for _, file := range files {
		// Extract UUID from filename
		basename := filepath.Base(file)
		uuid := strings.TrimSuffix(basename, ".tar.gz")

		// Check UUID file name format
		if !isValidUUID(uuid) {
			log.Printf("Skipping file with non-standard UUID format: %s (UUID: %s)", basename, uuid)
			continue
		}

		// Check if UUID exists in database
		if !validUUIDs[uuid] {
			log.Printf("Found orphaned backup file: %s (UUID: %s)", basename, uuid)

			// Delete file
			if err := os.Remove(file); err != nil {
				log.Printf("Failed to delete file %s: %v", basename, err)
				continue
			}

			log.Printf("Successfully deleted orphaned backup file: %s", basename)
			deletedCount++
		}
	}

	log.Printf("Cleanup completed, deleted %d orphaned backup files", deletedCount)
	return nil
}

func runCleanup(config *Config, db *gorm.DB) {
	log.Println("Starting backup cleanup task")

	// Get valid backup UUIDs
	validUUIDs, err := getValidBackupUUIDs(db)
	if err != nil {
		log.Printf("Failed to get valid backup UUIDs: %v", err)
		return
	}

	// Clean orphaned backup files
	if err := cleanOrphanedBackups(config, validUUIDs); err != nil {
		log.Printf("Failed to clean orphaned backup files: %v", err)
		return
	}

	log.Println("Backup cleanup task completed")
}

func main() {
	log.Println("Pterodactyl backup cleaner service starting...")

	// Load configuration
	config := loadConfig()
	log.Printf("Configuration loaded - Backup path: %s, Cron expression: [%s]", config.BackupPath, config.GCSchedule)

	// Connect to database
	db, err := connectDB(config)
	if err != nil {
		log.Fatalf("Database connection failed: %v", err)
	}

	// Create cron scheduler
	c := cron.New()

	// Add cleanup task
	_, err = c.AddFunc(config.GCSchedule, func() {
		runCleanup(config, db)
	})
	if err != nil {
		log.Fatalf("Failed to add cron job: %v", err)
	}

	// Start scheduler
	c.Start()
	log.Printf("Scheduled task started with cron expression: '%s'", config.GCSchedule)

	// Run initial cleanup
	log.Println("Running initial cleanup...")
	runCleanup(config, db)

	select {}
}
