package composed

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ObjWithConditions interface {
	client.Object
	Conditions() *[]metav1.Condition
	GetObjectMeta() *metav1.ObjectMeta
}

func UpdateStatus(obj ObjWithConditions) *UpdateStatusBuilder {
	return &UpdateStatusBuilder{
		obj:              obj,
		conditionsToKeep: map[string]struct{}{},
	}
}

type UpdateStatusBuilder struct {
	obj                ObjWithConditions
	doRemoveConditions bool
	conditionsToKeep   map[string]struct{}
	conditionsToSet    []metav1.Condition
	updateErrorLogMsg  string
	failedError        error
	successError       error
	updateErrorWrapper func(err error) error
	onUpdateError      func(ctx context.Context, err error) (error, context.Context)
	onUpdateSuccess    func(ctx context.Context) (error, context.Context)
}

func (b *UpdateStatusBuilder) RemoveAllConditionsExcept(conditionTypes ...string) *UpdateStatusBuilder {
	for _, c := range conditionTypes {
		b.conditionsToKeep[c] = struct{}{}
	}
	return b
}

func (b *UpdateStatusBuilder) KeepAllConditions() *UpdateStatusBuilder {
	b.doRemoveConditions = false
	return b
}

func (b *UpdateStatusBuilder) SetCondition(cond metav1.Condition) *UpdateStatusBuilder {
	b.conditionsToSet = append(b.conditionsToSet, cond)
	return b
}

func (b *UpdateStatusBuilder) ErrorLogMessage(msg string) *UpdateStatusBuilder {
	b.updateErrorLogMsg = msg
	return b
}

func (b *UpdateStatusBuilder) UpdateErrorWrapper(f func(err error) error) *UpdateStatusBuilder {
	b.updateErrorWrapper = f
	return b
}

func (b *UpdateStatusBuilder) OnUpdateError(f func(ctx context.Context, err error) (error, context.Context)) *UpdateStatusBuilder {
	b.onUpdateError = f
	return b
}

func (b *UpdateStatusBuilder) OnUpdateSuccess(f func(ctx context.Context) (error, context.Context)) *UpdateStatusBuilder {
	b.onUpdateSuccess = f
	return b
}

func (b *UpdateStatusBuilder) FailedError(err error) *UpdateStatusBuilder {
	b.failedError = err
	return b
}

func (b *UpdateStatusBuilder) SuccessError(err error) *UpdateStatusBuilder {
	b.successError = err
	return b
}

func (b *UpdateStatusBuilder) Run(ctx context.Context, state State) (error, context.Context) {
	b.setDefaults()

	if b.doRemoveConditions {
		var conditionsToRemove []string
		for _, c := range *b.obj.Conditions() {
			_, keep := b.conditionsToKeep[c.Type]
			if !keep {
				conditionsToRemove = append(conditionsToRemove, c.Type)
			}
		}
		for _, c := range conditionsToRemove {
			_ = meta.RemoveStatusCondition(b.obj.Conditions(), c)
		}
	}

	for _, c := range b.conditionsToSet {
		_ = meta.SetStatusCondition(b.obj.Conditions(), c)
	}

	err := state.UpdateObjStatus(ctx)
	if err != nil {
		err = b.updateErrorWrapper(err)
		return b.onUpdateError(ctx, err)
	}

	return b.onUpdateSuccess(ctx)
}

func (b *UpdateStatusBuilder) setDefaults() {
	b.updateErrorLogMsg = fmt.Sprintf("Error updating status for %T", b.obj)

	b.doRemoveConditions = true

	if b.updateErrorWrapper == nil {
		b.updateErrorWrapper = func(err error) error {
			return err
		}
	}

	if b.successError == nil {
		b.successError = StopAndForget
	}
	if b.failedError == nil {
		b.failedError = StopWithRequeue
	}

	if b.onUpdateError == nil {
		b.onUpdateError = func(ctx context.Context, err error) (error, context.Context) {
			return LogErrorAndReturn(err, b.updateErrorLogMsg, b.failedError, nil)
		}
	}

	if b.onUpdateSuccess == nil {
		b.onUpdateSuccess = func(ctx context.Context) (error, context.Context) {
			return b.successError, nil
		}
	}
}
