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
	recorderIndex := strings.Index(source, "openAIAutoSchedulerOutcomeRecorder := service.ProvideOpenAIAutoSchedulerOutcomeRecorder(openAIAutoSchedulerService)")
	gatewayIndex := strings.Index(source, "openAIGatewayService := service.ProvideOpenAIGatewayService(")
	handlerIndex := strings.Index(source, "handler.NewOpenAIGatewayHandler")
	require.NotEqual(t, -1, selectorIndex, "OpenAI auto scheduler selector must be constructed by production wire")
	require.NotEqual(t, -1, recorderIndex, "OpenAI auto scheduler outcome recorder must be constructed by production wire")
	require.NotEqual(t, -1, gatewayIndex, "OpenAI gateway must be constructed through the provider that wires scheduler dependencies")
	require.NotEqual(t, -1, handlerIndex, "OpenAI gateway handler construction must remain visible in production wire")
	require.Contains(t, source, "openAIAutoSchedulerSelector, openAIAutoSchedulerService, openAIAutoSchedulerOutcomeRecorder")
	require.Less(t, selectorIndex, gatewayIndex, "OpenAI auto scheduler selector must exist before gateway construction")
	require.Less(t, recorderIndex, gatewayIndex, "OpenAI outcome recorder must exist before gateway construction")
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
	require.Contains(t, providerSource, "svc.SetOpenAIAutoSchedulerOutcomeRecorder(openAIAutoSchedulerOutcomeRecorder)")
}

func TestWireGenInjectsOpenAIBalancedSchedulerIntoGateway(t *testing.T) {
	body, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)

	source := string(body)
	balancedIndex := strings.Index(source, "openAIBalancedScheduler := service.ProvideOpenAIBalancedScheduler(openAISchedulerHealthRepository)")
	gatewayIndex := strings.Index(source, "openAIGatewayService := service.ProvideOpenAIGatewayService(")
	require.NotEqual(t, -1, balancedIndex)
	require.NotEqual(t, -1, gatewayIndex)
	require.Contains(t, source, "openAIAutoSchedulerOutcomeRecorder, openAIBalancedScheduler, apiKeyService")
	require.Less(t, balancedIndex, gatewayIndex)

	providerBody, err := os.ReadFile("../../internal/service/wire.go")
	require.NoError(t, err)
	providerSource := string(providerBody)
	require.Contains(t, providerSource, "return NewOpenAIBalancedScheduler(repo)")
	require.Contains(t, providerSource, "svc.SetOpenAIBalancedScheduler(openAIBalancedScheduler)")
}

func TestWireGenInjectsOpenAIAutoSchedulerIntoAccountHandler(t *testing.T) {
	body, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)

	source := string(body)
	accountHandlerIndex := strings.Index(source, "accountHandler := handler.ProvideAdminAccountHandler(")
	schedulerHandlerIndex := strings.Index(source, "openAIAutoSchedulerHandler := admin.ProvideOpenAIAutoSchedulerHandler(")
	require.NotEqual(t, -1, accountHandlerIndex, "admin account handler provider must remain visible in production wire")
	require.NotEqual(t, -1, schedulerHandlerIndex, "OpenAI auto scheduler handler construction must remain visible in production wire")
	require.Contains(t, source, "sub2APICheckinService, openAIAutoSchedulerService)")
	require.Less(t, accountHandlerIndex, schedulerHandlerIndex, "account handler should be wired before admin handlers are assembled")

	providerBody, err := os.ReadFile("../../internal/handler/wire.go")
	require.NoError(t, err)
	providerSource := string(providerBody)
	require.Contains(t, providerSource, "h.SetOpenAIAutoSchedulerAccountSummaryService(openAIAutoSchedulerService)")
}

func TestWireGenInjectsGroupUpstreamBalanceRefreshRunnerIntoStartupAndCleanup(t *testing.T) {
	body, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)

	source := string(body)
	runnerIndex := strings.Index(source, "groupUpstreamBalanceRefreshRunner := service.ProvideGroupUpstreamBalanceRefreshRunner(groupRepository, accountRepository, openAIUpstreamBalanceService, leaderLockCache, db, settingRepository)")
	cleanupCallIndex := strings.Index(source, "provideCleanup(client, redisClient")
	cleanupStepIndex := strings.Index(source, "{\"GroupUpstreamBalanceRefreshRunner\", func() error {")
	require.NotEqual(t, -1, runnerIndex, "production wire must construct group upstream balance refresh runner")
	require.NotEqual(t, -1, cleanupCallIndex, "production wire must continue to build cleanup")
	require.NotEqual(t, -1, cleanupStepIndex, "cleanup must stop the group upstream balance refresh runner")
	require.Contains(t, source, "sub2APICheckinService, groupUpstreamBalanceRefreshRunner, openAIGatewayService")
	require.Less(t, runnerIndex, cleanupCallIndex, "runner must be constructed before cleanup wiring")
	require.Less(t, cleanupCallIndex, cleanupStepIndex, "cleanup wiring should remain before cleanup steps")
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
	groupUpstreamBalanceRefreshRunner := service.NewGroupUpstreamBalanceRefreshRunner(nil, nil, nil)

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
		&service.BatchImageCleanupService{},
		nil, // batchImageWorker
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
		groupUpstreamBalanceRefreshRunner,
		nil, // openAIGateway
		nil, // openAIAutoSchedulerProbeRunner
		nil, // openAIAutoSchedulerOutcomeRecorder
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
