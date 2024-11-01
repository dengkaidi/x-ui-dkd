package service

import (
	"container/list"
	"errors"
	"sync"
	"time"
	"x-ui/database"
	"x-ui/database/model"
	"x-ui/logger"

	"gorm.io/gorm"
)

type UserService struct {
}

type FailedLoginRecord struct {
	FailureCount int           // 失败次数
	LockedUntil  time.Time     // 锁定到期时间
	element      *list.Element // 记录在链表中的位置，便于删除
}

type LoginLockManager struct {
	maxFailureCount int           // 最大失败次数
	lockDuration    time.Duration // 锁定时间
	maxTrackedUsers int           // 最大跟踪失败用户数
	failedLoginMap  sync.Map      // 并发安全的 map，用于存储失败用户记录
	failOrder       *list.List    // 双向链表,按失败时间顺序记录用户
	orderMu         sync.Mutex    // 锁，用于保护 failOrder 的并发操作
}

var loginLockManager *LoginLockManager

func NewLoginLockManager(maxFailureCount int, lockDuration time.Duration, maxTrackedUsers int) *LoginLockManager {
	return &LoginLockManager{
		maxFailureCount: maxFailureCount,
		lockDuration:    lockDuration,
		maxTrackedUsers: maxTrackedUsers,
		failOrder:       list.New(),
	}
}

func init() {
	loginLockManager = NewLoginLockManager(3, 15*time.Minute, 50)
}

func (s *UserService) GetFirstUser() (*model.User, error) {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		First(user).
		Error
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) CheckUser(username string, password string) *model.User {
	db := database.GetDB()

	user := &model.User{}
	err := db.Model(model.User{}).
		Where("username = ? and password = ?", username, password).
		First(user).
		Error
	if err == gorm.ErrRecordNotFound {
		return nil
	} else if err != nil {
		logger.Warning("check user err:", err)
		return nil
	}
	return user
}

func (s *UserService) UpdateUser(id int, username string, password string) error {
	db := database.GetDB()
	return db.Model(model.User{}).
		Where("id = ?", id).
		Update("username", username).
		Update("password", password).
		Error
}

func (s *UserService) UpdateFirstUser(username string, password string) error {
	if username == "" {
		return errors.New("username can not be empty")
	} else if password == "" {
		return errors.New("password can not be empty")
	}
	db := database.GetDB()
	user := &model.User{}
	err := db.Model(model.User{}).First(user).Error
	if database.IsNotFound(err) {
		user.Username = username
		user.Password = password
		return db.Model(model.User{}).Create(user).Error
	} else if err != nil {
		return err
	}
	user.Username = username
	user.Password = password
	return db.Save(user).Error
}

// 检查用户是否可以登录
func (s *UserService) CanLogin(username string) bool {
	// admin 和 root 直接拒绝登陆
	if username == "admin" || username == "root" {
		return false
	}
	record, exists := loginLockManager.failedLoginMap.Load(username)
	if !exists {
		return true
	}

	userRecord := record.(*FailedLoginRecord)
	return !time.Now().Before(userRecord.LockedUntil)
}

func (s *UserService) LoginFailedAccumulate(username string) {
	record, _ := loginLockManager.failedLoginMap.LoadOrStore(username, &FailedLoginRecord{FailureCount: 0})
	userRecord := record.(*FailedLoginRecord)

	loginLockManager.orderMu.Lock()
	defer loginLockManager.orderMu.Unlock()

	if userRecord.FailureCount == 0 {
		userRecord.FailureCount++
		userRecord.element = loginLockManager.failOrder.PushBack(username)
	} else {
		userRecord.FailureCount++
	}

	if userRecord.FailureCount >= loginLockManager.maxFailureCount {
		userRecord.LockedUntil = time.Now().Add(loginLockManager.lockDuration)
	}

	logger.Infof("username:%s failed count:%d", username, userRecord.FailureCount)

	if loginLockManager.failOrder.Len() > loginLockManager.maxTrackedUsers {
		oldest := loginLockManager.failOrder.Remove(loginLockManager.failOrder.Front()).(string)
		loginLockManager.failedLoginMap.Delete(oldest)
	}

	newRecord, _ := loginLockManager.failedLoginMap.Load(username)
	logger.Infof("current cache size:%d, username: %s, map value:%+v",
		loginLockManager.failOrder.Len(), username, newRecord)
}

// 登录成功时重置用户失败记录
func (s *UserService) ResetFailures(username string) {
	record, exists := loginLockManager.failedLoginMap.Load(username)
	if exists {
		userRecord := record.(*FailedLoginRecord)

		loginLockManager.orderMu.Lock()
		defer loginLockManager.orderMu.Unlock()

		loginLockManager.failOrder.Remove(userRecord.element)
		loginLockManager.failedLoginMap.Delete(username)
	}
}
