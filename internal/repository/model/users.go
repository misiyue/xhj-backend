package model

import "time"

const (
	UsersGenderDefault = 3

	UsersStatusNormal   = 1
	UsersStatusDisabled = 2
)

const UserInviteCodeLen = 6

const inviteCodeAlphabet = "0123456789abcdefghijklmnopqrstuvwxyz"

// DeriveUserInviteCode 根据 user id 运算生成 6 位数字+小写字母邀请码（salt 用于碰撞重试）
func DeriveUserInviteCode(userID int, salt int) string {
	var out [UserInviteCodeLen]byte
	x := uint64(userID)*2654435761 + uint64(salt)*2246822519 + 97531
	for i := 0; i < UserInviteCodeLen; i++ {
		x = x*6364136223846793005 + 1
		out[i] = inviteCodeAlphabet[x%uint64(len(inviteCodeAlphabet))]
	}
	return string(out[:])
}

type Users struct {
	Id           int       `gorm:"column:id;primary_key;AUTO_INCREMENT" json:"id"` // 新 IM 系统自增 ID
	UserId       int       `gorm:"column:user_id;type:int(11);not null" json:"user_id"`
	Username     string    `gorm:"column:username;type:varchar(100)" json:"username"`                 // 登录用户名（新的登录名）
	Mobile       *string   `gorm:"column:mobile;type:varchar(32);index" json:"mobile"`                // 手机号
	Nickname     string    `gorm:"column:nickname;type:varchar(64);index" json:"nickname"`            // 用户昵称
	Avatar       string    `gorm:"column:avatar;type:varchar(255)" json:"avatar"`                     // 用户头像地址
	Gender       int       `gorm:"column:gender;" json:"gender"`                                      // 用户性别 1:男 2:女 3:未知
	Password     string    `gorm:"column:password;type:varchar(255)" json:"-"`                        // 用户密码
	Salt         string    `gorm:"column:salt;type:varchar(16)" json:"-"`                             // 密码盐值
	Motto        string    `gorm:"column:motto;type:varchar(255)" json:"motto"`                       // 用户座右铭
	Email        string    `gorm:"column:email;type:varchar(128);index" json:"email"`                 // 用户邮箱
	Birthday     string    `gorm:"column:birthday;type:varchar(32)" json:"birthday"`                  // 生日
	Uuid         int       `gorm:"column:uuid;type:int(11);default:0" json:"uuid"`                    // 第三方系统用户 ID
	UserCode     string    `gorm:"column:user_code;type:varchar(32)" json:"user_code"`                // u 卡系统用户唯一标识
	Money        float64   `gorm:"column:money;type:decimal(10,2);default:0.00" json:"money"`         // 余额
	Trans        string    `gorm:"column:trans;type:varchar(255)" json:"-"`                           // 交易密码（不返回给前端）
	IsTrans      int       `gorm:"column:is_trans;type:int(11);default:0" json:"is_trans"`            // 是否已设置交易密码 0/1
	IsRobot      int       `gorm:"column:is_robot;" json:"is_robot"`                                  // 是否机器人[1:否;2:是;]
	Status       int       `gorm:"column:status;" json:"status"`                                      // 用户状态[1:正常;2:停用;3:注销]
	InviteUserId int       `gorm:"column:invite_user_id;default:0" json:"invite_user_id"`             // 邀请人 users.id
	InviteCode   string    `gorm:"column:invite_code;type:varchar(6);uniqueIndex" json:"invite_code"` // 邀请码（6位数字+小写字母）
	DeviceCode   string    `gorm:"column:device_code;type:varchar(128)" json:"device_code"`           // 客户端设备码
	CreatedAt    time.Time `gorm:"column:created_at;" json:"created_at"`                              // 注册时间
	UpdatedAt    time.Time `gorm:"column:updated_at;" json:"updated_at"`                              // 更新时间
}

func (u Users) TableName() string {
	return "users"
}

func (u Users) TablePrimaryId() string {
	return "id"
}

func (u Users) TablePrimaryIdValue() int {
	return u.Id
}

func (u Users) IsDisabled() bool {
	return u.Status == UsersStatusDisabled
}
