package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gzydong/go-chat/config"
	"github.com/gzydong/go-chat/internal/entity"
	"github.com/gzydong/go-chat/internal/pkg/encrypt"
	"github.com/gzydong/go-chat/internal/pkg/jwtutil"
	"github.com/gzydong/go-chat/internal/pkg/logger"
	"github.com/gzydong/go-chat/internal/provider"
	"github.com/gzydong/go-chat/internal/repository/model"
	"github.com/samber/lo"
)

type UserCredentials struct {
	UserId int    `json:"user_id"`
	Token  string `json:"token"`
}

func main() {
	// 1. Load Config
	conf := config.New("./config.yaml")

	// 2. Init Logger
	logger.Init(conf.Log.LogFilePath("seeder.log"), logger.LevelInfo, "seeder", conf.App.Env == "dev" || conf.App.Debug)

	// 3. Init DB
	db := provider.NewMySQLClient(conf)

	logger.Infof("Starting user seeding...")

	const baseMobile int64 = 18800000000
	var credentials []UserCredentials

	for i := 0; i < 2000; i++ {
		mobile := fmt.Sprintf("%d", baseMobile+int64(i))
		nickname := fmt.Sprintf("User%d", i)
		salt := encrypt.GenerateSalt()
		password := encrypt.HashPassword("123456", salt)

		var user model.Users
		var count int64

		// Check if exists
		db.Model(&model.Users{}).Where("mobile = ?", mobile).Count(&count)

		if count > 0 {
			db.Model(&model.Users{}).Where("mobile = ?", mobile).First(&user)
			logger.Infof("User %s already exists (ID: %d), using existing user", mobile, user.Id)
		} else {
			user = model.Users{
				Mobile:    lo.ToPtr(mobile),
				Nickname:  nickname,
				Gender:    model.UsersGenderDefault,
				Password:  password,
				Salt:      salt,
				IsRobot:   model.No,
				Status:    model.UsersStatusNormal,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			}

			if err := db.Create(&user).Error; err != nil {
				logger.Errorf("Failed to create user %s: %v", mobile, err)
				continue
			}
			logger.Infof("Created user %s (ID: %d)", mobile, user.Id)
		}

		// Generate Token
		token, err := jwtutil.NewTokenWithClaims(
			[]byte(conf.Jwt.Secret), entity.WebClaims{
				UserId: int32(user.Id),
			},
			func(c *jwt.RegisteredClaims) {
				c.Issuer = entity.JwtIssuerWeb
			},
			jwtutil.WithTokenExpiresAt(time.Duration(conf.Jwt.ExpiresTime)*time.Second),
		)

		if err != nil {
			logger.Errorf("Failed to generate token for user %d: %v", user.Id, err)
			continue
		}

		credentials = append(credentials, UserCredentials{
			UserId: user.Id,
			Token:  token,
		})
	}

	// Write to JSON file
	file, err := os.Create("./users.json")
	if err != nil {
		logger.Errorf("Failed to create users.json: %v", err)
		os.Exit(1)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(credentials); err != nil {
		logger.Errorf("Failed to encode credentials: %v", err)
		os.Exit(1)
	}

	logger.Infof("Seeding completed. Credentials saved to users.json")
}
