package trip

import (
	"errors"
	"testing"
	"time"
)

func TestTripFinish(t *testing.T) {
	finishTime := time.Now()

	tests := []struct {
		name       string
		status     Status
		wantStatus Status
		wantErr    error
	}{
		{
			name:       "active trip",
			status:     StatusActive,
			wantStatus: StatusCompleted,
			wantErr:    nil,
		},
		{
			name:       "completed trip",
			status:     StatusCompleted,
			wantStatus: StatusCompleted,
			wantErr:    ErrTripCompleted,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			trip := Trip{Status: tt.status}

			err := trip.Finish(finishTime)

			if !errors.Is(err, tt.wantErr) {
				t.Errorf("Ожидали ошибку %v, получили %v", tt.wantErr, err)
			}

			if trip.Status != tt.wantStatus {
				t.Errorf("Ожидали статус %s, получили %s", tt.wantStatus, trip.Status)
			}

			if tt.wantErr == nil {
				if trip.FinishedAt == nil {
					t.Error("Время завершения не записано")
				} else if !trip.FinishedAt.Equal(finishTime) {
					t.Error("Время завершения записано неправильно")
				}
			} else if trip.FinishedAt != nil {
				t.Error("Время завершения не должно было измениться")
			}
		})
	}
}
