package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"gather/internal/collector"
	"gather/internal/config"
	"gather/internal/output"
)

var (
	// 基础参数
	startBlock uint64
	endBlock   uint64
	workers    int
	batchSize  int
	outputPath string
	format     string

	// 流处理参数
	stream bool

	// 高级参数
	configFile string
	verbose    bool
	dryRun     bool
	compress   bool

	// 进度管理参数
	resume        bool // 是否启用断点续传
	resetProgress bool // 是否重置进度
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "gather",
		Short: "ETH链数据采集工具",
		Long:  `高性能的以太坊区块链数据采集工具，专注于采集原始区块、交易和日志数据`,
		RunE:  run,
	}

	// 基础参数
	rootCmd.Flags().Uint64Var(&startBlock, "start-block", 0, "起始区块号")
	rootCmd.Flags().Uint64Var(&endBlock, "end-block", 0, "结束区块号")
	rootCmd.Flags().IntVar(&workers, "workers", 10, "工作协程数")
	rootCmd.Flags().IntVar(&batchSize, "batch-size", 100, "批次大小")
	rootCmd.Flags().StringVar(&outputPath, "output", "./outputs", "输出路径")
	rootCmd.Flags().StringVar(&format, "format", "json", "输出格式 (json)")

	// 流处理参数
	rootCmd.Flags().BoolVar(&stream, "stream", false, "启用实时流处理")

	// 高级参数
	rootCmd.Flags().StringVar(&configFile, "config", "configs/config.yaml", "配置文件路径")
	rootCmd.Flags().BoolVar(&verbose, "verbose", false, "详细输出")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", false, "试运行模式")
	rootCmd.Flags().BoolVar(&compress, "compress", false, "启用压缩")

	// 进度管理参数
	rootCmd.Flags().BoolVar(&resume, "resume", true, "启用断点续传（默认开启）")
	rootCmd.Flags().BoolVar(&resetProgress, "reset-progress", false, "重置进度重新开始")

	// 进度查询子命令
	progressCmd := &cobra.Command{
		Use:   "progress",
		Short: "查看采集进度",
		RunE:  showProgress,
	}

	rootCmd.AddCommand(progressCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

func run(cmd *cobra.Command, args []string) error {
	// 设置日志级别
	if verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 创建输出器
	outputter, err := output.NewOutput(outputPath, format, compress)
	if err != nil {
		return fmt.Errorf("创建输出器失败: %w", err)
	}
	defer outputter.Close()

	// 创建采集器（使用结构化日志）
	collector := collector.NewCollectorWithLogging(cfg.Blockchain, outputter, logger, cfg.Logging)
	defer collector.Close()

	// 处理进度重置
	if resetProgress {
		logger.Info("重置采集进度...")
		if err := collector.ResetProgress(); err != nil {
			logger.Warnf("重置进度失败: %v", err)
		} else {
			logger.Info("进度已重置")
		}
	}

	// 启动优雅停机监听
	collector.StartGracefulShutdown()

	// 使用采集器的停机上下文
	ctx := collector.GetShutdownContext()

	// 执行采集任务
	var runErr error
	if stream {
		runErr = runStreamMode(ctx, collector, logger)
	} else {
		runErr = runBatchMode(ctx, collector, logger)
	}

	// 等待优雅停机完成
	logger.Info("等待优雅停机完成...")
	collector.WaitForShutdown()

	return runErr
}

func runBatchMode(ctx context.Context, collector *collector.Collector, logger *logrus.Logger) error {
	if startBlock == 0 || endBlock == 0 {
		return fmt.Errorf("批量模式需要指定 --start-block 和 --end-block")
	}

	if startBlock > endBlock {
		return fmt.Errorf("起始区块号不能大于结束区块号")
	}

	logger.Infof("开始批量采集区块 %d - %d", startBlock, endBlock)

	// 执行批量采集
	result, err := collector.CollectBatch(ctx, startBlock, endBlock, workers, batchSize)
	if err != nil {
		return fmt.Errorf("批量采集失败: %w", err)
	}

	// 输出统计信息
	logger.Info("采集完成，统计信息:")
	logger.Infof("  处理区块数: %d", result.ProcessedBlocks)
	logger.Infof("  总区块数: %d", result.TotalBlocks)
	logger.Infof("  总交易数: %d", result.TotalTransactions)
	logger.Infof("  总日志数: %d", result.TotalLogs)
	logger.Infof("  耗时: %s", result.Duration)
	logger.Infof("  区块/秒: %.2f", result.BlocksPerSecond)
	logger.Infof("  交易/秒: %.2f", result.TransactionsPerSecond)

	return nil
}

func runStreamMode(ctx context.Context, collector *collector.Collector, logger *logrus.Logger) error {
	logger.Info("启动实时流处理模式")

	// 启动流处理
	return collector.CollectStream(ctx)
}

// showProgress 显示采集进度
func showProgress(cmd *cobra.Command, args []string) error {
	// 设置日志级别
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 加载配置
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 创建临时输出器（不会实际使用）
	outputter, err := output.NewOutput("./temp", "json", false)
	if err != nil {
		return fmt.Errorf("创建输出器失败: %w", err)
	}
	defer outputter.Close()

	// 创建采集器（使用结构化日志）
	collector := collector.NewCollectorWithLogging(cfg.Blockchain, outputter, logger, cfg.Logging)
	defer collector.Close()

	// 获取进度信息
	progressInfo := collector.GetProgressInfo()

	// 显示进度信息
	fmt.Println("📊 采集进度信息")
	fmt.Println(strings.Repeat("=", 50))

	for key, value := range progressInfo {
		fmt.Printf("%-20s: %v\n", key, value)
	}

	return nil
}
