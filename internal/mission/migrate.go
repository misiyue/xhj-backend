package mission

import (
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

		// 商户 C2C
		&model.Merchant{},
		&model.MerchantTask{},
		&model.MerchantPaytype{},
		&model.MerchantOrder{},
		&model.MerchantHdOrder{},
		&model.MerchantHmOrder{},
		&model.MerchantSession{},
		&model.MerchantMessage{},
		&model.MerchantPayment{},

		// 系统通知
		&model.NoticeLetter{},
		&model.NoticeTemplate{},
		&model.NoticeArticle{},
	)
	
	if err != nil {
		logger.Errorf("数据库表结构迁移失败 Err: %v", err)
		return err
	}

	logger.Infof("数据库表结构迁移成功")
	return nil
}
