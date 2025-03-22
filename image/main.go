package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	var parameters WhSvrParameters

	// get command line parameters
	flag.IntVar(&parameters.port, "port", 443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/cert.pem", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/key.pem", "File containing the x509 private key to --tlsCertFile.")
	flag.StringVar(&parameters.envCfgFile, "envCfgFile", "/etc/webhook/config/envconfig.yaml", "File containing the mutation configuration.")
	flag.Parse()

	envConfig, err := loadConfig(parameters.envCfgFile)
	if err != nil {
		structuredLog(LogLevelError, "Main", "加载配置文件失败: %v", err)
		os.Exit(1)
	}

	whsvr := &WebhookServer{
		envConfig: envConfig,
		server: &http.Server{
			Addr: fmt.Sprintf(":%v", parameters.port),
			TLSConfig: &tls.Config{
				GetCertificate: func(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
					pair, err := tls.LoadX509KeyPair(parameters.certFile, parameters.keyFile)
					if err != nil {
						return nil, fmt.Errorf("failed to load key pair: %w", err)
					}

					return &pair, nil
				},
			},
		},
	}

	// define http server and server handler
	mux := http.NewServeMux()
	mux.HandleFunc("/mutate", whsvr.serve)
	whsvr.server.Handler = mux

	// start webhook server in new rountine
	go func() {
		structuredLog(LogLevelInfo, "Main", "启动 webhook 服务器，监听端口 %v", parameters.port)
		if err := whsvr.server.ListenAndServeTLS("", ""); err != nil {
			structuredLog(LogLevelError, "Main", "启动 env-injector-webhook 服务器失败: %v", err)
			os.Exit(1)
		}
	}()

	// listening OS shutdown signal
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	structuredLog(LogLevelInfo, "Main", "收到系统关闭信号，正在关闭 env-injector-webhook 服务器...")
	whsvr.server.Shutdown(context.Background())
}
