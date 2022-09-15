package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var allSubscriptionConditions = MakeSubscriptionConditions()

// makeBackendConditions creates a map of all conditions which the Backend should have.
func makeBackendConditions() []Condition {
	conditions := []Condition{
		{
			Type:               ConditionPublisherProxyReady,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionTrue,
			Reason:             ConditionReasonPublisherDeploymentReady,
		},
		{
			Type:               ConditionControllerReady,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionTrue,
			Reason:             ConditionReasonSubscriptionControllerReady,
		},
	}
	return conditions
}

// MakeSubscriptionConditions creates a map of all conditions which the Subscription should have.
func MakeSubscriptionConditions() []Condition {
	conditions := []Condition{
		{
			Type:               ConditionAPIRuleStatus,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionUnknown,
		},
		{
			Type:               ConditionSubscribed,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionUnknown,
		},
		{
			Type:               ConditionSubscriptionActive,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionUnknown,
		},
		{
			Type:               ConditionWebhookCallStatus,
			LastTransitionTime: metav1.Now(),
			Status:             corev1.ConditionUnknown,
		},
	}
	return conditions
}

// initializeConditions sets unset conditions to Unknown.
func initializeConditions(initialConditions, currentConditions []Condition) []Condition {
	givenConditions := make(map[ConditionType]Condition)

	// create map of Condition per ConditionType
	for _, condition := range currentConditions {
		givenConditions[condition.Type] = condition
	}

	finalConditions := currentConditions
	// check if every Condition is present in the current Conditions
	for _, expectedCondition := range initialConditions {
		if _, ok := givenConditions[expectedCondition.Type]; !ok {
			// and add it if it is missing
			finalConditions = append(finalConditions, expectedCondition)
		}
	}
	return finalConditions
}

// InitializeConditions sets unset Subscription conditions to Unknown.
func (s *SubscriptionStatus) InitializeConditions() {
	initialConditions := MakeSubscriptionConditions()
	s.Conditions = initializeConditions(initialConditions, s.Conditions)
}

func ContainSameConditionTypes(conditions1, conditions2 []Condition) bool {
	if len(conditions1) != len(conditions2) {
		return false
	}

	for _, condition := range conditions1 {
		if !containConditionType(conditions2, condition.Type) {
			return false
		}
	}

	return true
}

func containConditionType(conditions []Condition, conditionType ConditionType) bool {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return true
		}
	}

	return false
}

// ShouldUpdateReadyStatus checks if there is a mismatch between the
// subscription Ready Status and the Ready status of all the conditions.
func (s SubscriptionStatus) ShouldUpdateReadyStatus() bool {
	if !s.Ready && s.IsReady() || s.Ready && !s.IsReady() {
		return true
	}
	return false
}

func (s SubscriptionStatus) IsReady() bool {
	if !ContainSameConditionTypes(allSubscriptionConditions, s.Conditions) {
		return false
	}

	// the subscription is ready if all its conditions are evaluated to true
	for _, c := range s.Conditions {
		if c.Status != corev1.ConditionTrue {
			return false
		}
	}
	return true
}
