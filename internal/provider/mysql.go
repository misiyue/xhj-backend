package provider

import (
	"fmt"
	"log"
	"time"

	logger2 "github.com/gzydong/go-chat/internal/pkg/logger"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/repository/model"
)

func NewMySQLClient(conf *config.Config) *gorm.DB {
	file := logger2.CreateFileWriter(conf.Log.LogFilePath("slow-sql.log"))

	gormConfig := &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
		Logger: logger.New(log.New(file, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             5 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	}

	db, err := gorm.Open(mysql.New(mysql.Config{
		DSN:                       conf.MySQL.Dsn(),
		DisableDatetimePrecision:  true,
		DontSupportRenameIndex:    true,
		DontSupportRenameColumn:   true,
		SkipInitializeWithVersion: false,
	}), gormConfig)

	if err != nil {
		panic(fmt.Errorf("mysql connect error :%v", err))
	}

	if db.Error != nil {
		panic(fmt.Errorf("database error :%v", db.Error))
	}

	// 自动迁移数据库表（逐表迁移，便于定位失败表）
	models := []any{
		&model.InviteCode{},
		&model.OAuthUser{},
		&model.SysMenu{},
		&model.SysRole{},
		&model.SysResource{},
		&model.SysAdminTotp{},
		&model.GroupRobot{},
		&model.GroupRobotMessage{},
		&model.WalletUser{},
		&model.AppVersion{},
		&model.AppExplore{},
		&model.AppModule{},
	}
	for _, m := range models {
		if m == (&model.AppExplore{}) || m == (&model.AppModule{}) {
			table := m.(interface{ TableName() string }).TableName()
			preFixAppTableNullTimestamps(db, table)
		}
		if err := db.AutoMigrate(m); err != nil {
			panic(fmt.Errorf("database auto migrate %T error: %w", m, err))
		}
		if m == (&model.AppExplore{}) || m == (&model.AppModule{}) {
			table := m.(interface{ TableName() string }).TableName()
			ensureAppDatetimeDefaults(db, table)
		}
	}

	sqlDB, _ := db.DB()

	sqlDB.SetMaxIdleConns(conf.MySQL.MaxIdleConnNum)
	sqlDB.SetMaxOpenConns(conf.MySQL.MaxOpenConnNum)
	sqlDB.SetConnMaxLifetime(time.Duration(conf.MySQL.ConnMaxLifetime) * time.Second)

	return db
}

// preFixAppTableNullTimestamps 迁移前补齐空时间，避免 NOT NULL + DEFAULT 变更失败
func preFixAppTableNullTimestamps(db *gorm.DB, table string) {
	if table == "" || !db.Migrator().HasTable(table) {
		return
	}
	_ = db.Exec(fmt.Sprintf("UPDATE `%s` SET `created_at` = NOW() WHERE `created_at` IS NULL", table)).Error
	_ = db.Exec(fmt.Sprintf("UPDATE `%s` SET `updated_at` = NOW() WHERE `updated_at` IS NULL", table)).Error
}

// ensureAppDatetimeDefaults 确保 app 表时间列具备数据库级 DEFAULT（GORM 对已存在列不一定补全默认值）
func ensureAppDatetimeDefaults(db *gorm.DB, table string) {
	if table == "" || !db.Migrator().HasTable(table) {
		return
	}
	err := db.Exec(fmt.Sprintf(
		"ALTER TABLE `%s` "+
			"MODIFY COLUMN `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间', "+
			"MODIFY COLUMN `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'",
		table,
	)).Error
	if err != nil {
		logger2.Warnf("ensure datetime defaults on %s: %v", table, err)
	}
}
