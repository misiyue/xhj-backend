package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gzydong/go-chat/external/wallet"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/utils"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/gzydong/go-chat/internal/repository/repo"
	"github.com/samber/lo"
	"gorm.io/gorm"
)

var _ IUserService = (*UserService)(nil)

type IUserService interface {
	Register(ctx context.Context, opt *UserRegisterOpt) (*model.Users, error)
	Login(ctx context.Context, account string, password string) (*model.Users, error)
	Forget(ctx context.Context, opt *UserForgetOpt) (bool, error)
	UpdatePassword(ctx context.Context, uid int, oldPassword string, password string) error
	OauthBind(ctx context.Context, mobile string, oauthUser *model.OAuthUser) (int, error)
}

type UserService struct {
	UsersRepo      *repo.Users
	OAuthUsersRepo *repo.OAuthUsers
}

// syncUCardIfNeeded 在用户缺少 U 卡信息时进行同步（注册第三方钱包账户）
// 规则：
// - 当 users.uuid == 0 或 users.user_code 为空时触发
// - third_uid 使用 IM 用户自增 Id（user.Id）
// - phone 优先 Mobile，其次 Email，再其次 Username
// - 同步成功后写回 users.uuid（钱包 user_id）与 users.user_code（钱包 uuid）
func (s *UserService) syncUCardIfNeeded(ctx context.Context, user *model.Users, plainPassword string) error {
	if user == nil || user.Id == 0 {
		return nil
	}
	if user.Uuid != 0 {
		return nil
	}
	// if user.Uuid != 0 && user.UserCode != "" {
	// 	return nil
	// }

	c := wallet.GetClient()
	if c == nil {
		return fmt.Errorf("钱包客户端未初始化，请配置 wallet.base_url 与 wallet.key")
	}

	phone := user.Username
	if user.Email != "" {
		phone = user.Email
	}
	if user.Mobile != nil && *user.Mobile != "" {
		phone = *user.Mobile
	}
	if phone == "" {
		phone = fmt.Sprintf("im_user_%d", user.Id)
	}

	reg, err := c.RegisterThirdParty(phone, plainPassword, fmt.Sprintf("%d", user.Id))
	if err != nil {
		return err
	}

	walletUID, err := wallet.ParseRegisterUserID(reg.UserID)
	if err != nil {
		return err
	}

	_, err = s.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"uuid":       walletUID,
		"user_code":  reg.UUID,
		"updated_at": time.Now(),
	})
	if err != nil {
		return err
	}

	user.Uuid = walletUID
	user.UserCode = reg.UUID
	return nil
}

type UserRegisterOpt struct {
	Nickname string
	Mobile   string
	Email    string
	Password string
	Platform string
	Username string // 可选：显式指定登录用户名
	// InviteUserId 邀请人用户 ID（来自有效邀请码的生成者）；0 表示无
	InviteUserId int
	// DeviceCode 客户端设备码（可选；开启设备注册限制时由接口层校验）
	DeviceCode string
}

// Register 注册用户（仅校验邮箱/手机号是否重复，不校验 username 唯一性）
func (s *UserService) Register(ctx context.Context, opt *UserRegisterOpt) (*model.Users, error) {
	// 检查手机号是否已存在
	if opt.Mobile != "" && s.UsersRepo.IsMobileExist(ctx, opt.Mobile) {
		return nil, errors.New("手机号已被注册")
	}

	// 检查邮箱是否已存在（不检查 username）
	if opt.Email != "" {
		if user, _ := s.UsersRepo.FindByEmail(ctx, opt.Email); user != nil && user.Id > 0 {
			return nil, errors.New("邮箱已被注册")
		}
	}

	salt := encrypt.GenerateSalt()
	user := &model.Users{
		Nickname:  opt.Nickname,
		Email:     opt.Email,
		Gender:    model.UsersGenderDefault,
		Password:  encrypt.HashPassword(opt.Password, salt),
		Salt:      salt,
		IsRobot:   model.No,
		Status:    model.UsersStatusNormal,
		UserId:    0,    // 旧 IM 系统用户 ID，新注册时先置 0，插入后再回填为自增 Id
		Uuid:      0,    // 第三方 ID，创建钱包时由原有钱包逻辑回填
		UserCode:  "",   // u 卡唯一标识，同上
		Money:     0.00, // 初始余额为 0
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if opt.InviteUserId > 0 {
		user.InviteUserId = opt.InviteUserId
	}
	user.DeviceCode = opt.DeviceCode

	// 设置手机号（如果提供）
	if opt.Mobile != "" {
		user.Mobile = lo.ToPtr(opt.Mobile)
	}

	// 设置用户名（登录名）：
	// 优先使用显式传入的 Username，否则依次回退到 Mobile / Email / Nickname
	if opt.Username != "" {
		user.Username = opt.Username
	} else if opt.Mobile != "" {
		user.Username = opt.Mobile
	} else if opt.Email != "" {
		user.Username = opt.Email
	} else {
		user.Username = opt.Nickname
	}

	if err := s.UsersRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	// 为新注册用户回填 user_id（旧系统 ID），这里直接使用当前自增 Id，保证不会与已有导入数据冲突
	// 如果未来有从旧 IM 导入的用户，可以在导入脚本中单独设置 user_id，这里只作用于通过 Register 创建的新用户。
	if user.Id > 0 {
		_, _ = s.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
			"user_id": user.Id,
		})
		user.UserId = user.Id
	}

	// 注册时同步 U 卡信息（写回 users.uuid / users.user_code）
	if err := s.syncUCardIfNeeded(ctx, user, opt.Password); err != nil {
		return nil, err
	}

	return user, nil
}

// Login 登录处理
// account 为前端登录框内容（接口字段名仍为 mobile）：按 users.username 或 users.email 精确匹配
func (s *UserService) Login(ctx context.Context, account string, password string) (*model.Users, error) {
	user, err := s.UsersRepo.FindByUsernameOrEmail(ctx, account)
	if err != nil {
		if utils.IsSqlNoRows(err) {
			return nil, entity.ErrAccountOrPassword
		}

		return nil, err
	}

	if user == nil || user.Id == 0 {
		return nil, entity.ErrAccountOrPassword
	}

	if !encrypt.VerifyPassword(user.Password, password, user.Salt) {
		return nil, entity.ErrAccountOrPassword
	}

	if user.IsCancelled() {
		return nil, entity.ErrAccountOrPassword
	}

	if user.IsDisabled() {
		return nil, entity.ErrAccountDisabled
	}

	// 登录时补同步 U 卡信息：若 uuid 或 user_code 不存在则调用接口同步写回
	if err := s.syncUCardIfNeeded(ctx, user, password); err != nil {
		return nil, err
	}

	return user, nil
}

// UserForgetOpt ForgetRequest 账号找回接口验证
type UserForgetOpt struct {
	Email     string
	Password  string
	EmailCode string
}

// Forget 账号找回
func (s *UserService) Forget(ctx context.Context, opt *UserForgetOpt) (bool, error) {
	user, err := s.UsersRepo.FindByEmail(ctx, opt.Email)
	if err != nil || user.Id == 0 {
		return false, errors.New("账号不存在! ")
	}

	newSalt := encrypt.GenerateSalt()
	affected, err := s.UsersRepo.UpdateById(context.TODO(), user.Id, map[string]any{
		"password": encrypt.HashPassword(opt.Password, newSalt),
		"salt":     newSalt,
	})

	return affected > 0, err
}

// UpdatePassword 修改用户密码
func (s *UserService) UpdatePassword(ctx context.Context, uid int, oldPassword string, password string) error {
	user, err := s.UsersRepo.FindById(ctx, uid)
	if err != nil {
		return errors.New("用户不存在！")
	}

	if !encrypt.VerifyPassword(user.Password, oldPassword, user.Salt) {
		return errors.New("密码验证不正确！")
	}

	newSalt := encrypt.GenerateSalt()
	_, err = s.UsersRepo.UpdateById(ctx, user.Id, map[string]any{
		"password": encrypt.HashPassword(password, newSalt),
		"salt":     newSalt,
	})

	return err
}

func (s *UserService) OauthBind(ctx context.Context, mobile string, oauthUser *model.OAuthUser) (int, error) {
	userinfo, err := s.UsersRepo.FindByMobile(ctx, mobile)
	if err != nil && !utils.IsSqlNoRows(err) {
		return 0, err
	}

	if userinfo != nil {
		oauth, err := s.OAuthUsersRepo.FindByWhere(ctx, "user_id = ? and oauth_type = ?", userinfo.Id, oauthUser.OAuthType)
		if err != nil && !utils.IsSqlNoRows(err) {
			return 0, err
		}

		if oauth != nil && oauth.UserId == int32(userinfo.Id) {
			return userinfo.Id, nil
		}

		if oauth != nil {
			return 0, entity.ErrAccountBinded
		}

		_, err = s.OAuthUsersRepo.UpdateByWhere(ctx, map[string]any{
			"user_id": userinfo.Id,
		}, "id = ?", oauthUser.Id)
		if err != nil {
			return 0, err
		}

		return userinfo.Id, nil
	}

	user := &model.Users{
		Mobile:    lo.ToPtr(mobile),
		Nickname:  oauthUser.Nickname,
		Avatar:    oauthUser.Avatar,
		Gender:    3,
		IsRobot:   1,
		Status:    model.UsersStatusNormal,
		CreatedAt: time.Time{},
		UpdatedAt: time.Time{},
	}

	err = s.UsersRepo.Txx(ctx, func(tx *gorm.DB) error {
		if err := tx.Create(user).Error; err != nil {
			return err
		}

		if err := tx.Model(&model.OAuthUser{}).Where("id = ?", oauthUser.Id).Updates(&model.OAuthUser{
			UserId: int32(user.Id),
		}).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return 0, err
	}

	return user.Id, nil
}
