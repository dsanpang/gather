package shutdown

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// GracefulShutdown 优雅停机管理器
type GracefulShutdown struct {
	logger         *logrus.Logger
	timeout        time.Duration
	shutdownFuncs  []ShutdownFunc
	mu             sync.Mutex
	signalChan     chan os.Signal
	ctx            context.Context
	cancel         context.CancelFunc
	wg             sync.WaitGroup
	isShuttingDown bool
}

// ShutdownFunc 停机处理函数
type ShutdownFunc struct {
	Name  string
	Func  func(ctx context.Context) error
	Order int // 执行顺序，数字越小越早执行
}

// NewGracefulShutdown 创建优雅停机管理器
func NewGracefulShutdown(timeout time.Duration, logger *logrus.Logger) *GracefulShutdown {
	if timeout <= 0 {
		timeout = 30 * time.Second // 默认30秒超时
	}

	ctx, cancel := context.WithCancel(context.Background())

	gs := &GracefulShutdown{
		logger:        logger,
		timeout:       timeout,
		shutdownFuncs: make([]ShutdownFunc, 0),
		signalChan:    make(chan os.Signal, 1),
		ctx:           ctx,
		cancel:        cancel,
	}

	// 监听操作系统信号
	signal.Notify(gs.signalChan,
		syscall.SIGINT,  // Ctrl+C
		syscall.SIGTERM, // 终止信号
		syscall.SIGQUIT, // 退出信号
	)

	return gs
}

// RegisterShutdownFunc 注册停机处理函数
func (gs *GracefulShutdown) RegisterShutdownFunc(name string, fn func(ctx context.Context) error, order int) {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	gs.shutdownFuncs = append(gs.shutdownFuncs, ShutdownFunc{
		Name:  name,
		Func:  fn,
		Order: order,
	})

	gs.logger.Debugf("注册停机处理函数: %s (order: %d)", name, order)
}

// Start 启动信号监听
func (gs *GracefulShutdown) Start() {
	gs.wg.Add(1)
	go gs.signalHandler()
	gs.logger.Info("优雅停机管理器已启动，监听信号: SIGINT, SIGTERM, SIGQUIT")
}

// Wait 等待停机完成
func (gs *GracefulShutdown) Wait() {
	gs.wg.Wait()
}

// Context 获取上下文
func (gs *GracefulShutdown) Context() context.Context {
	return gs.ctx
}

// Shutdown 手动触发停机
func (gs *GracefulShutdown) Shutdown() {
	gs.mu.Lock()
	if gs.isShuttingDown {
		gs.mu.Unlock()
		return
	}
	gs.isShuttingDown = true
	gs.mu.Unlock()

	gs.logger.Info("手动触发优雅停机...")
	gs.performShutdown()
}

// signalHandler 信号处理器
func (gs *GracefulShutdown) signalHandler() {
	defer gs.wg.Done()

	sig := <-gs.signalChan
	gs.logger.Infof("收到停机信号: %v", sig)

	gs.mu.Lock()
	if gs.isShuttingDown {
		gs.mu.Unlock()
		gs.logger.Warn("停机过程已在进行中，忽略信号")
		return
	}
	gs.isShuttingDown = true
	gs.mu.Unlock()

	gs.performShutdown()
}

// performShutdown 执行停机过程
func (gs *GracefulShutdown) performShutdown() {
	gs.logger.Info("开始优雅停机流程...")

	// 创建带超时的上下文
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), gs.timeout)
	defer shutdownCancel()

	// 按顺序排序停机函数
	gs.sortShutdownFuncs()

	// 执行所有停机函数
	var shutdownErrors []error
	for _, shutdownFunc := range gs.shutdownFuncs {
		gs.logger.Infof("执行停机处理: %s", shutdownFunc.Name)

		start := time.Now()
		err := shutdownFunc.Func(shutdownCtx)
		duration := time.Since(start)

		if err != nil {
			gs.logger.Errorf("停机处理 '%s' 失败 (耗时: %v): %v", shutdownFunc.Name, duration, err)
			shutdownErrors = append(shutdownErrors, fmt.Errorf("%s: %w", shutdownFunc.Name, err))
		} else {
			gs.logger.Infof("停机处理 '%s' 完成 (耗时: %v)", shutdownFunc.Name, duration)
		}

		// 检查是否超时
		select {
		case <-shutdownCtx.Done():
			gs.logger.Warn("停机超时，强制退出")
			gs.cancel()
			return
		default:
		}
	}

	// 取消主上下文，通知所有goroutines停止
	gs.cancel()

	if len(shutdownErrors) > 0 {
		gs.logger.Errorf("停机过程中发生 %d 个错误", len(shutdownErrors))
		for _, err := range shutdownErrors {
			gs.logger.Error(err)
		}
	}

	gs.logger.Info("优雅停机流程完成")
}

// sortShutdownFuncs 按执行顺序排序停机函数
func (gs *GracefulShutdown) sortShutdownFuncs() {
	// 使用简单的冒泡排序
	for i := 0; i < len(gs.shutdownFuncs)-1; i++ {
		for j := 0; j < len(gs.shutdownFuncs)-i-1; j++ {
			if gs.shutdownFuncs[j].Order > gs.shutdownFuncs[j+1].Order {
				gs.shutdownFuncs[j], gs.shutdownFuncs[j+1] = gs.shutdownFuncs[j+1], gs.shutdownFuncs[j]
			}
		}
	}
}

// IsShuttingDown 检查是否正在停机
func (gs *GracefulShutdown) IsShuttingDown() bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	return gs.isShuttingDown
}

// GetTimeout 获取停机超时时间
func (gs *GracefulShutdown) GetTimeout() time.Duration {
	return gs.timeout
}

// SetTimeout 设置停机超时时间
func (gs *GracefulShutdown) SetTimeout(timeout time.Duration) {
	if timeout > 0 {
		gs.timeout = timeout
		gs.logger.Debugf("停机超时时间设置为: %v", timeout)
	}
}

// GetRegisteredFunctions 获取已注册的停机函数列表
func (gs *GracefulShutdown) GetRegisteredFunctions() []string {
	gs.mu.Lock()
	defer gs.mu.Unlock()

	names := make([]string, len(gs.shutdownFuncs))
	for i, fn := range gs.shutdownFuncs {
		names[i] = fn.Name
	}
	return names
}

// Close 关闭优雅停机管理器
func (gs *GracefulShutdown) Close() error {
	signal.Stop(gs.signalChan)
	close(gs.signalChan)

	if !gs.isShuttingDown {
		gs.Shutdown()
	}

	return nil
}

// WaitForShutdown 等待停机信号并执行停机
func (gs *GracefulShutdown) WaitForShutdown() {
	gs.Start()
	gs.Wait()
}

// ShutdownOrder 定义停机顺序常量
const (
	// 应用层停机顺序
	OrderStopAcceptingRequests = 10 // 停止接受新请求
	OrderWaitForActiveRequests = 20 // 等待活跃请求完成
	OrderFlushProducers        = 30 // 刷新消息生产者缓冲区
	OrderCloseConnections      = 40 // 关闭数据库/外部服务连接
	OrderSaveState             = 50 // 保存状态和进度
	OrderCleanupResources      = 60 // 清理资源
)
