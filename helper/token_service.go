package helper

import (
	"context"
	"fmt"
	"nursor-envoy-rpc/models"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TokenPersistent manages synchronization of token-related data between Redis and the database.
type TokenPersistent struct {
	db          *gorm.DB
	initialized bool
}

// singleton tpInstance
var tpInstance *TokenPersistent
var tpOnce sync.Once

// GetTPInstance returns the singleton instance of TokenPersistent.
func GetTPInstance() *TokenPersistent {
	tpOnce.Do(func() {
		db := GetNewDB()
		tpInstance = &TokenPersistent{}
		tpInstance.initialize(db)
	})
	return tpInstance
}

// initialize sets up the TokenPersistent with the provided database connection.
func (tp *TokenPersistent) initialize(db *gorm.DB) {
	if tp.initialized {
		return
	}
	if db == nil {
		db = GetNewDB() // Assume this function exists to get a new GORM DB connection
	}
	tp.db = db
	tp.initialized = true
}

// GetAvailableTokenIdFromDB retrieves an available cursor token from the database.
func (tp *TokenPersistent) GetAvailableTokenIdFromDB(ctx context.Context, count int) ([]models.Cursor, error) {
	var cursors []models.Cursor
	currentTime := time.Now()
	err := tp.db.WithContext(ctx).
		Where("status = ? AND expires_at > ?", "active", currentTime).
		Order("`usage` ASC, expires_at ASC").
		Limit(count).
		Find(&cursors).Error
	if err == gorm.ErrRecordNotFound {
		logrus.Warn("No available Cursor found")
		return nil, nil
	}
	if err != nil {
		logrus.Errorf("Error retrieving available token: %v", err)
		return nil, err
	}
	if len(cursors) > 0 {
		var cursorIDs []string
		for _, cursor := range cursors {
			cursorIDs = append(cursorIDs, *cursor.CursorID) // Assuming Cursor model has an ID field
		}
		err = tp.db.WithContext(ctx).
			Model(&models.Cursor{}).
			Where("cursor_id IN ?", cursorIDs).
			Update("status", "dispatched").Error
		if err != nil {
			logrus.Errorf("Error updating cursor status to dispatched: %v", err)
			return nil, err
		}
	}

	return cursors, nil
}

// SaveTokenData saves token usage data from Redis to the database.
func (tp *TokenPersistent) SaveTokenData(ctx context.Context, tokenData map[string]interface{}) (bool, error) {
	token, ok := tokenData["token"].(string)
	if !ok || token == "" {
		logrus.Error("Missing or invalid token field in token_data")
		return false, nil
	}

	return tp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor models.Cursor
		if err := tx.Where("cursor_id = ?", token).First(&cursor).Error; err != nil {
			logrus.Errorf("Cursor not found for token: %s", token[:10])
			return err
		}

		usageCountFloat, _ := tokenData["usage_count"].(float64)
		usageCount := int(usageCountFloat)
		if cursor.Usage == nil || *cursor.Usage != usageCount {
			logrus.Infof("Updating Cursor usage: ID=%d, Old=%v, New=%d", cursor.ID, cursor.Usage, usageCount)
			usageCountPtr := usageCount
			cursor.Usage = &usageCountPtr
			if err := tx.Save(&cursor).Error; err != nil {
				return err
			}
		}

		boundUsersRaw, _ := tokenData["bound_users"].([]interface{})
		userUsageDetails, _ := tokenData["user_usage_details"].(map[string]interface{})
		processedUsers := 0

		for _, userIDRaw := range boundUsersRaw {
			userIDStr, ok := userIDRaw.(string)
			if !ok {
				logrus.Warnf("Invalid user ID format: %v", userIDRaw)
				continue
			}
			userID, err := strconv.Atoi(userIDStr)
			if err != nil {
				logrus.Warnf("Cannot convert user ID to int: %s", userIDStr)
				continue
			}

			var user models.User
			if err := tx.Where("id = ?", userID).First(&user).Error; err != nil {
				logrus.Warnf("User not found: ID=%d", userID)
				continue
			}

			userDetail, ok := userUsageDetails[userIDStr]
			if !ok {
				logrus.Warnf("No usage details for user: %d", userID)
				continue
			}

			var askCount int
			var tokenUsage int
			var modelUsages map[string]int
			var urlQueries map[string]int

			switch detail := userDetail.(type) {
			case float64:
				askCount = int(detail)
				tokenUsage = askCount * 100
				modelUsages = map[string]int{"default": askCount}
				urlQueries = map[string]int{}
			case map[string]interface{}:
				askCountFloat, _ := detail["count"].(float64)
				askCount = int(askCountFloat)
				tokenUsageFloat, _ := detail["token_usage"].(float64)
				if tokenUsageFloat == 0 {
					tokenUsage = askCount * 100
				} else {
					tokenUsage = int(tokenUsageFloat)
				}
				modelUsagesRaw, _ := detail["model_usages"].(map[string]interface{})
				modelUsages = make(map[string]int)
				for k, v := range modelUsagesRaw {
					if val, ok := v.(float64); ok {
						modelUsages[k] = int(val)
					}
				}
				urlQueriesRaw, _ := detail["url_queries"].(map[string]interface{})
				urlQueries = make(map[string]int)
				for k, v := range urlQueriesRaw {
					if val, ok := v.(float64); ok {
						urlQueries[k] = int(val)
					}
				}
			default:
				logrus.Warnf("Invalid usage detail format for user %d: %T", userID, detail)
				continue
			}

			if askCount <= 0 {
				continue
			}

			var bind models.UserCursorAccountBind
			err = tx.Where("user_id = ? AND cursor_id = ?", userID, cursor.ID).
				Assign(models.UserCursorAccountBind{AskCount: new(int), TokenUsage: new(int)}).
				FirstOrCreate(&bind).Error
			if err != nil {
				return err
			}

			askCountPtr := askCount
			tokenUsagePtr := tokenUsage
			bind.AskCount = &askCountPtr
			bind.TokenUsage = &tokenUsagePtr
			if err := tx.Save(&bind).Error; err != nil {
				return err
			}
			processedUsers++

			for url, count := range urlQueries {
				if count <= 0 {
					continue
				}
				var urlRecord models.CursorUrlQueryRecord
				currentDate := time.Now().Truncate(24 * time.Hour)
				err = tx.Where("cursor_id = ? AND user_id = ? AND url = ? AND date = ?",
					cursor.ID, userID, url, currentDate).
					Assign(models.CursorUrlQueryRecord{Count: new(int), TokenUsage: new(int)}).
					FirstOrCreate(&urlRecord).Error
				if err != nil {
					return err
				}
				urlRecordCount := 0
				if urlRecord.Count != nil {
					urlRecordCount = *urlRecord.Count
				}
				urlRecordCount += count
				urlRecord.Count = &urlRecordCount
				urlRecordTokenUsage := 0
				if urlRecord.TokenUsage != nil {
					urlRecordTokenUsage = *urlRecord.TokenUsage
				}
				if askCount > 0 {
					urlRecordTokenUsage += tokenUsage * (count / askCount)
				}
				urlRecord.TokenUsage = &urlRecordTokenUsage
				if err := tx.Save(&urlRecord).Error; err != nil {
					return err
				}
			}

			for modelName, modelCount := range modelUsages {
				if modelCount <= 0 {
					continue
				}
				var modelRecord models.UserCursorModelUsage
				err = tx.Where("cursor_id = ? AND user_id = ? AND model_name = ?",
					cursor.ID, userID, modelName).
					Assign(models.UserCursorModelUsage{AskCount: new(int)}).
					FirstOrCreate(&modelRecord).Error
				if err != nil {
					return err
				}
				modelCountPtr := modelCount
				modelRecord.AskCount = &modelCountPtr
				if err := tx.Save(&modelRecord).Error; err != nil {
					return err
				}
			}
		}

		logrus.Infof("Successfully saved token data: cursor_id=%d, bound_users=%d, processed=%d",
			cursor.ID, len(boundUsersRaw), processedUsers)
		return nil
	}) == nil, nil
}

// SaveCursorUnavailableReason saves the reason a cursor is unavailable.
func (tp *TokenPersistent) SaveCursorUnavailableReason(ctx context.Context, cursorID string, reason TokenUnavailableReason) (bool, error) {
	return tp.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cursor models.Cursor
		if err := tx.Where("cursor_id = ?", cursorID).First(&cursor).Error; err != nil {
			logrus.Warnf("Cursor not found for token: %s", cursorID[:10])
			return err
		}

		unavailableReason := models.CursorUnavaliableReason{
			CursorID:   cursorID,
			Reason:     reason.Reason,
			ReasonType: reason.ReasonType,
		}
		if err := tx.Create(&unavailableReason).Error; err != nil {
			logrus.Errorf("Error saving cursor unavailable reason: %v", err)
			return err
		}
		return nil
	}) == nil, nil
}

// SaveTokenUsage saves the usage count for a token.
func (tp *TokenPersistent) SaveTokenUsage(ctx context.Context, cursorID string, usage int) (bool, error) {
	var cursor models.Cursor
	if err := tp.db.WithContext(ctx).Where("cursor_id = ?", cursorID).First(&cursor).Error; err != nil {
		logrus.Warnf("Cursor not found for token: %s", cursorID[:10])
		return false, err
	}

	usagePtr := usage
	cursor.Usage = &usagePtr
	if err := tp.db.WithContext(ctx).Save(&cursor).Error; err != nil {
		logrus.Errorf("Error saving token usage: %v", err)
		return false, err
	}
	return true, nil
}

func (tp *TokenPersistent) GetTokenByTokenId(ctx context.Context, tokenId string) (*models.Cursor, error) {
	var cursor models.Cursor
	if err := tp.db.WithContext(ctx).Where("cursor_id = ?", tokenId).First(&cursor).Error; err != nil {
		logrus.Warnf("Cursor not found for token: %s", tokenId)
		return nil, err
	}
	return &cursor, nil
}

// TokenUnavailableReason mirrors the Python enum-like structure.
type TokenUnavailableReason struct {
	Reason     string
	ReasonType string
}

// GetNewDB is a placeholder for getting a new GORM DB connection.
func GetNewDB() *gorm.DB {
	// Implement GORM DB initialization (e.g., using models.InitDB)
	MYSQL_HOST := os.Getenv("MYSQL_HOST")
	MYSQL_PORT := os.Getenv("MYSQL_PORT")
	MYSQL_USER := os.Getenv("MYSQL_USER")
	MYSQL_PASSWORD := os.Getenv("MYSQL_PASSWORD")
	MYSQL_DATABASE := os.Getenv("MYSQL_DATABASE")
	if MYSQL_HOST == "" {
		MYSQL_HOST = "172.16.238.2"
	}
	if MYSQL_PORT == "" {
		MYSQL_PORT = "31494"
	}
	if MYSQL_USER == "" {
		MYSQL_USER = "root"
	}
	if MYSQL_PASSWORD == "" {
		MYSQL_PASSWORD = "asd123456"
	}
	if MYSQL_DATABASE == "" {
		MYSQL_DATABASE = "nursorv2"
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", MYSQL_USER, MYSQL_PASSWORD, MYSQL_HOST, MYSQL_PORT, MYSQL_DATABASE)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		logrus.Fatalf("Failed to initialize database: %v", err)
	}
	return db
}

// ResetInstance resets the singleton instance (mainly for testing).
func ResetInstance() {
	tpOnce = sync.Once{}
	tpInstance = nil
	logrus.Info("TokenPersistent singleton has been reset")
}
