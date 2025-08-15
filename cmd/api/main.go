package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"gather/internal/api"
	"gather/internal/config"
	"gather/internal/output"

	"github.com/sirupsen/logrus"
)

var (
	configPath = flag.String("config", "configs/config.yaml", "配置文件路径")
	outputPath = flag.String("output", "./outputs", "输出路径")
	port       = flag.Int("port", 8080, "API 服务端口")
	verbose    = flag.Bool("verbose", false, "详细输出")
)

func main() {
	flag.Parse()

	// 设置日志级别
	if *verbose {
		logrus.SetLevel(logrus.DebugLevel)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}

	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})

	// 自动检测并加载配置
	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		logger.Fatalf("加载配置失败: %v", err)
	}

	// 创建输出器
	var outputter output.Output
	if cfg.Output.Format == "kafka" {
		outputter, err = output.NewOutputWithConfig(*outputPath, cfg.Output.Format, cfg.Output.Compress, cfg.Output.Kafka)
	} else {
		outputter, err = output.NewOutput(*outputPath, cfg.Output.Format, cfg.Output.Compress)
	}
	if err != nil {
		logger.Fatalf("创建输出器失败: %v", err)
	}
	defer outputter.Close()

	// 创建API服务器
	server := api.NewServer(cfg, outputter, logger, *port)

	// 启动服务器
	go func() {
		if err := server.Start(); err != nil {
			logger.Errorf("启动服务器失败: %v", err)
		}
	}()

	logger.Infof("API服务器已启动，监听端口: %d", *port)

	// 等待中断信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	logger.Info("正在关闭服务器...")
	if err := server.Stop(); err != nil {
		logger.Errorf("关闭服务器失败: %v", err)
	}

	logger.Info("服务器已关闭")
}
