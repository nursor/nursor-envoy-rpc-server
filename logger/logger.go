package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	// 全局日志记录器
	logger *log.Logger
	// 日志文件
	logFile *os.File
)

func init() {
	logger = log.New(os.Stdout, "", log.LstdFlags)
	logger.SetFlags(log.LstdFlags | log.Lshortfile)
}

func Info(v ...interface{}) {
	log.Println(v...)
}

func Error(v ...interface{}) {
	log.Println(v...)
	fmt.Println(v...)
	//sentry.CaptureMessage(fmt.Sprintf("%v", v...))
	//go func() {
	//	sentry.Flush(2 * time.Second)
	//}()
}

// 初始化日志记录器
func Init() error {
	// 获取用户主目录
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("获取用户主目录失败: %v", err)
	}

	// 创建日志目录
	logDir := filepath.Join(homeDir, ".nursor")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("创建日志目录失败: %v", err)
	}

	// 打开日志文件
	logPath := filepath.Join(logDir, "app.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("打开日志文件失败: %v", err)
	}

	// 创建日志记录器
	logger = log.New(logFile, "", log.LstdFlags)
	logger.Printf("日志系统初始化成功，日志文件: %s", logPath)

	return nil
}

func GetCustomLogger() *log.Logger {
	return logger
}
