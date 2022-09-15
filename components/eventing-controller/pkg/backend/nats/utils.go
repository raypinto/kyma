package nats

import (
	"context"
	"encoding/json"
	"fmt"
	eventingv1alpha2 "github.com/kyma-project/kyma/components/eventing-controller/api/v1alpha2"
	"strconv"
	"strings"

	cev2event "github.com/cloudevents/sdk-go/v2/event"
	"github.com/nats-io/nats.go"
	"k8s.io/apimachinery/pkg/types"

	"github.com/kyma-project/kyma/components/eventing-controller/logger"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/application/applicationtest"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/application/fake"
	"github.com/kyma-project/kyma/components/eventing-controller/pkg/backend/eventtype"
)

func getUniqueEventTypes(eventTypes []string) []string {
	unique := make([]string, 0, len(eventTypes))
	mapper := make(map[string]bool)

	for _, val := range eventTypes {
		if _, ok := mapper[val]; !ok {
			mapper[val] = true
			unique = append(unique, val)
		}
	}

	return unique
}

func GetCleanedTypes(subscriptionStatus eventingv1alpha2.SubscriptionStatus) []string {
	var cleantypes []string
	for _, eventtypes := range subscriptionStatus.Types {
		cleantypes = append(cleantypes, eventtypes.CleanType)
	}
	return cleantypes
}

// GetCleanSubjects returns a list of clean eventTypes from the unique types in the subscription.
func GetCleanSubjects(sub *eventingv1alpha2.Subscription, cleaner eventtype.Cleaner) ([]eventingv1alpha2.EventType, error) {
	// TODO: Put this in the validation webhook
	//if sub.Spec.Types == nil || sub.Spec.Source == "" {
	//	return []eventingv1alpha2.EventType{}, errors.New("Source and Types must be provided")
	//}

	uniqueTypes := getUniqueEventTypes(sub.Spec.Types)
	var cleanEventTypes []eventingv1alpha2.EventType
	for _, eventType := range uniqueTypes {
		cleanType, err := GetCleanSubject(eventType, cleaner)
		if err != nil {
			return []eventingv1alpha2.EventType{}, err
		}
		newEventType := eventingv1alpha2.EventType{
			OriginalType: eventType,
			CleanType:    cleanType,
		}
		cleanEventTypes = append(cleanEventTypes, newEventType)
	}
	return cleanEventTypes, nil
}

func createKeySuffix(subject string, queueGoupInstanceNo int) string {
	return subject + string(types.Separator) + strconv.Itoa(queueGoupInstanceNo)
}

func CreateKey(sub *eventingv1alpha2.Subscription, subject string, queueGoupInstanceNo int) string {
	return fmt.Sprintf("%s.%s", CreateKeyPrefix(sub), createKeySuffix(subject, queueGoupInstanceNo))
}

func GetCleanSubject(eventType string, cleaner eventtype.Cleaner) (string, error) {
	if len(eventType) == 0 {
		return "", nats.ErrBadSubject
	}
	// clean the application name segment in the event-type from none-alphanumeric characters
	// return it as a NATS subject
	return cleaner.CleanJetStreamEvents(eventType)
}

func CreateKymaSubscriptionNamespacedName(key string, sub Subscriber) types.NamespacedName {
	nsn := types.NamespacedName{}
	nnvalues := strings.Split(key, string(types.Separator))
	nsn.Namespace = nnvalues[0]
	nsn.Name = strings.TrimSuffix(strings.TrimSuffix(nnvalues[1], sub.SubscriptionSubject()), ".")
	return nsn
}

// IsNatsSubAssociatedWithKymaSub checks if the NATS subscription is associated / related to Kyma subscription or not.
func IsNatsSubAssociatedWithKymaSub(natsSubKey string, natsSub Subscriber, sub *eventingv1alpha2.Subscription) bool {
	return CreateKeyPrefix(sub) == CreateKymaSubscriptionNamespacedName(natsSubKey, natsSub).String()
}

func ConvertMsgToCE(msg *nats.Msg) (*cev2event.Event, error) {
	event := cev2event.New(cev2event.CloudEventsVersionV1)
	err := json.Unmarshal(msg.Data, &event)
	if err != nil {
		return nil, err
	}
	if err := event.Validate(); err != nil {
		return nil, err
	}
	return &event, nil
}

func CreateKeyPrefix(sub *eventingv1alpha2.Subscription) string {
	namespacedName := types.NamespacedName{
		Namespace: sub.Namespace,
		Name:      sub.Name,
	}
	return namespacedName.String()
}

func CreateEventTypeCleaner(eventTypePrefix, applicationName string, logger *logger.Logger) eventtype.Cleaner { //nolint:unparam
	application := applicationtest.NewApplication(applicationName, nil)
	applicationLister := fake.NewApplicationListerOrDie(context.Background(), application)
	return eventtype.NewCleaner(eventTypePrefix, applicationLister, logger)
}
