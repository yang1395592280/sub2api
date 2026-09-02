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

func TestWireGenDoesNotInjectRemovedOpenAIAutoScheduler(t *testing.T) {
	body, err := os.ReadFile("wire_gen.go")
	require.NoError(t, err)
	source := string(body)
	require.NotContains(t, source, "ProvideOpenAIAutoSchedulerSelector")
	require.NotContains(t, source, "ProvideOpenAIBalancedScheduler")
	require.NotContains(t, source, "OpenAIAutoSchedulerProbeRunner")
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
	codexVersionSyncSvc := service.NewOpenAICodexVersionSyncService(nil, nil, nil, time.Second)
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
		nil, // opsService
		nil, // opsIngressRejectAggregator
		nil, // apiKeyService
		nil, // authCacheInvalidationWorker
		schedulerSnapshotSvc,
		tokenRefreshSvc,
		accountExpirySvc,
		nil, // cnProviderBalanceCheck
		codexVersionSyncSvc,
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
		nil, // scheduledTestRunner
		nil, // backupSvc
		nil, // paymentOrderExpiry
		nil, // channelMonitorRunner
		nil, // channelMonitorV2Aggregator
		nil, // quotaFlusher
		nil, // upstreamBillingProbe
		nil, // ollamaCloudUsage
		nil, // auditLog
		nil, // openAIAutoReset
		nil, // promptAudit
		nil, // pluginManager
	)

	require.NotPanics(t, func() {
		cleanup()
	})
}
