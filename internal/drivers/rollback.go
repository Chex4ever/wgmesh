package drivers

import (
	"context"
	"fmt"
)

// Step — единица выполнения задачи с возможностью отката (Undo).
type Step struct {
	Name string
	Do   func(ctx context.Context) error
	Undo func(ctx context.Context) error
}

// ApplyError — ошибка выполнения шагов с информацией об успехе/неудачи отката.
type ApplyError struct {
	FailedStep  string
	Err         error
	RollbackErr error
}

func (e *ApplyError) Error() string {
	if e.RollbackErr != nil {
		return fmt.Sprintf("ошибка на шаге %q: %v; ошибка при откате: %v", e.FailedStep, e.Err, e.RollbackErr)
	}
	return fmt.Sprintf("ошибка на шаге %q: %v (откат выполнен успешно)", e.FailedStep, e.Err)
}

// RunSteps поочерёдно выполняет шаги. В случае ошибки производит откат (Undo) в обратном порядке.
func RunSteps(ctx context.Context, steps []Step) error {
	var completed []Step
	for _, step := range steps {
		if err := step.Do(ctx); err != nil {
			var rbErr error
			for i := len(completed) - 1; i >= 0; i-- {
				if completed[i].Undo != nil {
					if uErr := completed[i].Undo(ctx); uErr != nil && rbErr == nil {
						rbErr = uErr
					}
				}
			}
			return &ApplyError{
				FailedStep:  step.Name,
				Err:         err,
				RollbackErr: rbErr,
			}
		}
		completed = append(completed, step)
	}
	return nil
}
