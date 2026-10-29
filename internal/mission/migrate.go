package mission

import (
	"fmt"

	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/urfave/cli/v2"
	"gorm.io/gorm"
)

type MigrateProvider struct {
	Config *config.Config
	DB     *gorm.DB
}

func Migrate(_ *cli.Context, app *MigrateProvider) error {
	logger.Infof("数据库初始化中...")
	defer logger.Infof("数据库初始化完成")

	// 使用 GORM AutoMigrate 自动创建或更新数据库表结构
	//
	// 此迁移包括所有应用程序表：
	// 1. 原 lumenim.sql 中的 28 张表（admin, users, contact*, group*, article*, talk*, emoticon*, organize*, robot, file_upload）
	// 2. 系统表（sys_*, group_robot*, invite_code, oauth_user, wallet_user, red_envelope, sequence）
	//    注意：部分系统表在 mysql.go 中也有 AutoMigrate 调用，这是安全的因为 GORM AutoMigrate 是幂等操作
	//    此处统一列出所有表以确保通过 migrate 命令进行完整的数据库初始化
	err := app.DB.AutoMigrate(
		// 用户相关
		&model.Users{},
		&model.Admin{},

		// 联系人相关
		&model.Contact{},
		&model.ContactApply{},
		&model.ContactGroup{},

		// 群组相关
		&model.Group{},
		&model.GroupApply{},
		&model.GroupMember{},
		&model.GroupNotice{},
		&model.GroupVote{},
		&model.GroupVoteAnswer{},

		// 文章相关
		&model.Article{},
		&model.ArticleAnnex{},
		&model.ArticleClass{},
		&model.ArticleTag{},
		&model.ArticleHistory{},

		// 消息相关
		&model.TalkSession{},
		&model.TalkUserMessage{},
		&model.TalkGroupMessage{},
		&model.TalkGroupMessageDel{},
		&model.TalkGroupMsgReader{},

		// 表情包相关
		&model.Emoticon{},
		&model.EmoticonItem{},
		&model.UsersEmoticon{},

		// 组织架构相关
		&model.Organize{},
		&model.OrganizeDept{},
		&model.OrganizePost{},

		// 机器人相关
		&model.Robot{},
		&model.GroupRobot{},
		&model.GroupRobotMessage{},

		// 文件上传相关
		&model.FileUpload{},

		// 系统管理相关
		&model.SysMenu{},
		&model.SysRole{},
		&model.SysResource{},
		&model.SysAdminTotp{},

		// 其他
		&model.InviteCode{},
		&model.OAuthUser{},
		&model.WalletUser{},
		&model.RedEnvelope{},
		&model.Sequence{},
		&model.AppVersion{},
		&model.AppExplore{},
		&model.AppModule{},
		&model.AppNews{},
		&model.AppNewsCategory{},
		&model.AppNewsView{},
		&model.MarzbanCheckin{},
		&model.MarzbanCheckinState{},

		// 商户 C2C
		&model.Merchant{},
		&model.MerchantTask{},
		&model.MerchantPaytype{},
		&model.MerchantOrder{},
		&model.MerchantOrderErrLog{},
		&model.MerchantHdOrder{},
		&model.MerchantHmOrder{},
		&model.MerchantSession{},
		&model.MerchantMessage{},
		&model.MerchantPayment{},

		// 系统通知
		&model.NoticeLetter{},
		&model.NoticeTemplate{},
		&model.NoticeArticle{},
		&model.UserFaker{},
		&model.GroupFaker{},
		&model.UserCmd{},
		&model.UserReport{},
		&model.GroupTotop{},
	)

	if err != nil {
		logger.Errorf("数据库表结构迁移失败 Err: %v", err)
		return err
	}

	// app_version / app_explore / app_module：确保时间列具备数据库级 DEFAULT
	for _, table := range []string{"app_version", "app_explore", "app_module"} {
		if !app.DB.Migrator().HasTable(table) {
			continue
		}
		_ = app.DB.Exec(fmt.Sprintf("UPDATE `%s` SET `created_at` = NOW() WHERE `created_at` IS NULL", table)).Error
		_ = app.DB.Exec(fmt.Sprintf("UPDATE `%s` SET `updated_at` = NOW() WHERE `updated_at` IS NULL", table)).Error
		if err := app.DB.Exec(fmt.Sprintf(
			"ALTER TABLE `%s` "+
				"MODIFY COLUMN `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间', "+
				"MODIFY COLUMN `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'",
			table,
		)).Error; err != nil {
			logger.Warnf("ensure datetime defaults on %s: %v", table, err)
		}
	}

	// user_report：补齐时间列默认值
	if app.DB.Migrator().HasTable("user_report") {
		if err := app.DB.Exec(
			"ALTER TABLE `user_report` " +
				"MODIFY COLUMN `created_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间', " +
				"MODIFY COLUMN `updated_at` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'",
		).Error; err != nil {
			logger.Warnf("ensure user_report datetime defaults: %v", err)
		}
	}

	// group：补齐 force_status / is_allow_invite / 时间列属性（AutoMigrate 对已存在列默认值不一定生效）
	if app.DB.Migrator().HasTable("group") {
		if !app.DB.Migrator().HasColumn(&model.Group{}, "force_status") {
			if err := app.DB.Exec(
				"ALTER TABLE `group` ADD COLUMN `force_status` tinyint(4) NULL DEFAULT 0 COMMENT '强制状态：1.群禁言'",
			).Error; err != nil {
				logger.Warnf("add group.force_status: %v", err)
			}
		}
		if err := app.DB.Exec(
			"ALTER TABLE `group` " +
				"MODIFY COLUMN `is_allow_invite` tinyint(4) NULL DEFAULT 1 COMMENT '群内是否允许成员互加好友', " +
				"MODIFY COLUMN `force_status` tinyint(4) NULL DEFAULT 0 COMMENT '强制状态：1.群禁言', " +
				"MODIFY COLUMN `created_at` datetime NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间', " +
				"MODIFY COLUMN `updated_at` datetime NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'",
		).Error; err != nil {
			logger.Warnf("ensure group column defaults: %v", err)
		}
	}

	logger.Infof("数据库表结构迁移成功")
	return nil
}
