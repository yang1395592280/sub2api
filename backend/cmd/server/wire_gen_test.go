package main

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestProvideServiceBuildInfo(t *testing.T) {
	in := handler.BuildInfo{
		Version:   "v-test",
		BuildType: "release",
	}
	out := provideServiceBuildInfo(in)
	require.Equal(t, in.Version, out.Version)
	require.Equal(t, in.BuildType, out.BuildType)
}

func TestWireGenInjectsOpenAIAutoSchedulerIntoGateway(t *testing.T) {
	body, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)

	source := string(body)
	selectorIndex := strings.Index(source, "openAIAutoSchedulerSelector := service.ProvideOpenAIAutoSchedulerSelector(openAIAutoSchedulerService)")
	gatewayIndex := strings.Index(source, "openAIGatewayService := service.ProvideOpenAIGatewayService(")
	handlerIndex := strings.Index(source, "handler.NewOpenAIGatewayHandler")
	require.NotEqual(t, -1, selectorIndex, "OpenAI auto scheduler selector must be constructed by production wire")
	require.NotEqual(t, -1, gatewayIndex, "OpenAI gateway must be constructed through the provider that wires scheduler dependencies")
	require.NotEqual(t, -1, handlerIndex, "OpenAI gateway handler construction must remain visible in production wire")
	require.Contains(t, source, "openAIAutoSchedulerSelector, openAIAutoSchedulerService")
	require.Less(t, selectorIndex, gatewayIndex, "OpenAI auto scheduler selector must exist before gateway construction")
	require.Less(t,
		gatewayIndex,
		handlerIndex,
		"OpenAI gateway must receive auto scheduler dependencies before handlers use it",
	)

	providerBody, err := os.ReadFile("../../internal/service/wire.go")
	require.NoError(t, err)
	providerSource := string(providerBody)
	require.Contains(t, providerSource, "return NewOpenAIAutoSchedulerSelector(svc)")
	require.Contains(t, providerSource, "svc.SetOpenAIAutoScheduler(openAIAutoSchedulerSelector, openAIAutoSchedulerService)")
}

func TestProvideCleanup_WithMinimalDependencies_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	oauthSvc := service.NewOAuthService(nil, nil)
	openAIOAuthSvc := service.NewOpenAIOAuthService(nil, nil)
	geminiOAuthSvc := service.NewGeminiOAuthService(nil, nil, nil, nil, cfg)
	antigravityOAuthSvc := service.NewAntigravityOAuthService(nil)

	tokenRefreshSvc := service.NewTokenRefreshService(
		nil,
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil,
		nil,
		cfg,
		nil,
	)
	accountExpirySvc := service.NewAccountExpiryService(nil, time.Second)
	proxyExpirySvc := service.NewProxyExpiryService(nil, time.Second)
	subscriptionExpirySvc := service.NewSubscriptionExpiryService(nil, time.Second)
	pricingSvc := service.NewPricingService(cfg, nil)
	emailQueueSvc := service.NewEmailQueueService(nil, 1)
	billingCacheSvc := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
	idempotencyCleanupSvc := service.NewIdempotencyCleanupService(nil, cfg)
	schedulerSnapshotSvc := service.NewSchedulerSnapshotService(nil, nil, nil, nil, cfg)
	opsSystemLogSinkSvc := service.NewOpsSystemLogSink(nil)

	cleanup := provideCleanup(
		nil, // entClient
		nil, // redis
		&service.OpsMetricsCollector{},
		&service.OpsAggregationService{},
		&service.OpsAlertEvaluatorService{},
		&service.OpsCleanupService{},
		&service.OpsScheduledReportService{},
		opsSystemLogSinkSvc,
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		proxyExpirySvc,
		subscriptionExpirySvc,
		&service.UsageCleanupService{},
		idempotencyCleanupSvc,
		pricingSvc,
		emailQueueSvc,
		billingCacheSvc,
		&service.UsageRecordWorkerPool{},
		&service.SubscriptionService{},
		oauthSvc,
		openAIOAuthSvc,
		geminiOAuthSvc,
		antigravityOAuthSvc,
		nil, // grokOAuth
		nil, // sub2apiCheckin
		nil, // openAIGateway
		nil, // openAIAutoSchedulerProbeRunner
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // channelMonitorRunner
		nil, // quotaFlusher
	)

	require.NotPanics(t, func() {
		cleanup()
	})
}
