package drivers

import (
	"context"
	"errors"
	"testing"
)

func TestRunStepsSuccess(t *testing.T) {
	ctx := context.Background()
	executed := 0

	steps := []Step{
		{
			Name: "step1",
			Do: func(ctx context.Context) error {
				executed++
				return nil
			},
		},
		{
			Name: "step2",
			Do: func(ctx context.Context) error {
				executed++
				return nil
			},
		},
	}

	if err := RunSteps(ctx, steps); err != nil {
		t.Fatalf("RunSteps failed: %v", err)
	}
	if executed != 2 {
		t.Fatalf("executed = %d, want 2", executed)
	}
}

func TestRunStepsRollback(t *testing.T) {
	ctx := context.Background()
	step1Undone := false

	steps := []Step{
		{
			Name: "step1",
			Do: func(ctx context.Context) error {
				return nil
			},
			Undo: func(ctx context.Context) error {
				step1Undone = true
				return nil
			},
		},
		{
			Name: "step2",
			Do: func(ctx context.Context) error {
				return errors.New("failed step 2")
			},
		},
	}

	err := RunSteps(ctx, steps)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if !step1Undone {
		t.Fatal("step1 Undo was not called on step2 failure")
	}
}
