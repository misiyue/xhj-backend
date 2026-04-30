package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gopkg.in/yaml.v2"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// ---------------------------------------------------------------------------
// Config
// ---------------------------------------------------------------------------

type Config struct {
	MySQL struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Database string `yaml:"database"`
		Username string `yaml:"username"`
		Password string `yaml:"password"`
		Charset  string `yaml:"charset"`
	} `yaml:"mysql"`
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg := &Config{}

	// Try top-level "mysql:" first, then "app.mysql:"
	var mysqlMap map[interface{}]interface{}
	if v, ok := raw["mysql"]; ok {
		if m, ok := v.(map[interface{}]interface{}); ok {
			mysqlMap = m
		}
	}
	if mysqlMap == nil {
		if appRaw, ok := raw["app"]; ok {
			if appMap, ok := appRaw.(map[interface{}]interface{}); ok {
				if v, ok := appMap["mysql"]; ok {
					if m, ok := v.(map[interface{}]interface{}); ok {
						mysqlMap = m
					}
				}
			}
		}
	}

	if mysqlMap != nil {
		if v, ok := mysqlMap["host"]; ok {
			cfg.MySQL.Host = fmt.Sprintf("%v", v)
		}
		if v, ok := mysqlMap["port"]; ok {
			cfg.MySQL.Port, _ = strconv.Atoi(fmt.Sprintf("%v", v))
		}
		if v, ok := mysqlMap["database"]; ok {
			cfg.MySQL.Database = fmt.Sprintf("%v", v)
		}
		if v, ok := mysqlMap["username"]; ok {
			cfg.MySQL.Username = fmt.Sprintf("%v", v)
		}
		if v, ok := mysqlMap["password"]; ok {
			cfg.MySQL.Password = fmt.Sprintf("%v", v)
		}
		if v, ok := mysqlMap["charset"]; ok {
			cfg.MySQL.Charset = fmt.Sprintf("%v", v)
		}
	}

	if cfg.MySQL.Charset == "" {
		cfg.MySQL.Charset = "utf8mb4"
	}
	return cfg, nil
}

// ---------------------------------------------------------------------------
// MySQL-to-SQLite escaping conversion
// ---------------------------------------------------------------------------

// convertMySQLToSQLite converts MySQL backslash escaping to SQLite-compatible SQL.
// MySQL uses \' for single quotes, \\ for backslash, etc.
// SQLite uses ” for single quotes and treats \ as literal.
func convertMySQLToSQLite(line string) string {
	var buf strings.Builder
	buf.Grow(len(line))
	i := 0
	for i < len(line) {
		if line[i] == '\\' && i+1 < len(line) {
			switch line[i+1] {
			case '\'':
				buf.WriteString("''")
				i += 2
			case '\\':
				buf.WriteByte('\\')
				i += 2
			case 'n':
				buf.WriteByte('\n')
				i += 2
			case 'r':
				buf.WriteByte('\r')
				i += 2
			case 't':
				buf.WriteByte('\t')
				i += 2
			case '0':
				buf.WriteByte(0)
				i += 2
			case '"':
				buf.WriteByte('"')
				i += 2
			default:
				// Keep other sequences as-is (e.g. \b, \Z)
				buf.WriteByte(line[i])
				buf.WriteByte(line[i+1])
				i += 2
			}
		} else {
			buf.WriteByte(line[i])
			i++
		}
	}
	return buf.String()
}

// ---------------------------------------------------------------------------
// Old system model structs (yu_* tables, column order matches MySQL dump)
// ---------------------------------------------------------------------------

type OldUser struct {
	UserID          int     `gorm:"column:user_id;primaryKey"`
	UUID            int     `gorm:"column:uuid"`
	UserCode        string  `gorm:"column:user_code"`
	FromType        int     `gorm:"column:from_type"`
	Account         string  `gorm:"column:account"`
	FromAccount     string  `gorm:"column:from_account"`
	Realname        string  `gorm:"column:realname"`
	Password        string  `gorm:"column:password"`
	Salt            string  `gorm:"column:salt"`
	Avatar          string  `gorm:"column:avatar"`
	Email           string  `gorm:"column:email"`
	Sex             int     `gorm:"column:sex"`
	Role            int     `gorm:"column:role"`
	Motto           string  `gorm:"column:motto"`
	Remark          string  `gorm:"column:remark"`
	NamePy          string  `gorm:"column:name_py"`
	CsUID           int     `gorm:"column:cs_uid"`
	Setting         string  `gorm:"column:setting"`
	FriendLimit     int     `gorm:"column:friend_limit"`
	GroupLimit      int     `gorm:"column:group_limit"`
	CreateTime      int64   `gorm:"column:create_time"`
	UpdateTime      int64   `gorm:"column:update_time"`
	LoginCount      int     `gorm:"column:login_count"`
	IsAuth          int     `gorm:"column:is_auth"`
	LastLoginTime   int64   `gorm:"column:last_login_time"`
	LastLoginIP     string  `gorm:"column:last_login_ip"`
	RegisterIP      string  `gorm:"column:register_ip"`
	DeleteTime      int64   `gorm:"column:delete_time"`
	TransFrozenTime int64   `gorm:"column:trans_frozen_time"`
	Status          int     `gorm:"column:status"`
	FromID          int     `gorm:"column:from_id"`
	Money           float64 `gorm:"column:money"`
	CanInvite       int     `gorm:"column:can_invite"`
	Trans           string  `gorm:"column:trans"`
	CanFriend       int     `gorm:"column:can_friend"`
	Qiandao         string  `gorm:"column:qiandao"`
	TagID           int     `gorm:"column:tag_id"`
	IsVip           int     `gorm:"column:is_vip"`
	VipDate         string  `gorm:"column:vip_date"`
	ShareID         string  `gorm:"column:shareid"`
	FromID2         string  `gorm:"column:fromid"`
	VerifyStatus    int     `gorm:"column:verify_status"`
	IsTrans         int     `gorm:"column:is_trans"`
	IsClose         int     `gorm:"column:is_close"`
}

func (OldUser) TableName() string { return "yu_user" }

type OldFriend struct {
	FriendID     int    `gorm:"column:friend_id;primaryKey"`
	FriendUserID string `gorm:"column:friend_user_id"`
	Nickname     string `gorm:"column:nickname"`
	IsInvite     int    `gorm:"column:is_invite"`
	IsTop        int    `gorm:"column:is_top"`
	IsNotice     int    `gorm:"column:is_notice"`
	CreateUser   int    `gorm:"column:create_user"`
	UpdateTime   int64  `gorm:"column:update_time"`
	CreateTime   int64  `gorm:"column:create_time"`
	DeleteTime   int64  `gorm:"column:delete_time"`
	Remark       string `gorm:"column:remark"`
	Status       int    `gorm:"column:status"`
	Tags         string `gorm:"column:tags"`
}

func (OldFriend) TableName() string { return "yu_friend" }

type OldGroup struct {
	GroupID      int    `gorm:"column:group_id;primaryKey"`
	Name         string `gorm:"column:name"`
	NamePy       string `gorm:"column:name_py"`
	Avatar       string `gorm:"column:avatar"`
	Level        int    `gorm:"column:level"`
	CreateUser   int    `gorm:"column:create_user"`
	CreateTime   int64  `gorm:"column:create_time"`
	OwnerID      int    `gorm:"column:owner_id"`
	IsPublic     int    `gorm:"column:is_public"`
	Notice       string `gorm:"column:notice"`
	Setting      string `gorm:"column:setting"`
	Status       int    `gorm:"column:status"`
	DeleteTime   int64  `gorm:"column:delete_time"`
	IsPrivate    int    `gorm:"column:is_private"`
	VirtualCount int    `gorm:"column:virtual_count"`
}

func (OldGroup) TableName() string { return "yu_group" }

type OldGroupUser struct {
	ID          int   `gorm:"column:id;primaryKey"`
	GroupID     int   `gorm:"column:group_id"`
	UserID      int   `gorm:"column:user_id"`
	Role        int   `gorm:"column:role"`
	InviteID    int   `gorm:"column:invite_id"`
	CreateTime  int64 `gorm:"column:create_time"`
	Unread      int   `gorm:"column:unread"`
	IsNotice    int   `gorm:"column:is_notice"`
	IsTop       int   `gorm:"column:is_top"`
	NoSpeakTime int64 `gorm:"column:no_speak_time"`
	Status      int   `gorm:"column:status"`
	DeleteTime  int64 `gorm:"column:delete_time"`
}

func (OldGroupUser) TableName() string { return "yu_group_user" }

type OldMessage struct {
	MsgID        int    `gorm:"column:msg_id;primaryKey"`
	ID           string `gorm:"column:id"`
	FromUser     int    `gorm:"column:from_user"`
	ToUser       int    `gorm:"column:to_user"`
	Content      string `gorm:"column:content"`
	ChatIdentify string `gorm:"column:chat_identify"`
	Type         string `gorm:"column:type"`
	IsGroup      int    `gorm:"column:is_group"`
	IsRead       int    `gorm:"column:is_read"`
	IsLast       int    `gorm:"column:is_last"`
	CreateTime   int64  `gorm:"column:create_time"`
	IsUndo       int    `gorm:"column:is_undo"`
	At           string `gorm:"column:at"`
	PID          int    `gorm:"column:pid"`
	FileID       int    `gorm:"column:file_id"`
	FileCate     int    `gorm:"column:file_cate"`
	FileSize     int    `gorm:"column:file_size"`
	FileName     string `gorm:"column:file_name"`
	Extends      string `gorm:"column:extends"`
	Status       int    `gorm:"column:status"`
	DelUser      string `gorm:"column:del_user"`
	VoiceText    string `gorm:"column:voice_text"`
}

func (OldMessage) TableName() string { return "yu_message" }

type OldEmoji struct {
	ID         int    `gorm:"column:id;primaryKey"`
	UserID     int    `gorm:"column:user_id"`
	Type       int    `gorm:"column:type"`
	Name       string `gorm:"column:name"`
	Src        string `gorm:"column:src"`
	FileID     int    `gorm:"column:file_id"`
	CreateTime int64  `gorm:"column:create_time"`
	UpdateTime int64  `gorm:"column:update_time"`
	DeleteTime int64  `gorm:"column:delete_time"`
	Status     int    `gorm:"column:status"`
}

func (OldEmoji) TableName() string { return "yu_emoji" }

// OldUserTag matches yu_user_tag: id, uid, name, createtime, updatetime
type OldUserTag struct {
	ID         int    `gorm:"column:id;primaryKey"`
	UID        int    `gorm:"column:uid"`
	Name       string `gorm:"column:name"`
	CreateTime string `gorm:"column:createtime"`
	UpdateTime string `gorm:"column:updatetime"`
}

func (OldUserTag) TableName() string { return "yu_user_tag" }

// OldUserTagFriend matches yu_user_tag_friend: id, uid, tag_id, friend_id, createtime, updatetime
type OldUserTagFriend struct {
	ID         int    `gorm:"column:id;primaryKey"`
	UID        int    `gorm:"column:uid"`
	TagID      int    `gorm:"column:tag_id"`
	FriendID   int    `gorm:"column:friend_id"`
	CreateTime string `gorm:"column:createtime"`
	UpdateTime string `gorm:"column:updatetime"`
}

func (OldUserTagFriend) TableName() string { return "yu_user_tag_friend" }

// OldHongbao matches yu_hongbao: 15 columns
type OldHongbao struct {
	ID           int     `gorm:"column:id;primaryKey"`
	ChatIdentify string  `gorm:"column:chat_identify"`
	MsgID        int     `gorm:"column:msg_id"`
	ToUser       string  `gorm:"column:to_user"`
	Msg          string  `gorm:"column:msg"`
	UserID       int     `gorm:"column:user_id"`
	SingleMoney  float64 `gorm:"column:single_money"`
	Money        float64 `gorm:"column:money"`
	Number       int     `gorm:"column:number"`
	SyMoney      float64 `gorm:"column:sy_money"`
	SyNumber     int     `gorm:"column:sy_number"`
	CreateTime   int64   `gorm:"column:create_time"`
	GenerateDesc string  `gorm:"column:generate_desc"`
	IsGroup      int     `gorm:"column:is_group"`
	IsBack       int     `gorm:"column:is_back"`
}

func (OldHongbao) TableName() string { return "yu_hongbao" }

// OldTransfer matches yu_transfer: 8 columns
type OldTransfer struct {
	ID           int     `gorm:"column:id;primaryKey"`
	ChatIdentify string  `gorm:"column:chat_identify"`
	MsgID        int     `gorm:"column:msg_id"`
	ToUser       int     `gorm:"column:to_user"`
	Remark       string  `gorm:"column:remark"`
	UserID       int     `gorm:"column:user_id"`
	Amount       float64 `gorm:"column:amount"`
	CreateTime   int64   `gorm:"column:create_time"`
}

func (OldTransfer) TableName() string { return "yu_transfer" }

// ---------------------------------------------------------------------------
// New system model structs (for writing to MySQL)
// ---------------------------------------------------------------------------

type NewUser struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement"`
	Mobile    *string   `gorm:"column:mobile;type:varchar(32)"`
	Nickname  string    `gorm:"column:nickname;type:varchar(64)"`
	Avatar    string    `gorm:"column:avatar;type:varchar(255)"`
	Gender    int       `gorm:"column:gender"`
	Password  string    `gorm:"column:password;type:varchar(255)"`
	Salt      string    `gorm:"column:salt;type:varchar(16)"`
	Motto     string    `gorm:"column:motto;type:varchar(255)"`
	Email     string    `gorm:"column:email;type:varchar(128)"`
	Birthday  string    `gorm:"column:birthday;type:varchar(32)"`
	IsRobot   int       `gorm:"column:is_robot"`
	Status    int       `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (NewUser) TableName() string { return "users" }

type NewContact struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement"`
	UserId    int       `gorm:"column:user_id"`
	FriendId  int       `gorm:"column:friend_id"`
	Remark    string    `gorm:"column:remark;type:varchar(64)"`
	Status    int       `gorm:"column:status"`
	GroupId   int       `gorm:"column:group_id"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (NewContact) TableName() string { return "contact" }

type NewGroup struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement"`
	Type      int       `gorm:"column:type"`
	CreatorId int       `gorm:"column:creator_id"`
	Name      string    `gorm:"column:name;type:varchar(64)"`
	Profile   string    `gorm:"column:profile;type:varchar(512)"`
	IsDismiss int       `gorm:"column:is_dismiss"`
	Avatar    string    `gorm:"column:avatar;type:varchar(255)"`
	MaxNum    int       `gorm:"column:max_num"`
	IsOvert   int       `gorm:"column:is_overt"`
	IsMute    int       `gorm:"column:is_mute"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (NewGroup) TableName() string { return "group" }

type NewGroupMember struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement"`
	GroupId   int       `gorm:"column:group_id"`
	UserId    int       `gorm:"column:user_id"`
	Leader    int       `gorm:"column:leader"`
	UserCard  string    `gorm:"column:user_card;type:varchar(64)"`
	IsQuit    int       `gorm:"column:is_quit"`
	IsMute    int       `gorm:"column:is_mute"`
	JoinTime  time.Time `gorm:"column:join_time"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (NewGroupMember) TableName() string { return "group_member" }

type NewTalkSession struct {
	Id         int       `gorm:"column:id;primaryKey;autoIncrement"`
	SessionId  int       `gorm:"column:session_id"`
	TalkMode   int       `gorm:"column:talk_mode"`
	UserId     int       `gorm:"column:user_id"`
	ReceiverId int       `gorm:"column:receiver_id"`
	IsTop      int       `gorm:"column:is_top"`
	IsDisturb  int       `gorm:"column:is_disturb"`
	IsDelete   int       `gorm:"column:is_delete"`
	IsRobot    int       `gorm:"column:is_robot"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (NewTalkSession) TableName() string { return "talk_session" }

type NewTalkUserMessage struct {
	Id         int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MsgId      string    `gorm:"column:msg_id;type:varchar(64)"`
	OrgMsgId   string    `gorm:"column:org_msg_id;type:varchar(64)"`
	SessionId  int       `gorm:"column:session_id"`
	MsgType    int       `gorm:"column:msg_type"`
	UserId     int       `gorm:"column:user_id"`
	ReceiverId int       `gorm:"column:receiver_id"`
	FromId     int       `gorm:"column:from_id"`
	IsRevoked  int       `gorm:"column:is_revoked"`
	IsDeleted  int       `gorm:"column:is_deleted"`
	Extra      string    `gorm:"column:extra;type:text"`
	Quote      string    `gorm:"column:quote;type:text"`
	SendTime   time.Time `gorm:"column:send_time"`
	CreatedAt  time.Time `gorm:"column:created_at"`
}

func (NewTalkUserMessage) TableName() string { return "talk_user_message" }

type NewTalkGroupMessage struct {
	Id        int64     `gorm:"column:id;primaryKey;autoIncrement"`
	MsgId     string    `gorm:"column:msg_id;type:varchar(64)"`
	MsgType   int       `gorm:"column:msg_type"`
	GroupId   int       `gorm:"column:group_id"`
	FromId    int       `gorm:"column:from_id"`
	IsRevoked int       `gorm:"column:is_revoked"`
	Extra     string    `gorm:"column:extra;type:text"`
	Quote     string    `gorm:"column:quote;type:text"`
	SendTime  time.Time `gorm:"column:send_time"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (NewTalkGroupMessage) TableName() string { return "talk_group_message" }

type NewContactGroup struct {
	Id        int       `gorm:"column:id;primaryKey;autoIncrement"`
	UserId    int       `gorm:"column:user_id"`
	Name      string    `gorm:"column:name;type:varchar(64)"`
	Num       int       `gorm:"column:num"`
	Sort      int       `gorm:"column:sort"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (NewContactGroup) TableName() string { return "contact_group" }

type NewEmoticonItem struct {
	Id         int       `gorm:"column:id;primaryKey;autoIncrement"`
	EmoticonId int       `gorm:"column:emoticon_id"`
	UserId     int       `gorm:"column:user_id"`
	Describe   string    `gorm:"column:describe;type:varchar(64)"`
	Url        string    `gorm:"column:url;type:varchar(255)"`
	CreatedAt  time.Time `gorm:"column:created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at"`
}

func (NewEmoticonItem) TableName() string { return "emoticon_item" }

// ---------------------------------------------------------------------------
// SQLite CREATE TABLE for old tables (column order matches dump INSERT order)
// ALL schemas now match the actual MySQL dump exactly
// ---------------------------------------------------------------------------

var sqliteSchemas = []string{
	// yu_user: 43 columns
	`CREATE TABLE IF NOT EXISTS yu_user (
		user_id INTEGER PRIMARY KEY, uuid INTEGER DEFAULT 0, user_code TEXT DEFAULT '',
		from_type INTEGER DEFAULT 0, account TEXT DEFAULT '', from_account TEXT DEFAULT '',
		realname TEXT DEFAULT '', password TEXT DEFAULT '', salt TEXT DEFAULT '',
		avatar TEXT DEFAULT '', email TEXT DEFAULT '', sex INTEGER DEFAULT 2,
		role INTEGER DEFAULT 0, motto TEXT DEFAULT '', remark TEXT DEFAULT '',
		name_py TEXT DEFAULT '', cs_uid INTEGER DEFAULT 0, setting TEXT DEFAULT '',
		friend_limit INTEGER DEFAULT 0, group_limit INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, update_time INTEGER DEFAULT 0,
		login_count INTEGER DEFAULT 0, is_auth INTEGER DEFAULT 0, last_login_time INTEGER DEFAULT 0,
		last_login_ip TEXT DEFAULT '', register_ip TEXT DEFAULT '', delete_time INTEGER DEFAULT 0,
		trans_frozen_time INTEGER DEFAULT 0, status INTEGER DEFAULT 1, from_id INTEGER DEFAULT 0,
		money REAL DEFAULT 0, can_invite INTEGER DEFAULT 1, trans TEXT DEFAULT '',
		can_friend INTEGER DEFAULT 1, qiandao TEXT DEFAULT '', tag_id INTEGER DEFAULT 0,
		is_vip INTEGER DEFAULT 0, vip_date TEXT DEFAULT '', shareid TEXT DEFAULT '',
		fromid TEXT DEFAULT '', verify_status INTEGER DEFAULT 0,
		is_trans INTEGER DEFAULT 0, is_close INTEGER DEFAULT 0
	)`,
	// yu_friend: 13 columns
	`CREATE TABLE IF NOT EXISTS yu_friend (
		friend_id INTEGER PRIMARY KEY, friend_user_id TEXT DEFAULT '',
		nickname TEXT DEFAULT '', is_invite INTEGER DEFAULT 0,
		is_top INTEGER DEFAULT 0, is_notice INTEGER DEFAULT 1,
		create_user INTEGER DEFAULT 0, update_time INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, delete_time INTEGER DEFAULT 0,
		remark TEXT DEFAULT '', status INTEGER DEFAULT 1, tags TEXT DEFAULT ''
	)`,
	// yu_group: 15 columns
	`CREATE TABLE IF NOT EXISTS yu_group (
		group_id INTEGER PRIMARY KEY, name TEXT DEFAULT '', name_py TEXT DEFAULT '',
		avatar TEXT DEFAULT '', level INTEGER DEFAULT 0, create_user INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, owner_id INTEGER DEFAULT 0,
		is_public INTEGER DEFAULT 0, notice TEXT DEFAULT '', setting TEXT DEFAULT '',
		status INTEGER DEFAULT 1, delete_time INTEGER DEFAULT 0,
		is_private INTEGER DEFAULT 0, virtual_count INTEGER DEFAULT 0
	)`,
	// yu_group_user: 12 columns
	`CREATE TABLE IF NOT EXISTS yu_group_user (
		id INTEGER PRIMARY KEY, group_id INTEGER DEFAULT 0,
		user_id INTEGER DEFAULT 0, role INTEGER DEFAULT 3,
		invite_id INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		unread INTEGER DEFAULT 0, is_notice INTEGER DEFAULT 1,
		is_top INTEGER DEFAULT 0, no_speak_time INTEGER DEFAULT 0,
		status INTEGER DEFAULT 1, delete_time INTEGER DEFAULT 0
	)`,
	// yu_message: 22 columns (id can be NULL/empty)
	`CREATE TABLE IF NOT EXISTS yu_message (
		msg_id INTEGER PRIMARY KEY, id TEXT DEFAULT '',
		from_user INTEGER DEFAULT 0, to_user INTEGER DEFAULT 0,
		content TEXT DEFAULT '', chat_identify TEXT DEFAULT '',
		type TEXT DEFAULT 'text', is_group INTEGER DEFAULT 0,
		is_read INTEGER DEFAULT 0, is_last INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, is_undo INTEGER DEFAULT 0,
		at TEXT DEFAULT '', pid INTEGER DEFAULT 0,
		file_id INTEGER DEFAULT 0, file_cate INTEGER DEFAULT 0,
		file_size INTEGER DEFAULT 0, file_name TEXT DEFAULT '',
		extends TEXT DEFAULT '', status INTEGER DEFAULT 1,
		del_user TEXT DEFAULT '', voice_text TEXT DEFAULT ''
	)`,
	// yu_emoji: 10 columns
	`CREATE TABLE IF NOT EXISTS yu_emoji (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		type INTEGER DEFAULT 0, name TEXT DEFAULT '', src TEXT DEFAULT '',
		file_id INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		update_time INTEGER DEFAULT 0, delete_time INTEGER DEFAULT 0, status INTEGER DEFAULT 1
	)`,
	// yu_user_tag: 5 columns (id, uid, name, createtime, updatetime)
	`CREATE TABLE IF NOT EXISTS yu_user_tag (
		id INTEGER PRIMARY KEY, uid INTEGER DEFAULT 0,
		name TEXT DEFAULT '',
		createtime TEXT DEFAULT '', updatetime TEXT DEFAULT ''
	)`,
	// yu_user_tag_friend: 6 columns (id, uid, tag_id, friend_id, createtime, updatetime)
	`CREATE TABLE IF NOT EXISTS yu_user_tag_friend (
		id INTEGER PRIMARY KEY, uid INTEGER DEFAULT 0,
		tag_id INTEGER DEFAULT 0, friend_id INTEGER DEFAULT 0,
		createtime TEXT DEFAULT '', updatetime TEXT DEFAULT ''
	)`,
	// yu_capital_log: 11 columns
	`CREATE TABLE IF NOT EXISTS yu_capital_log (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		money REAL DEFAULT 0, user_money REAL DEFAULT 0,
		is_group INTEGER DEFAULT 0, to_user INTEGER DEFAULT 0,
		chat_identify TEXT DEFAULT '', create_time INTEGER DEFAULT 0,
		type INTEGER DEFAULT 0, operator INTEGER DEFAULT 0, remark TEXT DEFAULT ''
	)`,
	// yu_config: 8 columns (id, name, value, create_user, update_time, create_time, remark, status)
	`CREATE TABLE IF NOT EXISTS yu_config (
		id INTEGER PRIMARY KEY, name TEXT DEFAULT '', value TEXT DEFAULT '',
		create_user INTEGER DEFAULT 0, update_time INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, remark TEXT DEFAULT '', status INTEGER DEFAULT 1
	)`,
	// yu_file: 15 columns
	`CREATE TABLE IF NOT EXISTS yu_file (
		file_id INTEGER PRIMARY KEY, cate INTEGER DEFAULT 0, file_type INTEGER DEFAULT 0,
		parent_id INTEGER DEFAULT 0, name TEXT DEFAULT '', src TEXT DEFAULT '',
		size INTEGER DEFAULT 0, ext TEXT DEFAULT '', md5 TEXT DEFAULT '',
		user_id INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		update_time INTEGER DEFAULT 0, delete_time INTEGER DEFAULT 0,
		status INTEGER DEFAULT 1, is_lock INTEGER DEFAULT 0
	)`,
	// yu_hongbao: 15 columns
	`CREATE TABLE IF NOT EXISTS yu_hongbao (
		id INTEGER PRIMARY KEY, chat_identify TEXT DEFAULT '',
		msg_id INTEGER DEFAULT 0, to_user TEXT DEFAULT '',
		msg TEXT DEFAULT '', user_id INTEGER DEFAULT 0,
		single_money REAL DEFAULT 0, money REAL DEFAULT 0,
		number INTEGER DEFAULT 0, sy_money REAL DEFAULT 0,
		sy_number INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		generate_desc TEXT DEFAULT '', is_group INTEGER DEFAULT 0,
		is_back INTEGER DEFAULT 0
	)`,
	// yu_hongbao_detail: 5 columns
	`CREATE TABLE IF NOT EXISTS yu_hongbao_detail (
		id INTEGER PRIMARY KEY, hongbao_id INTEGER DEFAULT 0,
		user_id INTEGER DEFAULT 0, money REAL DEFAULT 0, create_time INTEGER DEFAULT 0
	)`,
	// yu_label: 5 columns (id, uid, name, createtime, updatetime)
	`CREATE TABLE IF NOT EXISTS yu_label (
		id INTEGER PRIMARY KEY, uid INTEGER DEFAULT 0,
		name TEXT DEFAULT '', createtime INTEGER DEFAULT 0,
		updatetime INTEGER DEFAULT 0
	)`,
	// yu_order: 24 columns
	`CREATE TABLE IF NOT EXISTS yu_order (
		id INTEGER PRIMARY KEY, orderid TEXT DEFAULT '',
		buyer_id INTEGER DEFAULT 0, saler_id INTEGER DEFAULT 0,
		amount REAL DEFAULT 0, task_id INTEGER DEFAULT 0,
		counts REAL DEFAULT 0, pay_type INTEGER DEFAULT 0,
		buy_type INTEGER DEFAULT 0, status INTEGER DEFAULT 0,
		pay_img TEXT DEFAULT '', is_cancel INTEGER DEFAULT 0,
		is_appeal INTEGER DEFAULT 0, appeal_id INTEGER DEFAULT 0,
		appeal_time INTEGER DEFAULT 0, appeal_reason TEXT DEFAULT '',
		cancel_id INTEGER DEFAULT 0, remark TEXT DEFAULT '',
		create_time INTEGER DEFAULT 0, pay_time INTEGER DEFAULT 0,
		cancel_time INTEGER DEFAULT 0, wronger INTEGER DEFAULT 0,
		judge TEXT DEFAULT '', judge_time INTEGER DEFAULT 0
	)`,
	// yu_task: 14 columns
	`CREATE TABLE IF NOT EXISTS yu_task (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		currency_type INTEGER DEFAULT 1, price REAL DEFAULT 0,
		count REAL DEFAULT 0, paytype TEXT DEFAULT '',
		account TEXT DEFAULT '', nickname TEXT DEFAULT '',
		status INTEGER DEFAULT 0, is_up INTEGER DEFAULT 1,
		up_time INTEGER DEFAULT 0, is_deleted INTEGER DEFAULT 0,
		create_time INTEGER DEFAULT 0, delete_time INTEGER DEFAULT 0
	)`,
	// yu_tixian: 9 columns (id, user_id, status, create_time, img, money, account, type, remark)
	`CREATE TABLE IF NOT EXISTS yu_tixian (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		status INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		img TEXT DEFAULT '', money REAL DEFAULT 0,
		account TEXT DEFAULT '', type TEXT DEFAULT '',
		remark TEXT DEFAULT ''
	)`,
	// yu_topup: 7 columns
	`CREATE TABLE IF NOT EXISTS yu_topup (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		money REAL DEFAULT 0, order_id TEXT DEFAULT '',
		create_time INTEGER DEFAULT 0, update_time INTEGER DEFAULT 0, status INTEGER DEFAULT 0
	)`,
	// yu_transfer: 8 columns (id, chat_identify, msg_id, to_user, remark, user_id, amount, create_time)
	`CREATE TABLE IF NOT EXISTS yu_transfer (
		id INTEGER PRIMARY KEY, chat_identify TEXT DEFAULT '',
		msg_id INTEGER DEFAULT 0, to_user INTEGER DEFAULT 0,
		remark TEXT DEFAULT '', user_id INTEGER DEFAULT 0,
		amount REAL DEFAULT 0, create_time INTEGER DEFAULT 0
	)`,
	// yu_user_merchant: 19 columns
	`CREATE TABLE IF NOT EXISTS yu_user_merchant (
		id INTEGER PRIMARY KEY, user_id INTEGER DEFAULT 0,
		nickname TEXT DEFAULT '', realname TEXT DEFAULT '',
		nation TEXT DEFAULT '', id_type INTEGER DEFAULT 0,
		idcard TEXT DEFAULT '', image TEXT DEFAULT '',
		backimage TEXT DEFAULT '', surety REAL DEFAULT 0,
		status INTEGER DEFAULT 0, create_time INTEGER DEFAULT 0,
		update_time INTEGER DEFAULT 0, reason TEXT DEFAULT '',
		is_limit INTEGER DEFAULT 0, limit_time INTEGER DEFAULT 0,
		is_frozen INTEGER DEFAULT 0, frozen_time INTEGER DEFAULT 0,
		is_close INTEGER DEFAULT 0
	)`,
	// yu_user_merchant_paytype: 7 columns
	`CREATE TABLE IF NOT EXISTS yu_user_merchant_paytype (
		id INTEGER PRIMARY KEY, merchant_id INTEGER DEFAULT 0,
		type TEXT DEFAULT '', config TEXT DEFAULT '',
		create_time INTEGER DEFAULT 0, update_time INTEGER DEFAULT 0, status INTEGER DEFAULT 0
	)`,
}

// ---------------------------------------------------------------------------
// SQL dump import
// ---------------------------------------------------------------------------

func importDumpToSQLite(dumpPath string, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("get underlying db: %w", err)
	}

	log.Println("[SQLite] Creating old system tables...")
	for _, schema := range sqliteSchemas {
		if _, err := sqlDB.Exec(schema); err != nil {
			log.Printf("[SQLite] Warning: create table failed: %v\n  SQL: %.200s...", err, schema)
		}
	}

	log.Printf("[SQLite] Reading dump file: %s", dumpPath)
	file, err := os.Open(dumpPath)
	if err != nil {
		return fmt.Errorf("open dump file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0), 256*1024*1024)

	lineNum, insertCount, failCount := 0, 0, 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if !strings.HasPrefix(line, "INSERT INTO") {
			continue
		}
		tableName := extractTableName(line)

		// Convert MySQL backslash escaping to SQLite-compatible escaping
		converted := convertMySQLToSQLite(line)

		if _, err := sqlDB.Exec(converted); err != nil {
			failCount++
			log.Printf("[SQLite] Warning: insert %s (line %d): %v", tableName, lineNum, err)
			continue
		}
		insertCount++
		log.Printf("[SQLite] Imported: %s", tableName)
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner: %w", err)
	}

	log.Printf("[SQLite] Done: %d INSERT statements succeeded, %d failed, from %d lines", insertCount, failCount, lineNum)
	return nil
}

func extractTableName(sql string) string {
	s := strings.Index(sql, "`")
	if s < 0 {
		return "unknown"
	}
	e := strings.Index(sql[s+1:], "`")
	if e < 0 {
		return "unknown"
	}
	return sql[s+1 : s+1+e]
}

// ---------------------------------------------------------------------------
// Message type mapping (old string -> new int)
// ---------------------------------------------------------------------------

func mapMsgType(oldType string) int {
	switch strings.ToLower(strings.TrimSpace(oldType)) {
	case "text", "markdown":
		return 1
	case "image":
		return 3
	case "voice":
		return 4
	case "video":
		return 5
	case "file":
		return 6
	case "location":
		return 7
	case "card":
		return 8
	case "forward":
		return 9
	case "redpacket", "hongbao", "hongbao_receive":
		return 15
	case "transfer":
		return 16
	case "notice", "system":
		return 13
	case "trtc", "trtcinvite", "trtcrefused", "trtcaccept", "trtcend":
		return 14
	default:
		return 1
	}
}

// ---------------------------------------------------------------------------
// Extra field builder
// ---------------------------------------------------------------------------

func buildExtra(msg OldMessage, hongbaoMap map[int]*OldHongbao, transferMap map[int]*OldTransfer) string {
	msgType := mapMsgType(msg.Type)
	var extra map[string]interface{}

	switch msgType {
	case 1:
		extra = map[string]interface{}{"content": msg.Content}
		if msg.At != "" && msg.At != "[]" && msg.At != "null" {
			var mentions []int
			if err := json.Unmarshal([]byte(msg.At), &mentions); err == nil && len(mentions) > 0 {
				extra["mentions"] = mentions
			}
		}
	case 3:
		extra = map[string]interface{}{"name": msg.FileName, "size": msg.FileSize, "url": msg.Content, "width": 0, "height": 0}
		mergeExtends(msg.Extends, extra, "url", "width", "height")
	case 4:
		extra = map[string]interface{}{"name": msg.FileName, "size": msg.FileSize, "url": msg.Content, "duration": 0}
		mergeExtends(msg.Extends, extra, "url", "duration")
	case 5:
		extra = map[string]interface{}{"name": msg.FileName, "cover": "", "size": msg.FileSize, "url": msg.Content, "duration": 0}
		mergeExtends(msg.Extends, extra, "url", "cover", "duration")
	case 6:
		extra = map[string]interface{}{"name": msg.FileName, "size": msg.FileSize, "path": "", "url": msg.Content}
		mergeExtends(msg.Extends, extra, "url", "path")
	case 7:
		extra = map[string]interface{}{"longitude": "", "latitude": "", "description": msg.Content}
		mergeExtends(msg.Extends, extra, "longitude", "latitude", "description")
	case 8:
		extra = map[string]interface{}{"user_id": 0, "nickname": "", "avatar": "", "describe": msg.Content}
		mergeExtends(msg.Extends, extra, "user_id", "nickname", "avatar")
	case 14:
		extra = map[string]interface{}{"type": 1, "status": 1, "duration": 0}
		mergeExtends(msg.Extends, extra, "type", "status", "duration")
		switch strings.ToLower(msg.Type) {
		case "trtcinvite":
			extra["status"] = 1
		case "trtcrefused":
			extra["status"] = 3
		case "trtcaccept", "trtcend":
			extra["status"] = 4
		}
	case 15:
		extra = map[string]interface{}{"envelope_id": "", "amount": 0, "count": 0, "type": "normal", "greeting": msg.Content}
		// Enrich from hongbao table
		if hb, ok := hongbaoMap[msg.MsgID]; ok {
			extra["envelope_id"] = fmt.Sprintf("hb_%d", hb.ID)
			extra["amount"] = hb.Money
			extra["count"] = hb.Number
			extra["greeting"] = hb.Msg
		}
		mergeExtends(msg.Extends, extra)
	case 16:
		extra = map[string]interface{}{"transfer_id": "", "amount": 0, "remark": msg.Content}
		// Enrich from transfer table
		if tr, ok := transferMap[msg.MsgID]; ok {
			extra["transfer_id"] = fmt.Sprintf("tr_%d", tr.ID)
			extra["amount"] = tr.Amount
			extra["remark"] = tr.Remark
		}
		mergeExtends(msg.Extends, extra)
	default:
		extra = map[string]interface{}{"content": msg.Content}
	}

	b, _ := json.Marshal(extra)
	return string(b)
}

func mergeExtends(extends string, dst map[string]interface{}, keys ...string) {
	if extends == "" || extends == "null" {
		return
	}
	var ext map[string]interface{}
	if err := json.Unmarshal([]byte(extends), &ext); err != nil {
		return
	}
	if len(keys) == 0 {
		for k, v := range ext {
			dst[k] = v
		}
		return
	}
	for _, k := range keys {
		if v, ok := ext[k]; ok {
			dst[k] = v
		}
	}
}

// ---------------------------------------------------------------------------
// Migration: Users
// ---------------------------------------------------------------------------

func migrateUsers(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting user migration...")
	var oldUsers []OldUser
	if err := sqliteDB.Find(&oldUsers).Error; err != nil {
		return 0, fmt.Errorf("read old users: %w", err)
	}
	log.Printf("[Migrate] Found %d old users", len(oldUsers))

	count := 0
	for _, old := range oldUsers {
		gender := 3
		switch old.Sex {
		case 0:
			gender = 2
		case 1:
			gender = 1
		}

		status := 1
		if old.Status != 1 {
			status = 2
		}

		mobile := old.Account
		if mobile == "" {
			mobile = fmt.Sprintf("user_%d", old.UserID)
		}

		createdAt := time.Unix(old.CreateTime, 0)
		updatedAt := createdAt
		if old.UpdateTime > 0 {
			updatedAt = time.Unix(old.UpdateTime, 0)
		}

		nickname := old.Realname
		if nickname == "" {
			nickname = mobile
		}

		u := NewUser{
			Id: old.UserID, Mobile: &mobile, Nickname: nickname,
			Avatar: old.Avatar, Gender: gender, Password: old.Password,
			Salt: old.Salt, Motto: old.Motto, Email: old.Email,
			Birthday: "", IsRobot: 1, Status: status,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		if err := mysqlDB.Create(&u).Error; err != nil {
			log.Printf("[Migrate] Warning: user %d: %v", old.UserID, err)
			continue
		}
		count++
	}
	log.Printf("[Migrate] Migrated %d/%d users", count, len(oldUsers))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Contacts
// ---------------------------------------------------------------------------

func migrateContacts(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting contact migration...")
	var oldFriends []OldFriend
	if err := sqliteDB.Find(&oldFriends).Error; err != nil {
		return 0, fmt.Errorf("read old friends: %w", err)
	}
	log.Printf("[Migrate] Found %d old friend records", len(oldFriends))

	count := 0
	for _, old := range oldFriends {
		friendId, err := strconv.Atoi(old.FriendUserID)
		if err != nil {
			log.Printf("[Migrate] Warning: invalid friend_user_id '%s', skipping", old.FriendUserID)
			continue
		}

		status := 1
		if old.Status != 1 || old.DeleteTime > 0 {
			status = 2
		}

		createdAt := time.Unix(old.CreateTime, 0)
		updatedAt := createdAt
		if old.UpdateTime > 0 {
			updatedAt = time.Unix(old.UpdateTime, 0)
		}

		c := NewContact{
			UserId: old.CreateUser, FriendId: friendId, Remark: old.Nickname,
			Status: status, GroupId: 0, CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		if err := mysqlDB.Create(&c).Error; err != nil {
			log.Printf("[Migrate] Warning: contact (user:%d friend:%d): %v", old.CreateUser, friendId, err)
			continue
		}
		count++
	}
	log.Printf("[Migrate] Migrated %d/%d contacts", count, len(oldFriends))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Groups
// ---------------------------------------------------------------------------

func migrateGroups(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting group migration...")
	var oldGroups []OldGroup
	if err := sqliteDB.Find(&oldGroups).Error; err != nil {
		return 0, fmt.Errorf("read old groups: %w", err)
	}
	log.Printf("[Migrate] Found %d old groups", len(oldGroups))

	count := 0
	for _, old := range oldGroups {
		isDismiss := 1
		if old.Status == 0 || old.DeleteTime > 0 {
			isDismiss = 2
		}

		isOvert := 1
		if old.IsPublic == 1 {
			isOvert = 2
		}

		isMute := 1
		if old.Setting != "" && old.Setting != "null" {
			var s map[string]interface{}
			if err := json.Unmarshal([]byte(old.Setting), &s); err == nil {
				if fmt.Sprintf("%v", s["nospeak"]) == "1" {
					isMute = 2
				}
			}
		}

		profile := old.Notice
		if len(profile) > 512 {
			profile = profile[:512]
		}

		createdAt := time.Unix(old.CreateTime, 0)
		g := NewGroup{
			Id: old.GroupID, Type: 1, CreatorId: old.CreateUser,
			Name: old.Name, Profile: profile, IsDismiss: isDismiss,
			Avatar: old.Avatar, MaxNum: 500, IsOvert: isOvert,
			IsMute: isMute, CreatedAt: createdAt, UpdatedAt: createdAt,
		}
		if err := mysqlDB.Create(&g).Error; err != nil {
			log.Printf("[Migrate] Warning: group %d: %v", old.GroupID, err)
			continue
		}
		count++
	}
	log.Printf("[Migrate] Migrated %d/%d groups", count, len(oldGroups))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Group Members
// ---------------------------------------------------------------------------

func migrateGroupMembers(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting group member migration...")
	var oldMembers []OldGroupUser
	if err := sqliteDB.Find(&oldMembers).Error; err != nil {
		return 0, fmt.Errorf("read old group users: %w", err)
	}
	log.Printf("[Migrate] Found %d old group member records", len(oldMembers))

	count := 0
	for _, old := range oldMembers {
		leader := old.Role
		if leader < 1 || leader > 3 {
			leader = 3
		}

		isQuit := 1
		if old.Status == 0 || old.DeleteTime > 0 {
			isQuit = 2
		}

		isMute := 1
		if old.NoSpeakTime > 0 && old.NoSpeakTime > time.Now().Unix() {
			isMute = 2
		}

		joinTime := time.Unix(old.CreateTime, 0)
		m := NewGroupMember{
			GroupId: old.GroupID, UserId: old.UserID, Leader: leader,
			UserCard: "", IsQuit: isQuit, IsMute: isMute,
			JoinTime: joinTime, CreatedAt: joinTime, UpdatedAt: joinTime,
		}
		if err := mysqlDB.Create(&m).Error; err != nil {
			log.Printf("[Migrate] Warning: member (group:%d user:%d): %v", old.GroupID, old.UserID, err)
			continue
		}
		count++
	}
	log.Printf("[Migrate] Migrated %d/%d group members", count, len(oldMembers))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Messages (+ TalkSession creation)
// ---------------------------------------------------------------------------

func migrateMessages(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting message migration...")

	// Pre-load hongbao and transfer data for enriching Extra fields
	hongbaoMap := make(map[int]*OldHongbao)
	var hongbaos []OldHongbao
	if err := sqliteDB.Find(&hongbaos).Error; err == nil {
		for i := range hongbaos {
			hongbaoMap[hongbaos[i].MsgID] = &hongbaos[i]
		}
		log.Printf("[Migrate] Loaded %d hongbao records", len(hongbaos))
	}

	transferMap := make(map[int]*OldTransfer)
	var transfers []OldTransfer
	if err := sqliteDB.Find(&transfers).Error; err == nil {
		for i := range transfers {
			transferMap[transfers[i].MsgID] = &transfers[i]
		}
		log.Printf("[Migrate] Loaded %d transfer records", len(transfers))
	}

	var oldMessages []OldMessage
	if err := sqliteDB.Order("msg_id ASC").Find(&oldMessages).Error; err != nil {
		return 0, fmt.Errorf("read old messages: %w", err)
	}
	log.Printf("[Migrate] Found %d old messages", len(oldMessages))

	sessionCache := make(map[string]int) // "mode:uid:rid" -> talk_session.id
	sessionPairCounter := 0

	getOrCreateSession := func(talkMode, userId, receiverId int, ts time.Time) int {
		key := fmt.Sprintf("%d:%d:%d", talkMode, userId, receiverId)
		if sid, ok := sessionCache[key]; ok {
			return sid
		}
		sessionPairCounter++
		sess := NewTalkSession{
			SessionId: sessionPairCounter, TalkMode: talkMode,
			UserId: userId, ReceiverId: receiverId,
			IsTop: 1, IsDisturb: 1, IsDelete: 1, IsRobot: 1,
			CreatedAt: ts, UpdatedAt: ts,
		}
		if err := mysqlDB.Create(&sess).Error; err != nil {
			log.Printf("[Migrate] Warning: session (mode:%d u:%d r:%d): %v", talkMode, userId, receiverId, err)
			return 0
		}
		sessionCache[key] = sess.Id
		return sess.Id
	}

	count := 0
	for _, msg := range oldMessages {
		if msg.Status == 0 {
			continue
		}

		msgType := mapMsgType(msg.Type)
		extra := buildExtra(msg, hongbaoMap, transferMap)
		sendTime := time.Unix(msg.CreateTime, 0)

		isRevoked := 1
		if msg.IsUndo == 1 {
			isRevoked = 2
		}

		msgId := msg.ID
		if msgId == "" {
			msgId = fmt.Sprintf("migrated_%d", msg.MsgID)
		}

		if msg.IsGroup == 0 {
			// Private message: create records for both parties
			for _, ownerID := range []int{msg.FromUser, msg.ToUser} {
				receiverID := msg.ToUser
				if ownerID == msg.ToUser {
					receiverID = msg.FromUser
				}
				sessionId := getOrCreateSession(1, ownerID, receiverID, sendTime)

				isDeleted := 1
				if msg.DelUser != "" && msg.DelUser != "[]" && msg.DelUser != "null" {
					var delUsers []int
					if json.Unmarshal([]byte(msg.DelUser), &delUsers) == nil {
						for _, uid := range delUsers {
							if uid == ownerID {
								isDeleted = 2
								break
							}
						}
					}
				}

				m := NewTalkUserMessage{
					MsgId: msgId, OrgMsgId: "", SessionId: sessionId,
					MsgType: msgType, UserId: ownerID, ReceiverId: receiverID,
					FromId: msg.FromUser, IsRevoked: isRevoked, IsDeleted: isDeleted,
					Extra: extra, Quote: "", SendTime: sendTime, CreatedAt: sendTime,
				}
				if err := mysqlDB.Create(&m).Error; err != nil {
					log.Printf("[Migrate] Warning: user msg (id:%s owner:%d): %v", msgId, ownerID, err)
				} else {
					count++
				}
			}
		} else {
			// Group message
			_ = getOrCreateSession(2, msg.FromUser, msg.ToUser, sendTime)
			m := NewTalkGroupMessage{
				MsgId: msgId, MsgType: msgType, GroupId: msg.ToUser,
				FromId: msg.FromUser, IsRevoked: isRevoked,
				Extra: extra, Quote: "", SendTime: sendTime, CreatedAt: sendTime,
			}
			if err := mysqlDB.Create(&m).Error; err != nil {
				log.Printf("[Migrate] Warning: group msg (id:%s group:%d): %v", msgId, msg.ToUser, err)
			} else {
				count++
			}
		}
	}

	// Create sessions for all active group members
	log.Println("[Migrate] Creating group sessions for members...")
	var gms []OldGroupUser
	if err := sqliteDB.Where("status = 1 AND delete_time = 0").Find(&gms).Error; err == nil {
		for _, gm := range gms {
			getOrCreateSession(2, gm.UserID, gm.GroupID, time.Unix(gm.CreateTime, 0))
		}
	}

	log.Printf("[Migrate] Migrated %d message records, created %d sessions", count, len(sessionCache))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Contact Groups (tags)
// ---------------------------------------------------------------------------

func migrateContactGroups(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting contact group migration...")
	var oldTags []OldUserTag
	if err := sqliteDB.Find(&oldTags).Error; err != nil {
		return 0, fmt.Errorf("read old tags: %w", err)
	}
	log.Printf("[Migrate] Found %d old user tags", len(oldTags))

	tagMap := make(map[int]int) // old tag_id -> new contact_group.id
	count := 0
	for _, old := range oldTags {
		var friendCount int64
		sqliteDB.Model(&OldUserTagFriend{}).Where("tag_id = ?", old.ID).Count(&friendCount)

		now := time.Now()
		cg := NewContactGroup{
			UserId: old.UID, Name: old.Name, Num: int(friendCount),
			Sort: 0, CreatedAt: now, UpdatedAt: now,
		}
		if err := mysqlDB.Create(&cg).Error; err != nil {
			log.Printf("[Migrate] Warning: contact group (user:%d name:%s): %v", old.UID, old.Name, err)
			continue
		}
		tagMap[old.ID] = cg.Id
		count++
	}

	// Update contact.group_id
	log.Println("[Migrate] Updating contact group_id associations...")
	var tagFriends []OldUserTagFriend
	if err := sqliteDB.Find(&tagFriends).Error; err == nil {
		upd := 0
		for _, tf := range tagFriends {
			newGid, ok := tagMap[tf.TagID]
			if !ok {
				continue
			}
			r := mysqlDB.Model(&NewContact{}).
				Where("user_id = ? AND friend_id = ?", tf.UID, tf.FriendID).
				Update("group_id", newGid)
			if r.RowsAffected > 0 {
				upd++
			}
		}
		log.Printf("[Migrate] Updated %d contact group_id associations", upd)
	}

	log.Printf("[Migrate] Migrated %d/%d contact groups", count, len(oldTags))
	return count, nil
}

// ---------------------------------------------------------------------------
// Migration: Emoticons
// ---------------------------------------------------------------------------

func migrateEmoticons(sqliteDB, mysqlDB *gorm.DB) (int, error) {
	log.Println("[Migrate] Starting emoticon migration...")
	var oldEmojis []OldEmoji
	if err := sqliteDB.Where("status = 1 AND delete_time = 0").Find(&oldEmojis).Error; err != nil {
		return 0, fmt.Errorf("read old emojis: %w", err)
	}
	log.Printf("[Migrate] Found %d old emojis", len(oldEmojis))

	count := 0
	for _, old := range oldEmojis {
		createdAt := time.Unix(old.CreateTime, 0)
		updatedAt := createdAt
		if old.UpdateTime > 0 {
			updatedAt = time.Unix(old.UpdateTime, 0)
		}

		e := NewEmoticonItem{
			EmoticonId: 0, UserId: old.UserID, Describe: old.Name,
			Url: old.Src, CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		if err := mysqlDB.Create(&e).Error; err != nil {
			log.Printf("[Migrate] Warning: emoticon (user:%d): %v", old.UserID, err)
			continue
		}
		count++
	}
	log.Printf("[Migrate] Migrated %d/%d emoticons", count, len(oldEmojis))
	return count, nil
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("=== IM Data Migration Tool ===")
	log.Println("Old system (im_com) -> New system (lumen_im)")

	dumpFile := "im_com_2026-02-24_16-19-45_mysql_data_8gb31.sql"
	configFile := "config.yaml"
	if len(os.Args) > 1 {
		dumpFile = os.Args[1]
	}
	if len(os.Args) > 2 {
		configFile = os.Args[2]
	}

	if _, err := os.Stat(dumpFile); os.IsNotExist(err) {
		log.Fatalf("Dump file not found: %s", dumpFile)
	}

	log.Printf("Loading config: %s", configFile)
	cfg, err := loadConfig(configFile)
	if err != nil {
		log.Fatalf("Config error: %v", err)
	}
	log.Printf("Target: %s:%d/%s", cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database)

	if cfg.MySQL.Host == "" || cfg.MySQL.Database == "" {
		log.Fatalf("MySQL config incomplete: host=%q database=%q. Check config file.", cfg.MySQL.Host, cfg.MySQL.Database)
	}

	// --- Step 1: SQLite import ---
	sqlitePath := "migration_temp.db"
	os.Remove(sqlitePath)
	log.Printf("Creating SQLite: %s", sqlitePath)

	sqliteDB, err := gorm.Open(sqlite.Open(sqlitePath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("SQLite open: %v", err)
	}
	rawSqlite, _ := sqliteDB.DB()
	rawSqlite.Exec("PRAGMA journal_mode=WAL")
	rawSqlite.Exec("PRAGMA synchronous=OFF")
	rawSqlite.Exec("PRAGMA cache_size=10000")

	if err := importDumpToSQLite(dumpFile, sqliteDB); err != nil {
		log.Fatalf("Import failed: %v", err)
	}

	// Print row counts for key tables
	for _, t := range []string{"yu_user", "yu_friend", "yu_group", "yu_group_user", "yu_message", "yu_hongbao", "yu_transfer", "yu_user_tag", "yu_user_tag_friend", "yu_emoji"} {
		var c int64
		sqliteDB.Table(t).Count(&c)
		log.Printf("[SQLite] %-25s: %d rows", t, c)
	}

	// --- Step 2: Connect to MySQL ---
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=%s&parseTime=True&loc=Local&collation=utf8mb4_general_ci",
		cfg.MySQL.Username, cfg.MySQL.Password, cfg.MySQL.Host, cfg.MySQL.Port, cfg.MySQL.Database, cfg.MySQL.Charset)
	log.Println("Connecting to MySQL...")
	mysqlDB, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		log.Fatalf("MySQL connect: %v", err)
	}
	rawMySQL, _ := mysqlDB.DB()
	if err := rawMySQL.Ping(); err != nil {
		log.Fatalf("MySQL ping: %v", err)
	}
	log.Println("MySQL connected")

	// --- Step 3: Migrate ---
	start := time.Now()
	results := make(map[string]int)

	steps := []struct {
		name string
		fn   func(*gorm.DB, *gorm.DB) (int, error)
	}{
		{"Users", migrateUsers},
		{"Contacts", migrateContacts},
		{"Groups", migrateGroups},
		{"GroupMembers", migrateGroupMembers},
		{"Messages", migrateMessages},
		{"ContactGroups", migrateContactGroups},
		{"Emoticons", migrateEmoticons},
	}

	for _, step := range steps {
		log.Printf("\n========== %s ==========", step.name)
		n, err := step.fn(sqliteDB, mysqlDB)
		if err != nil {
			log.Printf("[ERROR] %s: %v", step.name, err)
		}
		results[step.name] = n
	}

	log.Println("\n========== Summary ==========")
	for _, step := range steps {
		log.Printf("  %-20s: %d records", step.name, results[step.name])
	}
	log.Printf("  Time: %v", time.Since(start))
	log.Println("=============================")

	rawSqlite.Close()
	log.Printf("Temp DB: %s (delete when done)", sqlitePath)
	log.Println("Migration complete!")
}
