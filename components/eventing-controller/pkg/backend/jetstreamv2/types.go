package jetstreamv2

import (
	cev2 "github.com/cloudevents/sdk-go/v2"
	eventingv1alpha2 "github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha2"
	"github.com/kyma-project/kyma/components/eventing-controller/logger"
	backendmetrics "github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/metrics"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/env"
	"github.com/nats-io/nats.go"
	"sync"
)

const (
	separator = "/"
)

type Backend interface {
	// Initialize should initialize the communication layer with the messaging backend system
	Initialize(connCloseHandler ConnClosedHandler) error

	// SyncSubscription should synchronize the Kyma eventing subscription with the subscriber infrastructure of JetStream.
	SyncSubscription(subscription *eventingv1alpha2.Subscription) error

	// DeleteSubscription should delete the corresponding subscriber data of messaging backend
	DeleteSubscription(subscription *eventingv1alpha2.Subscription) error

	// GetJetStreamSubjects returns a list of subjects appended with stream name as prefix if needed
	GetJetStreamSubjects(subjects []string) []string
}

type JetStream struct {
	Config        env.NatsConfig
	conn          *nats.Conn
	jsCtx         nats.JetStreamContext
	client        cev2.Client
	subscriptions map[SubscriptionSubjectIdentifier]Subscriber
	sinks         sync.Map
	// connClosedHandler gets called by the NATS server when conn is closed and retry attempts are exhausted.
	connClosedHandler ConnClosedHandler
	logger            *logger.Logger
	metricsCollector  *backendmetrics.Collector
}

type ConnClosedHandler func(conn *nats.Conn)

type Subscriber interface {
	SubscriptionSubject() string
	ConsumerInfo() (*nats.ConsumerInfo, error)
	IsValid() bool
	Unsubscribe() error
	SetPendingLimits(int, int) error
	PendingLimits() (int, int, error)
}

type Subscription struct {
	*nats.Subscription
}

// SubscriptionSubjectIdentifier is used to uniquely identify a Subscription subject.
// It should be used only with JetStream backend.
type SubscriptionSubjectIdentifier struct {
	consumerName, namespacedSubjectName string
}

type DefaultSubOpts []nats.SubOpt

//----------------------------------------
// JetStream Backend Test Types
//----------------------------------------

type jetStreamClient struct {
	nats.JetStreamContext
	natsConn *nats.Conn
}
