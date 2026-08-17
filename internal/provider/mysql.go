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

	// 自动迁移数据库表
	err = db.AutoMigrate(
		&model.InviteCode{},
		&model.OAuthUser{},
		&model.SysMenu{},
		&model.SysRole{},
		&model.SysResource{},
		&model.SysAdminTotp{},
		&model.GroupRobot{},
		&model.GroupRobotMessage{},
		&model.WalletUser{},
		&model.YunxinCredential{},
		&model.AppVersion{},
		&model.AppExplore{},
		&model.AppModule{},
	)
	if err != nil {
		panic(fmt.Errorf("database error :%v", err))
	}

	sqlDB, _ := db.DB()

	sqlDB.SetMaxIdleConns(conf.MySQL.MaxIdleConnNum)
	sqlDB.SetMaxOpenConns(conf.MySQL.MaxOpenConnNum)
	sqlDB.SetConnMaxLifetime(time.Duration(conf.MySQL.ConnMaxLifetime) * time.Second)

	return db
}
