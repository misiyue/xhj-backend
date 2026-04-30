package main

import (
	"fmt"
	"log"
	"os"

	"gopkg.in/yaml.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// DatabaseConfig represents the database configuration
type DatabaseConfig struct {
	Database struct {
		Mysql struct {
			Host     string `yaml:"host"`
			Port     int    `yaml:"port"`
			Username string `yaml:"username"`
			Password string `yaml:"password"`
			Database string `yaml:"database"`
		} `yaml:"mysql"`
	} `yaml:"database"`
}

// DatabaseCleaner handles database cleanup operations
type DatabaseCleaner struct {
	configPath string
	db         *gorm.DB
}

// NewDatabaseCleaner creates a new database cleaner
func NewDatabaseCleaner(configPath string) *DatabaseCleaner {
	return &DatabaseCleaner{
		configPath: configPath,
	}
}

// Cleanup performs database cleanup
func (c *DatabaseCleaner) Cleanup() error {
	// Load config
	configData, err := os.ReadFile(c.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config DatabaseConfig
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Initialize database connection
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.Database.Mysql.Username,
		config.Database.Mysql.Password,
		config.Database.Mysql.Host,
		config.Database.Mysql.Port,
		config.Database.Mysql.Database,
	)

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	c.db = db

	log.Println("Starting database cleanup...")

	// Delete test users and related data
	if err := c.deleteTestData(); err != nil {
		return fmt.Errorf("failed to delete test data: %w", err)
	}

	log.Println("Database cleanup completed successfully!")
	return nil
}

// deleteTestData deletes all test-related data
func (c *DatabaseCleaner) deleteTestData() error {
	// Begin transaction
	tx := c.db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Delete test users (users with stress_ prefix in mobile or nickname)
	log.Println("Deleting test users...")
	var testUserIDs []int
	
	if err := tx.Table("users").
		Where("mobile LIKE ? OR nickname LIKE ?", "stress_%", "stress_%").
		Pluck("id", &testUserIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(testUserIDs) == 0 {
		log.Println("No test users found to delete")
		tx.Commit()
		return nil
	}

	log.Printf("Found %d test users to delete\n", len(testUserIDs))

	// Delete related data for test users
	log.Println("Deleting user messages...")
	if err := tx.Table("talk_user_message").
		Where("from_id IN ? OR receiver_id IN ?", testUserIDs, testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	log.Println("Deleting contact applications...")
	if err := tx.Table("contact_apply").
		Where("user_id IN ? OR friend_id IN ?", testUserIDs, testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	log.Println("Deleting contacts...")
	if err := tx.Table("contact").
		Where("user_id IN ? OR friend_id IN ?", testUserIDs, testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	log.Println("Deleting user sessions...")
	if err := tx.Table("talk_session").
		Where("user_id IN ?", testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Get groups created by test users
	var testGroupIDs []int
	if err := tx.Table("group").
		Where("creator_id IN ?", testUserIDs).
		Pluck("id", &testGroupIDs).Error; err != nil {
		tx.Rollback()
		return err
	}

	if len(testGroupIDs) > 0 {
		log.Printf("Found %d test groups to delete\n", len(testGroupIDs))

		log.Println("Deleting group messages...")
		if err := tx.Table("talk_group_message").
			Where("group_id IN ?", testGroupIDs).
			Delete(nil).Error; err != nil {
			tx.Rollback()
			return err
		}

		log.Println("Deleting group members...")
		if err := tx.Table("group_member").
			Where("group_id IN ?", testGroupIDs).
			Delete(nil).Error; err != nil {
			tx.Rollback()
			return err
		}

		log.Println("Deleting groups...")
		if err := tx.Table("group").
			Where("id IN ?", testGroupIDs).
			Delete(nil).Error; err != nil {
			tx.Rollback()
			return err
		}
	}

	// Delete group memberships for test users
	log.Println("Deleting group memberships...")
	if err := tx.Table("group_member").
		Where("user_id IN ?", testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Finally, delete the test users themselves
	log.Println("Deleting test user accounts...")
	if err := tx.Table("users").
		Where("id IN ?", testUserIDs).
		Delete(nil).Error; err != nil {
		tx.Rollback()
		return err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return err
	}

	log.Printf("Successfully deleted %d test users and their related data\n", len(testUserIDs))
	return nil
}
