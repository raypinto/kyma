package jetstream

import (
	"context"
	"fmt"
	kymalogger "github.com/kyma-project/kyma/common/logging/logger"
	eventingv1alpha1 "github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha1"
	"github.com/kyma-project/kyma/components/eventing-controller/logger"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/env"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/handlers"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/handlers/eventtype"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/subscriptionmanager"
	controllertesting "github.com/kyma-project/kyma/components/eventing-controller/testing"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"testing"
	"time"
)

type jetStreamSubMgrMock struct {
	Client  dynamic.Interface
	Backend handlers.JetStreamBackend
}

func (c *jetStreamSubMgrMock) Init(_ manager.Manager) error {
	return nil
}

func (c *jetStreamSubMgrMock) Start(_ env.DefaultSubscriptionConfig, _ subscriptionmanager.Params) error {
	return nil
}

func (c *jetStreamSubMgrMock) Stop(_ bool) error {
	return nil
}

func TestCleanup(t *testing.T) {
	jsSubMgr := jetStreamSubMgrMock{}
	data := "sampledata"
	expectedDataInStore := fmt.Sprintf("%q", data)

	// create a test subscriber
	ctx := context.Background()
	subscriber := controllertesting.NewSubscriber()
	// Shutting down subscriber
	defer subscriber.Shutdown()

	// create NATS Server
	natsPort := 4222
	natsServer := controllertesting.RunNatsServerOnPort(
		controllertesting.WithPort(natsPort),
		controllertesting.WithJetStreamEnabled())
	natsURL := natsServer.ClientURL()
	defer controllertesting.ShutDownNATSServer(natsServer)

	defaultLogger, err := logger.New(string(kymalogger.JSON), string(kymalogger.INFO))
	require.NoError(t, err)

	envConf := env.NatsConfig{
		URL:                     natsURL,
		MaxReconnects:           10,
		ReconnectWait:           time.Second,
		EventTypePrefix:         controllertesting.EventTypePrefix,
		JSStreamName:            controllertesting.EventTypePrefix,
		JSStreamStorageType:     handlers.JetStreamStorageTypeMemory,
		JSStreamRetentionPolicy: handlers.JetStreamRetentionPolicyInterest,
	}
	conn, err := nats.Connect(envConf.URL)
	require.NoError(t, err)
	jsClient, err := conn.JetStream()
	require.NoError(t, err)
	subsConfig := env.DefaultSubscriptionConfig{MaxInFlightMessages: 9}
	jsBackend := handlers.NewJetStream(envConf, defaultLogger)
	jsSubMgr.Backend = jsBackend
	err = jsSubMgr.Backend.Initialize(nil)
	require.NoError(t, err)

	// create test subscription
	testSub := controllertesting.NewSubscription("test", "test",
		controllertesting.WithFakeSubscriptionStatus(),
		controllertesting.WithOrderCreatedFilter(),
		controllertesting.WithSinkURL(subscriber.SinkURL),
		controllertesting.WithStatusConfig(subsConfig),
	)

	// create fake Dynamic clients
	fakeClient, err := controllertesting.NewFakeSubscriptionClient(testSub)
	require.NoError(t, err)
	jsSubMgr.Client = fakeClient

	// create test PVC
	pvcName := fmt.Sprintf("%s-%s", jetstreamPVCPrefix, "test")
	pvc := controllertesting.NewPVC(pvcName, jetstreamPVCNamespace)
	unstructuredPVC, err := controllertesting.NewUnstructured(pvc)
	require.NoError(t, err)
	_, err = jsSubMgr.Client.Resource(handlers.PVCGroupVersionResource()).Namespace(jetstreamPVCNamespace).Create(ctx, unstructuredPVC, metav1.CreateOptions{})
	require.NoError(t, err)

	cleaner := func(et string) (string, error) {
		return et, nil
	}
	cleanedSubjects, _ := handlers.GetCleanSubjects(testSub, eventtype.CleanerFunc(cleaner))
	testSub.Status.CleanEventTypes = jsBackend.GetJetStreamSubjects(cleanedSubjects)

	// create Jetstream subscription
	err = jsSubMgr.Backend.SyncSubscription(testSub)
	require.NoError(t, err)

	// make sure subscriber works
	err = subscriber.CheckEvent("")
	if err != nil {
		t.Fatalf("subscriber did not receive the event: %v", err)
	}
	// send an event
	err = handlers.SendEventToJetStream(jsBackend, data)
	if err != nil {
		t.Fatalf("publish event failed: %v", err)
	}

	// check for the event
	err = subscriber.CheckEvent(expectedDataInStore)
	if err != nil {
		t.Fatalf("subscriber did not receive the event: %v", err)
	}

	// check the number of consumers is one
	info, err := jsClient.StreamInfo(envConf.JSStreamName)
	require.NoError(t, err)
	require.Equal(t, info.State.Consumers, 1)

	// when
	err = cleanup(jsSubMgr.Backend, jsSubMgr.Client, defaultLogger.WithContext())

	// then
	require.NoError(t, err)

	// verify that the subscription status is reset
	unstructuredSub, err := jsSubMgr.Client.Resource(handlers.SubscriptionGroupVersionResource()).Namespace("test").Get(ctx, testSub.Name, metav1.GetOptions{})
	require.NoError(t, err)
	gotSub, err := controllertesting.ToSubscription(unstructuredSub)
	require.NoError(t, err)
	wantSubStatus := eventingv1alpha1.SubscriptionStatus{}
	require.Equal(t, wantSubStatus, gotSub.Status)

	// verify that the pvc is deleted
	_, err = jsSubMgr.Client.Resource(handlers.PVCGroupVersionResource()).Namespace(jetstreamPVCNamespace).Get(ctx, pvcName, metav1.GetOptions{})
	require.EqualError(t, err, fmt.Sprintf("persistentvolumeclaims %q not found", pvcName))

	// test JetStream subscriptions/consumers are gone
	info, err = jsClient.StreamInfo(envConf.JSStreamName)
	require.NoError(t, err)
	require.Equal(t, info.State.Consumers, 0)

	// send again an event and check subscriber, check subscriber should fail
	err = handlers.SendEventToJetStream(jsBackend, data)
	if err != nil {
		t.Fatalf("publish event failed: %v", err)
	}
	err = subscriber.CheckEvent(expectedDataInStore)
	require.Error(t, err)
}
